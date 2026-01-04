package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrInvalidSchedule   = errors.New("invalid reminder schedule")
	ErrReminderNotFound  = errors.New("reminder not found")
	ErrCardNotEligible   = errors.New("card not eligible for reminders")
	ErrGoalCompleted     = errors.New("goal already completed")
	ErrRemindersDisabled = errors.New("reminders disabled")
)

type monthlySchedule struct {
	DayOfMonth int    `json:"day_of_month"`
	Time       string `json:"time"`
}

type oneTimeSchedule struct {
	SendAt string `json:"send_at"`
}

type checkinJob struct {
	ID                     uuid.UUID
	UserID                 uuid.UUID
	CardID                 uuid.UUID
	Frequency              string
	Schedule               []byte
	IncludeImage           bool
	IncludeRecommendations bool
	NextSendAt             time.Time
}

type goalReminderJob struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	CardID     uuid.UUID
	ItemID     uuid.UUID
	Kind       string
	Schedule   []byte
	NextSendAt time.Time
}

type reminderEmailStatus string

const (
	reminderEmailSent   reminderEmailStatus = "sent"
	reminderEmailFailed reminderEmailStatus = "failed"
)

type ReminderService struct {
	db           DB
	emailService EmailServiceInterface
	baseURL      string
	now          func() time.Time
}

func NewReminderService(db DB, emailService EmailServiceInterface, baseURL string) *ReminderService {
	trimmed := strings.TrimRight(baseURL, "/")
	return &ReminderService{
		db:           db,
		emailService: emailService,
		baseURL:      trimmed,
		now:          time.Now,
	}
}

func (s *ReminderService) GetSettings(ctx context.Context, userID uuid.UUID) (*models.ReminderSettings, error) {
	if err := s.ensureSettingsRow(ctx, userID); err != nil {
		return nil, err
	}
	return s.loadSettings(ctx, userID)
}

func (s *ReminderService) UpdateSettings(ctx context.Context, userID uuid.UUID, patch models.ReminderSettingsPatch) (*models.ReminderSettings, error) {
	if patch.EmailEnabled != nil && *patch.EmailEnabled {
		verified, err := s.isEmailVerified(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !verified {
			return nil, ErrEmailNotVerified
		}
	}

	if err := s.ensureSettingsRow(ctx, userID); err != nil {
		return nil, err
	}

	if patch.EmailEnabled == nil {
		return s.loadSettings(ctx, userID)
	}

	if _, err := s.db.Exec(ctx,
		"UPDATE reminder_settings SET email_enabled = $1, updated_at = NOW() WHERE user_id = $2",
		*patch.EmailEnabled,
		userID,
	); err != nil {
		return nil, fmt.Errorf("update reminder settings: %w", err)
	}

	return s.loadSettings(ctx, userID)
}

func (s *ReminderService) ListCardCheckins(ctx context.Context, userID uuid.UUID) ([]models.CardCheckinSummary, error) {
	rows, err := s.db.Query(ctx, `
		SELECT c.id, c.title, c.year, c.is_finalized, c.is_archived, c.has_free_space, c.grid_size,
		       r.id, r.user_id, r.card_id, r.enabled, r.frequency, r.schedule, r.include_image,
		       r.include_recommendations, r.next_send_at, r.last_sent_at, r.created_at, r.updated_at
		  FROM bingo_cards c
		  LEFT JOIN card_checkin_reminders r
		    ON r.card_id = c.id AND r.user_id = $1
		 WHERE c.user_id = $1
		   AND c.is_finalized = true
		   AND c.is_archived = false
		 ORDER BY c.year DESC, c.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list card checkins: %w", err)
	}
	defer rows.Close()

	var summaries []models.CardCheckinSummary
	for rows.Next() {
		var summary models.CardCheckinSummary
		var checkinID *uuid.UUID
		var checkinUserID *uuid.UUID
		var checkinCardID *uuid.UUID
		var enabled *bool
		var frequency *string
		var schedule []byte
		var includeImage *bool
		var includeRecommendations *bool
		var nextSendAt *time.Time
		var lastSentAt *time.Time
		var checkinCreatedAt *time.Time
		var checkinUpdatedAt *time.Time

		if err := rows.Scan(
			&summary.CardID,
			&summary.CardTitle,
			&summary.CardYear,
			&summary.IsFinalized,
			&summary.IsArchived,
			&summary.HasFreeSpace,
			&summary.GridSize,
			&checkinID,
			&checkinUserID,
			&checkinCardID,
			&enabled,
			&frequency,
			&schedule,
			&includeImage,
			&includeRecommendations,
			&nextSendAt,
			&lastSentAt,
			&checkinCreatedAt,
			&checkinUpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan card checkin: %w", err)
		}

		if checkinID != nil {
			checkin := &models.CardCheckinReminder{
				ID:                     *checkinID,
				UserID:                 derefUUID(checkinUserID),
				CardID:                 derefUUID(checkinCardID),
				Enabled:                derefBool(enabled),
				Frequency:              derefString(frequency),
				Schedule:               schedule,
				IncludeImage:           derefBool(includeImage),
				IncludeRecommendations: derefBool(includeRecommendations),
				NextSendAt:             nextSendAt,
				LastSentAt:             lastSentAt,
				CreatedAt:              derefTime(checkinCreatedAt),
				UpdatedAt:              derefTime(checkinUpdatedAt),
			}
			summary.Checkin = checkin
		}

		summaries = append(summaries, summary)
	}

	if summaries == nil {
		summaries = []models.CardCheckinSummary{}
	}
	return summaries, nil
}

func (s *ReminderService) UpsertCardCheckin(ctx context.Context, userID, cardID uuid.UUID, input models.CardCheckinScheduleInput) (*models.CardCheckinReminder, error) {
	frequency := strings.TrimSpace(input.Frequency)
	if frequency == "" {
		frequency = "monthly"
	}
	if frequency != "monthly" {
		return nil, ErrInvalidSchedule
	}

	if err := s.ensureCardEligible(ctx, userID, cardID); err != nil {
		return nil, err
	}
	if err := s.ensureSettingsRow(ctx, userID); err != nil {
		return nil, err
	}

	schedule, err := parseMonthlySchedule(input.Schedule)
	if err != nil {
		return nil, err
	}

	nextSendAt, err := nextMonthlySend(s.now(), schedule)
	if err != nil {
		return nil, err
	}

	includeImage := true
	includeRecommendations := true
	if input.IncludeImage != nil {
		includeImage = *input.IncludeImage
	}
	if input.IncludeRecommendations != nil {
		includeRecommendations = *input.IncludeRecommendations
	}

	scheduleJSON, err := json.Marshal(schedule)
	if err != nil {
		return nil, fmt.Errorf("encode schedule: %w", err)
	}

	reminder := &models.CardCheckinReminder{}
	err = s.db.QueryRow(ctx, `
		INSERT INTO card_checkin_reminders
			(user_id, card_id, frequency, schedule, include_image, include_recommendations, next_send_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, card_id)
		DO UPDATE SET frequency = EXCLUDED.frequency,
		              schedule = EXCLUDED.schedule,
		              include_image = EXCLUDED.include_image,
		              include_recommendations = EXCLUDED.include_recommendations,
		              enabled = true,
		              next_send_at = EXCLUDED.next_send_at,
		              updated_at = NOW()
		RETURNING id, user_id, card_id, enabled, frequency, schedule, include_image,
		          include_recommendations, next_send_at, last_sent_at, created_at, updated_at`,
		userID, cardID, frequency, scheduleJSON, includeImage, includeRecommendations, nextSendAt,
	).Scan(
		&reminder.ID,
		&reminder.UserID,
		&reminder.CardID,
		&reminder.Enabled,
		&reminder.Frequency,
		&reminder.Schedule,
		&reminder.IncludeImage,
		&reminder.IncludeRecommendations,
		&reminder.NextSendAt,
		&reminder.LastSentAt,
		&reminder.CreatedAt,
		&reminder.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert card checkin: %w", err)
	}

	return reminder, nil
}

func (s *ReminderService) DeleteCardCheckin(ctx context.Context, userID, cardID uuid.UUID) error {
	result, err := s.db.Exec(ctx,
		"DELETE FROM card_checkin_reminders WHERE user_id = $1 AND card_id = $2",
		userID,
		cardID,
	)
	if err != nil {
		return fmt.Errorf("delete card checkin: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrReminderNotFound
	}
	return nil
}

func (s *ReminderService) ListGoalReminders(ctx context.Context, userID uuid.UUID, cardID *uuid.UUID) ([]models.GoalReminderSummary, error) {
	query := `
		SELECT gr.id, gr.card_id, gr.item_id, gr.kind, gr.schedule, gr.next_send_at, gr.last_sent_at,
		       c.title, c.year, i.content
		  FROM goal_reminders gr
		  JOIN bingo_items i ON i.id = gr.item_id
		  JOIN bingo_cards c ON c.id = gr.card_id
		 WHERE gr.user_id = $1
		   AND gr.enabled = true
		   AND ($2::uuid IS NULL OR gr.card_id = $2)
		 ORDER BY gr.next_send_at NULLS LAST, gr.created_at DESC`

	var cardFilter any
	if cardID != nil {
		cardFilter = *cardID
	}

	rows, err := s.db.Query(ctx, query, userID, cardFilter)
	if err != nil {
		return nil, fmt.Errorf("list goal reminders: %w", err)
	}
	defer rows.Close()

	var reminders []models.GoalReminderSummary
	for rows.Next() {
		var reminder models.GoalReminderSummary
		if err := rows.Scan(
			&reminder.ID,
			&reminder.CardID,
			&reminder.ItemID,
			&reminder.Kind,
			&reminder.Schedule,
			&reminder.NextSendAt,
			&reminder.LastSentAt,
			&reminder.CardTitle,
			&reminder.CardYear,
			&reminder.ItemText,
		); err != nil {
			return nil, fmt.Errorf("scan goal reminder: %w", err)
		}
		reminders = append(reminders, reminder)
	}

	if reminders == nil {
		reminders = []models.GoalReminderSummary{}
	}
	return reminders, nil
}

func (s *ReminderService) UpsertGoalReminder(ctx context.Context, userID uuid.UUID, input models.GoalReminderInput) (*models.GoalReminder, error) {
	if input.ItemID == uuid.Nil {
		return nil, ErrInvalidSchedule
	}
	kind := strings.TrimSpace(input.Kind)
	if kind == "" {
		kind = "one_time"
	}
	if kind != "one_time" {
		return nil, ErrInvalidSchedule
	}

	if err := s.ensureSettingsRow(ctx, userID); err != nil {
		return nil, err
	}

	cardID, completed, finalized, archived, err := s.loadGoalCardState(ctx, userID, input.ItemID)
	if err != nil {
		return nil, err
	}
	if !finalized || archived {
		return nil, ErrCardNotEligible
	}
	if completed {
		return nil, ErrGoalCompleted
	}

	sendAt, err := parseOneTimeSchedule(input.Schedule, s.now())
	if err != nil {
		return nil, err
	}

	scheduleJSON, err := json.Marshal(oneTimeSchedule{SendAt: sendAt.UTC().Format(time.RFC3339)})
	if err != nil {
		return nil, fmt.Errorf("encode schedule: %w", err)
	}

	reminder := &models.GoalReminder{}
	if err := s.db.QueryRow(ctx, `
		INSERT INTO goal_reminders (user_id, card_id, item_id, kind, schedule, next_send_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, item_id)
		DO UPDATE SET kind = EXCLUDED.kind,
		              schedule = EXCLUDED.schedule,
		              enabled = true,
		              next_send_at = EXCLUDED.next_send_at,
		              updated_at = NOW()
		RETURNING id, user_id, card_id, item_id, enabled, kind, schedule, next_send_at,
		          last_sent_at, created_at, updated_at`,
		userID, cardID, input.ItemID, kind, scheduleJSON, sendAt.UTC(),
	).Scan(
		&reminder.ID,
		&reminder.UserID,
		&reminder.CardID,
		&reminder.ItemID,
		&reminder.Enabled,
		&reminder.Kind,
		&reminder.Schedule,
		&reminder.NextSendAt,
		&reminder.LastSentAt,
		&reminder.CreatedAt,
		&reminder.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("upsert goal reminder: %w", err)
	}

	return reminder, nil
}

func (s *ReminderService) DeleteGoalReminder(ctx context.Context, userID, reminderID uuid.UUID) error {
	result, err := s.db.Exec(ctx,
		"DELETE FROM goal_reminders WHERE id = $1 AND user_id = $2",
		reminderID,
		userID,
	)
	if err != nil {
		return fmt.Errorf("delete goal reminder: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrReminderNotFound
	}
	return nil
}

func (s *ReminderService) SendTestEmail(ctx context.Context, userID, cardID uuid.UUID) error {
	settings, err := s.GetSettings(ctx, userID)
	if err != nil {
		return err
	}
	if !settings.EmailEnabled {
		return ErrRemindersDisabled
	}

	verified, err := s.isEmailVerified(ctx, userID)
	if err != nil {
		return err
	}
	if !verified {
		return ErrEmailNotVerified
	}

	card, items, err := s.loadCardWithItems(ctx, userID, cardID)
	if err != nil {
		return err
	}
	if !card.IsFinalized || card.IsArchived {
		return ErrCardNotEligible
	}

	userEmail, err := s.loadUserEmail(ctx, userID)
	if err != nil {
		return err
	}

	imageURL := ""
	if token, err := s.createImageToken(ctx, userID, cardID, true); err == nil {
		imageURL = fmt.Sprintf("%s/r/img/%s.png", s.baseURL, token)
	}

	recommendations := pickReminderRecommendations(items, card.GridSize, card.FreeSpacePos, 3)
	stats := buildReminderStats(card, items)
	unsubscribeURL, err := s.createUnsubscribeURL(ctx, userID)
	if err != nil {
		return err
	}

	subject, html, text := buildCheckinEmail(checkinEmailParams{
		Card:            card,
		Stats:           stats,
		Recommendations: recommendations,
		BaseURL:         s.baseURL,
		ImageURL:        imageURL,
		UnsubscribeURL:  unsubscribeURL,
		IsTest:          true,
	})

	if s.emailService == nil {
		return fmt.Errorf("email service not configured")
	}
	return s.emailService.SendNotificationEmail(ctx, userEmail, subject, html, text)
}

func (s *ReminderService) RunDue(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}

	sent := 0
	checkinSent, err := s.runDueCheckins(ctx, now, limit)
	if err != nil {
		return sent, err
	}
	sent += checkinSent

	goalSent, err := s.runDueGoals(ctx, now, limit)
	if err != nil {
		return sent, err
	}
	sent += goalSent

	return sent, nil
}

func (s *ReminderService) CleanupOld(ctx context.Context) error {
	if _, err := s.db.Exec(ctx, "DELETE FROM reminder_image_tokens WHERE expires_at < NOW()"); err != nil {
		return fmt.Errorf("cleanup reminder image tokens: %w", err)
	}
	if _, err := s.db.Exec(ctx, "DELETE FROM reminder_unsubscribe_tokens WHERE expires_at < NOW()"); err != nil {
		return fmt.Errorf("cleanup reminder unsubscribe tokens: %w", err)
	}
	if _, err := s.db.Exec(ctx, "DELETE FROM reminder_email_log WHERE sent_at < NOW() - INTERVAL '90 days'"); err != nil {
		return fmt.Errorf("cleanup reminder email log: %w", err)
	}
	return nil
}

func (s *ReminderService) RenderImageByToken(ctx context.Context, token string) ([]byte, error) {
	imageToken, err := s.loadImageToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if imageToken.ExpiresAt.Before(s.now()) {
		return nil, ErrReminderNotFound
	}

	card, items, err := s.loadCardWithItems(ctx, imageToken.UserID, imageToken.CardID)
	if err != nil {
		if errors.Is(err, ErrCardNotFound) {
			return nil, ErrReminderNotFound
		}
		return nil, err
	}

	pngBytes, err := RenderReminderPNG(*card, items, RenderOptions{
		ShowCompletions: imageToken.ShowCompletions,
	})
	if err != nil {
		return nil, err
	}

	if err := s.touchImageToken(ctx, token); err != nil {
		logging.Warn("Failed to record reminder image access", map[string]interface{}{"error": err.Error()})
	}

	return pngBytes, nil
}

func (s *ReminderService) UnsubscribeByToken(ctx context.Context, token string) (bool, error) {
	var userID uuid.UUID
	var expiresAt time.Time
	var usedAt *time.Time
	err := s.db.QueryRow(ctx,
		"SELECT user_id, expires_at, used_at FROM reminder_unsubscribe_tokens WHERE token = $1",
		token,
	).Scan(&userID, &expiresAt, &usedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrReminderNotFound
	}
	if err != nil {
		return false, fmt.Errorf("load unsubscribe token: %w", err)
	}
	if usedAt != nil {
		return true, nil
	}
	if expiresAt.Before(s.now()) {
		return false, ErrReminderNotFound
	}

	if err := s.ensureSettingsRow(ctx, userID); err != nil {
		return false, err
	}

	var wasEnabled bool
	if err := s.db.QueryRow(ctx,
		"SELECT email_enabled FROM reminder_settings WHERE user_id = $1",
		userID,
	).Scan(&wasEnabled); err != nil {
		return false, fmt.Errorf("load reminder enabled: %w", err)
	}

	if _, err := s.db.Exec(ctx,
		"UPDATE reminder_settings SET email_enabled = false, updated_at = NOW() WHERE user_id = $1",
		userID,
	); err != nil {
		return false, fmt.Errorf("disable reminder settings: %w", err)
	}
	if _, err := s.db.Exec(ctx,
		"UPDATE reminder_unsubscribe_tokens SET used_at = NOW() WHERE token = $1",
		token,
	); err != nil {
		return false, fmt.Errorf("mark unsubscribe token used: %w", err)
	}

	return !wasEnabled, nil
}

func (s *ReminderService) runDueCheckins(ctx context.Context, now time.Time, limit int) (int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin checkin reminder tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT r.id, r.user_id, r.card_id, r.frequency, r.schedule, r.include_image,
		       r.include_recommendations, r.next_send_at
		  FROM card_checkin_reminders r
		  JOIN reminder_settings s ON s.user_id = r.user_id
		  JOIN users u ON u.id = r.user_id
		 WHERE r.enabled = true
		   AND r.next_send_at <= $1
		   AND s.email_enabled = true
		   AND u.email_verified = true
		 ORDER BY r.next_send_at ASC
		 LIMIT $2
		 FOR UPDATE SKIP LOCKED`,
		now,
		limit,
	)
	if err != nil {
		return 0, fmt.Errorf("query due card checkins: %w", err)
	}
	defer rows.Close()

	var jobs []checkinJob
	for rows.Next() {
		var job checkinJob
		if err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.CardID,
			&job.Frequency,
			&job.Schedule,
			&job.IncludeImage,
			&job.IncludeRecommendations,
			&job.NextSendAt,
		); err != nil {
			return 0, fmt.Errorf("scan checkin job: %w", err)
		}
		jobs = append(jobs, job)
	}

	sent := 0
	for _, job := range jobs {
		ok, err := s.processCheckin(ctx, tx, job, now)
		if err != nil {
			logging.Error("Failed to process card checkin", map[string]interface{}{"error": err.Error()})
			continue
		}
		if ok {
			sent++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return sent, fmt.Errorf("commit checkin reminder tx: %w", err)
	}
	return sent, nil
}

func (s *ReminderService) runDueGoals(ctx context.Context, now time.Time, limit int) (int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin goal reminder tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT gr.id, gr.user_id, gr.card_id, gr.item_id, gr.kind, gr.schedule, gr.next_send_at
		  FROM goal_reminders gr
		  JOIN reminder_settings s ON s.user_id = gr.user_id
		  JOIN users u ON u.id = gr.user_id
		 WHERE gr.enabled = true
		   AND gr.next_send_at <= $1
		   AND s.email_enabled = true
		   AND u.email_verified = true
		 ORDER BY gr.next_send_at ASC
		 LIMIT $2
		 FOR UPDATE SKIP LOCKED`,
		now,
		limit,
	)
	if err != nil {
		return 0, fmt.Errorf("query due goal reminders: %w", err)
	}
	defer rows.Close()

	var jobs []goalReminderJob
	for rows.Next() {
		var job goalReminderJob
		if err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.CardID,
			&job.ItemID,
			&job.Kind,
			&job.Schedule,
			&job.NextSendAt,
		); err != nil {
			return 0, fmt.Errorf("scan goal reminder job: %w", err)
		}
		jobs = append(jobs, job)
	}

	sent := 0
	for _, job := range jobs {
		ok, err := s.processGoalReminder(ctx, tx, job, now)
		if err != nil {
			logging.Error("Failed to process goal reminder", map[string]interface{}{"error": err.Error()})
			continue
		}
		if ok {
			sent++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return sent, fmt.Errorf("commit goal reminder tx: %w", err)
	}
	return sent, nil
}

func (s *ReminderService) processCheckin(ctx context.Context, tx Tx, job checkinJob, now time.Time) (bool, error) {
	card, items, err := s.loadCardWithItemsTx(ctx, tx, job.UserID, job.CardID)
	if err != nil {
		if errors.Is(err, ErrCardNotFound) {
			return s.disableCheckin(ctx, tx, job.ID)
		}
		return false, err
	}
	if !card.IsFinalized || card.IsArchived {
		return s.disableCheckin(ctx, tx, job.ID)
	}

	if err := s.lockReminderSettings(ctx, tx, job.UserID); err != nil {
		return false, err
	}

	capReached, err := s.cardCheckinCapReached(ctx, job.UserID, now)
	if err != nil {
		return false, err
	}
	if capReached {
		if err := s.deferCheckinAfterCapReached(ctx, tx, job, now); err != nil {
			return false, err
		}
		return false, nil
	}

	stats := buildReminderStats(card, items)
	var recommendations []models.BingoItem
	if job.IncludeRecommendations {
		recommendations = pickReminderRecommendations(items, card.GridSize, card.FreeSpacePos, 3)
	}

	imageURL := ""
	if job.IncludeImage {
		if token, err := s.createImageToken(ctx, job.UserID, job.CardID, true); err == nil {
			imageURL = fmt.Sprintf("%s/r/img/%s.png", s.baseURL, token)
		}
	}

	userEmail, err := s.loadUserEmail(ctx, job.UserID)
	if err != nil {
		return false, err
	}

	unsubscribeURL, err := s.createUnsubscribeURL(ctx, job.UserID)
	if err != nil {
		return false, err
	}
	subject, html, text := buildCheckinEmail(checkinEmailParams{
		Card:            card,
		Stats:           stats,
		Recommendations: recommendations,
		BaseURL:         s.baseURL,
		ImageURL:        imageURL,
		UnsubscribeURL:  unsubscribeURL,
		IsTest:          false,
	})

	sent := false
	status := reminderEmailSent
	if s.emailService == nil {
		status = reminderEmailFailed
	} else if err := s.emailService.SendNotificationEmail(ctx, userEmail, subject, html, text); err != nil {
		status = reminderEmailFailed
	} else {
		sent = true
	}

	if sent {
		nextSendAt, err := s.nextCheckinSendAt(now, job)
		if err != nil {
			return sent, err
		}
		if err := s.updateCheckinAfterSend(ctx, tx, job.ID, now, nextSendAt); err != nil {
			return sent, err
		}
	} else {
		if err := s.deferCheckinAfterFailure(ctx, tx, job.ID, now); err != nil {
			return sent, err
		}
	}

	if err := s.logReminderEmail(ctx, tx, job.UserID, "card_checkin", job.ID, status, now); err != nil {
		return sent, err
	}

	return sent, nil
}

func (s *ReminderService) processGoalReminder(ctx context.Context, tx Tx, job goalReminderJob, now time.Time) (bool, error) {
	ctxData, err := s.loadGoalReminderContext(ctx, job.UserID, job.ItemID)
	if err != nil {
		if errors.Is(err, ErrItemNotFound) {
			return s.disableGoalReminder(ctx, tx, job.ID)
		}
		return false, err
	}
	if !ctxData.CardFinalized || ctxData.CardArchived {
		return s.disableGoalReminder(ctx, tx, job.ID)
	}
	if ctxData.ItemCompleted {
		return s.disableGoalReminder(ctx, tx, job.ID)
	}

	if err := s.lockReminderSettings(ctx, tx, job.UserID); err != nil {
		return false, err
	}

	capReached, err := s.goalReminderCapReached(ctx, job.UserID, now, ctxData.DailyCap)
	if err != nil {
		return false, err
	}
	if capReached {
		if err := s.deferGoalAfterCapReached(ctx, tx, job, now); err != nil {
			return false, err
		}
		return false, nil
	}

	unsubscribeURL, err := s.createUnsubscribeURL(ctx, job.UserID)
	if err != nil {
		return false, err
	}
	subject, html, text := buildGoalReminderEmail(goalReminderEmailParams{
		CardID:         ctxData.CardID,
		ItemID:         job.ItemID,
		CardTitle:      ctxData.CardTitle,
		CardYear:       ctxData.CardYear,
		GoalText:       ctxData.ItemContent,
		BaseURL:        s.baseURL,
		UnsubscribeURL: unsubscribeURL,
	})

	sent := false
	status := reminderEmailSent
	if s.emailService == nil {
		status = reminderEmailFailed
	} else if err := s.emailService.SendNotificationEmail(ctx, ctxData.UserEmail, subject, html, text); err != nil {
		status = reminderEmailFailed
	} else {
		sent = true
	}

	if sent {
		if err := s.markGoalReminderSent(ctx, tx, job.ID, now); err != nil {
			return sent, err
		}
	} else {
		if err := s.deferGoalAfterFailure(ctx, tx, job.ID, now); err != nil {
			return sent, err
		}
	}

	if err := s.logReminderEmail(ctx, tx, job.UserID, "goal_reminder", job.ID, status, now); err != nil {
		return sent, err
	}

	return sent, nil
}

func (s *ReminderService) nextCheckinSendAt(now time.Time, job checkinJob) (time.Time, error) {
	var schedule monthlySchedule
	if err := json.Unmarshal(job.Schedule, &schedule); err != nil {
		return time.Time{}, ErrInvalidSchedule
	}
	return nextMonthlySend(now, schedule)
}

func (s *ReminderService) updateCheckinAfterSend(ctx context.Context, tx Tx, reminderID uuid.UUID, sentAt, nextSendAt time.Time) error {
	_, err := tx.Exec(ctx,
		"UPDATE card_checkin_reminders SET last_sent_at = $1, next_send_at = $2, updated_at = NOW() WHERE id = $3",
		sentAt,
		nextSendAt,
		reminderID,
	)
	if err != nil {
		return fmt.Errorf("update card checkin: %w", err)
	}
	return nil
}

func (s *ReminderService) deferCheckinAfterFailure(ctx context.Context, tx Tx, reminderID uuid.UUID, now time.Time) error {
	if tx == nil {
		return fmt.Errorf("defer checkin after failure: missing transaction")
	}
	next := now.Add(15 * time.Minute)
	_, err := tx.Exec(ctx,
		"UPDATE card_checkin_reminders SET next_send_at = $1, updated_at = NOW() WHERE id = $2",
		next,
		reminderID,
	)
	if err != nil {
		return fmt.Errorf("defer card checkin after failure: %w", err)
	}
	return nil
}

func (s *ReminderService) deferCheckinAfterCapReached(ctx context.Context, tx Tx, job checkinJob, now time.Time) error {
	var schedule monthlySchedule
	if err := json.Unmarshal(job.Schedule, &schedule); err != nil {
		return ErrInvalidSchedule
	}
	parsed, err := time.Parse("15:04", schedule.Time)
	if err != nil {
		return ErrInvalidSchedule
	}

	loc := now.Location()
	nextDay := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, loc).AddDate(0, 0, 1)
	_, err = tx.Exec(ctx,
		"UPDATE card_checkin_reminders SET next_send_at = $1, updated_at = NOW() WHERE id = $2",
		nextDay,
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("defer card checkin after cap: %w", err)
	}
	return nil
}

func (s *ReminderService) deferGoalAfterFailure(ctx context.Context, tx Tx, reminderID uuid.UUID, now time.Time) error {
	if tx == nil {
		return fmt.Errorf("defer goal reminder after failure: missing transaction")
	}
	next := now.Add(15 * time.Minute)
	_, err := tx.Exec(ctx,
		"UPDATE goal_reminders SET next_send_at = $1, updated_at = NOW() WHERE id = $2",
		next,
		reminderID,
	)
	if err != nil {
		return fmt.Errorf("defer goal reminder after failure: %w", err)
	}
	return nil
}

func (s *ReminderService) deferGoalAfterCapReached(ctx context.Context, tx Tx, job goalReminderJob, now time.Time) error {
	if tx == nil {
		return fmt.Errorf("defer goal reminder after cap: missing transaction")
	}

	base := job.NextSendAt
	loc := base.Location()
	nextDay := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), base.Hour(), base.Minute(), 0, 0, loc).AddDate(0, 0, 1)
	_, err := tx.Exec(ctx,
		"UPDATE goal_reminders SET next_send_at = $1, updated_at = NOW() WHERE id = $2",
		nextDay,
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("defer goal reminder after cap: %w", err)
	}
	return nil
}

func (s *ReminderService) markGoalReminderSent(ctx context.Context, tx Tx, reminderID uuid.UUID, sentAt time.Time) error {
	if tx == nil {
		return fmt.Errorf("mark goal reminder sent: missing transaction")
	}
	_, err := tx.Exec(ctx,
		"UPDATE goal_reminders SET last_sent_at = $1, enabled = false, next_send_at = NULL, updated_at = NOW() WHERE id = $2",
		sentAt,
		reminderID,
	)
	if err != nil {
		return fmt.Errorf("mark goal reminder sent: %w", err)
	}
	return nil
}

func (s *ReminderService) logReminderEmail(ctx context.Context, tx Tx, userID uuid.UUID, sourceType string, sourceID uuid.UUID, status reminderEmailStatus, sentAt time.Time) error {
	sentOn := time.Date(sentAt.Year(), sentAt.Month(), sentAt.Day(), 0, 0, 0, 0, sentAt.Location())
	if tx != nil {
		err := s.upsertReminderEmailLog(ctx, tx, userID, sourceType, sourceID, status, sentAt, sentOn)
		if err != nil {
			return fmt.Errorf("log reminder email: %w", err)
		}
		return nil
	}

	err := s.upsertReminderEmailLog(ctx, s.db, userID, sourceType, sourceID, status, sentAt, sentOn)
	if err != nil {
		return fmt.Errorf("log reminder email: %w", err)
	}
	return nil
}

type reminderEmailLogDB interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
}

func (s *ReminderService) upsertReminderEmailLog(ctx context.Context, db reminderEmailLogDB, userID uuid.UUID, sourceType string, sourceID uuid.UUID, status reminderEmailStatus, sentAt, sentOn time.Time) error {
	if sourceType == "card_checkin" {
		_, err := db.Exec(ctx, `
			INSERT INTO reminder_email_log (user_id, source_type, source_id, status, sent_at, sent_on)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (source_type, source_id, sent_on) WHERE source_type = 'card_checkin'
			DO UPDATE SET status = EXCLUDED.status, sent_at = EXCLUDED.sent_at`,
			userID,
			sourceType,
			sourceID,
			status,
			sentAt,
			sentOn,
		)
		return err
	}
	_, err := db.Exec(ctx,
		"INSERT INTO reminder_email_log (user_id, source_type, source_id, status, sent_at, sent_on) VALUES ($1, $2, $3, $4, $5, $6)",
		userID,
		sourceType,
		sourceID,
		status,
		sentAt,
		sentOn,
	)
	return err
}

func (s *ReminderService) disableCheckin(ctx context.Context, tx Tx, reminderID uuid.UUID) (bool, error) {
	_, err := tx.Exec(ctx,
		"UPDATE card_checkin_reminders SET enabled = false, next_send_at = NULL, updated_at = NOW() WHERE id = $1",
		reminderID,
	)
	if err != nil {
		return false, fmt.Errorf("disable card checkin: %w", err)
	}
	return false, nil
}

func (s *ReminderService) disableGoalReminder(ctx context.Context, tx Tx, reminderID uuid.UUID) (bool, error) {
	if tx == nil {
		return false, fmt.Errorf("disable goal reminder: missing transaction")
	}
	_, err := tx.Exec(ctx,
		"UPDATE goal_reminders SET enabled = false, next_send_at = NULL, updated_at = NOW() WHERE id = $1",
		reminderID,
	)
	if err != nil {
		return false, fmt.Errorf("disable goal reminder: %w", err)
	}
	return false, nil
}

func (s *ReminderService) cardCheckinCapReached(ctx context.Context, userID uuid.UUID, now time.Time) (bool, error) {
	sentOn := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var count int
	if err := s.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM reminder_email_log WHERE user_id = $1 AND source_type = 'card_checkin' AND status = 'sent' AND sent_on = $2",
		userID,
		sentOn,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("check card checkin cap: %w", err)
	}
	return count >= 1, nil
}

func (s *ReminderService) goalReminderCapReached(ctx context.Context, userID uuid.UUID, now time.Time, cap int) (bool, error) {
	if cap <= 0 {
		cap = 3
	}
	sentOn := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var count int
	if err := s.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM reminder_email_log WHERE user_id = $1 AND source_type = 'goal_reminder' AND status = 'sent' AND sent_on = $2",
		userID,
		sentOn,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("check goal reminder cap: %w", err)
	}
	return count >= cap, nil
}

func (s *ReminderService) ensureSettingsRow(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		"INSERT INTO reminder_settings (user_id) VALUES ($1) ON CONFLICT DO NOTHING",
		userID,
	)
	if err != nil {
		return fmt.Errorf("ensure reminder settings: %w", err)
	}
	return nil
}

func (s *ReminderService) loadSettings(ctx context.Context, userID uuid.UUID) (*models.ReminderSettings, error) {
	settings := &models.ReminderSettings{}
	if err := s.db.QueryRow(ctx,
		"SELECT user_id, email_enabled, daily_email_cap, created_at, updated_at FROM reminder_settings WHERE user_id = $1",
		userID,
	).Scan(
		&settings.UserID,
		&settings.EmailEnabled,
		&settings.DailyEmailCap,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("load reminder settings: %w", err)
	}
	return settings, nil
}

func (s *ReminderService) isEmailVerified(ctx context.Context, userID uuid.UUID) (bool, error) {
	var verified bool
	if err := s.db.QueryRow(ctx, "SELECT email_verified FROM users WHERE id = $1", userID).Scan(&verified); err != nil {
		return false, fmt.Errorf("load email verification: %w", err)
	}
	return verified, nil
}

func (s *ReminderService) ensureCardEligible(ctx context.Context, userID, cardID uuid.UUID) error {
	var finalized bool
	var archived bool
	if err := s.db.QueryRow(ctx,
		"SELECT is_finalized, is_archived FROM bingo_cards WHERE id = $1 AND user_id = $2",
		cardID,
		userID,
	).Scan(&finalized, &archived); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCardNotFound
		}
		return fmt.Errorf("load card state: %w", err)
	}
	if !finalized || archived {
		return ErrCardNotEligible
	}
	return nil
}

func (s *ReminderService) loadGoalCardState(ctx context.Context, userID, itemID uuid.UUID) (uuid.UUID, bool, bool, bool, error) {
	var cardID uuid.UUID
	var completed bool
	var finalized bool
	var archived bool
	if err := s.db.QueryRow(ctx, `
		SELECT i.card_id, i.is_completed, c.is_finalized, c.is_archived
		  FROM bingo_items i
		  JOIN bingo_cards c ON c.id = i.card_id
		 WHERE i.id = $1 AND c.user_id = $2`,
		itemID,
		userID,
	).Scan(&cardID, &completed, &finalized, &archived); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, false, false, ErrItemNotFound
		}
		return uuid.Nil, false, false, false, fmt.Errorf("load goal reminder card state: %w", err)
	}
	return cardID, completed, finalized, archived, nil
}

func (s *ReminderService) loadCardWithItems(ctx context.Context, userID, cardID uuid.UUID) (*models.BingoCard, []models.BingoItem, error) {
	card := &models.BingoCard{}
	if err := s.db.QueryRow(ctx, `
		SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		       is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
		  FROM bingo_cards WHERE id = $1 AND user_id = $2`,
		cardID,
		userID,
	).Scan(
		&card.ID,
		&card.UserID,
		&card.Year,
		&card.Category,
		&card.Title,
		&card.GridSize,
		&card.HeaderText,
		&card.HasFreeSpace,
		&card.FreeSpacePos,
		&card.IsActive,
		&card.IsFinalized,
		&card.VisibleToFriends,
		&card.IsArchived,
		&card.CreatedAt,
		&card.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrCardNotFound
		}
		return nil, nil, fmt.Errorf("load card: %w", err)
	}

	items, err := s.loadItemsForCard(ctx, cardID)
	if err != nil {
		return nil, nil, err
	}
	card.Items = items
	return card, items, nil
}

func (s *ReminderService) loadCardWithItemsTx(ctx context.Context, tx Tx, userID, cardID uuid.UUID) (*models.BingoCard, []models.BingoItem, error) {
	card := &models.BingoCard{}
	if err := tx.QueryRow(ctx, `
		SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		       is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
		  FROM bingo_cards WHERE id = $1 AND user_id = $2`,
		cardID,
		userID,
	).Scan(
		&card.ID,
		&card.UserID,
		&card.Year,
		&card.Category,
		&card.Title,
		&card.GridSize,
		&card.HeaderText,
		&card.HasFreeSpace,
		&card.FreeSpacePos,
		&card.IsActive,
		&card.IsFinalized,
		&card.VisibleToFriends,
		&card.IsArchived,
		&card.CreatedAt,
		&card.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrCardNotFound
		}
		return nil, nil, fmt.Errorf("load card: %w", err)
	}

	items, err := s.loadItemsForCardTx(ctx, tx, cardID)
	if err != nil {
		return nil, nil, err
	}
	card.Items = items
	return card, items, nil
}

func (s *ReminderService) loadItemsForCard(ctx context.Context, cardID uuid.UUID) ([]models.BingoItem, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, card_id, position, content, is_completed, completed_at, notes, proof_url, created_at FROM bingo_items WHERE card_id = $1 ORDER BY position",
		cardID,
	)
	if err != nil {
		return nil, fmt.Errorf("load card items: %w", err)
	}
	defer rows.Close()

	var items []models.BingoItem
	for rows.Next() {
		var item models.BingoItem
		if err := rows.Scan(
			&item.ID,
			&item.CardID,
			&item.Position,
			&item.Content,
			&item.IsCompleted,
			&item.CompletedAt,
			&item.Notes,
			&item.ProofURL,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan card item: %w", err)
		}
		items = append(items, item)
	}
	if items == nil {
		items = []models.BingoItem{}
	}
	return items, nil
}

func (s *ReminderService) loadItemsForCardTx(ctx context.Context, tx Tx, cardID uuid.UUID) ([]models.BingoItem, error) {
	rows, err := tx.Query(ctx,
		"SELECT id, card_id, position, content, is_completed, completed_at, notes, proof_url, created_at FROM bingo_items WHERE card_id = $1 ORDER BY position",
		cardID,
	)
	if err != nil {
		return nil, fmt.Errorf("load card items: %w", err)
	}
	defer rows.Close()

	var items []models.BingoItem
	for rows.Next() {
		var item models.BingoItem
		if err := rows.Scan(
			&item.ID,
			&item.CardID,
			&item.Position,
			&item.Content,
			&item.IsCompleted,
			&item.CompletedAt,
			&item.Notes,
			&item.ProofURL,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan card item: %w", err)
		}
		items = append(items, item)
	}
	if items == nil {
		items = []models.BingoItem{}
	}
	return items, nil
}

func (s *ReminderService) loadUserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	var email string
	if err := s.db.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", userID).Scan(&email); err != nil {
		return "", fmt.Errorf("load user email: %w", err)
	}
	return email, nil
}

type goalReminderContext struct {
	CardID        uuid.UUID
	CardTitle     *string
	CardYear      int
	CardFinalized bool
	CardArchived  bool
	ItemContent   string
	ItemCompleted bool
	UserEmail     string
	DailyCap      int
}

func (s *ReminderService) loadGoalReminderContext(ctx context.Context, userID, itemID uuid.UUID) (*goalReminderContext, error) {
	ctxData := &goalReminderContext{}
	if err := s.db.QueryRow(ctx, `
		SELECT c.id, c.title, c.year, c.is_finalized, c.is_archived,
		       i.content, i.is_completed,
		       u.email,
		       rs.daily_email_cap
		  FROM bingo_items i
		  JOIN bingo_cards c ON c.id = i.card_id
		  JOIN users u ON u.id = c.user_id
		  JOIN reminder_settings rs ON rs.user_id = u.id
		 WHERE i.id = $1 AND c.user_id = $2`,
		itemID,
		userID,
	).Scan(
		&ctxData.CardID,
		&ctxData.CardTitle,
		&ctxData.CardYear,
		&ctxData.CardFinalized,
		&ctxData.CardArchived,
		&ctxData.ItemContent,
		&ctxData.ItemCompleted,
		&ctxData.UserEmail,
		&ctxData.DailyCap,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemNotFound
		}
		return nil, fmt.Errorf("load goal reminder context: %w", err)
	}
	return ctxData, nil
}

func (s *ReminderService) lockReminderSettings(ctx context.Context, tx Tx, userID uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("lock reminder settings: missing transaction")
	}
	var locked uuid.UUID
	if err := tx.QueryRow(ctx,
		"SELECT user_id FROM reminder_settings WHERE user_id = $1 FOR UPDATE",
		userID,
	).Scan(&locked); err != nil {
		return fmt.Errorf("lock reminder settings: %w", err)
	}
	return nil
}

func (s *ReminderService) createImageToken(ctx context.Context, userID, cardID uuid.UUID, showCompletions bool) (string, error) {
	var existing string
	if err := s.db.QueryRow(ctx, `
		SELECT token
		  FROM reminder_image_tokens
		 WHERE user_id = $1
		   AND card_id = $2
		   AND show_completions = $3
		   AND expires_at > NOW()
		 ORDER BY expires_at DESC
		 LIMIT 1`,
		userID,
		cardID,
		showCompletions,
	).Scan(&existing); err == nil && existing != "" {
		if _, err := s.db.Exec(ctx,
			"UPDATE reminder_image_tokens SET expires_at = $1 WHERE token = $2",
			s.now().Add(14*24*time.Hour),
			existing,
		); err != nil {
			return "", fmt.Errorf("refresh reminder image token: %w", err)
		}
		return existing, nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("load reminder image token: %w", err)
	}

	token, err := randomToken(24)
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO reminder_image_tokens (token, user_id, card_id, show_completions, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		token,
		userID,
		cardID,
		showCompletions,
		s.now().Add(14*24*time.Hour),
	)
	if err != nil {
		return "", fmt.Errorf("create reminder image token: %w", err)
	}
	return token, nil
}

func (s *ReminderService) loadImageToken(ctx context.Context, token string) (*models.ReminderImageToken, error) {
	imageToken := &models.ReminderImageToken{}
	if err := s.db.QueryRow(ctx, `
		SELECT token, user_id, card_id, show_completions, expires_at, created_at, last_accessed_at, access_count
		  FROM reminder_image_tokens WHERE token = $1`,
		token,
	).Scan(
		&imageToken.Token,
		&imageToken.UserID,
		&imageToken.CardID,
		&imageToken.ShowCompletions,
		&imageToken.ExpiresAt,
		&imageToken.CreatedAt,
		&imageToken.LastAccessedAt,
		&imageToken.AccessCount,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReminderNotFound
		}
		return nil, fmt.Errorf("load reminder image token: %w", err)
	}
	return imageToken, nil
}

func (s *ReminderService) touchImageToken(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE reminder_image_tokens SET last_accessed_at = NOW(), access_count = access_count + 1 WHERE token = $1",
		token,
	)
	if err != nil {
		return fmt.Errorf("touch reminder image token: %w", err)
	}
	return nil
}

func (s *ReminderService) createUnsubscribeURL(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := randomToken(24)
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(ctx,
		"INSERT INTO reminder_unsubscribe_tokens (token, user_id, expires_at) VALUES ($1, $2, $3)",
		token,
		userID,
		s.now().Add(30*24*time.Hour),
	)
	if err != nil {
		return "", fmt.Errorf("create unsubscribe token: %w", err)
	}
	return fmt.Sprintf("%s/r/unsubscribe?token=%s", s.baseURL, token), nil
}

func randomToken(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func parseMonthlySchedule(input models.CardCheckinSchedulePayload) (monthlySchedule, error) {
	if input.DayOfMonth < 1 {
		return monthlySchedule{}, ErrInvalidSchedule
	}
	day := input.DayOfMonth
	if day > 28 {
		day = 28
	}
	if strings.TrimSpace(input.Time) == "" {
		return monthlySchedule{}, ErrInvalidSchedule
	}
	if _, err := time.Parse("15:04", input.Time); err != nil {
		return monthlySchedule{}, ErrInvalidSchedule
	}
	return monthlySchedule{DayOfMonth: day, Time: input.Time}, nil
}

func nextMonthlySend(after time.Time, schedule monthlySchedule) (time.Time, error) {
	if schedule.DayOfMonth < 1 {
		return time.Time{}, ErrInvalidSchedule
	}
	day := schedule.DayOfMonth
	if day > 28 {
		day = 28
	}
	parsed, err := time.Parse("15:04", schedule.Time)
	if err != nil {
		return time.Time{}, ErrInvalidSchedule
	}

	year, month, _ := after.Date()
	loc := after.Location()
	candidate := time.Date(year, month, day, parsed.Hour(), parsed.Minute(), 0, 0, loc)
	if !candidate.After(after) {
		nextMonth := time.Date(year, month, 1, parsed.Hour(), parsed.Minute(), 0, 0, loc).AddDate(0, 1, 0)
		candidate = time.Date(nextMonth.Year(), nextMonth.Month(), day, parsed.Hour(), parsed.Minute(), 0, 0, loc)
	}
	return candidate, nil
}

func parseOneTimeSchedule(input models.GoalReminderScheduleInput, now time.Time) (time.Time, error) {
	if strings.TrimSpace(input.SendAt) == "" {
		return time.Time{}, ErrInvalidSchedule
	}
	if parsed, err := time.Parse(time.RFC3339, input.SendAt); err == nil {
		if !parsed.After(now) {
			return time.Time{}, ErrInvalidSchedule
		}
		return parsed, nil
	}
	localParsed, err := time.ParseInLocation("2006-01-02T15:04", input.SendAt, time.Local)
	if err != nil {
		return time.Time{}, ErrInvalidSchedule
	}
	if !localParsed.After(now) {
		return time.Time{}, ErrInvalidSchedule
	}
	return localParsed, nil
}

func buildReminderStats(card *models.BingoCard, items []models.BingoItem) reminderStats {
	capacity := card.Capacity()
	completed := 0
	for _, item := range items {
		if item.IsCompleted {
			completed++
		}
	}
	bingos := countReminderBingos(items, card.GridSize, card.FreeSpacePos)
	return reminderStats{Completed: completed, Total: capacity, Bingos: bingos}
}

func countReminderBingos(items []models.BingoItem, gridSize int, freePos *int) int {
	if !models.IsValidGridSize(gridSize) {
		gridSize = models.MaxGridSize
	}
	total := gridSize * gridSize
	grid := make([]bool, total)

	if freePos != nil && *freePos >= 0 && *freePos < total {
		grid[*freePos] = true
	}

	for _, item := range items {
		if item.IsCompleted && item.Position >= 0 && item.Position < total {
			grid[item.Position] = true
		}
	}

	bingos := 0
	for row := 0; row < gridSize; row++ {
		complete := true
		for col := 0; col < gridSize; col++ {
			if !grid[row*gridSize+col] {
				complete = false
				break
			}
		}
		if complete {
			bingos++
		}
	}
	for col := 0; col < gridSize; col++ {
		complete := true
		for row := 0; row < gridSize; row++ {
			if !grid[row*gridSize+col] {
				complete = false
				break
			}
		}
		if complete {
			bingos++
		}
	}

	complete := true
	for i := 0; i < gridSize; i++ {
		if !grid[i*gridSize+i] {
			complete = false
			break
		}
	}
	if complete {
		bingos++
	}

	complete = true
	for i := 0; i < gridSize; i++ {
		if !grid[i*gridSize+(gridSize-1-i)] {
			complete = false
			break
		}
	}
	if complete {
		bingos++
	}

	return bingos
}

func pickReminderRecommendations(items []models.BingoItem, gridSize int, freePos *int, limit int) []models.BingoItem {
	if limit <= 0 {
		return []models.BingoItem{}
	}
	if !models.IsValidGridSize(gridSize) {
		gridSize = models.MaxGridSize
	}
	itemByPos := make(map[int]models.BingoItem, len(items))
	for _, item := range items {
		itemByPos[item.Position] = item
	}

	free := -1
	if freePos != nil {
		free = *freePos
	}

	lines := buildLines(gridSize)
	minMissing := gridSize + 1
	lineMissing := make([][]int, 0, len(lines))

	for _, line := range lines {
		missing := make([]int, 0, len(line))
		for _, pos := range line {
			if pos == free {
				continue
			}
			item, ok := itemByPos[pos]
			if !ok {
				missing = append(missing, pos)
				continue
			}
			if !item.IsCompleted {
				missing = append(missing, pos)
			}
		}
		if len(missing) == 0 {
			lineMissing = append(lineMissing, nil)
			continue
		}
		if len(missing) < minMissing {
			minMissing = len(missing)
		}
		lineMissing = append(lineMissing, missing)
	}

	scores := map[int]int{}
	if minMissing <= gridSize {
		for _, missing := range lineMissing {
			if len(missing) == 0 || len(missing) != minMissing {
				continue
			}
			for _, pos := range missing {
				scores[pos]++
			}
		}
	}

	type scoredItem struct {
		Item  models.BingoItem
		Score int
	}
	var scored []scoredItem
	for pos, score := range scores {
		item, ok := itemByPos[pos]
		if !ok {
			continue
		}
		if item.IsCompleted || pos == free {
			continue
		}
		scored = append(scored, scoredItem{Item: item, Score: score})
	}

	if len(scored) == 0 {
		return fallbackRecommendations(items, free, limit)
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Item.Position < scored[j].Item.Position
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	result := make([]models.BingoItem, 0, len(scored))
	for _, entry := range scored {
		result = append(result, entry.Item)
	}
	return result
}

func fallbackRecommendations(items []models.BingoItem, freePos int, limit int) []models.BingoItem {
	candidates := make([]models.BingoItem, 0, len(items))
	for _, item := range items {
		if item.IsCompleted || item.Position == freePos {
			continue
		}
		candidates = append(candidates, item)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if !candidates[i].CreatedAt.IsZero() && !candidates[j].CreatedAt.IsZero() {
			if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
				return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
			}
		}
		return candidates[i].Position < candidates[j].Position
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}

func buildLines(gridSize int) [][]int {
	lines := make([][]int, 0, gridSize*2+2)
	for row := 0; row < gridSize; row++ {
		line := make([]int, 0, gridSize)
		for col := 0; col < gridSize; col++ {
			line = append(line, row*gridSize+col)
		}
		lines = append(lines, line)
	}
	for col := 0; col < gridSize; col++ {
		line := make([]int, 0, gridSize)
		for row := 0; row < gridSize; row++ {
			line = append(line, row*gridSize+col)
		}
		lines = append(lines, line)
	}

	line := make([]int, 0, gridSize)
	for i := 0; i < gridSize; i++ {
		line = append(line, i*gridSize+i)
	}
	lines = append(lines, line)

	line = make([]int, 0, gridSize)
	for i := 0; i < gridSize; i++ {
		line = append(line, i*gridSize+(gridSize-1-i))
	}
	lines = append(lines, line)

	return lines
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefBool(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func derefUUID(value *uuid.UUID) uuid.UUID {
	if value == nil {
		return uuid.Nil
	}
	return *value
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
