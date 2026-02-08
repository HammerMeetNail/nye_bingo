package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

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

	if card.VisibleToFriends {
		updatedItems := make([]models.BingoItem, len(card.Items))
		copy(updatedItems, card.Items)
		for i := range updatedItems {
			if updatedItems[i].Position == position {
				updatedItems[i].IsCompleted = true
				updatedItems[i].CompletedAt = &now
				updatedItems[i].Notes = params.Notes
				updatedItems[i].ProofURL = params.ProofURL
				break
			}
		}
		var freePos *int
		if card.HasFreePositionSet() {
			freePos = card.FreeSpacePos
		}
		bingos := s.countBingos(updatedItems, card.GridSize, freePos)
		if bingos > 0 {
			s.notifyFriendsBingo(ctx, userID, cardID, bingos)
		}
	}

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
