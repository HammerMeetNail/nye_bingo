package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestNextMonthlySend_ClampsDay(t *testing.T) {
	after := time.Date(2025, time.January, 10, 8, 0, 0, 0, time.UTC)
	next, err := nextMonthlySend(after, monthlySchedule{DayOfMonth: 31, Time: "09:00"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, time.January, 28, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}

	after = time.Date(2025, time.January, 28, 10, 0, 0, 0, time.UTC)
	next, err = nextMonthlySend(after, monthlySchedule{DayOfMonth: 28, Time: "09:00"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected = time.Date(2025, time.February, 28, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}
}

func TestReminderService_RunDue_UsesSkipLocked(t *testing.T) {
	var queries []string
	beginCalls := 0
	fakeBegin := func(ctx context.Context) (Tx, error) {
		beginCalls++
		return &fakeTx{
			QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
				queries = append(queries, sql)
				return &fakeRows{rows: [][]any{}}, nil
			},
			CommitFunc:   func(ctx context.Context) error { return nil },
			RollbackFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	db := &fakeDB{BeginFunc: fakeBegin}
	svc := NewReminderService(db, nil, "http://example.com")
	if _, err := svc.RunDue(context.Background(), time.Now(), 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if beginCalls == 0 {
		t.Fatal("expected transaction begin")
	}
	if len(queries) == 0 {
		t.Fatal("expected due reminder queries")
	}
	for _, query := range queries {
		if !strings.Contains(query, "SKIP LOCKED") {
			t.Fatalf("expected SKIP LOCKED in query, got %q", query)
		}
	}
}

func TestReminderService_ProcessGoalReminder_DisablesCompleted(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	itemID := uuid.New()
	reminderID := uuid.New()

	var disabled bool
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM bingo_items") {
				return rowFromValues(
					cardID,
					nil,
					2025,
					true,
					false,
					"Finish project",
					true,
					"user@test.com",
					3,
				)
			}
			return rowFromValues(0)
		},
	}
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "UPDATE goal_reminders") {
				disabled = true
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	sent, err := svc.processGoalReminder(context.Background(), tx, goalReminderJob{
		ID:     reminderID,
		UserID: userID,
		CardID: cardID,
		ItemID: itemID,
		Kind:   "one_time",
	}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent {
		t.Fatal("expected no email to be sent")
	}
	if !disabled {
		t.Fatal("expected completed goal reminder to be disabled")
	}
}

func TestReminderService_GoalReminderCapReached(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC)

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM reminder_email_log") {
				return rowFromValues(3)
			}
			return rowFromValues(0)
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	reached, err := svc.goalReminderCapReached(context.Background(), userID, now, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reached {
		t.Fatal("expected cap to be reached")
	}
}

func TestPickReminderRecommendations_ExcludesCompletedAndFree(t *testing.T) {
	freePos := 4
	items := []models.BingoItem{
		{Position: 0, Content: "A", IsCompleted: true},
		{Position: 1, Content: "B", IsCompleted: true},
		{Position: 2, Content: "C"},
		{Position: 3, Content: "D", IsCompleted: true},
		{Position: 5, Content: "E", IsCompleted: true},
		{Position: 6, Content: "F"},
		{Position: 7, Content: "G", IsCompleted: true},
		{Position: 8, Content: "H"},
	}

	recs := pickReminderRecommendations(items, 3, &freePos, 3)
	if len(recs) < 3 {
		t.Fatalf("expected at least 3 recommendations, got %d", len(recs))
	}
	for _, rec := range recs {
		if rec.IsCompleted {
			t.Fatalf("expected incomplete recommendation, got %+v", rec)
		}
		if rec.Position == freePos {
			t.Fatalf("expected free position excluded, got %d", rec.Position)
		}
	}

	expected := []int{2, 6, 8}
	for i, pos := range expected {
		if recs[i].Position != pos {
			t.Fatalf("expected recommendation %d to be position %d, got %d", i, pos, recs[i].Position)
		}
	}
}
