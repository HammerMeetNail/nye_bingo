package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

const (
	ShareExpiryMinDays = 1
	ShareExpiryMaxDays = 3650
)

var (
	ErrShareNotFound = errors.New("share not found")
)

func (s *CardService) CreateOrRotateShare(ctx context.Context, userID, cardID uuid.UUID, expiresAt *time.Time) (*models.CardShare, error) {
	cardOwnerID, finalized, err := s.loadCardOwner(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if cardOwnerID != userID {
		return nil, ErrNotCardOwner
	}
	if !finalized {
		return nil, ErrCardNotFinalized
	}

	token, err := generateShareToken()
	if err != nil {
		return nil, err
	}

	share := &models.CardShare{}
	err = s.db.QueryRow(ctx, `
		INSERT INTO bingo_card_shares (card_id, token, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (card_id)
		DO UPDATE SET token = EXCLUDED.token,
		              expires_at = EXCLUDED.expires_at,
		              created_at = NOW(),
		              last_accessed_at = NULL,
		              access_count = 0
		RETURNING card_id, token, created_at, expires_at, last_accessed_at, access_count
	`, cardID, token, expiresAt).Scan(
		&share.CardID,
		&share.Token,
		&share.CreatedAt,
		&share.ExpiresAt,
		&share.LastAccessedAt,
		&share.AccessCount,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting card share: %w", err)
	}

	return share, nil
}

func (s *CardService) GetShareStatus(ctx context.Context, userID, cardID uuid.UUID) (*models.CardShare, error) {
	cardOwnerID, _, err := s.loadCardOwner(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if cardOwnerID != userID {
		return nil, ErrNotCardOwner
	}

	share := &models.CardShare{}
	err = s.db.QueryRow(ctx, `
		SELECT card_id, token, created_at, expires_at, last_accessed_at, access_count
		FROM bingo_card_shares
		WHERE card_id = $1
	`, cardID).Scan(
		&share.CardID,
		&share.Token,
		&share.CreatedAt,
		&share.ExpiresAt,
		&share.LastAccessedAt,
		&share.AccessCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrShareNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading card share: %w", err)
	}

	return share, nil
}

func (s *CardService) RevokeShare(ctx context.Context, userID, cardID uuid.UUID) error {
	cardOwnerID, _, err := s.loadCardOwner(ctx, cardID)
	if err != nil {
		return err
	}
	if cardOwnerID != userID {
		return ErrNotCardOwner
	}

	if _, err := s.db.Exec(ctx, "DELETE FROM bingo_card_shares WHERE card_id = $1", cardID); err != nil {
		return fmt.Errorf("revoking card share: %w", err)
	}

	return nil
}

func (s *CardService) GetSharedCardByToken(ctx context.Context, token string) (*models.SharedCard, error) {
	card := models.PublicBingoCard{}
	var expiresAt *time.Time

	err := s.db.QueryRow(ctx, `
		SELECT c.id, c.year, c.category, c.title, c.grid_size, c.header_text, c.has_free_space,
		       c.free_space_position, c.is_finalized, s.expires_at
		FROM bingo_card_shares s
		JOIN bingo_cards c ON c.id = s.card_id
		WHERE s.token = $1
	`, token).Scan(
		&card.ID,
		&card.Year,
		&card.Category,
		&card.Title,
		&card.GridSize,
		&card.HeaderText,
		&card.HasFreeSpace,
		&card.FreeSpacePos,
		&card.IsFinalized,
		&expiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrShareNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading shared card: %w", err)
	}
	if !card.IsFinalized {
		return nil, ErrShareNotFound
	}
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return nil, ErrShareNotFound
	}

	rows, err := s.db.Query(ctx, `
		SELECT position, content, is_completed
		FROM bingo_items
		WHERE card_id = $1
		ORDER BY position
	`, card.ID)
	if err != nil {
		return nil, fmt.Errorf("loading shared items: %w", err)
	}
	defer rows.Close()

	items := make([]models.PublicBingoItem, 0)
	for rows.Next() {
		var item models.PublicBingoItem
		if err := rows.Scan(&item.Position, &item.Content, &item.IsCompleted); err != nil {
			return nil, fmt.Errorf("scanning shared item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating shared items: %w", err)
	}

	if err := s.touchShareToken(ctx, token); err != nil {
		logging.Warn("Failed to record share access", map[string]interface{}{"error": err.Error()})
	}

	return &models.SharedCard{Card: card, Items: items}, nil
}

func (s *CardService) loadCardOwner(ctx context.Context, cardID uuid.UUID) (uuid.UUID, bool, error) {
	var ownerID uuid.UUID
	var finalized bool
	err := s.db.QueryRow(ctx,
		"SELECT user_id, is_finalized FROM bingo_cards WHERE id = $1",
		cardID,
	).Scan(&ownerID, &finalized)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, false, ErrCardNotFound
	}
	if err != nil {
		return uuid.UUID{}, false, fmt.Errorf("loading card owner: %w", err)
	}
	return ownerID, finalized, nil
}

func (s *CardService) touchShareToken(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE bingo_card_shares
		 SET last_accessed_at = NOW(),
		     access_count = access_count + 1
		 WHERE token = $1
		   AND (last_accessed_at IS NULL OR last_accessed_at < NOW() - INTERVAL '1 hour')`,
		token,
	)
	if err != nil {
		return fmt.Errorf("touch share token: %w", err)
	}
	return nil
}

func generateShareToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate share token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
