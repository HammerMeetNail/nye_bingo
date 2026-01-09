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
				now,
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
			return rowFromValues(&deletedAt)
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
			return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
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

func firstLine(data []byte) string {
	parts := strings.SplitN(string(data), "\n", 2)
	return strings.TrimSpace(parts[0])
}
