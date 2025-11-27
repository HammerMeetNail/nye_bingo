package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HammerMeetNail/nye_bingo/internal/models"
)

var (
	ErrReactionNotFound = errors.New("reaction not found")
	ErrInvalidEmoji     = errors.New("invalid emoji")
	ErrCannotReactToOwn = errors.New("cannot react to your own items")
	ErrItemNotCompleted = errors.New("can only react to completed items")
)

type ReactionService struct {
	db            *pgxpool.Pool
	friendService *FriendService
}

func NewReactionService(db *pgxpool.Pool, friendService *FriendService) *ReactionService {
	return &ReactionService{
		db:            db,
		friendService: friendService,
	}
}

func (s *ReactionService) AddReaction(ctx context.Context, userID, itemID uuid.UUID, emoji string) (*models.Reaction, error) {
	// Validate emoji
	if !isValidEmoji(emoji) {
		return nil, ErrInvalidEmoji
	}

	// Get the item and its card to check ownership and completion
	var cardUserID uuid.UUID
	var isCompleted bool
	err := s.db.QueryRow(ctx,
		`SELECT bc.user_id, bi.is_completed
		 FROM bingo_items bi
		 JOIN bingo_cards bc ON bi.card_id = bc.id
		 WHERE bi.id = $1`,
		itemID,
	).Scan(&cardUserID, &isCompleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting item info: %w", err)
	}

	// Cannot react to own items
	if cardUserID == userID {
		return nil, ErrCannotReactToOwn
	}

	// Can only react to completed items
	if !isCompleted {
		return nil, ErrItemNotCompleted
	}

	// Check if users are friends
	isFriend, err := s.friendService.IsFriend(ctx, userID, cardUserID)
	if err != nil {
		return nil, err
	}
	if !isFriend {
		return nil, ErrNotFriend
	}

	// Upsert the reaction (insert or update if exists)
	reaction := &models.Reaction{}
	err = s.db.QueryRow(ctx,
		`INSERT INTO reactions (item_id, user_id, emoji)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (item_id, user_id)
		 DO UPDATE SET emoji = $3
		 RETURNING id, item_id, user_id, emoji, created_at`,
		itemID, userID, emoji,
	).Scan(&reaction.ID, &reaction.ItemID, &reaction.UserID, &reaction.Emoji, &reaction.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("adding reaction: %w", err)
	}

	return reaction, nil
}

func (s *ReactionService) RemoveReaction(ctx context.Context, userID, itemID uuid.UUID) error {
	result, err := s.db.Exec(ctx,
		"DELETE FROM reactions WHERE item_id = $1 AND user_id = $2",
		itemID, userID,
	)
	if err != nil {
		return fmt.Errorf("removing reaction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrReactionNotFound
	}
	return nil
}

func (s *ReactionService) GetReactionsForItem(ctx context.Context, itemID uuid.UUID) ([]models.ReactionWithUser, error) {
	rows, err := s.db.Query(ctx,
		`SELECT r.id, r.item_id, r.user_id, r.emoji, r.created_at, u.display_name
		 FROM reactions r
		 JOIN users u ON r.user_id = u.id
		 WHERE r.item_id = $1
		 ORDER BY r.created_at`,
		itemID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting reactions: %w", err)
	}
	defer rows.Close()

	var reactions []models.ReactionWithUser
	for rows.Next() {
		var r models.ReactionWithUser
		if err := rows.Scan(&r.ID, &r.ItemID, &r.UserID, &r.Emoji, &r.CreatedAt, &r.UserDisplayName); err != nil {
			return nil, fmt.Errorf("scanning reaction: %w", err)
		}
		reactions = append(reactions, r)
	}

	if reactions == nil {
		reactions = []models.ReactionWithUser{}
	}

	return reactions, nil
}

func (s *ReactionService) GetReactionSummaryForItem(ctx context.Context, itemID uuid.UUID) ([]models.ReactionSummary, error) {
	rows, err := s.db.Query(ctx,
		`SELECT emoji, COUNT(*) as count
		 FROM reactions
		 WHERE item_id = $1
		 GROUP BY emoji
		 ORDER BY count DESC`,
		itemID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting reaction summary: %w", err)
	}
	defer rows.Close()

	var summaries []models.ReactionSummary
	for rows.Next() {
		var summary models.ReactionSummary
		if err := rows.Scan(&summary.Emoji, &summary.Count); err != nil {
			return nil, fmt.Errorf("scanning summary: %w", err)
		}
		summaries = append(summaries, summary)
	}

	if summaries == nil {
		summaries = []models.ReactionSummary{}
	}

	return summaries, nil
}

func (s *ReactionService) GetReactionsForCard(ctx context.Context, cardID uuid.UUID) (map[uuid.UUID][]models.ReactionWithUser, error) {
	rows, err := s.db.Query(ctx,
		`SELECT r.id, r.item_id, r.user_id, r.emoji, r.created_at, u.display_name
		 FROM reactions r
		 JOIN users u ON r.user_id = u.id
		 JOIN bingo_items bi ON r.item_id = bi.id
		 WHERE bi.card_id = $1
		 ORDER BY r.item_id, r.created_at`,
		cardID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting card reactions: %w", err)
	}
	defer rows.Close()

	reactions := make(map[uuid.UUID][]models.ReactionWithUser)
	for rows.Next() {
		var r models.ReactionWithUser
		if err := rows.Scan(&r.ID, &r.ItemID, &r.UserID, &r.Emoji, &r.CreatedAt, &r.UserDisplayName); err != nil {
			return nil, fmt.Errorf("scanning reaction: %w", err)
		}
		reactions[r.ItemID] = append(reactions[r.ItemID], r)
	}

	return reactions, nil
}

func (s *ReactionService) GetUserReactionForItem(ctx context.Context, userID, itemID uuid.UUID) (*models.Reaction, error) {
	reaction := &models.Reaction{}
	err := s.db.QueryRow(ctx,
		`SELECT id, item_id, user_id, emoji, created_at
		 FROM reactions
		 WHERE item_id = $1 AND user_id = $2`,
		itemID, userID,
	).Scan(&reaction.ID, &reaction.ItemID, &reaction.UserID, &reaction.Emoji, &reaction.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting user reaction: %w", err)
	}
	return reaction, nil
}

func isValidEmoji(emoji string) bool {
	for _, e := range models.AllowedEmojis {
		if e == emoji {
			return true
		}
	}
	return false
}
