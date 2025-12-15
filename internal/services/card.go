package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrCardNotFound      = errors.New("card not found")
	ErrCardAlreadyExists = errors.New("card already exists for this year")
	ErrCardTitleExists   = errors.New("you already have a card with this title for this year")
	ErrCardFinalized     = errors.New("card is finalized and cannot be modified")
	ErrCardNotFinalized  = errors.New("card must be finalized first")
	ErrCardFull          = errors.New("card is full")
	ErrItemNotFound      = errors.New("item not found")
	ErrPositionOccupied  = errors.New("position is already occupied")
	ErrInvalidPosition   = errors.New("invalid position")
	ErrNotCardOwner      = errors.New("you do not own this card")
	ErrInvalidCategory   = errors.New("invalid category")
	ErrTitleTooLong      = errors.New("title must be 100 characters or less")
	ErrInvalidGridSize   = errors.New("invalid grid size")
	ErrInvalidHeaderText = errors.New("invalid header text")
	ErrNoSpaceForFree    = errors.New("no space available for free space")
)

type CardService struct {
	db *pgxpool.Pool
}

func NewCardService(db *pgxpool.Pool) *CardService {
	return &CardService{db: db}
}

func (s *CardService) Create(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error) {
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
	if params.Header == "" {
		params.Header = models.DefaultHeaderText(params.GridSize)
	}
	params.Header = models.NormalizeHeaderText(params.Header)
	if err := models.ValidateHeaderText(params.Header, params.GridSize); err != nil {
		return nil, ErrInvalidHeaderText
	}

	freePos := (*int)(nil)
	if params.HasFree {
		pos := models.BingoCard{GridSize: params.GridSize}.DefaultFreeSpacePosition()
		freePos = &pos
	}

	// Check for duplicate: same user, year, and title
	// If title is provided, check for existing card with same title
	// If title is nil/empty, check for existing card with null title
	var exists bool
	if params.Title != nil && *params.Title != "" {
		err := s.db.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM bingo_cards WHERE user_id = $1 AND year = $2 AND title = $3)",
			params.UserID, params.Year, *params.Title,
		).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("checking card existence: %w", err)
		}
		if exists {
			return nil, ErrCardTitleExists
		}
	} else {
		// Check for existing card without a title for this year
		err := s.db.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM bingo_cards WHERE user_id = $1 AND year = $2 AND title IS NULL)",
			params.UserID, params.Year,
		).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("checking card existence: %w", err)
		}
		if exists {
			return nil, ErrCardAlreadyExists
		}
	}

	card := &models.BingoCard{}
	err := s.db.QueryRow(ctx,
		`INSERT INTO bingo_cards (user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position, is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at`,
		params.UserID, params.Year, params.Category, params.Title, params.GridSize, params.Header, params.HasFree, freePos,
	).Scan(
		&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title,
		&card.GridSize, &card.HeaderText, &card.HasFreeSpace, &card.FreeSpacePos,
		&card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating card: %w", err)
	}

	card.Items = []models.BingoItem{}
	return card, nil
}

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

func (s *CardService) AddItem(ctx context.Context, userID uuid.UUID, params models.AddItemParams) (*models.BingoItem, error) {
	if params.Position == nil {
		// Choose a random available position atomically (important for small grids + concurrent adds).
		tx, err := s.db.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("starting transaction: %w", err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after commit

		card := &models.BingoCard{}
		err = tx.QueryRow(ctx,
			`SELECT id, user_id, grid_size, header_text, has_free_space, free_space_position, is_finalized
			 FROM bingo_cards
			 WHERE id = $1
			 FOR UPDATE`,
			params.CardID,
		).Scan(&card.ID, &card.UserID, &card.GridSize, &card.HeaderText, &card.HasFreeSpace, &card.FreeSpacePos, &card.IsFinalized)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCardNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("locking card: %w", err)
		}
		if card.UserID != userID {
			return nil, ErrNotCardOwner
		}
		if card.IsFinalized {
			return nil, ErrCardFinalized
		}

		rows, err := tx.Query(ctx, "SELECT position FROM bingo_items WHERE card_id = $1", params.CardID)
		if err != nil {
			return nil, fmt.Errorf("getting occupied positions: %w", err)
		}
		defer rows.Close()

		occupied := make(map[int]bool)
		itemCount := 0
		for rows.Next() {
			var pos int
			if err := rows.Scan(&pos); err != nil {
				return nil, fmt.Errorf("scanning occupied position: %w", err)
			}
			occupied[pos] = true
			itemCount++
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating occupied positions: %w", err)
		}

		if itemCount >= card.Capacity() {
			return nil, ErrCardFull
		}

		available := make([]int, 0, card.Capacity()-itemCount)
		for i := 0; i < card.TotalSquares(); i++ {
			if card.IsFreeSpacePosition(i) {
				continue
			}
			if !occupied[i] {
				available = append(available, i)
			}
		}
		if len(available) == 0 {
			return nil, ErrCardFull
		}
		position := available[rand.Intn(len(available))]

		item := &models.BingoItem{}
		err = tx.QueryRow(ctx,
			`INSERT INTO bingo_items (card_id, position, content)
			 VALUES ($1, $2, $3)
			 RETURNING id, card_id, position, content, is_completed, completed_at, notes, proof_url, created_at`,
			params.CardID, position, params.Content,
		).Scan(&item.ID, &item.CardID, &item.Position, &item.Content, &item.IsCompleted, &item.CompletedAt, &item.Notes, &item.ProofURL, &item.CreatedAt)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, ErrPositionOccupied
			}
			return nil, fmt.Errorf("adding item: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("committing transaction: %w", err)
		}
		return item, nil
	}

	// Explicit position (drag/drop or manual assignment)
	card, err := s.GetByID(ctx, params.CardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}
	if card.IsFinalized {
		return nil, ErrCardFinalized
	}
	if len(card.Items) >= card.Capacity() {
		return nil, ErrCardFull
	}

	position := *params.Position
	if !card.IsValidItemPosition(position) {
		return nil, ErrInvalidPosition
	}
	for _, existing := range card.Items {
		if existing.Position == position {
			return nil, ErrPositionOccupied
		}
	}

	item := &models.BingoItem{}
	err = s.db.QueryRow(ctx,
		`INSERT INTO bingo_items (card_id, position, content)
		 VALUES ($1, $2, $3)
		 RETURNING id, card_id, position, content, is_completed, completed_at, notes, proof_url, created_at`,
		params.CardID, position, params.Content,
	).Scan(&item.ID, &item.CardID, &item.Position, &item.Content, &item.IsCompleted, &item.CompletedAt, &item.Notes, &item.ProofURL, &item.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrPositionOccupied
		}
		return nil, fmt.Errorf("adding item: %w", err)
	}

	return item, nil
}

func (s *CardService) UpdateItem(ctx context.Context, userID, cardID uuid.UUID, position int, params models.UpdateItemParams) (*models.BingoItem, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}
	if card.IsFinalized {
		return nil, ErrCardFinalized
	}

	// Find the item
	var item *models.BingoItem
	for _, i := range card.Items {
		if i.Position == position {
			item = &i
			break
		}
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	// Update content if provided
	if params.Content != nil {
		_, err = s.db.Exec(ctx,
			"UPDATE bingo_items SET content = $1 WHERE id = $2",
			*params.Content, item.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("updating item content: %w", err)
		}
		item.Content = *params.Content
	}

	// Update position if provided
	if params.Position != nil {
		newPos := *params.Position
		if !card.IsValidItemPosition(newPos) {
			return nil, ErrInvalidPosition
		}
		// Check if new position is occupied
		for _, i := range card.Items {
			if i.Position == newPos && i.ID != item.ID {
				return nil, ErrPositionOccupied
			}
		}
		_, err = s.db.Exec(ctx,
			"UPDATE bingo_items SET position = $1 WHERE id = $2",
			newPos, item.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("updating item position: %w", err)
		}
		item.Position = newPos
	}

	return item, nil
}

func (s *CardService) SwapItems(ctx context.Context, userID, cardID uuid.UUID, pos1, pos2 int) error {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return err
	}
	if card.UserID != userID {
		return ErrNotCardOwner
	}
	if card.IsFinalized {
		return ErrCardFinalized
	}

	// Validate positions
	if pos1 == pos2 {
		return nil // No-op
	}
	if !card.IsPositionInRange(pos1) || !card.IsPositionInRange(pos2) {
		return ErrInvalidPosition
	}

	// If this swap involves the FREE cell, move FREE (draft-only).
	if card.HasFreePositionSet() && (pos1 == *card.FreeSpacePos || pos2 == *card.FreeSpacePos) {
		return s.moveFreeSpace(ctx, card, pos1, pos2)
	}
	if !card.IsValidItemPosition(pos1) || !card.IsValidItemPosition(pos2) {
		return ErrInvalidPosition
	}

	// Find items at both positions
	var item1, item2 *models.BingoItem
	for _, i := range card.Items {
		if i.Position == pos1 {
			item1 = &i
		}
		if i.Position == pos2 {
			item2 = &i
		}
	}

	// At least one item must exist
	if item1 == nil && item2 == nil {
		return ErrItemNotFound
	}

	// Use a transaction to swap atomically
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after commit

	// Use a temporary position to avoid unique constraint violation
	tempPos := -1

	if item1 != nil && item2 != nil {
		// Both positions occupied - swap them
		// Move item1 to temp position
		_, err = tx.Exec(ctx, "UPDATE bingo_items SET position = $1 WHERE id = $2", tempPos, item1.ID)
		if err != nil {
			return fmt.Errorf("moving item1 to temp: %w", err)
		}
		// Move item2 to pos1
		_, err = tx.Exec(ctx, "UPDATE bingo_items SET position = $1 WHERE id = $2", pos1, item2.ID)
		if err != nil {
			return fmt.Errorf("moving item2 to pos1: %w", err)
		}
		// Move item1 from temp to pos2
		_, err = tx.Exec(ctx, "UPDATE bingo_items SET position = $1 WHERE id = $2", pos2, item1.ID)
		if err != nil {
			return fmt.Errorf("moving item1 to pos2: %w", err)
		}
	} else if item1 != nil {
		// Only item1 exists - move to pos2
		_, err = tx.Exec(ctx, "UPDATE bingo_items SET position = $1 WHERE id = $2", pos2, item1.ID)
		if err != nil {
			return fmt.Errorf("moving item1 to pos2: %w", err)
		}
	} else {
		// Only item2 exists - move to pos1
		_, err = tx.Exec(ctx, "UPDATE bingo_items SET position = $1 WHERE id = $2", pos1, item2.ID)
		if err != nil {
			return fmt.Errorf("moving item2 to pos1: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (s *CardService) moveFreeSpace(ctx context.Context, card *models.BingoCard, pos1, pos2 int) error {
	if !card.HasFreePositionSet() {
		return ErrInvalidPosition
	}

	oldFree := *card.FreeSpacePos
	newFree := pos1
	if pos1 == oldFree {
		newFree = pos2
	}
	if !card.IsPositionInRange(newFree) {
		return ErrInvalidPosition
	}
	if newFree == oldFree {
		return nil
	}

	// Find if an item is being displaced.
	var displaced *models.BingoItem
	for _, it := range card.Items {
		if it.Position == newFree {
			itemCopy := it
			displaced = &itemCopy
			break
		}
	}

	// Determine empty positions after FREE moves (old FREE becomes available).
	occupied := make(map[int]bool, len(card.Items))
	for _, it := range card.Items {
		occupied[it.Position] = true
	}
	occupied[newFree] = false // displaced item will move

	candidates := make([]int, 0, card.TotalSquares())
	for p := 0; p < card.TotalSquares(); p++ {
		if p == newFree {
			continue
		}
		if occupied[p] {
			continue
		}
		candidates = append(candidates, p)
	}
	if displaced != nil && len(candidates) == 0 {
		return ErrNoSpaceForFree
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after commit

	_, err = tx.Exec(ctx, "UPDATE bingo_cards SET free_space_position = $1 WHERE id = $2", newFree, card.ID)
	if err != nil {
		return fmt.Errorf("updating free space: %w", err)
	}

	if displaced != nil {
		newPos := candidates[rand.Intn(len(candidates))]
		_, err = tx.Exec(ctx, "UPDATE bingo_items SET position = $1 WHERE id = $2", newPos, displaced.ID)
		if err != nil {
			return fmt.Errorf("relocating displaced item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (s *CardService) RemoveItem(ctx context.Context, userID, cardID uuid.UUID, position int) error {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return err
	}
	if card.UserID != userID {
		return ErrNotCardOwner
	}
	if card.IsFinalized {
		return ErrCardFinalized
	}

	result, err := s.db.Exec(ctx,
		"DELETE FROM bingo_items WHERE card_id = $1 AND position = $2",
		cardID, position,
	)
	if err != nil {
		return fmt.Errorf("removing item: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrItemNotFound
	}

	return nil
}

func (s *CardService) Delete(ctx context.Context, userID, cardID uuid.UUID) error {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return err
	}
	if card.UserID != userID {
		return ErrNotCardOwner
	}

	// Delete items first (foreign key constraint)
	_, err = s.db.Exec(ctx, "DELETE FROM bingo_items WHERE card_id = $1", cardID)
	if err != nil {
		return fmt.Errorf("deleting card items: %w", err)
	}

	// Delete the card
	result, err := s.db.Exec(ctx, "DELETE FROM bingo_cards WHERE id = $1", cardID)
	if err != nil {
		return fmt.Errorf("deleting card: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrCardNotFound
	}

	return nil
}

// UpdateMeta updates the category and/or title of a card
func (s *CardService) UpdateMeta(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardMetaParams) (*models.BingoCard, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}

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

	// Check for duplicate title if changing to a non-empty title
	if params.Title != nil && *params.Title != "" {
		var exists bool
		err := s.db.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM bingo_cards WHERE user_id = $1 AND year = $2 AND title = $3 AND id != $4)",
			card.UserID, card.Year, *params.Title, cardID,
		).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("checking title uniqueness: %w", err)
		}
		if exists {
			return nil, ErrCardTitleExists
		}
	}

	// Build update query dynamically based on what's provided
	if params.Category != nil || params.Title != nil {
		_, err = s.db.Exec(ctx,
			`UPDATE bingo_cards SET category = COALESCE($1, category), title = COALESCE($2, title) WHERE id = $3`,
			params.Category, params.Title, cardID,
		)
		if err != nil {
			return nil, fmt.Errorf("updating card meta: %w", err)
		}
	}

	// Return updated card
	return s.GetByID(ctx, cardID)
}

func (s *CardService) Shuffle(ctx context.Context, userID, cardID uuid.UUID) (*models.BingoCard, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}
	if card.IsFinalized {
		return nil, ErrCardFinalized
	}

	if len(card.Items) == 0 {
		return card, nil
	}

	// Get all available positions (excluding FREE if enabled)
	availablePositions := make([]int, 0, card.TotalSquares())
	for i := 0; i < card.TotalSquares(); i++ {
		if !card.IsFreeSpacePosition(i) {
			availablePositions = append(availablePositions, i)
		}
	}

	// Shuffle positions
	rand.Shuffle(len(availablePositions), func(i, j int) {
		availablePositions[i], availablePositions[j] = availablePositions[j], availablePositions[i]
	})

	// Update items with new positions
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// First, set all positions to negative to avoid unique constraint violations
	for i, item := range card.Items {
		_, err = tx.Exec(ctx,
			"UPDATE bingo_items SET position = $1 WHERE id = $2",
			-(i + 1), item.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("clearing position: %w", err)
		}
	}

	// Then assign new positions
	for i, item := range card.Items {
		_, err = tx.Exec(ctx,
			"UPDATE bingo_items SET position = $1 WHERE id = $2",
			availablePositions[i], item.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("assigning new position: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	// Reload card with updated positions
	return s.GetByID(ctx, cardID)
}

// FinalizeParams contains optional parameters for finalizing a card
type FinalizeParams struct {
	VisibleToFriends *bool // Optional; if nil, keeps current value (default true for new cards)
}

func (s *CardService) Finalize(ctx context.Context, userID, cardID uuid.UUID, params *FinalizeParams) (*models.BingoCard, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}
	if card.IsFinalized {
		return card, nil // Already finalized
	}

	// Ensure card has all items for the configured grid
	if len(card.Items) < card.Capacity() {
		return nil, fmt.Errorf("card needs %d items, has %d", card.Capacity(), len(card.Items))
	}

	// Determine visibility setting
	visibleToFriends := card.VisibleToFriends // Keep current value by default
	if params != nil && params.VisibleToFriends != nil {
		visibleToFriends = *params.VisibleToFriends
	}

	_, err = s.db.Exec(ctx,
		"UPDATE bingo_cards SET is_finalized = true, visible_to_friends = $2 WHERE id = $1",
		cardID, visibleToFriends,
	)
	if err != nil {
		return nil, fmt.Errorf("finalizing card: %w", err)
	}

	card.IsFinalized = true
	card.VisibleToFriends = visibleToFriends
	return card, nil
}

// UpdateVisibility updates the visibility of a card to friends
func (s *CardService) UpdateVisibility(ctx context.Context, userID, cardID uuid.UUID, visibleToFriends bool) (*models.BingoCard, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}

	_, err = s.db.Exec(ctx,
		"UPDATE bingo_cards SET visible_to_friends = $2, updated_at = NOW() WHERE id = $1",
		cardID, visibleToFriends,
	)
	if err != nil {
		return nil, fmt.Errorf("updating visibility: %w", err)
	}

	card.VisibleToFriends = visibleToFriends
	return card, nil
}

// BulkUpdateVisibility updates the visibility of multiple cards owned by the user
// Returns the count of cards updated (cards not owned by user are silently skipped)
func (s *CardService) BulkUpdateVisibility(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, visibleToFriends bool) (int, error) {
	if len(cardIDs) == 0 {
		return 0, nil
	}

	result, err := s.db.Exec(ctx,
		`UPDATE bingo_cards SET visible_to_friends = $1, updated_at = NOW()
		 WHERE id = ANY($2) AND user_id = $3`,
		visibleToFriends, cardIDs, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk updating visibility: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// BulkDelete deletes multiple cards owned by the user
// Returns the count of cards deleted (cards not owned by user are silently skipped)
func (s *CardService) BulkDelete(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID) (int, error) {
	if len(cardIDs) == 0 {
		return 0, nil
	}

	// First delete items for these cards (owned by user)
	_, err := s.db.Exec(ctx,
		`DELETE FROM bingo_items WHERE card_id IN (
			SELECT id FROM bingo_cards WHERE id = ANY($1) AND user_id = $2
		)`,
		cardIDs, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk deleting card items: %w", err)
	}

	// Then delete the cards
	result, err := s.db.Exec(ctx,
		`DELETE FROM bingo_cards WHERE id = ANY($1) AND user_id = $2`,
		cardIDs, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk deleting cards: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// BulkUpdateArchive updates the archive status of multiple cards owned by the user
// Returns the count of cards updated (cards not owned by user are silently skipped)
func (s *CardService) BulkUpdateArchive(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, isArchived bool) (int, error) {
	if len(cardIDs) == 0 {
		return 0, nil
	}

	result, err := s.db.Exec(ctx,
		`UPDATE bingo_cards SET is_archived = $1, updated_at = NOW()
		 WHERE id = ANY($2) AND user_id = $3`,
		isArchived, cardIDs, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk updating archive status: %w", err)
	}

	return int(result.RowsAffected()), nil
}

func (s *CardService) CompleteItem(ctx context.Context, userID, cardID uuid.UUID, position int, params models.CompleteItemParams) (*models.BingoItem, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}
	if !card.IsFinalized {
		return nil, ErrCardNotFinalized
	}

	// Find the item
	var item *models.BingoItem
	for _, i := range card.Items {
		if i.Position == position {
			itemCopy := i
			item = &itemCopy
			break
		}
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	now := time.Now()
	_, err = s.db.Exec(ctx,
		`UPDATE bingo_items
		 SET is_completed = true, completed_at = $1, notes = $2, proof_url = $3
		 WHERE id = $4`,
		now, params.Notes, params.ProofURL, item.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("completing item: %w", err)
	}

	item.IsCompleted = true
	item.CompletedAt = &now
	item.Notes = params.Notes
	item.ProofURL = params.ProofURL

	return item, nil
}

func (s *CardService) UncompleteItem(ctx context.Context, userID, cardID uuid.UUID, position int) (*models.BingoItem, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}
	if !card.IsFinalized {
		return nil, ErrCardNotFinalized
	}

	// Find the item
	var item *models.BingoItem
	for _, i := range card.Items {
		if i.Position == position {
			itemCopy := i
			item = &itemCopy
			break
		}
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	_, err = s.db.Exec(ctx,
		`UPDATE bingo_items
		 SET is_completed = false, completed_at = NULL
		 WHERE id = $1`,
		item.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("uncompleting item: %w", err)
	}

	item.IsCompleted = false
	item.CompletedAt = nil

	return item, nil
}

func (s *CardService) UpdateItemNotes(ctx context.Context, userID, cardID uuid.UUID, position int, notes, proofURL *string) (*models.BingoItem, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}

	// Find the item
	var item *models.BingoItem
	for _, i := range card.Items {
		if i.Position == position {
			itemCopy := i
			item = &itemCopy
			break
		}
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	_, err = s.db.Exec(ctx,
		"UPDATE bingo_items SET notes = $1, proof_url = $2 WHERE id = $3",
		notes, proofURL, item.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("updating notes: %w", err)
	}

	item.Notes = notes
	item.ProofURL = proofURL

	return item, nil
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

func (s *CardService) findRandomPosition(card *models.BingoCard) (int, error) {
	occupied := make(map[int]bool, len(card.Items)+1)
	for _, item := range card.Items {
		occupied[item.Position] = true
	}
	if card.HasFreePositionSet() {
		occupied[*card.FreeSpacePos] = true
	}

	available := make([]int, 0, card.Capacity()-len(card.Items))
	for i := 0; i < card.TotalSquares(); i++ {
		if !occupied[i] {
			available = append(available, i)
		}
	}

	if len(available) == 0 {
		return 0, ErrCardFull
	}

	return available[rand.Intn(len(available))], nil
}

// GetArchive returns all finalized cards from past years (not current year)
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

// CheckForConflict checks if a card already exists for the given user, year, and optional title
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

	return card, nil
}

func (s *CardService) UpdateConfig(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardConfigParams) (*models.BingoCard, error) {
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}
	if card.IsFinalized {
		return nil, ErrCardFinalized
	}

	headerText := (*string)(nil)
	if params.HeaderText != nil {
		normalized := models.NormalizeHeaderText(*params.HeaderText)
		if err := models.ValidateHeaderText(normalized, card.GridSize); err != nil {
			return nil, ErrInvalidHeaderText
		}
		headerText = &normalized
	}

	hasFree := card.HasFreeSpace
	freePos := card.FreeSpacePos

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after commit

	if params.HasFreeSpace != nil && *params.HasFreeSpace != card.HasFreeSpace {
		if *params.HasFreeSpace {
			total := card.GridSize * card.GridSize
			occupied := make(map[int]bool, len(card.Items))
			for _, it := range card.Items {
				occupied[it.Position] = true
			}

			desired := -1
			if card.GridSize%2 == 1 {
				desired = total / 2
			} else {
				empties := make([]int, 0, total-len(card.Items))
				for p := 0; p < total; p++ {
					if !occupied[p] {
						empties = append(empties, p)
					}
				}
				if len(empties) == 0 {
					return nil, ErrNoSpaceForFree
				}
				desired = empties[rand.Intn(len(empties))]
			}

			if occupied[desired] {
				empties := make([]int, 0, total-len(card.Items))
				for p := 0; p < total; p++ {
					if p == desired {
						continue
					}
					if !occupied[p] {
						empties = append(empties, p)
					}
				}
				if len(empties) == 0 {
					return nil, ErrNoSpaceForFree
				}
				newPos := empties[rand.Intn(len(empties))]
				_, err := tx.Exec(ctx,
					"UPDATE bingo_items SET position = $1 WHERE card_id = $2 AND position = $3",
					newPos, card.ID, desired,
				)
				if err != nil {
					return nil, fmt.Errorf("relocating item for free space: %w", err)
				}
			}

			hasFree = true
			freePos = &desired
		} else {
			hasFree = false
			freePos = nil
		}
	}

	_, err = tx.Exec(ctx,
		`UPDATE bingo_cards
		 SET header_text = COALESCE($1, header_text),
		     has_free_space = $2,
		     free_space_position = $3
		 WHERE id = $4`,
		headerText, hasFree, freePos, card.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("updating card config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return s.GetByID(ctx, card.ID)
}

type CloneParams struct {
	Year         *int
	Title        *string
	Category     *string
	GridSize     int
	HeaderText   string
	HasFreeSpace *bool
}

type CloneResult struct {
	Card               *models.BingoCard
	TruncatedItemCount int
}

func resolveCloneHasFreeSpace(sourceHasFreeSpace bool, override *bool) bool {
	if override == nil {
		return sourceHasFreeSpace
	}
	return *override
}

func mapBingoCardsUniqueViolationToCardExistsError(pgErr *pgconn.PgError, title *string) error {
	if pgErr == nil || pgErr.Code != "23505" {
		return nil
	}

	switch pgErr.ConstraintName {
	case "idx_bingo_cards_user_year_null_title":
		return ErrCardAlreadyExists
	case "idx_bingo_cards_user_year_title":
		return ErrCardTitleExists
	}

	if title == nil || strings.TrimSpace(*title) == "" {
		return ErrCardAlreadyExists
	}
	return ErrCardTitleExists
}

func (s *CardService) Clone(ctx context.Context, userID, sourceCardID uuid.UUID, params CloneParams) (*CloneResult, error) {
	source, err := s.GetByID(ctx, sourceCardID)
	if err != nil {
		return nil, err
	}
	if source.UserID != userID {
		return nil, ErrNotCardOwner
	}

	if params.GridSize == 0 {
		params.GridSize = source.GridSize
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

	year := source.Year
	if params.Year != nil && *params.Year != 0 {
		year = *params.Year
	}

	title := (*string)(nil)
	if params.Title != nil {
		trimmed := strings.TrimSpace(*params.Title)
		title = &trimmed
	}
	if title == nil || *title == "" {
		fallback := source.DisplayName() + " (Copy)"
		title = &fallback
	}

	category := source.Category
	if params.Category != nil {
		category = params.Category
	}

	hasFreeSpace := resolveCloneHasFreeSpace(source.HasFreeSpace, params.HasFreeSpace)

	freePos := (*int)(nil)
	if hasFreeSpace {
		pos := models.BingoCard{GridSize: params.GridSize}.DefaultFreeSpacePosition()
		freePos = &pos
	}

	totalSquares := params.GridSize * params.GridSize
	capacity := totalSquares
	if hasFreeSpace {
		capacity = totalSquares - 1
	}

	itemsToCopy := make([]models.BingoItem, 0, len(source.Items))
	itemsToCopy = append(itemsToCopy, source.Items...)
	truncated := 0
	if len(itemsToCopy) > capacity {
		truncated = len(itemsToCopy) - capacity
		itemsToCopy = itemsToCopy[:capacity]
	}

	availablePositions := make([]int, 0, capacity)
	for p := 0; p < totalSquares; p++ {
		if freePos != nil && p == *freePos {
			continue
		}
		availablePositions = append(availablePositions, p)
	}
	rand.Shuffle(len(availablePositions), func(i, j int) {
		availablePositions[i], availablePositions[j] = availablePositions[j], availablePositions[i]
	})

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after commit

	newCard := &models.BingoCard{}
	err = tx.QueryRow(ctx,
		`INSERT INTO bingo_cards (user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		           is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at`,
		userID, year, category, title, params.GridSize, params.HeaderText, hasFreeSpace, freePos,
	).Scan(
		&newCard.ID, &newCard.UserID, &newCard.Year, &newCard.Category, &newCard.Title,
		&newCard.GridSize, &newCard.HeaderText, &newCard.HasFreeSpace, &newCard.FreeSpacePos,
		&newCard.IsActive, &newCard.IsFinalized, &newCard.VisibleToFriends, &newCard.IsArchived, &newCard.CreatedAt, &newCard.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if mapped := mapBingoCardsUniqueViolationToCardExistsError(pgErr, title); mapped != nil {
				return nil, mapped
			}
		}
		return nil, fmt.Errorf("creating cloned card: %w", err)
	}

	for i, it := range itemsToCopy {
		pos := availablePositions[i]
		_, err := tx.Exec(ctx,
			`INSERT INTO bingo_items (card_id, position, content)
			 VALUES ($1, $2, $3)`,
			newCard.ID, pos, it.Content,
		)
		if err != nil {
			return nil, fmt.Errorf("copying item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	created, err := s.GetByID(ctx, newCard.ID)
	if err != nil {
		return nil, err
	}

	return &CloneResult{Card: created, TruncatedItemCount: truncated}, nil
}
