package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrCardNotFound      = errors.New("card not found")
	ErrCardAlreadyExists = errors.New("card already exists for this year")
	ErrCardTitleExists   = errors.New("you already have a card with this title for this year")
	ErrCardFinalized     = errors.New("card is finalized and cannot be modified")
	ErrCardNotFinalized  = errors.New("card must be finalized first")
	ErrCardFull          = errors.New("card already has 24 items")
	ErrItemNotFound      = errors.New("item not found")
	ErrPositionOccupied  = errors.New("position is already occupied")
	ErrInvalidPosition   = errors.New("invalid position")
	ErrNotCardOwner      = errors.New("you do not own this card")
	ErrInvalidCategory   = errors.New("invalid category")
	ErrTitleTooLong      = errors.New("title must be 100 characters or less")
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
		`INSERT INTO bingo_cards (user_id, year, category, title)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, year, category, title, is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at`,
		params.UserID, params.Year, params.Category, params.Title,
	).Scan(&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title, &card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating card: %w", err)
	}

	card.Items = []models.BingoItem{}
	return card, nil
}

func (s *CardService) GetByID(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error) {
	card := &models.BingoCard{}
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, year, category, title, is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
		 FROM bingo_cards WHERE id = $1`,
		cardID,
	).Scan(&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title, &card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt)
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
		`SELECT id, user_id, year, category, title, is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
		 FROM bingo_cards WHERE user_id = $1 AND year = $2`,
		userID, year,
	).Scan(&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title, &card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt)
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
		`SELECT id, user_id, year, category, title, is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
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
		if err := rows.Scan(&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title, &card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt); err != nil {
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
	// Get and verify card ownership
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

	// Check item count
	if len(card.Items) >= models.ItemsRequired {
		return nil, ErrCardFull
	}

	// Determine position
	var position int
	if params.Position != nil {
		position = *params.Position
		if position < 0 || position >= models.TotalSquares || position == models.FreeSpacePos {
			return nil, ErrInvalidPosition
		}
		// Check if position is occupied
		for _, item := range card.Items {
			if item.Position == position {
				return nil, ErrPositionOccupied
			}
		}
	} else {
		// Find random available position
		position, err = s.findRandomPosition(card.Items)
		if err != nil {
			return nil, err
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
		if newPos < 0 || newPos >= models.TotalSquares || newPos == models.FreeSpacePos {
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

	// Get all available positions (excluding free space)
	availablePositions := make([]int, 0, models.ItemsRequired)
	for i := 0; i < models.TotalSquares; i++ {
		if i != models.FreeSpacePos {
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

	// Ensure card has all 24 items
	if len(card.Items) < models.ItemsRequired {
		return nil, fmt.Errorf("card needs %d items, has %d", models.ItemsRequired, len(card.Items))
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

func (s *CardService) findRandomPosition(existingItems []models.BingoItem) (int, error) {
	occupied := make(map[int]bool)
	occupied[models.FreeSpacePos] = true
	for _, item := range existingItems {
		occupied[item.Position] = true
	}

	available := make([]int, 0)
	for i := 0; i < models.TotalSquares; i++ {
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
		`SELECT id, user_id, year, category, title, is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
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
		if err := rows.Scan(&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title, &card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt); err != nil {
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
		TotalItems: len(card.Items),
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
	stats.BingosAchieved = s.countBingos(card.Items)

	return stats, nil
}

// countBingos counts how many bingos (rows, columns, diagonals) are complete
func (s *CardService) countBingos(items []models.BingoItem) int {
	// Create a 5x5 grid of completion status
	grid := make([]bool, models.TotalSquares)

	// Mark free space as completed
	grid[models.FreeSpacePos] = true

	// Mark completed items
	for _, item := range items {
		if item.IsCompleted {
			grid[item.Position] = true
		}
	}

	bingos := 0

	// Check rows
	for row := 0; row < 5; row++ {
		complete := true
		for col := 0; col < 5; col++ {
			if !grid[row*5+col] {
				complete = false
				break
			}
		}
		if complete {
			bingos++
		}
	}

	// Check columns
	for col := 0; col < 5; col++ {
		complete := true
		for row := 0; row < 5; row++ {
			if !grid[row*5+col] {
				complete = false
				break
			}
		}
		if complete {
			bingos++
		}
	}

	// Check diagonals
	// Top-left to bottom-right: 0, 6, 12, 18, 24
	diagonal1 := []int{0, 6, 12, 18, 24}
	complete := true
	for _, pos := range diagonal1 {
		if !grid[pos] {
			complete = false
			break
		}
	}
	if complete {
		bingos++
	}

	// Top-right to bottom-left: 4, 8, 12, 16, 20
	diagonal2 := []int{4, 8, 12, 16, 20}
	complete = true
	for _, pos := range diagonal2 {
		if !grid[pos] {
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
		query = `SELECT id, user_id, year, category, title, is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
			FROM bingo_cards WHERE user_id = $1 AND year = $2 AND title = $3`
		args = []interface{}{userID, year, *title}
	} else {
		// Check for any card with null title (default card)
		query = `SELECT id, user_id, year, category, title, is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
			FROM bingo_cards WHERE user_id = $1 AND year = $2 AND title IS NULL`
		args = []interface{}{userID, year}
	}

	err := s.db.QueryRow(ctx, query, args...).Scan(
		&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title,
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

	// Validate item positions
	positions := make(map[int]bool)
	for _, item := range params.Items {
		// Position must be 0-24 excluding 12 (free space)
		if item.Position < 0 || item.Position > 24 || item.Position == models.FreeSpacePos {
			return nil, ErrInvalidPosition
		}
		// Check for duplicate positions
		if positions[item.Position] {
			return nil, ErrPositionOccupied
		}
		positions[item.Position] = true
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
		`INSERT INTO bingo_cards (user_id, year, category, title, is_finalized, visible_to_friends)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, year, category, title, is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at`,
		params.UserID, params.Year, params.Category, params.Title, params.Finalize, visibleToFriends,
	).Scan(&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title, &card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt)
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
