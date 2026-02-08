package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

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
