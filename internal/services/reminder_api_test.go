package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

type stubEmailService struct {
	SendNotificationEmailFunc func(ctx context.Context, toEmail, subject, html, text string) error
}

func (s stubEmailService) SendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error {
	return nil
}
func (s stubEmailService) VerifyEmail(ctx context.Context, token string) error { return nil }
func (s stubEmailService) SendMagicLinkEmail(ctx context.Context, email string) error {
	return nil
}
func (s stubEmailService) VerifyMagicLink(ctx context.Context, token string) (string, error) {
	return "", nil
}
func (s stubEmailService) SendPasswordResetEmail(ctx context.Context, userID uuid.UUID, email string) error {
	return nil
}
func (s stubEmailService) VerifyPasswordResetToken(ctx context.Context, token string) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (s stubEmailService) MarkPasswordResetUsed(ctx context.Context, token string) error { return nil }
func (s stubEmailService) SendNotificationEmail(ctx context.Context, toEmail, subject, html, text string) error {
	if s.SendNotificationEmailFunc != nil {
		return s.SendNotificationEmailFunc(ctx, toEmail, subject, html, text)
	}
	return nil
}
func (s stubEmailService) SendSupportEmail(ctx context.Context, fromEmail, category, message string, userID string) error {
	return nil
}

func TestReminderService_GetSettings_InsertsThenLoads(t *testing.T) {
	userID := uuid.New()
	createdAt := time.Now().Add(-time.Hour)
	updatedAt := time.Now()

	execCalls := 0
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execCalls++
			if !strings.Contains(sql, "INSERT INTO reminder_settings") {
				t.Fatalf("unexpected exec sql: %q", sql)
			}
			if len(args) != 1 || args[0] != userID {
				t.Fatalf("unexpected exec args: %#v", args)
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if !strings.Contains(sql, "FROM reminder_settings") {
				t.Fatalf("unexpected query sql: %q", sql)
			}
			return rowFromValues(userID, true, 3, createdAt, updatedAt)
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	settings, err := svc.GetSettings(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if execCalls != 1 {
		t.Fatalf("expected 1 exec call, got %d", execCalls)
	}
	if settings.UserID != userID || !settings.EmailEnabled || settings.DailyEmailCap != 3 {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}

func TestReminderService_UpdateSettings_EnableRequiresVerifiedEmail(t *testing.T) {
	userID := uuid.New()
	enabled := true

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT email_verified") {
				return rowFromValues(false)
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues(false)
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	_, err := svc.UpdateSettings(context.Background(), userID, models.ReminderSettingsPatch{EmailEnabled: &enabled})
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
}

func TestReminderService_UpdateSettings_UpdatesAndReturnsSettings(t *testing.T) {
	userID := uuid.New()
	enabled := true
	createdAt := time.Now().Add(-time.Hour)
	updatedAt := time.Now()

	var updated bool
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO reminder_settings") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			if strings.Contains(sql, "UPDATE reminder_settings SET email_enabled") {
				updated = true
				if len(args) != 2 || args[0] != true || args[1] != userID {
					t.Fatalf("unexpected update args: %#v", args)
				}
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			t.Fatalf("unexpected exec sql: %q", sql)
			return fakeCommandTag{}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT email_verified") {
				return rowFromValues(true)
			}
			if strings.Contains(sql, "FROM reminder_settings") {
				return rowFromValues(userID, true, 3, createdAt, updatedAt)
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues(false)
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	settings, err := svc.UpdateSettings(context.Background(), userID, models.ReminderSettingsPatch{EmailEnabled: &enabled})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Fatal("expected settings update to be executed")
	}
	if settings.UserID != userID || !settings.EmailEnabled {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}

func TestReminderService_ListCardCheckins_MapsOptionalReminder(t *testing.T) {
	userID := uuid.New()
	cardID1 := uuid.New()
	cardID2 := uuid.New()
	title := "Card"
	checkinID := uuid.New()
	next := time.Now().Add(24 * time.Hour)
	lastSent := time.Now().Add(-24 * time.Hour)
	createdAt := time.Now().Add(-48 * time.Hour)
	updatedAt := time.Now().Add(-time.Hour)
	enabled := true
	frequency := "monthly"
	includeImage := true
	includeRecommendations := false
	schedule := []byte(`{"day_of_month":28,"time":"09:00"}`)

	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if !strings.Contains(sql, "FROM bingo_cards") {
				t.Fatalf("unexpected query sql: %q", sql)
			}
			if len(args) != 1 || args[0] != userID {
				t.Fatalf("unexpected query args: %#v", args)
			}
			return &fakeRows{rows: [][]any{
				// Card with no reminder (NULL reminder fields)
				{cardID1, &title, 2025, true, false, false, 3, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil},
				// Card with a reminder
				{cardID2, &title, 2024, true, false, true, 5, &checkinID, &userID, &cardID2, &enabled, &frequency, schedule, &includeImage, &includeRecommendations, &next, &lastSent, &createdAt, &updatedAt},
			}}, nil
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	summaries, err := svc.ListCardCheckins(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	if summaries[0].CardID != cardID1 || summaries[0].Checkin != nil {
		t.Fatalf("unexpected first summary: %#v", summaries[0])
	}
	if summaries[1].CardID != cardID2 || summaries[1].Checkin == nil {
		t.Fatalf("unexpected second summary: %#v", summaries[1])
	}
	if summaries[1].Checkin.ID != checkinID || !summaries[1].Checkin.Enabled || summaries[1].Checkin.Frequency != "monthly" {
		t.Fatalf("unexpected checkin: %#v", summaries[1].Checkin)
	}
	if string(summaries[1].Checkin.Schedule) != string(schedule) {
		t.Fatalf("expected schedule %q, got %q", schedule, summaries[1].Checkin.Schedule)
	}
}

func TestReminderService_UpsertCardCheckin_DefaultsAndClampsSchedule(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	checkinID := uuid.New()
	fixedNow := time.Date(2026, time.January, 10, 8, 0, 0, 0, time.UTC)
	expectedNext := time.Date(2026, time.January, 28, 9, 0, 0, 0, time.UTC)

	var insertArgs []any
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO reminder_settings") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			t.Fatalf("unexpected exec sql: %q", sql)
			return fakeCommandTag{}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT is_finalized, is_archived") {
				return rowFromValues(true, false)
			}
			if strings.Contains(sql, "INSERT INTO card_checkin_reminders") {
				insertArgs = args
				return rowFromValues(
					checkinID,
					userID,
					cardID,
					true,
					"monthly",
					[]byte(`{"day_of_month":28,"time":"09:00"}`),
					true,
					false,
					&expectedNext,
					nil,
					fixedNow.Add(-time.Hour),
					fixedNow,
				)
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues(false)
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	svc.now = func() time.Time { return fixedNow }

	includeRecommendations := false
	checkin, err := svc.UpsertCardCheckin(context.Background(), userID, cardID, models.CardCheckinScheduleInput{
		Frequency: " ", // should default
		Schedule: models.CardCheckinSchedulePayload{
			DayOfMonth: 31, // should clamp
			Time:       "09:00",
		},
		IncludeRecommendations: &includeRecommendations,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checkin.ID != checkinID || checkin.CardID != cardID {
		t.Fatalf("unexpected reminder: %#v", checkin)
	}
	if checkin.NextSendAt == nil || !checkin.NextSendAt.Equal(expectedNext) {
		t.Fatalf("expected next send %v, got %v", expectedNext, checkin.NextSendAt)
	}
	if len(insertArgs) != 7 {
		t.Fatalf("expected 7 insert args, got %#v", insertArgs)
	}
	scheduleJSON, ok := insertArgs[3].([]byte)
	if !ok {
		t.Fatalf("expected schedule arg to be []byte, got %T", insertArgs[3])
	}
	var parsed monthlySchedule
	if err := json.Unmarshal(scheduleJSON, &parsed); err != nil {
		t.Fatalf("unmarshal schedule json: %v", err)
	}
	if parsed.DayOfMonth != 28 || parsed.Time != "09:00" {
		t.Fatalf("unexpected parsed schedule: %#v", parsed)
	}
	if insertArgs[4] != true || insertArgs[5] != false {
		t.Fatalf("unexpected include args: %#v", insertArgs[4:6])
	}
	if gotNext, ok := insertArgs[6].(time.Time); !ok || !gotNext.Equal(expectedNext) {
		t.Fatalf("expected next send arg %v, got %#v", expectedNext, insertArgs[6])
	}
}

func TestReminderService_DeleteCardCheckin_NotFound(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}
	svc := NewReminderService(db, nil, "http://example.com")
	err := svc.DeleteCardCheckin(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrReminderNotFound) {
		t.Fatalf("expected ErrReminderNotFound, got %v", err)
	}
}

func TestReminderService_ListGoalReminders_OptionalCardFilter(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	itemID := uuid.New()
	reminderID := uuid.New()
	sendAt := time.Now().Add(24 * time.Hour)

	var gotArgs []any
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			gotArgs = args
			title := "Card"
			return &fakeRows{rows: [][]any{
				{reminderID, cardID, itemID, "one_time", []byte(`{"send_at":"2030-01-02T15:04:05Z"}`), &sendAt, nil, &title, 2025, "Goal"},
			}}, nil
		},
	}
	svc := NewReminderService(db, nil, "http://example.com")

	_, err := svc.ListGoalReminders(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotArgs) != 2 || gotArgs[0] != userID || gotArgs[1] != nil {
		t.Fatalf("expected nil card filter, got args %#v", gotArgs)
	}

	_, err = svc.ListGoalReminders(context.Background(), userID, &cardID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotArgs) != 2 || gotArgs[0] != userID || gotArgs[1] != cardID {
		t.Fatalf("expected card filter %v, got args %#v", cardID, gotArgs)
	}
}

func TestReminderService_UpsertGoalReminder_SuccessAndSerializesSchedule(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	itemID := uuid.New()
	reminderID := uuid.New()
	fixedNow := time.Date(2026, time.January, 10, 8, 0, 0, 0, time.UTC)
	sendAt := fixedNow.Add(2 * time.Hour)

	var insertArgs []any
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO reminder_settings") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			t.Fatalf("unexpected exec sql: %q", sql)
			return fakeCommandTag{}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM bingo_items i") {
				return rowFromValues(cardID, false, true, false)
			}
			if strings.Contains(sql, "INSERT INTO goal_reminders") {
				insertArgs = args
				return rowFromValues(reminderID, userID, cardID, itemID, true, "one_time", []byte(`{"send_at":"`+sendAt.UTC().Format(time.RFC3339)+`"}`), &sendAt, nil, fixedNow, fixedNow)
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues(false)
		},
	}
	svc := NewReminderService(db, nil, "http://example.com")
	svc.now = func() time.Time { return fixedNow }

	reminder, err := svc.UpsertGoalReminder(context.Background(), userID, models.GoalReminderInput{
		ItemID: itemID,
		Kind:   "one_time",
		Schedule: models.GoalReminderScheduleInput{
			SendAt: sendAt.Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reminder.ID != reminderID || reminder.ItemID != itemID {
		t.Fatalf("unexpected reminder: %#v", reminder)
	}
	if len(insertArgs) != 6 {
		t.Fatalf("expected 6 insert args, got %#v", insertArgs)
	}
	scheduleJSON, ok := insertArgs[4].([]byte)
	if !ok {
		t.Fatalf("expected schedule arg to be []byte, got %T", insertArgs[4])
	}
	var parsed oneTimeSchedule
	if err := json.Unmarshal(scheduleJSON, &parsed); err != nil {
		t.Fatalf("unmarshal schedule json: %v", err)
	}
	if parsed.SendAt != sendAt.UTC().Format(time.RFC3339) {
		t.Fatalf("expected send_at %q, got %q", sendAt.UTC().Format(time.RFC3339), parsed.SendAt)
	}
}

func TestReminderService_CleanupOld_ExecutesDeletes(t *testing.T) {
	var queries []string
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			queries = append(queries, sql)
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	svc := NewReminderService(db, nil, "http://example.com")
	if err := svc.CleanupOld(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(queries) != 3 {
		t.Fatalf("expected 3 cleanup queries, got %d", len(queries))
	}
}

func TestReminderService_SendTestEmail_SendsNotificationEmail(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	title := "Card"
	createdAt := time.Now().Add(-48 * time.Hour)
	updatedAt := time.Now().Add(-time.Hour)
	nextSend := time.Now().Add(24 * time.Hour)
	existingToken := "tok123"

	var sentTo string
	var sentSubject string
	var sentHTML string
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			switch {
			case strings.Contains(sql, "INSERT INTO reminder_settings"):
				return fakeCommandTag{rowsAffected: 1}, nil
			case strings.Contains(sql, "UPDATE reminder_image_tokens SET expires_at"):
				return fakeCommandTag{rowsAffected: 1}, nil
			case strings.Contains(sql, "INSERT INTO reminder_unsubscribe_tokens"):
				return fakeCommandTag{rowsAffected: 1}, nil
			default:
				t.Fatalf("unexpected exec sql: %q", sql)
				return fakeCommandTag{}, nil
			}
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if !strings.Contains(sql, "FROM bingo_items") {
				t.Fatalf("unexpected items query sql: %q", sql)
			}
			itemID := uuid.New()
			return &fakeRows{rows: [][]any{
				{itemID, cardID, 0, "Do thing", false, nil, nil, nil, createdAt},
				{uuid.New(), cardID, 1, "Done thing", true, &createdAt, nil, nil, createdAt},
			}}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			switch {
			case strings.Contains(sql, "FROM reminder_settings"):
				return rowFromValues(userID, true, 3, createdAt, updatedAt)
			case strings.Contains(sql, "SELECT email_verified"):
				return rowFromValues(true)
			case strings.Contains(sql, "FROM bingo_cards WHERE id"):
				freePos := 4
				return rowFromValues(
					cardID,
					userID,
					2025,
					nil,
					&title,
					3,
					"BING",
					false,
					&freePos,
					true,
					true,
					false,
					false,
					createdAt,
					updatedAt,
				)
			case strings.Contains(sql, "SELECT email FROM users"):
				return rowFromValues("user@test.com")
			case strings.Contains(sql, "FROM reminder_image_tokens"):
				return rowFromValues(existingToken)
			default:
				t.Fatalf("unexpected query sql: %q", sql)
				return rowFromValues(false)
			}
		},
	}

	emailSvc := stubEmailService{
		SendNotificationEmailFunc: func(ctx context.Context, toEmail, subject, html, text string) error {
			sentTo = toEmail
			sentSubject = subject
			sentHTML = html
			return nil
		},
	}

	svc := NewReminderService(db, emailSvc, "http://example.com")
	svc.now = func() time.Time { return nextSend.Add(-time.Minute) }

	if err := svc.SendTestEmail(context.Background(), userID, cardID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentTo != "user@test.com" {
		t.Fatalf("expected email to user@test.com, got %q", sentTo)
	}
	if strings.TrimSpace(sentSubject) == "" {
		t.Fatal("expected non-empty email subject")
	}
	if !strings.Contains(sentHTML, "/r/img/"+existingToken+".png") {
		t.Fatalf("expected html to include image url, got %q", sentHTML)
	}
}

func TestParseMonthlySchedule_Validates(t *testing.T) {
	_, err := parseMonthlySchedule(models.CardCheckinSchedulePayload{DayOfMonth: 0, Time: "09:00"})
	if !errors.Is(err, ErrInvalidSchedule) {
		t.Fatalf("expected ErrInvalidSchedule, got %v", err)
	}

	out, err := parseMonthlySchedule(models.CardCheckinSchedulePayload{DayOfMonth: 31, Time: "09:00"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.DayOfMonth != 28 {
		t.Fatalf("expected clamped day 28, got %d", out.DayOfMonth)
	}
}

func TestParseOneTimeSchedule_RequiresFutureTime(t *testing.T) {
	now := time.Date(2026, time.January, 10, 8, 0, 0, 0, time.UTC)
	_, err := parseOneTimeSchedule(models.GoalReminderScheduleInput{SendAt: now.Add(-time.Minute).Format(time.RFC3339)}, now)
	if !errors.Is(err, ErrInvalidSchedule) {
		t.Fatalf("expected ErrInvalidSchedule, got %v", err)
	}

	out, err := parseOneTimeSchedule(models.GoalReminderScheduleInput{SendAt: now.Add(time.Hour).Format(time.RFC3339)}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.After(now) {
		t.Fatalf("expected future send time, got %v", out)
	}
}

func TestReminderService_EnsureCardEligible_NotFound(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	svc := NewReminderService(db, nil, "http://example.com")
	err := svc.ensureCardEligible(context.Background(), userID, cardID)
	if !errors.Is(err, ErrCardNotFound) {
		t.Fatalf("expected ErrCardNotFound, got %v", err)
	}
}
