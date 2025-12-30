package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrCannotBlockSelf     = errors.New("cannot block yourself")
	ErrBlockExists         = errors.New("user is already blocked")
	ErrBlockNotFound       = errors.New("block not found")
	ErrBlockedUserNotFound = errors.New("blocked user not found")
)

type BlockService struct {
	db DB
}

func NewBlockService(db DB) *BlockService {
	return &BlockService{db: db}
}

func (s *BlockService) Block(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	if blockerID == blockedID {
		return ErrCannotBlockSelf
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin block transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := lockUserPairForUpdate(ctx, tx, blockerID, blockedID); errors.Is(err, pgx.ErrNoRows) {
		return ErrBlockedUserNotFound
	} else if err != nil {
		return fmt.Errorf("lock users: %w", err)
	}

	result, err := tx.Exec(ctx,
		`INSERT INTO user_blocks (blocker_id, blocked_id)
		 VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		blockerID, blockedID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrBlockedUserNotFound
		}
		return fmt.Errorf("insert block: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrBlockExists
	}

	_, err = tx.Exec(ctx,
		`DELETE FROM friendships
		 WHERE (user_id = $1 AND friend_id = $2)
		    OR (user_id = $2 AND friend_id = $1)`,
		blockerID, blockedID,
	)
	if err != nil {
		return fmt.Errorf("remove friendships: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit block: %w", err)
	}
	committed = true
	return nil
}

func (s *BlockService) Unblock(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	result, err := s.db.Exec(ctx,
		"DELETE FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2",
		blockerID, blockedID,
	)
	if err != nil {
		return fmt.Errorf("delete block: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrBlockNotFound
	}
	return nil
}

func (s *BlockService) IsBlocked(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error) {
	var blocked bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE (blocker_id = $1 AND blocked_id = $2)
			   OR (blocker_id = $2 AND blocked_id = $1)
		)`,
		userID, otherUserID,
	).Scan(&blocked)
	if err != nil {
		return false, fmt.Errorf("check block status: %w", err)
	}
	return blocked, nil
}

func (s *BlockService) ListBlocked(ctx context.Context, blockerID uuid.UUID) ([]models.BlockedUser, error) {
	rows, err := s.db.Query(ctx,
		`SELECT u.id, u.username, ub.created_at
		 FROM user_blocks ub
		 JOIN users u ON ub.blocked_id = u.id
		 WHERE ub.blocker_id = $1
		 ORDER BY u.username`,
		blockerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list blocked users: %w", err)
	}
	defer rows.Close()

	var blocked []models.BlockedUser
	for rows.Next() {
		var u models.BlockedUser
		if err := rows.Scan(&u.ID, &u.Username, &u.BlockedAt); err != nil {
			return nil, fmt.Errorf("scan blocked user: %w", err)
		}
		blocked = append(blocked, u)
	}
	if blocked == nil {
		blocked = []models.BlockedUser{}
	}
	return blocked, nil
}
