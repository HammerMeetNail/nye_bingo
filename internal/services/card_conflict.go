package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func (s *CardService) CheckForConflict(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
	var card models.BingoCard

	// Build the query based on whether title is provided
	var query string
	var args []interface{}

	if title != nil && *title != "" {
		// Check for card with this specific title
		query = `SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		                is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
			FROM bingo_cards WHERE user_id = $1 AND year = $2 AND title = $3`
		args = []interface{}{userID, year, *title}
	} else {
		// Check for any card with null title (default card)
		query = `SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		                is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
			FROM bingo_cards WHERE user_id = $1 AND year = $2 AND title IS NULL`
		args = []interface{}{userID, year}
	}

	err := s.db.QueryRow(ctx, query, args...).Scan(
		&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title,
		&card.GridSize, &card.HeaderText, &card.HasFreeSpace, &card.FreeSpacePos,
		&card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("checking for conflict: %w", err)
	}

	// Load items for the card
	items, err := s.getCardItems(ctx, card.ID)
	if err != nil {
		return nil, err
	}
	card.Items = items

	return &card, nil
}

// Import imports an anonymous card, creating the card and all items in one transaction
