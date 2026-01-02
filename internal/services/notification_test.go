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

func TestNotificationService_GetSettings_CreatesRow(t *testing.T) {
	userID := uuid.New()
	var insertSQL string
	var selectSQL string
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			insertSQL = sql
			if len(args) != 1 || args[0] != userID {
				t.Fatalf("expected insert with userID %v, got %v", userID, args)
			}
			return fakeCommandTag{}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			selectSQL = sql
			return rowFromValues(
				userID,
				true,
				true,
				true,
				true,
				true,
				false,
				false,
				false,
				false,
				false,
				time.Now(),
				time.Now(),
			)
		},
	}

	svc := NewNotificationService(db, nil, "http://example.com")
	settings, err := svc.GetSettings(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.UserID != userID {
		t.Fatalf("expected userID %v, got %v", userID, settings.UserID)
	}
	if !strings.Contains(insertSQL, "INSERT INTO notification_settings") {
		t.Fatalf("expected insert into notification_settings, got %q", insertSQL)
	}
	if !strings.Contains(selectSQL, "FROM notification_settings") {
		t.Fatalf("expected select from notification_settings, got %q", selectSQL)
	}
}

func TestNotificationService_UpdateSettings_EmailRequiresVerification(t *testing.T) {
	userID := uuid.New()
	var execCalled bool
	var userChecked bool
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM users") {
				userChecked = true
				return rowFromValues(false)
			}
			return rowFromValues(
				userID,
				true,
				true,
				true,
				true,
				true,
				false,
				false,
				false,
				false,
				false,
				time.Now(),
				time.Now(),
			)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execCalled = true
			return fakeCommandTag{}, nil
		},
	}

	svc := NewNotificationService(db, nil, "http://example.com")
	_, err := svc.UpdateSettings(context.Background(), userID, models.NotificationSettingsPatch{
		EmailEnabled: boolPtr(true),
	})
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
	if !userChecked {
		t.Fatal("expected email verification check")
	}
	if execCalled {
		t.Fatal("expected no update when email is unverified")
	}
}

func TestNotificationService_List_UnreadOnlyFilters(t *testing.T) {
	userID := uuid.New()
	var gotSQL string
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			gotSQL = sql
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	svc := NewNotificationService(db, nil, "http://example.com")
	_, err := svc.List(context.Background(), userID, NotificationListParams{
		Limit:      10,
		UnreadOnly: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotSQL, "in_app_delivered") {
		t.Fatalf("expected in_app_delivered filter, got %q", gotSQL)
	}
	if !strings.Contains(gotSQL, "read_at IS NULL") {
		t.Fatalf("expected unread filter, got %q", gotSQL)
	}
}

func TestNotificationService_NotifyFriendsNewCard_UsesSettingsAndBlocks(t *testing.T) {
	actorID := uuid.New()
	cardID := uuid.New()
	var gotSQL string
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			gotSQL = sql
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	svc := NewNotificationService(db, nil, "http://example.com")
	if err := svc.NotifyFriendsNewCard(context.Background(), actorID, cardID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotSQL, "FROM friendships") {
		t.Fatalf("expected friendships join, got %q", gotSQL)
	}
	if !strings.Contains(gotSQL, "notification_settings") {
		t.Fatalf("expected notification_settings join, got %q", gotSQL)
	}
	if !strings.Contains(gotSQL, "user_blocks") {
		t.Fatalf("expected user_blocks exclusion, got %q", gotSQL)
	}
	if !strings.Contains(gotSQL, "ON CONFLICT DO NOTHING") {
		t.Fatalf("expected ON CONFLICT DO NOTHING, got %q", gotSQL)
	}
}

func TestNotificationService_NotifyFriendRequestReceived_UsesScenarioToggles(t *testing.T) {
	recipientID := uuid.New()
	actorID := uuid.New()
	friendshipID := uuid.New()
	var gotSQL string
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			gotSQL = sql
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	svc := NewNotificationService(db, nil, "http://example.com")
	if err := svc.NotifyFriendRequestReceived(context.Background(), recipientID, actorID, friendshipID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotSQL, "in_app_friend_request_received") {
		t.Fatalf("expected in_app_friend_request_received gating, got %q", gotSQL)
	}
	if !strings.Contains(gotSQL, "email_friend_request_received") {
		t.Fatalf("expected email_friend_request_received gating, got %q", gotSQL)
	}
	if !strings.Contains(gotSQL, "LEFT JOIN notification_settings") {
		t.Fatalf("expected left join for notification_settings, got %q", gotSQL)
	}
	if !strings.Contains(gotSQL, "COALESCE(ns.in_app_enabled, true)") {
		t.Fatalf("expected default in-app settings for missing rows, got %q", gotSQL)
	}
	if !strings.Contains(gotSQL, "ON CONFLICT DO NOTHING") {
		t.Fatalf("expected ON CONFLICT DO NOTHING, got %q", gotSQL)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
