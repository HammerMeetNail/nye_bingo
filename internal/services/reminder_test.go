package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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
		if !strings.Contains(query, "FOR UPDATE OF") {
			t.Fatalf("expected FOR UPDATE OF <alias> to avoid locking join tables, got %q", query)
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

func TestReminderService_RenderImageByToken_MissingCardReturnsNotFound(t *testing.T) {
	token := "abc123"
	userID := uuid.New()
	cardID := uuid.New()
	expiresAt := time.Now().Add(1 * time.Hour)

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM reminder_image_tokens") {
				return rowFromValues(
					token,
					userID,
					cardID,
					true,
					expiresAt,
					time.Now(),
					(*time.Time)(nil),
					0,
				)
			}
			if strings.Contains(sql, "FROM bingo_cards") {
				return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			}
			return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	_, err := svc.RenderImageByToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrReminderNotFound) {
		t.Fatalf("expected ErrReminderNotFound, got %v", err)
	}
}

func TestReminderService_UnsubscribeByToken_ReturnsAlreadyDisabled(t *testing.T) {
	userID := uuid.New()
	expiresAt := time.Now().Add(2 * time.Hour)

	var execCalls []string
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			switch {
			case strings.Contains(sql, "FROM reminder_unsubscribe_tokens"):
				return rowFromValues(userID, expiresAt, (*time.Time)(nil))
			case strings.Contains(sql, "SELECT email_enabled FROM reminder_settings"):
				return rowFromValues(false)
			default:
				return rowFromValues(false)
			}
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execCalls = append(execCalls, sql)
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	alreadyDisabled, err := svc.UnsubscribeByToken(context.Background(), "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !alreadyDisabled {
		t.Fatal("expected alreadyDisabled=true")
	}
	if len(execCalls) < 3 {
		t.Fatalf("expected exec calls for ensureSettingsRow + updates, got %d", len(execCalls))
	}
}

func TestReminderService_CapQueriesCountOnlySent(t *testing.T) {
	t.Run("checkin", func(t *testing.T) {
		var got string
		db := &fakeDB{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				got = sql
				return rowFromValues(0)
			},
		}
		svc := NewReminderService(db, nil, "http://example.com")
		_, _ = svc.cardCheckinCapReached(context.Background(), uuid.New(), time.Now())
		if !strings.Contains(got, "status = 'sent'") {
			t.Fatalf("expected status filter in query, got %q", got)
		}
	})

	t.Run("goal", func(t *testing.T) {
		var got string
		db := &fakeDB{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				got = sql
				return rowFromValues(0)
			},
		}
		svc := NewReminderService(db, nil, "http://example.com")
		_, _ = svc.goalReminderCapReached(context.Background(), uuid.New(), time.Now(), 3)
		if !strings.Contains(got, "status = 'sent'") {
			t.Fatalf("expected status filter in query, got %q", got)
		}
	})
}

func TestReminderService_ProcessCheckin_CapReachedDefersToNextDay(t *testing.T) {
	now := time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	cardID := uuid.New()
	reminderID := uuid.New()

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM reminder_email_log") {
				return rowFromValues(1)
			}
			return rowFromValues(0)
		},
	}

	var updatedTo time.Time
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			switch {
			case strings.Contains(sql, "FROM bingo_cards"):
				return rowFromValues(
					cardID,
					userID,
					2025,
					nil,
					nil,
					5,
					"BINGO",
					true,
					nil,
					true,
					true,
					true,
					false,
					now,
					now,
				)
			case strings.Contains(sql, "FROM reminder_settings"):
				return rowFromValues(userID)
			default:
				return rowFromValues(userID)
			}
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "UPDATE card_checkin_reminders SET next_send_at") {
				if len(args) < 1 {
					t.Fatalf("expected next_send_at arg, got %v", args)
				}
				updatedTo, _ = args[0].(time.Time)
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	sent, err := svc.processCheckin(context.Background(), tx, checkinJob{
		ID:         reminderID,
		UserID:     userID,
		CardID:     cardID,
		Frequency:  "monthly",
		Schedule:   []byte(`{"day_of_month":1,"time":"09:00"}`),
		NextSendAt: now.Add(-1 * time.Minute),
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent {
		t.Fatal("expected no email to be sent")
	}

	expected := time.Date(2025, time.January, 3, 9, 0, 0, 0, time.UTC)
	if updatedTo.IsZero() {
		t.Fatal("expected checkin to be deferred")
	}
	if !updatedTo.Equal(expected) {
		t.Fatalf("expected deferred time %v, got %v", expected, updatedTo)
	}
}

func TestReminderService_ProcessGoalReminder_CapReachedDefersToNextDay(t *testing.T) {
	now := time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	cardID := uuid.New()
	itemID := uuid.New()
	reminderID := uuid.New()

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
					false,
					"user@test.com",
					3,
				)
			}
			if strings.Contains(sql, "FROM reminder_email_log") {
				return rowFromValues(3)
			}
			return rowFromValues(0)
		},
	}

	var updatedTo time.Time
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM reminder_settings") {
				return rowFromValues(userID)
			}
			return rowFromValues(userID)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "UPDATE goal_reminders SET next_send_at") {
				updatedTo, _ = args[0].(time.Time)
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	base := time.Date(2025, time.January, 2, 9, 30, 0, 0, time.UTC)
	sent, err := svc.processGoalReminder(context.Background(), tx, goalReminderJob{
		ID:         reminderID,
		UserID:     userID,
		CardID:     cardID,
		ItemID:     itemID,
		Kind:       "one_time",
		NextSendAt: base,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent {
		t.Fatal("expected no email to be sent")
	}
	expected := time.Date(2025, time.January, 3, 9, 30, 0, 0, time.UTC)
	if updatedTo.IsZero() {
		t.Fatal("expected goal reminder to be deferred")
	}
	if !updatedTo.Equal(expected) {
		t.Fatalf("expected deferred time %v, got %v", expected, updatedTo)
	}
}

func TestReminderService_ProcessCheckin_FailureDefers15MinutesAndDoesNotAdvanceSchedule(t *testing.T) {
	now := time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	cardID := uuid.New()
	reminderID := uuid.New()

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM reminder_email_log") {
				return rowFromValues(0)
			}
			if strings.Contains(sql, "SELECT email FROM users") {
				return rowFromValues("user@test.com")
			}
			return rowFromValues(0)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			// createUnsubscribeURL insert
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	var deferredTo time.Time
	var sawAdvance bool
	var sawUpsert bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			switch {
			case strings.Contains(sql, "FROM bingo_cards"):
				return rowFromValues(
					cardID,
					userID,
					2025,
					nil,
					nil,
					5,
					"BINGO",
					true,
					nil,
					true,
					true,
					true,
					false,
					now,
					now,
				)
			case strings.Contains(sql, "FROM reminder_settings"):
				return rowFromValues(userID)
			default:
				return rowFromValues(userID)
			}
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "UPDATE card_checkin_reminders SET last_sent_at") {
				sawAdvance = true
			}
			if strings.Contains(sql, "UPDATE card_checkin_reminders SET next_send_at") {
				deferredTo, _ = args[0].(time.Time)
			}
			if strings.Contains(sql, "INSERT INTO reminder_email_log") && strings.Contains(sql, "ON CONFLICT") {
				sawUpsert = true
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	sent, err := svc.processCheckin(context.Background(), tx, checkinJob{
		ID:                     reminderID,
		UserID:                 userID,
		CardID:                 cardID,
		Frequency:              "monthly",
		Schedule:               []byte(`{"day_of_month":1,"time":"09:00"}`),
		IncludeImage:           false,
		NextSendAt:             now.Add(-1 * time.Minute),
		IncludeRecommendations: false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent {
		t.Fatal("expected no email to be sent")
	}
	if sawAdvance {
		t.Fatal("expected schedule not to advance on failure")
	}
	if deferredTo.IsZero() {
		t.Fatal("expected checkin to be deferred after failure")
	}
	expected := now.Add(15 * time.Minute)
	if !deferredTo.Equal(expected) {
		t.Fatalf("expected deferred time %v, got %v", expected, deferredTo)
	}
	if !sawUpsert {
		t.Fatal("expected checkin email log upsert")
	}
}

func TestReminderService_ProcessGoalReminder_FailureDefers15MinutesAndDoesNotDisable(t *testing.T) {
	now := time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	cardID := uuid.New()
	itemID := uuid.New()
	reminderID := uuid.New()

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
					false,
					"user@test.com",
					3,
				)
			}
			if strings.Contains(sql, "FROM reminder_email_log") {
				return rowFromValues(0)
			}
			return rowFromValues(0)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			// createUnsubscribeURL insert
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	var deferredTo time.Time
	var sawDisable bool
	var sawUpsert bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM reminder_settings") {
				return rowFromValues(userID)
			}
			return rowFromValues(userID)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "enabled = false") {
				sawDisable = true
			}
			if strings.Contains(sql, "UPDATE goal_reminders SET next_send_at") {
				deferredTo, _ = args[0].(time.Time)
			}
			if strings.Contains(sql, "INSERT INTO reminder_email_log") && strings.Contains(sql, "ON CONFLICT") {
				sawUpsert = true
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	sent, err := svc.processGoalReminder(context.Background(), tx, goalReminderJob{
		ID:         reminderID,
		UserID:     userID,
		CardID:     cardID,
		ItemID:     itemID,
		Kind:       "one_time",
		NextSendAt: now.Add(-1 * time.Minute),
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent {
		t.Fatal("expected no email to be sent")
	}
	if sawDisable {
		t.Fatal("expected goal reminder not to be disabled on failure")
	}
	if deferredTo.IsZero() {
		t.Fatal("expected goal reminder to be deferred after failure")
	}
	expected := now.Add(15 * time.Minute)
	if !deferredTo.Equal(expected) {
		t.Fatalf("expected deferred time %v, got %v", expected, deferredTo)
	}
	if sawUpsert {
		t.Fatal("expected no upsert for goal reminder logs")
	}
}

func TestReminderService_LogReminderEmail_CheckinsUseUpsert(t *testing.T) {
	var sawUpsert bool
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "ON CONFLICT") {
				sawUpsert = true
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	svc := NewReminderService(&fakeDB{}, nil, "http://example.com")
	err := svc.logReminderEmail(context.Background(), tx, uuid.New(), "card_checkin", uuid.New(), reminderEmailFailed, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawUpsert {
		t.Fatal("expected ON CONFLICT upsert for checkin logs")
	}
}

func TestReminderService_LogReminderEmail_GoalsDoNotUseUpsert(t *testing.T) {
	var sawUpsert bool
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "ON CONFLICT") {
				sawUpsert = true
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	svc := NewReminderService(&fakeDB{}, nil, "http://example.com")
	err := svc.logReminderEmail(context.Background(), tx, uuid.New(), "goal_reminder", uuid.New(), reminderEmailFailed, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawUpsert {
		t.Fatal("expected no ON CONFLICT for goal reminder logs")
	}
}
