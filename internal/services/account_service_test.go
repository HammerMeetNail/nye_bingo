package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestAccountService_BuildExportZip_CreatesFiles(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	verifiedAt := now

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if !strings.Contains(sql, "FROM users") {
				return fakeRow{scanFunc: func(dest ...any) error { return errors.New("unexpected query") }}
			}
			return rowFromValues(
				userID,
				"test@example.com",
				"testuser",
				true,
				&verifiedAt,
				2,
				true,
				now,
				now,
				nil,
			)
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{}, nil
		},
	}

	service := NewAccountService(db)
	data, err := service.BuildExportZip(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to read zip: %v", err)
	}

	expectedFiles := map[string]bool{
		"README.txt":                      false,
		"user.csv":                        false,
		"cards.csv":                       false,
		"items.csv":                       false,
		"friendships.csv":                 false,
		"blocks.csv":                      false,
		"api_tokens.csv":                  false,
		"notification_settings.csv":       false,
		"notifications.csv":               false,
		"reminder_settings.csv":           false,
		"card_checkin_reminders.csv":      false,
		"goal_reminders.csv":              false,
		"reminder_email_log.csv":          false,
		"reminder_image_tokens.csv":       false,
		"reminder_unsubscribe_tokens.csv": false,
		"email_verification_tokens.csv":   false,
		"password_reset_tokens.csv":       false,
		"ai_generation_logs.csv":          false,
		"card_shares.csv":                 false,
		"friend_invites.csv":              false,
		"sessions.csv":                    false,
	}

	var apiTokensHeader string
	var sessionsHeader string
	var friendInvitesHeader string
	var userCSV string

	for _, f := range reader.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content, _ := io.ReadAll(rc)
		_ = rc.Close()
		if f.Name == "api_tokens.csv" {
			apiTokensHeader = firstLine(content)
		}
		if f.Name == "sessions.csv" {
			sessionsHeader = firstLine(content)
		}
		if f.Name == "friend_invites.csv" {
			friendInvitesHeader = firstLine(content)
		}
		if f.Name == "user.csv" {
			userCSV = string(content)
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Fatalf("expected %s in export", name)
		}
	}

	if !strings.Contains(userCSV, "test@example.com") {
		t.Fatalf("expected user email in user.csv, got %q", userCSV)
	}
	if strings.Contains(apiTokensHeader, "token_hash") {
		t.Fatalf("expected api_tokens.csv to omit token_hash, got %q", apiTokensHeader)
	}
	if strings.Contains(sessionsHeader, "token_hash") {
		t.Fatalf("expected sessions.csv to omit token_hash, got %q", sessionsHeader)
	}
	if strings.Contains(friendInvitesHeader, "invite_token_hash") {
		t.Fatalf("expected friend_invites.csv to omit invite_token_hash, got %q", friendInvitesHeader)
	}
}

func TestAccountService_BuildExportZip_UserNotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	service := NewAccountService(db)
	_, err := service.BuildExportZip(context.Background(), uuid.New())
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestAccountService_Delete_Success(t *testing.T) {
	var execSQL []string
	var committed bool
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execSQL = append(execSQL, sql)
			if strings.Contains(sql, "UPDATE users") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT email FROM users") {
				return rowFromValues("test@example.com")
			}
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("unexpected query")
			}}
		},
		CommitFunc: func(ctx context.Context) error {
			committed = true
			return nil
		},
	}

	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}

	service := NewAccountService(db)
	if err := service.Delete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !committed {
		t.Fatal("expected transaction commit")
	}
	if !containsSQL(execSQL, "DELETE FROM email_verification_tokens") {
		t.Fatal("expected email verification tokens to be revoked")
	}
	if !containsSQL(execSQL, "DELETE FROM password_reset_tokens") {
		t.Fatal("expected password reset tokens to be revoked")
	}
	if !containsSQL(execSQL, "DELETE FROM magic_link_tokens") {
		t.Fatal("expected magic link tokens to be revoked")
	}
}

func TestAccountService_Delete_Idempotent(t *testing.T) {
	deletedAt := time.Now()
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "UPDATE users") {
				return fakeCommandTag{rowsAffected: 0}, nil
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT email FROM users") {
				return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			}
			if strings.Contains(sql, "SELECT deleted_at FROM users") {
				return rowFromValues(&deletedAt)
			}
			return fakeRow{scanFunc: func(dest ...any) error { return errors.New("unexpected query") }}
		},
	}

	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}

	service := NewAccountService(db)
	if err := service.Delete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAccountService_Delete_NotFound(t *testing.T) {
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "UPDATE users") {
				return fakeCommandTag{rowsAffected: 0}, nil
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT email FROM users") {
				return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			}
			if strings.Contains(sql, "SELECT deleted_at FROM users") {
				return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			}
			return fakeRow{scanFunc: func(dest ...any) error { return errors.New("unexpected query") }}
		},
	}

	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}

	service := NewAccountService(db)
	if err := service.Delete(context.Background(), uuid.New()); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestAccountService_Writers_WriteRowsCoverNullables(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	title := "Title,\nWith Newline"
	category := "Category"
	freePos := 12
	notes := "note"
	proofURL := "https://example.com"
	actorID := uuid.New()
	friendshipID := uuid.New()
	cardID := uuid.New()
	bingoCount := 2
	expiresAt := now.Add(24 * time.Hour)
	checkinID := uuid.New()
	goalReminderID := uuid.New()
	itemID := uuid.New()
	cardShareCreated := now.Add(-2 * time.Hour)
	emailLogID := uuid.New()
	providerMessageID := "provider-msg"
	passwordResetID := uuid.New()
	emailVerificationID := uuid.New()
	aiLogID := uuid.New()

	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			switch {
			case strings.Contains(sql, "FROM bingo_cards"):
				return &fakeRows{rows: [][]any{{
					cardID, userID, 2025, &category, &title, 5, "BINGO", true, &freePos, true, true, true, false, now, now,
				}}}, nil
			case strings.Contains(sql, "FROM bingo_items"):
				itemID := uuid.New()
				completedAt := now.Add(-time.Hour)
				return &fakeRows{rows: [][]any{{
					itemID, cardID, 3, "Do something", true, &completedAt, &notes, &proofURL, now,
				}}}, nil
			case strings.Contains(sql, "FROM friendships"):
				friendID := uuid.New()
				return &fakeRows{rows: [][]any{{
					friendshipID, userID, friendID, "accepted", now,
				}}}, nil
			case strings.Contains(sql, "FROM user_blocks"):
				blocked := uuid.New()
				return &fakeRows{rows: [][]any{{
					userID, blocked, now,
				}}}, nil
			case strings.Contains(sql, "FROM notifications"):
				notificationID := uuid.New()
				emailSentAt := now.Add(-time.Minute)
				readAt := now.Add(-time.Second)
				return &fakeRows{rows: [][]any{{
					notificationID, userID, "friend_bingo", &actorID, &friendshipID, &cardID, &bingoCount, true, true, &emailSentAt, &readAt, now,
				}}}, nil
			case strings.Contains(sql, "FROM api_tokens"):
				tokenID := uuid.New()
				lastUsed := now.Add(-time.Hour)
				return &fakeRows{rows: [][]any{{
					tokenID, userID, "My Token", "pref", "read", &expiresAt, &lastUsed, now,
				}}}, nil
			case strings.Contains(sql, "FROM notification_settings"):
				return &fakeRows{rows: [][]any{{
					userID,
					true, true, true, true, true,
					true, true, true, true, true,
					now, now,
				}}}, nil
			case strings.Contains(sql, "FROM friend_invites"):
				inviteID := uuid.New()
				acceptedBy := uuid.New()
				acceptedAt := now.Add(-2 * time.Hour)
				return &fakeRows{rows: [][]any{{
					inviteID, userID, &expiresAt, nil, &acceptedBy, &acceptedAt, now,
				}}}, nil
			case strings.Contains(sql, "FROM sessions"):
				sessionID := uuid.New()
				return &fakeRows{rows: [][]any{{
					sessionID, userID, expiresAt, now,
				}}}, nil
			case strings.Contains(sql, "FROM reminder_settings"):
				return &fakeRows{rows: [][]any{{
					userID, true, 3, now, now,
				}}}, nil
			case strings.Contains(sql, "FROM card_checkin_reminders"):
				nextSendAt := now.Add(48 * time.Hour)
				return &fakeRows{rows: [][]any{{
					checkinID,
					userID,
					cardID,
					true,
					"monthly",
					[]byte(`{"day_of_month":28,"time":"09:00"}`),
					true,
					true,
					&nextSendAt,
					nil,
					now,
					now,
				}}}, nil
			case strings.Contains(sql, "FROM goal_reminders"):
				nextSendAt := now.Add(72 * time.Hour)
				return &fakeRows{rows: [][]any{{
					goalReminderID,
					userID,
					cardID,
					itemID,
					true,
					"one_time",
					[]byte(`{"send_at":"2030-01-02T15:04:05Z"}`),
					&nextSendAt,
					nil,
					now,
					now,
				}}}, nil
			case strings.Contains(sql, "FROM reminder_email_log"):
				sentAt := now.Add(-30 * time.Minute)
				sentOn := time.Date(sentAt.Year(), sentAt.Month(), sentAt.Day(), 0, 0, 0, 0, sentAt.Location())
				return &fakeRows{rows: [][]any{{
					emailLogID,
					userID,
					"card_checkin",
					checkinID,
					sentAt,
					sentOn,
					&providerMessageID,
					"sent",
				}}}, nil
			case strings.Contains(sql, "FROM reminder_image_tokens"):
				lastAccessedAt := now.Add(-time.Hour)
				return &fakeRows{rows: [][]any{{
					userID,
					cardID,
					true,
					expiresAt,
					now,
					&lastAccessedAt,
					2,
				}}}, nil
			case strings.Contains(sql, "FROM reminder_unsubscribe_tokens"):
				usedAt := now.Add(-time.Minute)
				return &fakeRows{rows: [][]any{{
					userID,
					expiresAt,
					now,
					&usedAt,
				}}}, nil
			case strings.Contains(sql, "FROM email_verification_tokens"):
				return &fakeRows{rows: [][]any{{
					emailVerificationID,
					userID,
					expiresAt,
					now,
				}}}, nil
			case strings.Contains(sql, "FROM password_reset_tokens"):
				usedAt := now.Add(-time.Minute)
				return &fakeRows{rows: [][]any{{
					passwordResetID,
					userID,
					expiresAt,
					&usedAt,
					now,
				}}}, nil
			case strings.Contains(sql, "FROM ai_generation_logs"):
				return &fakeRows{rows: [][]any{{
					aiLogID,
					userID,
					"model",
					10,
					20,
					30,
					"ok",
					now,
				}}}, nil
			case strings.Contains(sql, "FROM bingo_card_shares"):
				lastAccessedAt := now.Add(-time.Hour)
				return &fakeRows{rows: [][]any{{
					cardID,
					cardShareCreated,
					&expiresAt,
					&lastAccessedAt,
					2,
				}}}, nil
			default:
				return &fakeRows{}, nil
			}
		},
	}

	service := NewAccountService(db)
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	if err := service.writeCardsCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeCardsCSV: %v", err)
	}
	if err := service.writeItemsCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeItemsCSV: %v", err)
	}
	if err := service.writeFriendshipsCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeFriendshipsCSV: %v", err)
	}
	if err := service.writeBlocksCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeBlocksCSV: %v", err)
	}
	if err := service.writeNotificationsCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeNotificationsCSV: %v", err)
	}
	if err := service.writeAPITokensCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeAPITokensCSV: %v", err)
	}
	if err := service.writeNotificationSettingsCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeNotificationSettingsCSV: %v", err)
	}
	if err := service.writeFriendInvitesCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeFriendInvitesCSV: %v", err)
	}
	if err := service.writeSessionsCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeSessionsCSV: %v", err)
	}
	if err := service.writeReminderSettingsCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeReminderSettingsCSV: %v", err)
	}
	if err := service.writeCardCheckinRemindersCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeCardCheckinRemindersCSV: %v", err)
	}
	if err := service.writeGoalRemindersCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeGoalRemindersCSV: %v", err)
	}
	if err := service.writeCardSharesCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeCardSharesCSV: %v", err)
	}
	if err := service.writeReminderEmailLogCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeReminderEmailLogCSV: %v", err)
	}
	if err := service.writeReminderImageTokensCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeReminderImageTokensCSV: %v", err)
	}
	if err := service.writeReminderUnsubscribeTokensCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeReminderUnsubscribeTokensCSV: %v", err)
	}
	if err := service.writeEmailVerificationTokensCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeEmailVerificationTokensCSV: %v", err)
	}
	if err := service.writePasswordResetTokensCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writePasswordResetTokensCSV: %v", err)
	}
	if err := service.writeAIGenerationLogsCSV(context.Background(), zipWriter, userID); err != nil {
		t.Fatalf("writeAIGenerationLogsCSV: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	if len(reader.File) < 6 {
		t.Fatalf("expected zip entries, got %d", len(reader.File))
	}
}

func TestNullableHelpers(t *testing.T) {
	if nullableString(nil) != "" {
		t.Fatal("expected nullableString(nil) to be empty")
	}
	if nullableInt(nil) != "" {
		t.Fatal("expected nullableInt(nil) to be empty")
	}
	if nullableUUID(nil) != "" {
		t.Fatal("expected nullableUUID(nil) to be empty")
	}
}

func TestSanitizeCSVValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "formula prefix with single quote inside",
			input:    "=1+1'test",
			expected: "'=1+1''test",
		},
		{
			name:     "formula prefix without single quote",
			input:    "=2+2",
			expected: "'=2+2",
		},
		{
			name:     "plain value unchanged",
			input:    "hello",
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeCSVValue(tt.input); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func firstLine(data []byte) string {
	parts := strings.SplitN(string(data), "\n", 2)
	return strings.TrimSpace(parts[0])
}

func containsSQL(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
