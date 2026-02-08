package services

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func (s *CardService) Import(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error) {
	// Validate category if provided
	if params.Category != nil && *params.Category != "" {
		if !models.IsValidCategory(*params.Category) {
			return nil, ErrInvalidCategory
		}
	}

	// Validate title length if provided
	if params.Title != nil && len(*params.Title) > 100 {
		return nil, ErrTitleTooLong
	}

	if params.GridSize == 0 {
		params.GridSize = models.MaxGridSize
	}
	if !models.IsValidGridSize(params.GridSize) {
		return nil, ErrInvalidGridSize
	}
	if params.HeaderText == "" {
		params.HeaderText = models.DefaultHeaderText(params.GridSize)
	}
	params.HeaderText = models.NormalizeHeaderText(params.HeaderText)
	if err := models.ValidateHeaderText(params.HeaderText, params.GridSize); err != nil {
		return nil, ErrInvalidHeaderText
	}

	if params.HasFreeSpace && params.FreeSpacePos == nil {
		total := params.GridSize * params.GridSize
		if params.GridSize%2 == 1 {
			pos := total / 2
			params.FreeSpacePos = &pos
		} else {
			occupied := make(map[int]bool, len(params.Items))
			for _, it := range params.Items {
				occupied[it.Position] = true
			}
			empties := make([]int, 0, total-len(params.Items))
			for p := 0; p < total; p++ {
				if !occupied[p] {
					empties = append(empties, p)
				}
			}
			if len(empties) == 0 {
				return nil, ErrNoSpaceForFree
			}
			pos := empties[rand.Intn(len(empties))]
			params.FreeSpacePos = &pos
		}
	}
	if !params.HasFreeSpace {
		params.FreeSpacePos = nil
	}

	// Validate item positions
	totalSquares := params.GridSize * params.GridSize
	capacity := totalSquares
	if params.HasFreeSpace {
		capacity = totalSquares - 1
	}

	positions := make(map[int]bool)
	for _, item := range params.Items {
		if item.Position < 0 || item.Position >= totalSquares {
			return nil, ErrInvalidPosition
		}
		if params.FreeSpacePos != nil && item.Position == *params.FreeSpacePos {
			return nil, ErrInvalidPosition
		}
		if positions[item.Position] {
			return nil, ErrPositionOccupied
		}
		positions[item.Position] = true
	}

	if params.Finalize && len(params.Items) != capacity {
		return nil, fmt.Errorf("card needs %d items, has %d", capacity, len(params.Items))
	}

	// Start a transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Determine visibility (default to true if not specified)
	visibleToFriends := true
	if params.VisibleToFriends != nil {
		visibleToFriends = *params.VisibleToFriends
	}

	// Create the card
	card := &models.BingoCard{}
	err = tx.QueryRow(ctx,
		`INSERT INTO bingo_cards (user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position, is_finalized, visible_to_friends)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		           is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at`,
		params.UserID, params.Year, params.Category, params.Title, params.GridSize, params.HeaderText, params.HasFreeSpace, params.FreeSpacePos, params.Finalize, visibleToFriends,
	).Scan(
		&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title,
		&card.GridSize, &card.HeaderText, &card.HasFreeSpace, &card.FreeSpacePos,
		&card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating card: %w", err)
	}

	// Insert all items
	card.Items = make([]models.BingoItem, len(params.Items))
	for i, itemParam := range params.Items {
		var item models.BingoItem
		err = tx.QueryRow(ctx,
			`INSERT INTO bingo_items (card_id, position, content)
			 VALUES ($1, $2, $3)
			 RETURNING id, card_id, position, content, is_completed, completed_at, notes, proof_url, created_at`,
			card.ID, itemParam.Position, itemParam.Content,
		).Scan(&item.ID, &item.CardID, &item.Position, &item.Content, &item.IsCompleted, &item.CompletedAt, &item.Notes, &item.ProofURL, &item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating item: %w", err)
		}
		card.Items[i] = item
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	if card.IsFinalized && card.VisibleToFriends {
		s.notifyFriendsNewCard(ctx, card.UserID, card.ID)
	}

	return card, nil
}
