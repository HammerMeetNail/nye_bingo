package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func lockUserPairForUpdate(ctx context.Context, q DBConn, userA, userB uuid.UUID) error {
	first := userA
	second := userB

	if bytes.Compare(first[:], second[:]) > 0 {
		first, second = second, first
	}

	if err := lockUserForUpdate(ctx, q, first); err != nil {
		return err
	}
	if first == second {
		return nil
	}
	if err := lockUserForUpdate(ctx, q, second); err != nil {
		return err
	}
	return nil
}

func lockUserForUpdate(ctx context.Context, q DBConn, userID uuid.UUID) error {
	var lockedID uuid.UUID
	err := q.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	return nil
}
