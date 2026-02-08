package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func (s *CardService) GetByID(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error) {
	card := &models.BingoCard{}
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		        is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
		 FROM bingo_cards WHERE id = $1`,
		cardID,
	).Scan(
		&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title,
		&card.GridSize, &card.HeaderText, &card.HasFreeSpace, &card.FreeSpacePos,
		&card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting card: %w", err)
	}

	items, err := s.getCardItems(ctx, cardID)
	if err != nil {
		return nil, err
	}
	card.Items = items

	return card, nil
}

func (s *CardService) GetByUserAndYear(ctx context.Context, userID uuid.UUID, year int) (*models.BingoCard, error) {
	card := &models.BingoCard{}
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		        is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
		 FROM bingo_cards WHERE user_id = $1 AND year = $2`,
		userID, year,
	).Scan(
		&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title,
		&card.GridSize, &card.HeaderText, &card.HasFreeSpace, &card.FreeSpacePos,
		&card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting card: %w", err)
	}

	items, err := s.getCardItems(ctx, card.ID)
	if err != nil {
		return nil, err
	}
	card.Items = items

	return card, nil
}

func (s *CardService) ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		        is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
		 FROM bingo_cards WHERE user_id = $1 ORDER BY year DESC, created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing cards: %w", err)
	}
	defer rows.Close()

	var cards []*models.BingoCard
	for rows.Next() {
		card := &models.BingoCard{}
		if err := rows.Scan(
			&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title,
			&card.GridSize, &card.HeaderText, &card.HasFreeSpace, &card.FreeSpacePos,
			&card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning card: %w", err)
		}
		cards = append(cards, card)
	}

	// Load items for each card
	for _, card := range cards {
		items, err := s.getCardItems(ctx, card.ID)
		if err != nil {
			return nil, err
		}
		card.Items = items
	}

	return cards, nil
}

func (s *CardService) getCardItems(ctx context.Context, cardID uuid.UUID) ([]models.BingoItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, card_id, position, content, is_completed, completed_at, notes, proof_url, created_at
		 FROM bingo_items WHERE card_id = $1 ORDER BY position`,
		cardID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting card items: %w", err)
	}
	defer rows.Close()

	var items []models.BingoItem
	for rows.Next() {
		var item models.BingoItem
		if err := rows.Scan(&item.ID, &item.CardID, &item.Position, &item.Content, &item.IsCompleted, &item.CompletedAt, &item.Notes, &item.ProofURL, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning item: %w", err)
		}
		items = append(items, item)
	}

	if items == nil {
		items = []models.BingoItem{}
	}

	return items, nil
}

func (s *CardService) GetArchive(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
	currentYear := time.Now().Year()

	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		        is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
		 FROM bingo_cards
		 WHERE user_id = $1 AND year < $2 AND is_finalized = true
		 ORDER BY year DESC, created_at DESC`,
		userID, currentYear,
	)
	if err != nil {
		return nil, fmt.Errorf("listing archive cards: %w", err)
	}
	defer rows.Close()

	var cards []*models.BingoCard
	for rows.Next() {
		card := &models.BingoCard{}
		if err := rows.Scan(
			&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title,
			&card.GridSize, &card.HeaderText, &card.HasFreeSpace, &card.FreeSpacePos,
			&card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning card: %w", err)
		}
		cards = append(cards, card)
	}

	// Load items for each card
	for _, card := range cards {
		items, err := s.getCardItems(ctx, card.ID)
		if err != nil {
			return nil, err
		}
		card.Items = items
	}

	return cards, nil
}

// GetStats calculates statistics for a specific card
func (s *CardService) GetStats(ctx context.Context, userID, cardID uuid.UUID) (*models.CardStats, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}

	stats := &models.CardStats{
		CardID:     card.ID,
		Year:       card.Year,
		TotalItems: card.Capacity(),
	}

	// Count completed items and find first/last completion
	var firstCompletion, lastCompletion *time.Time
	for _, item := range card.Items {
		if item.IsCompleted {
			stats.CompletedItems++
			if item.CompletedAt != nil {
				if firstCompletion == nil || item.CompletedAt.Before(*firstCompletion) {
					firstCompletion = item.CompletedAt
				}
				if lastCompletion == nil || item.CompletedAt.After(*lastCompletion) {
					lastCompletion = item.CompletedAt
				}
			}
		}
	}

	stats.FirstCompletion = firstCompletion
	stats.LastCompletion = lastCompletion

	// Calculate completion rate
	if stats.TotalItems > 0 {
		stats.CompletionRate = float64(stats.CompletedItems) / float64(stats.TotalItems) * 100
	}

	// Count bingos achieved
	stats.BingosAchieved = s.countBingos(card.Items, card.GridSize, func() *int {
		if card.HasFreeSpace {
			return card.FreeSpacePos
		}
		return nil
	}())

	return stats, nil
}

// countBingos counts how many bingos (rows, columns, diagonals) are complete
func (s *CardService) countBingos(items []models.BingoItem, gridSize int, freePos *int) int {
	if !models.IsValidGridSize(gridSize) {
		gridSize = models.MaxGridSize
	}
	total := gridSize * gridSize
	grid := make([]bool, total)

	if freePos != nil && *freePos >= 0 && *freePos < total {
		grid[*freePos] = true
	}

	// Mark completed items
	for _, item := range items {
		if item.IsCompleted {
			grid[item.Position] = true
		}
	}

	bingos := 0

	// Check rows
	for row := 0; row < gridSize; row++ {
		complete := true
		for col := 0; col < gridSize; col++ {
			if !grid[row*gridSize+col] {
				complete = false
				break
			}
		}
		if complete {
			bingos++
		}
	}

	// Check columns
	for col := 0; col < gridSize; col++ {
		complete := true
		for row := 0; row < gridSize; row++ {
			if !grid[row*gridSize+col] {
				complete = false
				break
			}
		}
		if complete {
			bingos++
		}
	}

	// Check diagonals
	complete := true
	for i := 0; i < gridSize; i++ {
		if !grid[i*gridSize+i] {
			complete = false
			break
		}
	}
	if complete {
		bingos++
	}

	complete = true
	for i := 0; i < gridSize; i++ {
		if !grid[i*gridSize+(gridSize-1-i)] {
			complete = false
			break
		}
	}
	if complete {
		bingos++
	}

	return bingos
}
