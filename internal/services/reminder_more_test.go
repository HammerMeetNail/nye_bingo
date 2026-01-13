package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestReminderService_DeleteGoalReminder_NotFound(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}
	svc := NewReminderService(db, nil, "http://example.com")
	err := svc.DeleteGoalReminder(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrReminderNotFound) {
		t.Fatalf("expected ErrReminderNotFound, got %v", err)
	}
}

func TestReminderService_NextCheckinSendAt_ParsesSchedule(t *testing.T) {
	svc := NewReminderService(&fakeDB{}, nil, "http://example.com")
	now := time.Date(2026, time.January, 10, 8, 0, 0, 0, time.UTC)

	_, err := svc.nextCheckinSendAt(now, checkinJob{Schedule: []byte("not-json")})
	if !errors.Is(err, ErrInvalidSchedule) {
		t.Fatalf("expected ErrInvalidSchedule, got %v", err)
	}

	sched, _ := json.Marshal(monthlySchedule{DayOfMonth: 28, Time: "09:00"})
	next, err := svc.nextCheckinSendAt(now, checkinJob{Schedule: sched})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !next.After(now) {
		t.Fatalf("expected next send after now, got %v", next)
	}
}

func TestReminderService_UpdateCheckinAfterSend_WrapsTxErrors(t *testing.T) {
	reminderID := uuid.New()
	sentAt := time.Now().Add(-time.Minute)
	next := time.Now().Add(time.Hour)

	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	svc := NewReminderService(&fakeDB{}, nil, "http://example.com")
	err := svc.updateCheckinAfterSend(context.Background(), tx, reminderID, sentAt, next)
	if err == nil || !strings.Contains(err.Error(), "update card checkin") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestReminderService_MarkGoalReminderSent_RequiresTx(t *testing.T) {
	svc := NewReminderService(&fakeDB{}, nil, "http://example.com")
	err := svc.markGoalReminderSent(context.Background(), nil, uuid.New(), time.Now())
	if err == nil {
		t.Fatal("expected error for nil tx")
	}
}

func TestReminderService_DisableCheckin_WrapsTxErrors(t *testing.T) {
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}
	svc := NewReminderService(&fakeDB{}, nil, "http://example.com")
	_, err := svc.disableCheckin(context.Background(), tx, uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReminderService_TouchImageToken_WrapsErrors(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}
	svc := NewReminderService(db, nil, "http://example.com")
	err := svc.touchImageToken(context.Background(), "tok")
	if err == nil {
		t.Fatal("expected error")
	}
}
