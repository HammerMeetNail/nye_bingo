package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type Service struct {
	enabled       bool
	baseURL       string
	webhookSecret string
	priceMonthly  string
	priceYearly   string
	priceLifetime string
	priceTip5     string
	priceTip10    string
	priceTip20    string

	store  StoreInterface
	stripe StripeClient
}

func NewService(cfg config.BillingConfig, baseURL string, store StoreInterface, stripe StripeClient) *Service {
	return &Service{
		enabled:       cfg.Enabled,
		baseURL:       strings.TrimRight(baseURL, "/"),
		webhookSecret: cfg.StripeWebhookSecret,
		priceMonthly:  cfg.StripePremiumMonthlyPriceID,
		priceYearly:   cfg.StripePremiumYearlyPriceID,
		priceLifetime: cfg.StripePremiumLifetimePriceID,
		priceTip5:     cfg.StripeTip5PriceID,
		priceTip10:    cfg.StripeTip10PriceID,
		priceTip20:    cfg.StripeTip20PriceID,
		store:         store,
		stripe:        stripe,
	}
}

func (s *Service) Status(user *models.User, now time.Time) BillingStatus {
	plan := user.BillingPlan
	if plan == "" {
		plan = "free"
	}
	status := user.BillingStatus
	if status == "" {
		status = "inactive"
	}
	return BillingStatus{
		BillingEnabled:    s.enabled,
		IsPremium:         IsPremium(user, now),
		Plan:              plan,
		Status:            status,
		CurrentPeriodEnd:  user.BillingCurrentPeriodEnd,
		CancelAtPeriodEnd: user.BillingCancelAtPeriodEnd,
	}
}

func (s *Service) CreateSubscriptionCheckoutURL(ctx context.Context, user *models.User, interval CheckoutInterval) (string, error) {
	if !s.enabled {
		return "", ErrBillingDisabled
	}
	priceID := ""
	switch interval {
	case IntervalMonth:
		priceID = s.priceMonthly
	case IntervalYear:
		priceID = s.priceYearly
	default:
		return "", fmt.Errorf("invalid interval")
	}
	if strings.TrimSpace(priceID) == "" {
		return "", fmt.Errorf("missing stripe price id for interval %q", interval)
	}

	customerID, err := s.store.EnsureStripeCustomerID(ctx, user.ID, func(ctx context.Context) (string, error) {
		return s.stripe.CreateCustomer(ctx, user.Email, user.ID.String())
	})
	if err != nil {
		return "", err
	}

	params := CheckoutSessionParams{
		Mode:       CheckoutSessionModeSubscription,
		CustomerID: customerID,
		PriceID:    priceID,
		SuccessURL: s.successURL(),
		CancelURL:  s.cancelURL(),
		Metadata: map[string]string{
			"user_id":  user.ID.String(),
			"purchase": "subscription",
			"interval": string(interval),
		},
	}
	return s.stripe.CreateCheckoutSession(ctx, params)
}

func (s *Service) CreateLifetimeCheckoutURL(ctx context.Context, user *models.User) (string, error) {
	if !s.enabled {
		return "", ErrBillingDisabled
	}
	if strings.TrimSpace(s.priceLifetime) == "" {
		return "", fmt.Errorf("missing stripe lifetime price id")
	}
	customerID, err := s.store.EnsureStripeCustomerID(ctx, user.ID, func(ctx context.Context) (string, error) {
		return s.stripe.CreateCustomer(ctx, user.Email, user.ID.String())
	})
	if err != nil {
		return "", err
	}
	params := CheckoutSessionParams{
		Mode:       CheckoutSessionModePayment,
		CustomerID: customerID,
		PriceID:    s.priceLifetime,
		SuccessURL: s.successURL(),
		CancelURL:  s.cancelURL(),
		Metadata: map[string]string{
			"user_id":  user.ID.String(),
			"purchase": "lifetime",
		},
	}
	return s.stripe.CreateCheckoutSession(ctx, params)
}

func (s *Service) CreateTipCheckoutURL(ctx context.Context, user *models.User, amount CheckoutTipAmount) (string, error) {
	if !s.enabled {
		return "", ErrBillingDisabled
	}
	priceID := ""
	switch amount {
	case Tip5:
		priceID = s.priceTip5
	case Tip10:
		priceID = s.priceTip10
	case Tip20:
		priceID = s.priceTip20
	default:
		return "", fmt.Errorf("invalid tip amount")
	}
	if strings.TrimSpace(priceID) == "" {
		return "", fmt.Errorf("missing stripe tip price id for amount %d", amount)
	}
	customerID, err := s.store.EnsureStripeCustomerID(ctx, user.ID, func(ctx context.Context) (string, error) {
		return s.stripe.CreateCustomer(ctx, user.Email, user.ID.String())
	})
	if err != nil {
		return "", err
	}
	params := CheckoutSessionParams{
		Mode:       CheckoutSessionModePayment,
		CustomerID: customerID,
		PriceID:    priceID,
		SuccessURL: s.successURL(),
		CancelURL:  s.cancelURL(),
		Metadata: map[string]string{
			"user_id":  user.ID.String(),
			"purchase": "tip",
			"amount":   fmt.Sprintf("%d", amount),
		},
	}
	return s.stripe.CreateCheckoutSession(ctx, params)
}

func (s *Service) CreatePortalURL(ctx context.Context, user *models.User) (string, error) {
	if !s.enabled {
		return "", ErrBillingDisabled
	}
	customerID, err := s.store.EnsureStripeCustomerID(ctx, user.ID, func(ctx context.Context) (string, error) {
		return s.stripe.CreateCustomer(ctx, user.Email, user.ID.String())
	})
	if err != nil {
		return "", err
	}
	return s.stripe.CreatePortalSession(ctx, customerID, s.returnURL())
}

var premiumCodeRegex = regexp.MustCompile(`^YOBP[A-Z2-7]{24}$`)

func (s *Service) RedeemCode(ctx context.Context, user *models.User, code string, now time.Time) error {
	if !s.enabled {
		return ErrBillingDisabled
	}
	normalized := normalizePremiumCode(code)
	if !premiumCodeRegex.MatchString(normalized) {
		return ErrInvalidCode
	}
	sum := sha256.Sum256([]byte(normalized))
	codeHashHex := hex.EncodeToString(sum[:])
	return s.store.RedeemPremiumCode(ctx, user.ID, codeHashHex, now)
}

func normalizePremiumCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func (s *Service) HandleWebhook(ctx context.Context, payload []byte, signatureHeader string, now time.Time) error {
	if err := VerifyStripeSignature(s.webhookSecret, payload, signatureHeader); err != nil {
		return err
	}

	var evt stripeEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("decode stripe event: %w", err)
	}
	if strings.TrimSpace(evt.ID) == "" {
		return fmt.Errorf("stripe event missing id")
	}

	meta := WebhookEventMeta{
		StripeEventID: evt.ID,
		EventType:     evt.Type,
		Livemode:      evt.Livemode,
		CreatedAt:     time.Unix(evt.Created, 0).UTC(),
	}

	alreadyProcessed, err := s.store.WithWebhookEvent(ctx, meta, func(ctx context.Context, tx services.Tx) error {
		switch evt.Type {
		case "checkout.session.completed":
			var session stripeCheckoutSession
			if err := json.Unmarshal(evt.Data.Object, &session); err != nil {
				return fmt.Errorf("decode checkout session: %w", err)
			}
			return s.handleCheckoutCompleted(ctx, tx, session)
		case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
			var sub stripeSubscription
			if err := json.Unmarshal(evt.Data.Object, &sub); err != nil {
				return fmt.Errorf("decode subscription: %w", err)
			}
			return s.handleSubscriptionEvent(ctx, tx, sub, now)
		default:
			return nil
		}
	})
	if alreadyProcessed {
		return nil
	}
	return err
}

type stripeEvent struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Livemode bool   `json:"livemode"`
	Created  int64  `json:"created"`
	Data     struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type stripeCheckoutSession struct {
	ID           string            `json:"id"`
	Customer     string            `json:"customer"`
	Subscription string            `json:"subscription"`
	Metadata     map[string]string `json:"metadata"`
}

type stripeSubscription struct {
	ID                string `json:"id"`
	Customer          string `json:"customer"`
	Status            string `json:"status"`
	CurrentPeriodEnd  int64  `json:"current_period_end"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
}

func (s *Service) handleCheckoutCompleted(ctx context.Context, tx services.DBConn, session stripeCheckoutSession) error {
	userIDStr := ""
	purchase := ""
	if session.Metadata != nil {
		userIDStr = session.Metadata["user_id"]
		purchase = session.Metadata["purchase"]
	}

	userID, err := uuid.Parse(strings.TrimSpace(userIDStr))
	if err != nil {
		// Ignore untrusted/missing metadata.
		return nil
	}

	if err := s.store.SetStripeIDs(ctx, userID, session.Customer, session.Subscription, tx); err != nil {
		return err
	}

	switch purchase {
	case "lifetime":
		return s.store.GrantLifetime(ctx, userID, session.Customer, tx)
	case "subscription", "tip", "":
		// Subscription entitlements are handled via subscription webhooks.
		return nil
	default:
		return nil
	}
}

func (s *Service) handleSubscriptionEvent(ctx context.Context, tx services.DBConn, sub stripeSubscription, now time.Time) error {
	status := mapStripeSubscriptionStatus(sub.Status)
	periodEnd := time.Unix(sub.CurrentPeriodEnd, 0).UTC()

	var userID uuid.UUID
	var err error
	if strings.TrimSpace(sub.ID) != "" {
		userID, err = s.store.FindUserIDByStripeSubscriptionID(ctx, sub.ID, tx)
	}
	if err != nil && errors.Is(err, ErrBillingUserNotFound) && strings.TrimSpace(sub.Customer) != "" {
		userID, err = s.store.FindUserIDByStripeCustomerID(ctx, sub.Customer, tx)
	}
	if err != nil {
		if errors.Is(err, ErrBillingUserNotFound) {
			return nil
		}
		return err
	}

	// Always enforce the "fully ended" rule.
	if status == "inactive" || status == "canceled" || !periodEnd.After(now) {
		return s.store.ResetToFree(ctx, userID, tx)
	}

	return s.store.SetSubscriptionState(ctx, userID, sub.Customer, sub.ID, status, periodEnd, sub.CancelAtPeriodEnd, tx)
}

func mapStripeSubscriptionStatus(stripeStatus string) string {
	switch stripeStatus {
	case "active":
		return "active"
	case "trialing":
		return "trialing"
	case "past_due":
		return "past_due"
	case "canceled":
		return "canceled"
	default:
		return "inactive"
	}
}

func (s *Service) successURL() string {
	return s.baseURL + "/?billing=success&session_id={CHECKOUT_SESSION_ID}#profile"
}

func (s *Service) cancelURL() string {
	return s.baseURL + "/?billing=cancel#profile"
}

func (s *Service) returnURL() string {
	return s.baseURL + "/#profile"
}
