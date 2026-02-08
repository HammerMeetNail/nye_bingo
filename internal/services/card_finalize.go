package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

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
	if card.VisibleToFriends {
		s.notifyFriendsNewCard(ctx, userID, cardID)
	}
	return card, nil
}

func (s *CardService) suggestEditTitle(ctx context.Context, userID uuid.UUID, year int, baseTitle string) (string, error) {
	baseTitle = strings.TrimSpace(baseTitle)
	if baseTitle == "" {
		baseTitle = fmt.Sprintf("%d Bingo Card", year)
	}

	for i := 1; i <= 25; i++ {
		candidate := withEditSuffix(baseTitle, i)
		t := candidate
		_, err := s.CheckForConflict(ctx, userID, year, &t)
		if errors.Is(err, ErrCardNotFound) {
			return candidate, nil
		}
		if err != nil && !errors.Is(err, ErrCardNotFound) {
			return "", fmt.Errorf("checking suggested title conflict: %w", err)
		}
	}

	// Last resort: add a random numeric suffix.
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // UX-only title suggestion.
	return withEditSuffix(baseTitle, rng.Intn(9999)+100), nil
}

func (s *CardService) EditFinalized(ctx context.Context, userID, sourceCardID uuid.UUID, params EditFinalizedCardParams) (*models.BingoCard, error) {
	source, err := s.GetByID(ctx, sourceCardID)
	if err != nil {
		return nil, err
	}
	if source.UserID != userID {
		return nil, ErrNotCardOwner
	}
	if !source.IsFinalized {
		return nil, ErrCardNotFinalized
	}

	title := ""
	if params.Title != nil {
		title = strings.TrimSpace(*params.Title)
	}
	if title == "" {
		title = withEditSuffix(source.DisplayName(), 1)
	}
	if utf8.RuneCountInString(title) > 100 {
		return nil, ErrTitleTooLong
	}
	titlePtr := &title

	existing, err := s.CheckForConflict(ctx, userID, source.Year, titlePtr)
	if err != nil && !errors.Is(err, ErrCardNotFound) {
		return nil, fmt.Errorf("checking for conflict: %w", err)
	}
	if existing != nil {
		suggested, err := s.suggestEditTitle(ctx, userID, source.Year, title)
		if err != nil {
			return nil, err
		}
		return nil, &CardConflictError{
			Year:           source.Year,
			Title:          title,
			SuggestedTitle: suggested,
		}
	}

	itemsToCopy := make([]models.BingoItem, len(source.Items))
	copy(itemsToCopy, source.Items)

	if params.ShuffleLayout {
		total := source.GridSize * source.GridSize
		positions := make([]int, 0, len(itemsToCopy))
		for p := 0; p < total; p++ {
			if source.FreeSpacePos != nil && p == *source.FreeSpacePos {
				continue
			}
			positions = append(positions, p)
		}
		if len(itemsToCopy) > len(positions) {
			return nil, fmt.Errorf("source card has %d items but only %d positions are available", len(itemsToCopy), len(positions))
		}
		rand.Shuffle(len(positions), func(i, j int) {
			positions[i], positions[j] = positions[j], positions[i]
		})
		for i := range itemsToCopy {
			itemsToCopy[i].Position = positions[i]
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after commit

	newCard := &models.BingoCard{}
	err = tx.QueryRow(ctx,
		`INSERT INTO bingo_cards (user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position, is_finalized, visible_to_friends)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false, $9)
		 RETURNING id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		           is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at`,
		userID, source.Year, source.Category, titlePtr, source.GridSize, source.HeaderText, source.HasFreeSpace, source.FreeSpacePos, source.VisibleToFriends,
	).Scan(
		&newCard.ID, &newCard.UserID, &newCard.Year, &newCard.Category, &newCard.Title,
		&newCard.GridSize, &newCard.HeaderText, &newCard.HasFreeSpace, &newCard.FreeSpacePos,
		&newCard.IsActive, &newCard.IsFinalized, &newCard.VisibleToFriends, &newCard.IsArchived, &newCard.CreatedAt, &newCard.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if mapped := mapBingoCardsUniqueViolationToCardExistsError(pgErr, titlePtr); mapped != nil {
				suggested, suggestErr := s.suggestEditTitle(ctx, userID, source.Year, title)
				if suggestErr != nil {
					return nil, suggestErr
				}
				return nil, &CardConflictError{
					Year:           source.Year,
					Title:          title,
					SuggestedTitle: suggested,
				}
			}
		}
		return nil, fmt.Errorf("creating editable card: %w", err)
	}

	for _, it := range itemsToCopy {
		if params.ResetProgress {
			_, err = tx.Exec(ctx,
				`INSERT INTO bingo_items (card_id, position, content)
				 VALUES ($1, $2, $3)`,
				newCard.ID, it.Position, it.Content,
			)
		} else {
			_, err = tx.Exec(ctx,
				`INSERT INTO bingo_items (card_id, position, content, is_completed, completed_at, notes, proof_url)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				newCard.ID, it.Position, it.Content, it.IsCompleted, it.CompletedAt, it.Notes, it.ProofURL,
			)
		}
		if err != nil {
			return nil, fmt.Errorf("copying item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return s.GetByID(ctx, newCard.ID)
}
