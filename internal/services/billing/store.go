package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

var ErrBillingUserNotFound = errors.New("billing user not found")

type StoreInterface interface {
	GetStripeCustomerID(ctx context.Context, userID uuid.UUID) (*string, error)
	EnsureStripeCustomerID(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error)
	SetStripeIDs(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string, conn services.DBConn) error
	WithWebhookEvent(ctx context.Context, meta WebhookEventMeta, fn func(ctx context.Context, tx services.Tx) error) (alreadyProcessed bool, err error)
	FindUserIDByStripeCustomerID(ctx context.Context, customerID string, conn services.DBConn) (uuid.UUID, error)
	FindUserIDByStripeSubscriptionID(ctx context.Context, subscriptionID string, conn services.DBConn) (uuid.UUID, error)
	GrantLifetime(ctx context.Context, userID uuid.UUID, customerID string, conn services.DBConn) error
	SetSubscriptionState(ctx context.Context, userID uuid.UUID, customerID, subscriptionID, status string, currentPeriodEnd time.Time, cancelAtPeriodEnd bool, conn services.DBConn) error
	ResetToFree(ctx context.Context, userID uuid.UUID, conn services.DBConn) error
	RedeemPremiumCode(ctx context.Context, userID uuid.UUID, codeHashHex string, now time.Time) error
}

type Store struct {
	db services.DB
}

func NewStore(db services.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetStripeCustomerID(ctx context.Context, userID uuid.UUID) (*string, error) {
	var customerID *string
	if err := s.db.QueryRow(ctx,
		`SELECT stripe_customer_id
		 FROM users
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&customerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBillingUserNotFound
		}
		return nil, fmt.Errorf("get stripe customer id: %w", err)
	}
	return customerID, nil
}

func (s *Store) EnsureStripeCustomerID(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error) {
	existing, err := s.GetStripeCustomerID(ctx, userID)
	if err != nil {
		return "", err
	}
	if existing != nil && *existing != "" {
		return *existing, nil
	}

	newID, err := createFn(ctx)
	if err != nil {
		return "", err
	}

	tag, err := s.db.Exec(ctx,
		`UPDATE users
		 SET stripe_customer_id = $2, billing_updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL AND stripe_customer_id IS NULL`,
		userID, newID,
	)
	if err != nil {
		return "", fmt.Errorf("set stripe customer id: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return newID, nil
	}

	// Concurrent creation: re-read.
	got, err := s.GetStripeCustomerID(ctx, userID)
	if err != nil {
		return "", err
	}
	if got == nil || *got == "" {
		return "", fmt.Errorf("stripe customer id not set after create")
	}
	return *got, nil
}

func (s *Store) SetStripeIDs(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string, conn services.DBConn) error {
	_, err := conn.Exec(ctx,
		`UPDATE users
		 SET stripe_customer_id = COALESCE(stripe_customer_id, NULLIF($2, '')),
		     stripe_subscription_id = COALESCE(stripe_subscription_id, NULLIF($3, '')),
		     billing_updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID, customerID, subscriptionID,
	)
	if err != nil {
		return fmt.Errorf("set stripe ids: %w", err)
	}
	return nil
}

type WebhookEventMeta struct {
	StripeEventID string
	EventType     string
	Livemode      bool
	CreatedAt     time.Time
}

func (s *Store) WithWebhookEvent(ctx context.Context, meta WebhookEventMeta, fn func(ctx context.Context, tx services.Tx) error) (alreadyProcessed bool, err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin webhook tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // best effort

	// Insert if new; if existing, still lock and check processed_at.
	_, _ = tx.Exec(ctx,
		`INSERT INTO stripe_webhook_events (stripe_event_id, event_type, livemode, created_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (stripe_event_id) DO NOTHING`,
		meta.StripeEventID, meta.EventType, meta.Livemode, meta.CreatedAt,
	)

	var processedAt *time.Time
	if err := tx.QueryRow(ctx,
		`SELECT processed_at
		 FROM stripe_webhook_events
		 WHERE stripe_event_id = $1
		 FOR UPDATE`,
		meta.StripeEventID,
	).Scan(&processedAt); err != nil {
		return false, fmt.Errorf("lock webhook event row: %w", err)
	}

	if processedAt != nil {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit webhook no-op: %w", err)
		}
		return true, nil
	}

	if err := fn(ctx, tx); err != nil {
		_, _ = tx.Exec(ctx,
			`UPDATE stripe_webhook_events
			 SET processing_error = $2
			 WHERE stripe_event_id = $1`,
			meta.StripeEventID, err.Error(),
		)
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return false, fmt.Errorf("commit webhook error: %w", commitErr)
		}
		return false, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE stripe_webhook_events
		 SET processed_at = NOW(), processing_error = NULL
		 WHERE stripe_event_id = $1`,
		meta.StripeEventID,
	); err != nil {
		return false, fmt.Errorf("mark webhook processed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit webhook processed: %w", err)
	}
	return false, nil
}

func (s *Store) FindUserIDByStripeCustomerID(ctx context.Context, customerID string, tx services.DBConn) (uuid.UUID, error) {
	var userID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT id
		 FROM users
		 WHERE stripe_customer_id = $1 AND deleted_at IS NULL`,
		customerID,
	).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrBillingUserNotFound
		}
		return uuid.Nil, fmt.Errorf("find user by customer id: %w", err)
	}
	return userID, nil
}

func (s *Store) FindUserIDByStripeSubscriptionID(ctx context.Context, subscriptionID string, tx services.DBConn) (uuid.UUID, error) {
	var userID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT id
		 FROM users
		 WHERE stripe_subscription_id = $1 AND deleted_at IS NULL`,
		subscriptionID,
	).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrBillingUserNotFound
		}
		return uuid.Nil, fmt.Errorf("find user by subscription id: %w", err)
	}
	return userID, nil
}

func (s *Store) GrantLifetime(ctx context.Context, userID uuid.UUID, customerID string, tx services.DBConn) error {
	_, err := tx.Exec(ctx,
		`UPDATE users
		 SET stripe_customer_id = COALESCE(stripe_customer_id, NULLIF($2, '')),
		     billing_plan = 'premium',
		     billing_source = 'stripe_lifetime',
		     billing_status = 'active',
		     billing_current_period_end = NULL,
		     billing_cancel_at_period_end = false,
		     billing_updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID, customerID,
	)
	if err != nil {
		return fmt.Errorf("grant lifetime: %w", err)
	}
	return nil
}

func (s *Store) SetSubscriptionState(ctx context.Context, userID uuid.UUID, customerID, subscriptionID, status string, currentPeriodEnd time.Time, cancelAtPeriodEnd bool, tx services.DBConn) error {
	_, err := tx.Exec(ctx,
		`UPDATE users
		 SET stripe_customer_id = COALESCE(stripe_customer_id, NULLIF($2, '')),
		     stripe_subscription_id = NULLIF($3, ''),
		     billing_plan = 'premium',
		     billing_source = 'stripe_subscription',
		     billing_status = $4,
		     billing_current_period_end = $5,
		     billing_cancel_at_period_end = $6,
		     billing_updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID, customerID, subscriptionID, status, currentPeriodEnd, cancelAtPeriodEnd,
	)
	if err != nil {
		return fmt.Errorf("set subscription state: %w", err)
	}
	return nil
}

func (s *Store) ResetToFree(ctx context.Context, userID uuid.UUID, tx services.DBConn) error {
	_, err := tx.Exec(ctx,
		`UPDATE users
		 SET stripe_subscription_id = NULL,
		     billing_plan = 'free',
		     billing_source = 'none',
		     billing_status = 'inactive',
		     billing_current_period_end = NULL,
		     billing_cancel_at_period_end = false,
		     billing_updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("reset billing: %w", err)
	}
	return nil
}

func (s *Store) RedeemPremiumCode(ctx context.Context, userID uuid.UUID, codeHashHex string, now time.Time) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin redeem tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // best effort

	var codeID uuid.UUID
	var durationDays *int
	var expiresAt *time.Time
	var redeemedAt *time.Time
	err = tx.QueryRow(ctx,
		`SELECT id, duration_days, expires_at, redeemed_at
		 FROM premium_codes
		 WHERE code_hash = $1
		 FOR UPDATE`,
		codeHashHex,
	).Scan(&codeID, &durationDays, &expiresAt, &redeemedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidCode
		}
		return fmt.Errorf("load premium code: %w", err)
	}

	if redeemedAt != nil {
		return ErrInvalidCode
	}
	if expiresAt != nil && !expiresAt.After(now) {
		return ErrInvalidCode
	}

	if _, err := tx.Exec(ctx,
		`UPDATE premium_codes
		 SET redeemed_by_user_id = $2, redeemed_at = NOW()
		 WHERE id = $1`,
		codeID, userID,
	); err != nil {
		return fmt.Errorf("mark code redeemed: %w", err)
	}

	var periodEnd *time.Time
	if durationDays != nil {
		pe := now.Add(time.Duration(*durationDays) * 24 * time.Hour)
		periodEnd = &pe
	}

	if _, err := tx.Exec(ctx,
		`UPDATE users
		 SET billing_plan = 'premium',
		     billing_source = 'code',
		     billing_status = 'active',
		     billing_current_period_end = $2,
		     billing_cancel_at_period_end = false,
		     billing_updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID, periodEnd,
	); err != nil {
		return fmt.Errorf("apply premium code entitlement: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit redeem tx: %w", err)
	}
	return nil
}
