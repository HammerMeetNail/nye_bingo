package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AccountService struct {
	db DB
}

func NewAccountService(db DB) *AccountService {
	return &AccountService{db: db}
}

func (s *AccountService) BuildExportZip(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	var user struct {
		ID                    uuid.UUID
		Email                 string
		Username              string
		EmailVerified         bool
		EmailVerifiedAt       *time.Time
		AIFreeGenerationsUsed int
		Searchable            bool
		CreatedAt             time.Time
		UpdatedAt             time.Time
		DeletedAt             *time.Time
	}

	err := s.db.QueryRow(ctx,
		`SELECT id, email, username, email_verified, email_verified_at, ai_free_generations_used,
		        searchable, created_at, updated_at, deleted_at
		 FROM users
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.EmailVerified,
		&user.EmailVerifiedAt,
		&user.AIFreeGenerationsUsed,
		&user.Searchable,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load user for export: %w", err)
	}

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	if err := writeReadme(zipWriter, time.Now().UTC()); err != nil {
		return nil, err
	}

	if err := writeCSVFile(zipWriter, "user.csv", []string{
		"id",
		"email",
		"username",
		"email_verified",
		"email_verified_at",
		"ai_free_generations_used",
		"searchable",
		"created_at",
		"updated_at",
		"deleted_at",
	}, func(w *csv.Writer) error {
		return w.Write([]string{
			user.ID.String(),
			sanitizeCSVValue(user.Email),
			sanitizeCSVValue(user.Username),
			boolString(user.EmailVerified),
			formatTime(user.EmailVerifiedAt),
			fmt.Sprintf("%d", user.AIFreeGenerationsUsed),
			boolString(user.Searchable),
			formatTimeValue(user.CreatedAt),
			formatTimeValue(user.UpdatedAt),
			formatTime(user.DeletedAt),
		})
	}); err != nil {
		return nil, err
	}

	if err := s.writeCardsCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeItemsCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeFriendshipsCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeBlocksCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeAPITokensCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeNotificationSettingsCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeNotificationsCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeReminderSettingsCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeCardCheckinRemindersCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeGoalRemindersCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeReminderEmailLogCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeReminderImageTokensCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeReminderUnsubscribeTokensCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeEmailVerificationTokensCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writePasswordResetTokensCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeAIGenerationLogsCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeCardSharesCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeFriendInvitesCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}
	if err := s.writeSessionsCSV(ctx, zipWriter, userID); err != nil {
		return nil, err
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close export zip: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *AccountService) Delete(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin account delete: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	var email string
	if err := tx.QueryRow(ctx, "SELECT email FROM users WHERE id = $1 AND deleted_at IS NULL", userID).Scan(&email); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("load user email: %w", err)
		}
	}

	scrubEmail := fmt.Sprintf("deleted+%s@deleted.invalid", userID.String())
	scrubUsername := fmt.Sprintf("deleted-%s", userID.String())
	scrubPassword := fmt.Sprintf("deleted-%s", uuid.New().String())

	result, err := tx.Exec(ctx, `
		UPDATE users
		SET deleted_at = NOW(),
		    email = $2,
		    username = $3,
		    password_hash = $4,
		    email_verified = false,
		    email_verified_at = NULL,
		    searchable = false
		WHERE id = $1 AND deleted_at IS NULL
	`, userID, scrubEmail, scrubUsername, scrubPassword)
	if err != nil {
		return fmt.Errorf("soft delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		var deletedAt *time.Time
		err := tx.QueryRow(ctx, "SELECT deleted_at FROM users WHERE id = $1", userID).Scan(&deletedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("check deleted user: %w", err)
		}
		if deletedAt == nil {
			return fmt.Errorf("expected deleted account to be marked")
		}
	}

	if _, err := tx.Exec(ctx, "DELETE FROM api_tokens WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("revoke api tokens: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM email_verification_tokens WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("revoke email verification tokens: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM password_reset_tokens WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("revoke password reset tokens: %w", err)
	}
	if email != "" {
		if _, err := tx.Exec(ctx, "DELETE FROM magic_link_tokens WHERE email = $1", email); err != nil {
			return fmt.Errorf("revoke magic link tokens: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM bingo_card_shares
		WHERE card_id IN (SELECT id FROM bingo_cards WHERE user_id = $1)
	`, userID); err != nil {
		return fmt.Errorf("revoke card shares: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM reminder_image_tokens WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("revoke reminder image tokens: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM reminder_unsubscribe_tokens WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("revoke reminder unsubscribe tokens: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM sessions WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit account delete: %w", err)
	}
	committed = true
	return nil
}

func writeReadme(zipWriter *zip.Writer, generatedAt time.Time) error {
	file, err := zipWriter.Create("README.txt")
	if err != nil {
		return fmt.Errorf("create README.txt: %w", err)
	}
	content := fmt.Sprintf(
		"Year of Bingo account export\nexport_version: 1\ngenerated_at: %s\nnotes: token hashes and secret token values are excluded from this export.\n",
		generatedAt.Format(time.RFC3339),
	)
	if _, err := io.WriteString(file, content); err != nil {
		return fmt.Errorf("write README.txt: %w", err)
	}
	return nil
}

func writeCSVFile(zipWriter *zip.Writer, name string, header []string, writeRows func(*csv.Writer) error) error {
	file, err := zipWriter.Create(name)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	writer := csv.NewWriter(file)
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write %s header: %w", name, err)
	}
	if err := writeRows(writer); err != nil {
		return fmt.Errorf("write %s rows: %w", name, err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush %s: %w", name, err)
	}
	return nil
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatTimeValue(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func sanitizeCSVValue(value string) string {
	first := firstNonSpace(value)
	if first == 0 {
		return value
	}
	switch first {
	case '=', '+', '-', '@':
		return "'" + strings.ReplaceAll(value, "'", "''")
	case '\'':
		return value
	default:
		return value
	}
}

func firstNonSpace(value string) rune {
	for _, r := range value {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return r
		}
	}
	return 0
}

func (s *AccountService) writeCardsCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space,
		        free_space_position, is_active, is_finalized, visible_to_friends, is_archived,
		        created_at, updated_at
		 FROM bingo_cards
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query cards: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"year",
		"category",
		"title",
		"grid_size",
		"header_text",
		"has_free_space",
		"free_space_position",
		"is_active",
		"is_finalized",
		"visible_to_friends",
		"is_archived",
		"created_at",
		"updated_at",
	}

	return writeCSVFile(zipWriter, "cards.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				cardID           uuid.UUID
				ownerID          uuid.UUID
				year             int
				category         *string
				title            *string
				gridSize         int
				headerText       string
				hasFreeSpace     bool
				freeSpacePos     *int
				isActive         bool
				isFinalized      bool
				visibleToFriends bool
				isArchived       bool
				createdAt        time.Time
				updatedAt        time.Time
			)
			if err := rows.Scan(
				&cardID,
				&ownerID,
				&year,
				&category,
				&title,
				&gridSize,
				&headerText,
				&hasFreeSpace,
				&freeSpacePos,
				&isActive,
				&isFinalized,
				&visibleToFriends,
				&isArchived,
				&createdAt,
				&updatedAt,
			); err != nil {
				return fmt.Errorf("scan cards: %w", err)
			}
			if err := w.Write([]string{
				cardID.String(),
				ownerID.String(),
				fmt.Sprintf("%d", year),
				nullableString(category),
				nullableString(title),
				fmt.Sprintf("%d", gridSize),
				sanitizeCSVValue(headerText),
				boolString(hasFreeSpace),
				nullableInt(freeSpacePos),
				boolString(isActive),
				boolString(isFinalized),
				boolString(visibleToFriends),
				boolString(isArchived),
				formatTimeValue(createdAt),
				formatTimeValue(updatedAt),
			}); err != nil {
				return fmt.Errorf("write cards row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate cards: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeItemsCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT bi.id, bi.card_id, bi.position, bi.content, bi.is_completed, bi.completed_at,
		        bi.notes, bi.proof_url, bi.created_at
		 FROM bingo_items bi
		 JOIN bingo_cards bc ON bi.card_id = bc.id
		 WHERE bc.user_id = $1
		 ORDER BY bi.card_id, bi.position`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"card_id",
		"position",
		"content",
		"is_completed",
		"completed_at",
		"notes",
		"proof_url",
		"created_at",
	}

	return writeCSVFile(zipWriter, "items.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				itemID      uuid.UUID
				cardID      uuid.UUID
				position    int
				content     string
				isCompleted bool
				completedAt *time.Time
				notes       *string
				proofURL    *string
				createdAt   time.Time
			)
			if err := rows.Scan(
				&itemID,
				&cardID,
				&position,
				&content,
				&isCompleted,
				&completedAt,
				&notes,
				&proofURL,
				&createdAt,
			); err != nil {
				return fmt.Errorf("scan items: %w", err)
			}
			if err := w.Write([]string{
				itemID.String(),
				cardID.String(),
				fmt.Sprintf("%d", position),
				sanitizeCSVValue(content),
				boolString(isCompleted),
				formatTime(completedAt),
				nullableString(notes),
				nullableString(proofURL),
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write items row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate items: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeFriendshipsCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, friend_id, status, created_at
		 FROM friendships
		 WHERE user_id = $1 OR friend_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query friendships: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"friend_id",
		"status",
		"created_at",
	}

	return writeCSVFile(zipWriter, "friendships.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				friendshipID uuid.UUID
				userA        uuid.UUID
				userB        uuid.UUID
				status       string
				createdAt    time.Time
			)
			if err := rows.Scan(&friendshipID, &userA, &userB, &status, &createdAt); err != nil {
				return fmt.Errorf("scan friendships: %w", err)
			}
			if err := w.Write([]string{
				friendshipID.String(),
				userA.String(),
				userB.String(),
				status,
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write friendships row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate friendships: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeBlocksCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT blocker_id, blocked_id, created_at
		 FROM user_blocks
		 WHERE blocker_id = $1 OR blocked_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query blocks: %w", err)
	}
	defer rows.Close()

	header := []string{
		"blocker_id",
		"blocked_id",
		"created_at",
	}

	return writeCSVFile(zipWriter, "blocks.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				blockerID uuid.UUID
				blockedID uuid.UUID
				createdAt time.Time
			)
			if err := rows.Scan(&blockerID, &blockedID, &createdAt); err != nil {
				return fmt.Errorf("scan blocks: %w", err)
			}
			if err := w.Write([]string{
				blockerID.String(),
				blockedID.String(),
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write blocks row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate blocks: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeAPITokensCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, name, token_prefix, scope, expires_at, last_used_at, created_at
		 FROM api_tokens
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query api tokens: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"name",
		"token_prefix",
		"scope",
		"expires_at",
		"last_used_at",
		"created_at",
	}

	return writeCSVFile(zipWriter, "api_tokens.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				tokenID    uuid.UUID
				ownerID    uuid.UUID
				name       string
				prefix     string
				scope      string
				expiresAt  *time.Time
				lastUsedAt *time.Time
				createdAt  time.Time
			)
			if err := rows.Scan(&tokenID, &ownerID, &name, &prefix, &scope, &expiresAt, &lastUsedAt, &createdAt); err != nil {
				return fmt.Errorf("scan api tokens: %w", err)
			}
			if err := w.Write([]string{
				tokenID.String(),
				ownerID.String(),
				sanitizeCSVValue(name),
				prefix,
				scope,
				formatTime(expiresAt),
				formatTime(lastUsedAt),
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write api tokens row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate api tokens: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeNotificationSettingsCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT user_id, in_app_enabled, in_app_friend_request_received, in_app_friend_request_accepted,
		        in_app_friend_bingo, in_app_friend_new_card, email_enabled, email_friend_request_received,
		        email_friend_request_accepted, email_friend_bingo, email_friend_new_card, created_at, updated_at
		 FROM notification_settings
		 WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query notification settings: %w", err)
	}
	defer rows.Close()

	header := []string{
		"user_id",
		"in_app_enabled",
		"in_app_friend_request_received",
		"in_app_friend_request_accepted",
		"in_app_friend_bingo",
		"in_app_friend_new_card",
		"email_enabled",
		"email_friend_request_received",
		"email_friend_request_accepted",
		"email_friend_bingo",
		"email_friend_new_card",
		"created_at",
		"updated_at",
	}

	return writeCSVFile(zipWriter, "notification_settings.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				rowUserID                uuid.UUID
				inAppEnabled             bool
				inAppFriendRequestRec    bool
				inAppFriendRequestAccept bool
				inAppFriendBingo         bool
				inAppFriendNewCard       bool
				emailEnabled             bool
				emailFriendRequestRec    bool
				emailFriendRequestAccept bool
				emailFriendBingo         bool
				emailFriendNewCard       bool
				createdAt                time.Time
				updatedAt                time.Time
			)
			if err := rows.Scan(
				&rowUserID,
				&inAppEnabled,
				&inAppFriendRequestRec,
				&inAppFriendRequestAccept,
				&inAppFriendBingo,
				&inAppFriendNewCard,
				&emailEnabled,
				&emailFriendRequestRec,
				&emailFriendRequestAccept,
				&emailFriendBingo,
				&emailFriendNewCard,
				&createdAt,
				&updatedAt,
			); err != nil {
				return fmt.Errorf("scan notification settings: %w", err)
			}
			if err := w.Write([]string{
				rowUserID.String(),
				boolString(inAppEnabled),
				boolString(inAppFriendRequestRec),
				boolString(inAppFriendRequestAccept),
				boolString(inAppFriendBingo),
				boolString(inAppFriendNewCard),
				boolString(emailEnabled),
				boolString(emailFriendRequestRec),
				boolString(emailFriendRequestAccept),
				boolString(emailFriendBingo),
				boolString(emailFriendNewCard),
				formatTimeValue(createdAt),
				formatTimeValue(updatedAt),
			}); err != nil {
				return fmt.Errorf("write notification settings row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate notification settings: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeNotificationsCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, type, actor_user_id, friendship_id, card_id, bingo_count,
		        in_app_delivered, email_delivered, email_sent_at, read_at, created_at
		 FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"type",
		"actor_user_id",
		"friendship_id",
		"card_id",
		"bingo_count",
		"in_app_delivered",
		"email_delivered",
		"email_sent_at",
		"read_at",
		"created_at",
	}

	return writeCSVFile(zipWriter, "notifications.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				notificationID uuid.UUID
				rowUserID      uuid.UUID
				nType          string
				actorUserID    *uuid.UUID
				friendshipID   *uuid.UUID
				cardID         *uuid.UUID
				bingoCount     *int
				inAppDelivered bool
				emailDelivered bool
				emailSentAt    *time.Time
				readAt         *time.Time
				createdAt      time.Time
			)
			if err := rows.Scan(
				&notificationID,
				&rowUserID,
				&nType,
				&actorUserID,
				&friendshipID,
				&cardID,
				&bingoCount,
				&inAppDelivered,
				&emailDelivered,
				&emailSentAt,
				&readAt,
				&createdAt,
			); err != nil {
				return fmt.Errorf("scan notifications: %w", err)
			}
			if err := w.Write([]string{
				notificationID.String(),
				rowUserID.String(),
				nType,
				nullableUUID(actorUserID),
				nullableUUID(friendshipID),
				nullableUUID(cardID),
				nullableInt(bingoCount),
				boolString(inAppDelivered),
				boolString(emailDelivered),
				formatTime(emailSentAt),
				formatTime(readAt),
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write notifications row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate notifications: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeReminderSettingsCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT user_id, email_enabled, daily_email_cap, created_at, updated_at
		 FROM reminder_settings
		 WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query reminder settings: %w", err)
	}
	defer rows.Close()

	header := []string{
		"user_id",
		"email_enabled",
		"daily_email_cap",
		"created_at",
		"updated_at",
	}

	return writeCSVFile(zipWriter, "reminder_settings.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				rowUserID     uuid.UUID
				emailEnabled  bool
				dailyEmailCap int
				createdAt     time.Time
				updatedAt     time.Time
			)
			if err := rows.Scan(&rowUserID, &emailEnabled, &dailyEmailCap, &createdAt, &updatedAt); err != nil {
				return fmt.Errorf("scan reminder settings: %w", err)
			}
			if err := w.Write([]string{
				rowUserID.String(),
				boolString(emailEnabled),
				fmt.Sprintf("%d", dailyEmailCap),
				formatTimeValue(createdAt),
				formatTimeValue(updatedAt),
			}); err != nil {
				return fmt.Errorf("write reminder settings row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate reminder settings: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeCardCheckinRemindersCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, card_id, enabled, frequency, schedule, include_image, include_recommendations,
		        next_send_at, last_sent_at, created_at, updated_at
		 FROM card_checkin_reminders
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query card checkin reminders: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"card_id",
		"enabled",
		"frequency",
		"schedule",
		"include_image",
		"include_recommendations",
		"next_send_at",
		"last_sent_at",
		"created_at",
		"updated_at",
	}

	return writeCSVFile(zipWriter, "card_checkin_reminders.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				reminderID             uuid.UUID
				rowUserID              uuid.UUID
				cardID                 uuid.UUID
				enabled                bool
				frequency              string
				schedule               []byte
				includeImage           bool
				includeRecommendations bool
				nextSendAt             *time.Time
				lastSentAt             *time.Time
				createdAt              time.Time
				updatedAt              time.Time
			)
			if err := rows.Scan(
				&reminderID,
				&rowUserID,
				&cardID,
				&enabled,
				&frequency,
				&schedule,
				&includeImage,
				&includeRecommendations,
				&nextSendAt,
				&lastSentAt,
				&createdAt,
				&updatedAt,
			); err != nil {
				return fmt.Errorf("scan card checkin reminders: %w", err)
			}
			if err := w.Write([]string{
				reminderID.String(),
				rowUserID.String(),
				cardID.String(),
				boolString(enabled),
				frequency,
				string(schedule),
				boolString(includeImage),
				boolString(includeRecommendations),
				formatTime(nextSendAt),
				formatTime(lastSentAt),
				formatTimeValue(createdAt),
				formatTimeValue(updatedAt),
			}); err != nil {
				return fmt.Errorf("write card checkin reminders row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate card checkin reminders: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeGoalRemindersCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, card_id, item_id, enabled, kind, schedule, next_send_at,
		        last_sent_at, created_at, updated_at
		 FROM goal_reminders
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query goal reminders: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"card_id",
		"item_id",
		"enabled",
		"kind",
		"schedule",
		"next_send_at",
		"last_sent_at",
		"created_at",
		"updated_at",
	}

	return writeCSVFile(zipWriter, "goal_reminders.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				reminderID uuid.UUID
				rowUserID  uuid.UUID
				cardID     uuid.UUID
				itemID     uuid.UUID
				enabled    bool
				kind       string
				schedule   []byte
				nextSendAt *time.Time
				lastSentAt *time.Time
				createdAt  time.Time
				updatedAt  time.Time
			)
			if err := rows.Scan(
				&reminderID,
				&rowUserID,
				&cardID,
				&itemID,
				&enabled,
				&kind,
				&schedule,
				&nextSendAt,
				&lastSentAt,
				&createdAt,
				&updatedAt,
			); err != nil {
				return fmt.Errorf("scan goal reminders: %w", err)
			}
			if err := w.Write([]string{
				reminderID.String(),
				rowUserID.String(),
				cardID.String(),
				itemID.String(),
				boolString(enabled),
				kind,
				string(schedule),
				formatTime(nextSendAt),
				formatTime(lastSentAt),
				formatTimeValue(createdAt),
				formatTimeValue(updatedAt),
			}); err != nil {
				return fmt.Errorf("write goal reminders row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate goal reminders: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeReminderEmailLogCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, source_type, source_id, sent_at, sent_on, provider_message_id, status
		 FROM reminder_email_log
		 WHERE user_id = $1
		 ORDER BY sent_at DESC`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query reminder email log: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"source_type",
		"source_id",
		"sent_at",
		"sent_on",
		"provider_message_id",
		"status",
	}

	return writeCSVFile(zipWriter, "reminder_email_log.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				logID             uuid.UUID
				rowUserID         uuid.UUID
				sourceType        string
				sourceID          uuid.UUID
				sentAt            time.Time
				sentOn            time.Time
				providerMessageID *string
				status            string
			)
			if err := rows.Scan(
				&logID,
				&rowUserID,
				&sourceType,
				&sourceID,
				&sentAt,
				&sentOn,
				&providerMessageID,
				&status,
			); err != nil {
				return fmt.Errorf("scan reminder email log: %w", err)
			}
			if err := w.Write([]string{
				logID.String(),
				rowUserID.String(),
				sourceType,
				sourceID.String(),
				formatTimeValue(sentAt),
				sentOn.UTC().Format("2006-01-02"),
				nullableString(providerMessageID),
				status,
			}); err != nil {
				return fmt.Errorf("write reminder email log row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate reminder email log: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeReminderImageTokensCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT user_id, card_id, show_completions, expires_at, created_at, last_accessed_at, access_count
		 FROM reminder_image_tokens
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query reminder image tokens: %w", err)
	}
	defer rows.Close()

	header := []string{
		"user_id",
		"card_id",
		"show_completions",
		"expires_at",
		"created_at",
		"last_accessed_at",
		"access_count",
	}

	return writeCSVFile(zipWriter, "reminder_image_tokens.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				rowUserID      uuid.UUID
				cardID         uuid.UUID
				showCompletion bool
				expiresAt      time.Time
				createdAt      time.Time
				lastAccessedAt *time.Time
				accessCount    int
			)
			if err := rows.Scan(
				&rowUserID,
				&cardID,
				&showCompletion,
				&expiresAt,
				&createdAt,
				&lastAccessedAt,
				&accessCount,
			); err != nil {
				return fmt.Errorf("scan reminder image tokens: %w", err)
			}
			if err := w.Write([]string{
				rowUserID.String(),
				cardID.String(),
				boolString(showCompletion),
				formatTimeValue(expiresAt),
				formatTimeValue(createdAt),
				formatTime(lastAccessedAt),
				fmt.Sprintf("%d", accessCount),
			}); err != nil {
				return fmt.Errorf("write reminder image tokens row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate reminder image tokens: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeReminderUnsubscribeTokensCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT user_id, expires_at, created_at, used_at
		 FROM reminder_unsubscribe_tokens
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query reminder unsubscribe tokens: %w", err)
	}
	defer rows.Close()

	header := []string{
		"user_id",
		"expires_at",
		"created_at",
		"used_at",
	}

	return writeCSVFile(zipWriter, "reminder_unsubscribe_tokens.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				rowUserID uuid.UUID
				expiresAt time.Time
				createdAt time.Time
				usedAt    *time.Time
			)
			if err := rows.Scan(&rowUserID, &expiresAt, &createdAt, &usedAt); err != nil {
				return fmt.Errorf("scan reminder unsubscribe tokens: %w", err)
			}
			if err := w.Write([]string{
				rowUserID.String(),
				formatTimeValue(expiresAt),
				formatTimeValue(createdAt),
				formatTime(usedAt),
			}); err != nil {
				return fmt.Errorf("write reminder unsubscribe tokens row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate reminder unsubscribe tokens: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeEmailVerificationTokensCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, expires_at, created_at
		 FROM email_verification_tokens
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query email verification tokens: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"expires_at",
		"created_at",
	}

	return writeCSVFile(zipWriter, "email_verification_tokens.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				tokenID   uuid.UUID
				rowUserID uuid.UUID
				expiresAt time.Time
				createdAt time.Time
			)
			if err := rows.Scan(&tokenID, &rowUserID, &expiresAt, &createdAt); err != nil {
				return fmt.Errorf("scan email verification tokens: %w", err)
			}
			if err := w.Write([]string{
				tokenID.String(),
				rowUserID.String(),
				formatTimeValue(expiresAt),
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write email verification tokens row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate email verification tokens: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writePasswordResetTokensCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, expires_at, used_at, created_at
		 FROM password_reset_tokens
		 WHERE user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query password reset tokens: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"expires_at",
		"used_at",
		"created_at",
	}

	return writeCSVFile(zipWriter, "password_reset_tokens.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				tokenID   uuid.UUID
				rowUserID uuid.UUID
				expiresAt time.Time
				usedAt    *time.Time
				createdAt time.Time
			)
			if err := rows.Scan(&tokenID, &rowUserID, &expiresAt, &usedAt, &createdAt); err != nil {
				return fmt.Errorf("scan password reset tokens: %w", err)
			}
			if err := w.Write([]string{
				tokenID.String(),
				rowUserID.String(),
				formatTimeValue(expiresAt),
				formatTime(usedAt),
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write password reset tokens row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate password reset tokens: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeAIGenerationLogsCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, model, tokens_input, tokens_output, duration_ms, status, created_at
		 FROM ai_generation_logs
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query ai generation logs: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"model",
		"tokens_input",
		"tokens_output",
		"duration_ms",
		"status",
		"created_at",
	}

	return writeCSVFile(zipWriter, "ai_generation_logs.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				logID       uuid.UUID
				rowUserID   uuid.UUID
				model       string
				tokensInput int
				tokensOut   int
				durationMs  int
				status      string
				createdAt   time.Time
			)
			if err := rows.Scan(&logID, &rowUserID, &model, &tokensInput, &tokensOut, &durationMs, &status, &createdAt); err != nil {
				return fmt.Errorf("scan ai generation logs: %w", err)
			}
			if err := w.Write([]string{
				logID.String(),
				rowUserID.String(),
				model,
				fmt.Sprintf("%d", tokensInput),
				fmt.Sprintf("%d", tokensOut),
				fmt.Sprintf("%d", durationMs),
				status,
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write ai generation logs row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate ai generation logs: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeCardSharesCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT s.card_id, s.created_at, s.expires_at, s.last_accessed_at, s.access_count
		 FROM bingo_card_shares s
		 JOIN bingo_cards c ON s.card_id = c.id
		 WHERE c.user_id = $1
		 ORDER BY s.created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query card shares: %w", err)
	}
	defer rows.Close()

	header := []string{
		"card_id",
		"created_at",
		"expires_at",
		"last_accessed_at",
		"access_count",
	}

	return writeCSVFile(zipWriter, "card_shares.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				cardID         uuid.UUID
				createdAt      time.Time
				expiresAt      *time.Time
				lastAccessedAt *time.Time
				accessCount    int
			)
			if err := rows.Scan(&cardID, &createdAt, &expiresAt, &lastAccessedAt, &accessCount); err != nil {
				return fmt.Errorf("scan card shares: %w", err)
			}
			if err := w.Write([]string{
				cardID.String(),
				formatTimeValue(createdAt),
				formatTime(expiresAt),
				formatTime(lastAccessedAt),
				fmt.Sprintf("%d", accessCount),
			}); err != nil {
				return fmt.Errorf("write card shares row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate card shares: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeFriendInvitesCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, inviter_user_id, expires_at, revoked_at, accepted_by_user_id, accepted_at, created_at
		 FROM friend_invites
		 WHERE inviter_user_id = $1 OR accepted_by_user_id = $1
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query friend invites: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"inviter_user_id",
		"expires_at",
		"revoked_at",
		"accepted_by_user_id",
		"accepted_at",
		"created_at",
	}

	return writeCSVFile(zipWriter, "friend_invites.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				inviteID         uuid.UUID
				inviterID        uuid.UUID
				expiresAt        *time.Time
				revokedAt        *time.Time
				acceptedByUserID *uuid.UUID
				acceptedAt       *time.Time
				createdAt        time.Time
			)
			if err := rows.Scan(&inviteID, &inviterID, &expiresAt, &revokedAt, &acceptedByUserID, &acceptedAt, &createdAt); err != nil {
				return fmt.Errorf("scan friend invites: %w", err)
			}
			if err := w.Write([]string{
				inviteID.String(),
				inviterID.String(),
				formatTime(expiresAt),
				formatTime(revokedAt),
				nullableUUID(acceptedByUserID),
				formatTime(acceptedAt),
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write friend invites row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate friend invites: %w", err)
		}
		return nil
	})
}

func (s *AccountService) writeSessionsCSV(ctx context.Context, zipWriter *zip.Writer, userID uuid.UUID) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, expires_at, created_at
		 FROM sessions
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	header := []string{
		"id",
		"user_id",
		"expires_at",
		"created_at",
	}

	return writeCSVFile(zipWriter, "sessions.csv", header, func(w *csv.Writer) error {
		for rows.Next() {
			var (
				sessionID uuid.UUID
				rowUserID uuid.UUID
				expiresAt time.Time
				createdAt time.Time
			)
			if err := rows.Scan(&sessionID, &rowUserID, &expiresAt, &createdAt); err != nil {
				return fmt.Errorf("scan sessions: %w", err)
			}
			if err := w.Write([]string{
				sessionID.String(),
				rowUserID.String(),
				formatTimeValue(expiresAt),
				formatTimeValue(createdAt),
			}); err != nil {
				return fmt.Errorf("write sessions row: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate sessions: %w", err)
		}
		return nil
	})
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return sanitizeCSVValue(*value)
}

func nullableInt(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func nullableUUID(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}
