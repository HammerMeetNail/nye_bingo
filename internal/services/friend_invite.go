package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrInviteNotFound         = errors.New("invite not found")
	ErrInviteExpiryOutOfRange = errors.New("invite expiry out of range")
	ErrInviteLimitReached     = errors.New("invite limit reached")
)

const (
	InviteExpiryMinDays     = 1
	InviteExpiryMaxDays     = 365
	InviteExpiryDefaultDays = 14
	InviteMaxActive         = 5
)

type FriendInviteService struct {
	db                  DB
	notificationService NotificationServiceInterface
}

func NewFriendInviteService(db DB) *FriendInviteService {
	return &FriendInviteService{db: db}
}

func (s *FriendInviteService) SetNotificationService(notificationService NotificationServiceInterface) {
	s.notificationService = notificationService
}

func (s *FriendInviteService) CreateInvite(ctx context.Context, inviterID uuid.UUID, expiresInDays int) (*models.FriendInvite, string, error) {
	if err := validateInviteExpiryDays(expiresInDays); err != nil {
		return nil, "", err
	}
	if err := s.ensureInviteLimit(ctx, inviterID); err != nil {
		return nil, "", err
	}

	token, err := generateInviteToken()
	if err != nil {
		return nil, "", err
	}
	tokenHash := hashInviteToken(token)

	var expiresAt *time.Time
	if expiresInDays > 0 {
		t := time.Now().Add(time.Duration(expiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}

	invite := &models.FriendInvite{}
	err = s.db.QueryRow(ctx,
		`INSERT INTO friend_invites (inviter_user_id, invite_token_hash, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id, inviter_user_id, expires_at, revoked_at, accepted_by_user_id, accepted_at, created_at`,
		inviterID, tokenHash, expiresAt,
	).Scan(&invite.ID, &invite.InviterUserID, &invite.ExpiresAt, &invite.RevokedAt, &invite.AcceptedByUserID, &invite.AcceptedAt, &invite.CreatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("insert invite: %w", err)
	}

	return invite, token, nil
}

func (s *FriendInviteService) ListInvites(ctx context.Context, inviterID uuid.UUID) ([]models.FriendInvite, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, inviter_user_id, expires_at, revoked_at, accepted_by_user_id, accepted_at, created_at
		 FROM friend_invites
		 WHERE inviter_user_id = $1
		   AND revoked_at IS NULL
		   AND accepted_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())
		 ORDER BY created_at DESC`,
		inviterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	defer rows.Close()

	var invites []models.FriendInvite
	for rows.Next() {
		var invite models.FriendInvite
		if err := rows.Scan(&invite.ID, &invite.InviterUserID, &invite.ExpiresAt, &invite.RevokedAt, &invite.AcceptedByUserID, &invite.AcceptedAt, &invite.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invite: %w", err)
		}
		invites = append(invites, invite)
	}
	if invites == nil {
		invites = []models.FriendInvite{}
	}
	return invites, nil
}

func (s *FriendInviteService) ensureInviteLimit(ctx context.Context, inviterID uuid.UUID) error {
	var activeCount int
	err := s.db.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM friend_invites
		 WHERE inviter_user_id = $1
		   AND revoked_at IS NULL
		   AND accepted_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())`,
		inviterID,
	).Scan(&activeCount)
	if err != nil {
		return fmt.Errorf("count invites: %w", err)
	}
	if activeCount >= InviteMaxActive {
		return ErrInviteLimitReached
	}
	return nil
}

func (s *FriendInviteService) RevokeInvite(ctx context.Context, inviterID, inviteID uuid.UUID) error {
	result, err := s.db.Exec(ctx,
		`UPDATE friend_invites
		 SET revoked_at = NOW()
		 WHERE id = $1 AND inviter_user_id = $2 AND revoked_at IS NULL AND accepted_at IS NULL`,
		inviteID, inviterID,
	)
	if err != nil {
		return fmt.Errorf("revoke invite: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInviteNotFound
	}
	return nil
}

func (s *FriendInviteService) AcceptInvite(ctx context.Context, recipientID uuid.UUID, token string) (*models.UserSearchResult, error) {
	tokenHash := hashInviteToken(token)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin invite accept transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	var inviteID uuid.UUID
	var inviterID uuid.UUID
	var inviterUsername string
	err = tx.QueryRow(ctx,
		`SELECT fi.id, fi.inviter_user_id, u.username
		 FROM friend_invites fi
		 JOIN users u ON fi.inviter_user_id = u.id AND u.deleted_at IS NULL
		 WHERE fi.invite_token_hash = $1
		   AND fi.revoked_at IS NULL
		   AND fi.accepted_at IS NULL
		   AND (fi.expires_at IS NULL OR fi.expires_at > NOW())
		 FOR UPDATE`,
		tokenHash,
	).Scan(&inviteID, &inviterID, &inviterUsername)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInviteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load invite: %w", err)
	}

	if inviterID == recipientID {
		return nil, ErrCannotFriendSelf
	}

	if err := lockUserPairForUpdate(ctx, tx, inviterID, recipientID); err != nil {
		return nil, fmt.Errorf("lock users: %w", err)
	}

	var blocked bool
	err = tx.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE (blocker_id = $1 AND blocked_id = $2)
			   OR (blocker_id = $2 AND blocked_id = $1)
		)`,
		inviterID, recipientID,
	).Scan(&blocked)
	if err != nil {
		return nil, fmt.Errorf("check block status: %w", err)
	}
	if blocked {
		return nil, ErrUserBlocked
	}

	var exists bool
	err = tx.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM friendships
			WHERE (user_id = $1 AND friend_id = $2)
			   OR (user_id = $2 AND friend_id = $1)
		)`,
		inviterID, recipientID,
	).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check friendship: %w", err)
	}
	if exists {
		return nil, ErrFriendshipExists
	}

	var friendshipID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO friendships (user_id, friend_id, status)
		 VALUES ($1, $2, 'accepted')
		 RETURNING id`,
		inviterID, recipientID,
	).Scan(&friendshipID)
	if err != nil {
		return nil, fmt.Errorf("insert friendship: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE friend_invites
		 SET accepted_by_user_id = $1, accepted_at = NOW()
		 WHERE id = $2`,
		recipientID, inviteID,
	)
	if err != nil {
		return nil, fmt.Errorf("accept invite: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit invite accept: %w", err)
	}
	committed = true

	if s.notificationService != nil {
		if err := s.notificationService.NotifyFriendRequestAccepted(ctx, inviterID, recipientID, friendshipID); err != nil {
			logging.Error("Failed to send invite acceptance notification", map[string]interface{}{
				"error":         err.Error(),
				"inviter_id":    inviterID.String(),
				"recipient_id":  recipientID.String(),
				"friendship_id": friendshipID.String(),
			})
		}
	}

	return &models.UserSearchResult{ID: inviterID, Username: inviterUsername}, nil
}

func generateInviteToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashInviteToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func validateInviteExpiryDays(expiresInDays int) error {
	if expiresInDays < InviteExpiryMinDays || expiresInDays > InviteExpiryMaxDays {
		return ErrInviteExpiryOutOfRange
	}
	return nil
}
