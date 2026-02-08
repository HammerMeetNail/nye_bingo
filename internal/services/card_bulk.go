package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func (s *CardService) UpdateVisibility(ctx context.Context, userID, cardID uuid.UUID, visibleToFriends bool) (*models.BingoCard, error) {
	// Get and verify card ownership
	card, err := s.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}

	wasVisible := card.VisibleToFriends
	_, err = s.db.Exec(ctx,
		"UPDATE bingo_cards SET visible_to_friends = $2, updated_at = NOW() WHERE id = $1",
		cardID, visibleToFriends,
	)
	if err != nil {
		return nil, fmt.Errorf("updating visibility: %w", err)
	}

	card.VisibleToFriends = visibleToFriends
	if card.IsFinalized && !wasVisible && visibleToFriends {
		s.notifyFriendsNewCard(ctx, userID, cardID)
	}
	return card, nil
}

// BulkUpdateVisibility updates the visibility of multiple cards owned by the user
// Returns the count of cards updated (cards not owned by user are silently skipped)
func (s *CardService) BulkUpdateVisibility(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, visibleToFriends bool) (int, error) {
	if len(cardIDs) == 0 {
		return 0, nil
	}

	var notifyCardIDs []uuid.UUID
	if visibleToFriends {
		rows, err := s.db.Query(ctx,
			`SELECT id FROM bingo_cards
			 WHERE id = ANY($1) AND user_id = $2 AND is_finalized = true AND visible_to_friends = false`,
			cardIDs, userID,
		)
		if err != nil {
			return 0, fmt.Errorf("fetching visibility changes: %w", err)
		}
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return 0, fmt.Errorf("scanning visibility changes: %w", err)
			}
			notifyCardIDs = append(notifyCardIDs, id)
		}
		rows.Close()
	}

	result, err := s.db.Exec(ctx,
		`UPDATE bingo_cards SET visible_to_friends = $1, updated_at = NOW()
		 WHERE id = ANY($2) AND user_id = $3`,
		visibleToFriends, cardIDs, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk updating visibility: %w", err)
	}

	if visibleToFriends {
		for _, cardID := range notifyCardIDs {
			s.notifyFriendsNewCard(ctx, userID, cardID)
		}
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
