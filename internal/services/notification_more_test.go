package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestNotificationService_MarkReadAndUnreadCountAndCleanupOld(t *testing.T) {
	userID := uuid.New()
	notificationID := uuid.New()

	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			switch {
			case strings.Contains(sql, "UPDATE notifications SET read_at"):
				return fakeCommandTag{rowsAffected: 0}, nil
			case strings.Contains(sql, "DELETE FROM notifications WHERE created_at"):
				return fakeCommandTag{rowsAffected: 1}, nil
			default:
				t.Fatalf("unexpected exec sql: %q", sql)
				return fakeCommandTag{}, nil
			}
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if !strings.Contains(sql, "SELECT COUNT(*) FROM notifications") {
				t.Fatalf("unexpected query sql: %q", sql)
			}
			return rowFromValues(7)
		},
	}

	svc := NewNotificationService(db, nil, "http://example.com")

	if err := svc.MarkRead(context.Background(), userID, notificationID); !errors.Is(err, ErrNotificationNotFound) {
		t.Fatalf("expected ErrNotificationNotFound, got %v", err)
	}
	count, err := svc.UnreadCount(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 7 {
		t.Fatalf("expected count 7, got %d", count)
	}
	if err := svc.CleanupOld(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotificationService_UpdateSettings_EmailEnabledRequiresVerifiedEmail(t *testing.T) {
	userID := uuid.New()
	enabled := true

	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO notification_settings") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			if strings.Contains(sql, "UPDATE notification_settings") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			t.Fatalf("unexpected exec sql: %q", sql)
			return fakeCommandTag{}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT email_verified") {
				return rowFromValues(false)
			}
			if strings.Contains(sql, "FROM notification_settings") {
				return rowFromValues(
					userID,
					true, true, true, true, true,
					true, true, true, true, true,
					time.Now().Add(-time.Hour),
					time.Now(),
				)
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues(false)
		},
	}

	svc := NewNotificationService(db, nil, "http://example.com")
	_, err := svc.UpdateSettings(context.Background(), userID, models.NotificationSettingsPatch{EmailEnabled: &enabled})
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
}

func TestNotificationService_UpdateSettings_UpdatesWhenVerified(t *testing.T) {
	userID := uuid.New()
	enabled := true
	friendBingo := false

	execCalls := 0
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execCalls++
			if strings.Contains(sql, "INSERT INTO notification_settings") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			if strings.Contains(sql, "UPDATE notification_settings SET") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			t.Fatalf("unexpected exec sql: %q", sql)
			return fakeCommandTag{}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT email_verified") {
				return rowFromValues(true)
			}
			if strings.Contains(sql, "FROM notification_settings") {
				return rowFromValues(
					userID,
					true, true, true, true, true,
					true, true, true, friendBingo, true,
					time.Now().Add(-time.Hour),
					time.Now(),
				)
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues(false)
		},
	}

	svc := NewNotificationService(db, nil, "http://example.com")
	_, err := svc.UpdateSettings(context.Background(), userID, models.NotificationSettingsPatch{
		EmailEnabled:     &enabled,
		EmailFriendBingo: &friendBingo,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if execCalls < 2 {
		t.Fatalf("expected insert+update calls, got %d", execCalls)
	}
}

func TestNotificationService_NotifyFriendsBingo_NoOpWhenCountZero(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			t.Fatal("expected no DB calls when bingoCount<=0")
			return nil, nil
		},
	}
	svc := NewNotificationService(db, nil, "http://example.com")
	if err := svc.NotifyFriendsBingo(context.Background(), uuid.New(), uuid.New(), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotificationService_DispatchEmails_SendsAndMarksSent(t *testing.T) {
	notificationID := uuid.New()
	actor := "Alice"
	cardTitle := "My Card"
	cardYear := 2025
	bingoCount := 2

	var sentTo string
	var sentSubject string
	execCalls := 0
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if !strings.Contains(sql, "FROM notifications n") {
				t.Fatalf("unexpected query sql: %q", sql)
			}
			return &fakeRows{rows: [][]any{
				{notificationID, string(models.NotificationTypeFriendBingo), "to@test.com", "recipient", &actor, nil, &cardTitle, &cardYear, &bingoCount},
			}}, nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if !strings.Contains(sql, "UPDATE notifications SET email_sent_at") {
				t.Fatalf("unexpected exec sql: %q", sql)
			}
			execCalls++
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	emailSvc := stubEmailService{
		SendNotificationEmailFunc: func(ctx context.Context, toEmail, subject, html, text string) error {
			sentTo = toEmail
			sentSubject = subject
			if !strings.Contains(html, "Year of Bingo") {
				t.Fatalf("expected html to contain brand header, got %q", html)
			}
			if !strings.Contains(text, "View notifications:") {
				t.Fatalf("expected text to include links, got %q", text)
			}
			return nil
		},
	}

	svc := NewNotificationService(db, emailSvc, "http://example.com")
	svc.asyncCtx = nil
	svc.SetAsync(func(fn func()) { fn() })

	svc.dispatchEmails([]uuid.UUID{notificationID})
	if sentTo != "to@test.com" {
		t.Fatalf("expected to@test.com, got %q", sentTo)
	}
	if strings.TrimSpace(sentSubject) == "" {
		t.Fatal("expected a subject")
	}
	if execCalls != 1 {
		t.Fatalf("expected 1 update call, got %d", execCalls)
	}
}

func TestNotificationService_BuildNotificationEmail_CoversScenarios(t *testing.T) {
	svc := NewNotificationService(&fakeDB{}, stubEmailService{}, "http://example.com")

	actor := "Bob"
	title := "Card"
	year := 2025
	bingos := 3

	subject, html, text := svc.buildNotificationEmail(models.NotificationTypeFriendRequestAccepted, &actor, &title, &year, nil)
	if !strings.Contains(subject, "accepted") || !strings.Contains(html, "accepted") || !strings.Contains(text, "accepted") {
		t.Fatalf("expected accepted notification text, got subject=%q", subject)
	}

	subject, _, _ = svc.buildNotificationEmail(models.NotificationTypeFriendBingo, &actor, &title, &year, &bingos)
	if !strings.Contains(subject, "bingo") {
		t.Fatalf("expected bingo subject, got %q", subject)
	}

	subject, _, _ = svc.buildNotificationEmail(models.NotificationTypeFriendNewCard, nil, nil, nil, nil)
	if !strings.Contains(subject, "new") {
		t.Fatalf("expected new-card subject, got %q", subject)
	}
}
