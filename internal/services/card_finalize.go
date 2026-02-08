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

	title := source.Title
	if params.Title != nil {
		trimmed := strings.TrimSpace(*params.Title)
		requestedTitle := ""
		if trimmed == "" {
			title = nil
			requestedTitle = fmt.Sprintf("%d Bingo Card", source.Year)
		} else {
			if utf8.RuneCountInString(trimmed) > 100 {
				return nil, ErrTitleTooLong
			}
			title = &trimmed
			requestedTitle = trimmed
		}

		existing, err := s.CheckForConflict(ctx, userID, source.Year, title)
		if err != nil && !errors.Is(err, ErrCardNotFound) {
			return nil, fmt.Errorf("checking for conflict: %w", err)
		}
		if existing != nil && existing.ID != sourceCardID {
			suggested, err := s.suggestEditTitle(ctx, userID, source.Year, requestedTitle)
			if err != nil {
				return nil, err
			}
			return nil, &CardConflictError{
				Year:           source.Year,
				Title:          requestedTitle,
				SuggestedTitle: suggested,
			}
		}
	}

	itemsToUpdate := make([]models.BingoItem, len(source.Items))
	copy(itemsToUpdate, source.Items)

	if params.ShuffleLayout {
		total := source.TotalSquares()
		positions := make([]int, 0, len(itemsToUpdate))
		for p := 0; p < total; p++ {
			if source.FreeSpacePos != nil && p == *source.FreeSpacePos {
				continue
			}
			positions = append(positions, p)
		}
		if len(itemsToUpdate) > len(positions) {
			return nil, fmt.Errorf("source card has %d items but only %d positions are available", len(itemsToUpdate), len(positions))
		}
		rand.Shuffle(len(positions), func(i, j int) {
			positions[i], positions[j] = positions[j], positions[i]
		})
		for i := range itemsToUpdate {
			itemsToUpdate[i].Position = positions[i]
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after commit

	updatedCard := &models.BingoCard{}
	err = tx.QueryRow(ctx,
		`UPDATE bingo_cards
		    SET title = $2,
		        is_finalized = false
		  WHERE id = $1
		 RETURNING id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		           is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at`,
		sourceCardID, title,
	).Scan(
		&updatedCard.ID, &updatedCard.UserID, &updatedCard.Year, &updatedCard.Category, &updatedCard.Title,
		&updatedCard.GridSize, &updatedCard.HeaderText, &updatedCard.HasFreeSpace, &updatedCard.FreeSpacePos,
		&updatedCard.IsActive, &updatedCard.IsFinalized, &updatedCard.VisibleToFriends, &updatedCard.IsArchived, &updatedCard.CreatedAt, &updatedCard.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if mapped := mapBingoCardsUniqueViolationToCardExistsError(pgErr, title); mapped != nil {
				return nil, mapped
			}
		}
		return nil, fmt.Errorf("updating finalized card: %w", err)
	}

	if params.ShuffleLayout {
		_, err = tx.Exec(ctx,
			`UPDATE bingo_items
			    SET position = position + $2
			  WHERE card_id = $1`,
			sourceCardID, source.TotalSquares(),
		)
		if err != nil {
			return nil, fmt.Errorf("shifting existing item positions: %w", err)
		}
	}

	for _, it := range itemsToUpdate {
		if params.ShuffleLayout && params.ResetProgress {
			_, err = tx.Exec(ctx,
				`UPDATE bingo_items
				    SET position = $3,
				        is_completed = false,
				        completed_at = NULL,
				        notes = NULL,
				        proof_url = NULL
				  WHERE id = $1 AND card_id = $2`,
				it.ID, sourceCardID, it.Position,
			)
		} else if params.ShuffleLayout {
			_, err = tx.Exec(ctx,
				`UPDATE bingo_items
				    SET position = $3
				  WHERE id = $1 AND card_id = $2`,
				it.ID, sourceCardID, it.Position,
			)
		}
		if err != nil {
			return nil, fmt.Errorf("updating item: %w", err)
		}
	}

	if params.ResetProgress && !params.ShuffleLayout {
		_, err = tx.Exec(ctx,
			`UPDATE bingo_items
			    SET is_completed = false,
			        completed_at = NULL,
			        notes = NULL,
			        proof_url = NULL
			  WHERE card_id = $1`,
			sourceCardID,
		)
		if err != nil {
			return nil, fmt.Errorf("resetting item progress: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return s.GetByID(ctx, sourceCardID)
}
