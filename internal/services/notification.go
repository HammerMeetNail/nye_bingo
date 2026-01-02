package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrEmailNotVerified     = errors.New("email not verified")
)

type NotificationListParams struct {
	Limit      int
	Before     *time.Time
	UnreadOnly bool
}

type NotificationService struct {
	db           DB
	emailService EmailServiceInterface
	baseURL      string
	async        func(fn func())
}

func NewNotificationService(db DB, emailService EmailServiceInterface, baseURL string) *NotificationService {
	trimmed := strings.TrimRight(baseURL, "/")
	return &NotificationService{
		db:           db,
		emailService: emailService,
		baseURL:      trimmed,
		async: func(fn func()) {
			go fn()
		},
	}
}

func (s *NotificationService) SetAsync(fn func(fn func())) {
	s.async = fn
}

func (s *NotificationService) GetSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error) {
	if err := s.ensureSettingsRow(ctx, userID); err != nil {
		return nil, err
	}
	return s.loadSettings(ctx, userID)
}

func (s *NotificationService) UpdateSettings(ctx context.Context, userID uuid.UUID, patch models.NotificationSettingsPatch) (*models.NotificationSettings, error) {
	if enablesEmail(patch) {
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

	setClauses := []string{}
	args := []any{}
	idx := 1

	addBool := func(column string, value *bool) {
		if value == nil {
			return
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", column, idx))
		args = append(args, *value)
		idx++
	}

	addBool("in_app_enabled", patch.InAppEnabled)
	addBool("in_app_friend_request_received", patch.InAppFriendRequestReceived)
	addBool("in_app_friend_request_accepted", patch.InAppFriendRequestAccepted)
	addBool("in_app_friend_bingo", patch.InAppFriendBingo)
	addBool("in_app_friend_new_card", patch.InAppFriendNewCard)
	addBool("email_enabled", patch.EmailEnabled)
	addBool("email_friend_request_received", patch.EmailFriendRequestReceived)
	addBool("email_friend_request_accepted", patch.EmailFriendRequestAccepted)
	addBool("email_friend_bingo", patch.EmailFriendBingo)
	addBool("email_friend_new_card", patch.EmailFriendNewCard)

	if len(setClauses) == 0 {
		return s.loadSettings(ctx, userID)
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	args = append(args, userID)
	query := fmt.Sprintf(
		"UPDATE notification_settings SET %s WHERE user_id = $%d",
		strings.Join(setClauses, ", "),
		idx,
	)

	if _, err := s.db.Exec(ctx, query, args...); err != nil {
		return nil, fmt.Errorf("updating notification settings: %w", err)
	}

	return s.loadSettings(ctx, userID)
}

func (s *NotificationService) List(ctx context.Context, userID uuid.UUID, params NotificationListParams) ([]models.Notification, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	conditions := []string{"n.user_id = $1", "n.in_app_delivered = true"}
	args := []any{userID}
	idx := 2

	if params.Before != nil {
		conditions = append(conditions, fmt.Sprintf("n.created_at < $%d", idx))
		args = append(args, *params.Before)
		idx++
	}

	if params.UnreadOnly {
		conditions = append(conditions, "n.read_at IS NULL")
	}

	query := fmt.Sprintf(
		`SELECT n.id, n.user_id, n.type, n.actor_user_id, au.username,
		        n.friendship_id, n.card_id, c.title, c.year, n.bingo_count,
		        n.in_app_delivered, n.email_delivered, n.email_sent_at, n.read_at, n.created_at
		 FROM notifications n
		 LEFT JOIN users au ON n.actor_user_id = au.id
		 LEFT JOIN bingo_cards c ON n.card_id = c.id
		 WHERE %s
		 ORDER BY n.created_at DESC
		 LIMIT $%d`,
		strings.Join(conditions, " AND "),
		idx,
	)
	args = append(args, limit)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing notifications: %w", err)
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var n models.Notification
		var nType string
		if err := rows.Scan(
			&n.ID,
			&n.UserID,
			&nType,
			&n.ActorUserID,
			&n.ActorUsername,
			&n.FriendshipID,
			&n.CardID,
			&n.CardTitle,
			&n.CardYear,
			&n.BingoCount,
			&n.InAppDelivered,
			&n.EmailDelivered,
			&n.EmailSentAt,
			&n.ReadAt,
			&n.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning notification: %w", err)
		}
		n.Type = models.NotificationType(nType)
		notifications = append(notifications, n)
	}

	if notifications == nil {
		notifications = []models.Notification{}
	}
	return notifications, nil
}

func (s *NotificationService) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	result, err := s.db.Exec(ctx,
		"UPDATE notifications SET read_at = NOW() WHERE id = $1 AND user_id = $2",
		notificationID, userID,
	)
	if err != nil {
		return fmt.Errorf("marking notification read: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		"UPDATE notifications SET read_at = NOW() WHERE user_id = $1 AND read_at IS NULL AND in_app_delivered = true",
		userID,
	)
	if err != nil {
		return fmt.Errorf("marking all notifications read: %w", err)
	}
	return nil
}

func (s *NotificationService) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL AND in_app_delivered = true",
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting unread notifications: %w", err)
	}
	return count, nil
}

func (s *NotificationService) NotifyFriendRequestReceived(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error {
	return s.notifySingle(ctx, recipientID, actorID, friendshipID, nil, nil, models.NotificationTypeFriendRequestReceived)
}

func (s *NotificationService) NotifyFriendRequestAccepted(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error {
	return s.notifySingle(ctx, recipientID, actorID, friendshipID, nil, nil, models.NotificationTypeFriendRequestAccepted)
}

func (s *NotificationService) NotifyFriendsNewCard(ctx context.Context, actorID, cardID uuid.UUID) error {
	return s.notifyFriends(ctx, actorID, cardID, nil, models.NotificationTypeFriendNewCard)
}

func (s *NotificationService) NotifyFriendsBingo(ctx context.Context, actorID, cardID uuid.UUID, bingoCount int) error {
	if bingoCount <= 0 {
		return nil
	}
	return s.notifyFriends(ctx, actorID, cardID, &bingoCount, models.NotificationTypeFriendBingo)
}

func (s *NotificationService) CleanupOld(ctx context.Context) error {
	_, err := s.db.Exec(ctx, "DELETE FROM notifications WHERE created_at < NOW() - INTERVAL '1 year'")
	if err != nil {
		return fmt.Errorf("cleanup notifications: %w", err)
	}
	return nil
}

func (s *NotificationService) notifySingle(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID, cardID *uuid.UUID, bingoCount *int, nType models.NotificationType) error {
	inAppCol, emailCol, err := notificationScenarioColumns(nType)
	if err != nil {
		return err
	}

	inAppEnabled := "COALESCE(ns.in_app_enabled, true)"
	emailEnabled := "COALESCE(ns.email_enabled, false)"
	inAppSetting := fmt.Sprintf("COALESCE(ns.%s, true)", inAppCol)
	emailSetting := fmt.Sprintf("COALESCE(ns.%s, false)", emailCol)

	query := fmt.Sprintf(
		`INSERT INTO notifications (user_id, type, actor_user_id, friendship_id, card_id, bingo_count, in_app_delivered, email_delivered)
		 SELECT u.id, $2, $3, $4, $5, $6,
		        (%s AND %s) AS in_app_delivered,
		        (%s AND %s AND u.email_verified) AS email_delivered
		 FROM users u
		 LEFT JOIN notification_settings ns ON ns.user_id = u.id
		 WHERE u.id = $1
		   AND ((%s AND %s) OR (%s AND %s AND u.email_verified))
		   AND NOT EXISTS (
		     SELECT 1 FROM user_blocks
		     WHERE (blocker_id = $1 AND blocked_id = $3)
		        OR (blocker_id = $3 AND blocked_id = $1)
		   )
		 ON CONFLICT DO NOTHING
		 RETURNING id, user_id, email_delivered`,
		inAppEnabled,
		inAppSetting,
		emailEnabled,
		emailSetting,
		inAppEnabled,
		inAppSetting,
		emailEnabled,
		emailSetting,
	)

	rows, err := s.db.Query(ctx, query, recipientID, string(nType), actorID, friendshipID, cardID, bingoCount)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	defer rows.Close()

	inserted := collectInserted(rows)
	if len(inserted.emailIDs) > 0 {
		s.dispatchEmails(inserted.emailIDs)
	}

	return nil
}

func (s *NotificationService) notifyFriends(ctx context.Context, actorID, cardID uuid.UUID, bingoCount *int, nType models.NotificationType) error {
	inAppCol, emailCol, err := notificationScenarioColumns(nType)
	if err != nil {
		return err
	}

	inAppEnabled := "COALESCE(ns.in_app_enabled, true)"
	emailEnabled := "COALESCE(ns.email_enabled, false)"
	inAppSetting := fmt.Sprintf("COALESCE(ns.%s, true)", inAppCol)
	emailSetting := fmt.Sprintf("COALESCE(ns.%s, false)", emailCol)

	query := fmt.Sprintf(
		`INSERT INTO notifications (user_id, type, actor_user_id, friendship_id, card_id, bingo_count, in_app_delivered, email_delivered)
		 SELECT f.recipient_id, $2, $1, f.id, $3, $4,
		        (%s AND %s) AS in_app_delivered,
		        (%s AND %s AND u.email_verified) AS email_delivered
		 FROM (
		   SELECT id, user_id, friend_id,
		          CASE WHEN user_id = $1 THEN friend_id ELSE user_id END AS recipient_id
		   FROM friendships
		   WHERE status = 'accepted'
		     AND (user_id = $1 OR friend_id = $1)
		 ) AS f
		 JOIN users u ON u.id = f.recipient_id
		 LEFT JOIN notification_settings ns ON ns.user_id = f.recipient_id
		 WHERE ((%s AND %s) OR (%s AND %s AND u.email_verified))
		   AND NOT EXISTS (
		     SELECT 1 FROM user_blocks
		     WHERE (blocker_id = $1 AND blocked_id = f.recipient_id)
		        OR (blocker_id = f.recipient_id AND blocked_id = $1)
		   )
		 ON CONFLICT DO NOTHING
		 RETURNING id, user_id, email_delivered`,
		inAppEnabled,
		inAppSetting,
		emailEnabled,
		emailSetting,
		inAppEnabled,
		inAppSetting,
		emailEnabled,
		emailSetting,
	)

	rows, err := s.db.Query(ctx, query, actorID, string(nType), cardID, bingoCount)
	if err != nil {
		return fmt.Errorf("insert notifications: %w", err)
	}
	defer rows.Close()

	inserted := collectInserted(rows)
	if len(inserted.emailIDs) > 0 {
		s.dispatchEmails(inserted.emailIDs)
	}

	return nil
}

func (s *NotificationService) dispatchEmails(notificationIDs []uuid.UUID) {
	if s.emailService == nil || len(notificationIDs) == 0 {
		return
	}

	if s.async == nil {
		return
	}

	s.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.sendNotificationEmails(ctx, notificationIDs)
	})
}

func (s *NotificationService) sendNotificationEmails(ctx context.Context, notificationIDs []uuid.UUID) {
	rows, err := s.db.Query(ctx,
		`SELECT n.id, n.type, u.email, u.username, au.username, n.friendship_id, c.title, c.year, n.bingo_count
		 FROM notifications n
		 JOIN users u ON n.user_id = u.id
		 LEFT JOIN users au ON n.actor_user_id = au.id
		 LEFT JOIN bingo_cards c ON n.card_id = c.id
		 WHERE n.id = ANY($1) AND n.email_delivered = true`,
		notificationIDs,
	)
	if err != nil {
		logging.Error("Failed to load notification emails", map[string]interface{}{"error": err.Error()})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var nType string
		var recipientEmail string
		var actorName *string
		var friendshipID *uuid.UUID
		var cardTitle *string
		var cardYear *int
		var bingoCount *int
		if err := rows.Scan(
			&id,
			&nType,
			&recipientEmail,
			new(string),
			&actorName,
			&friendshipID,
			&cardTitle,
			&cardYear,
			&bingoCount,
		); err != nil {
			logging.Error("Failed to scan notification email", map[string]interface{}{"error": err.Error()})
			continue
		}

		subject, html, text := s.buildNotificationEmail(models.NotificationType(nType), actorName, cardTitle, cardYear, bingoCount)
		if err := s.emailService.SendNotificationEmail(ctx, recipientEmail, subject, html, text); err != nil {
			logging.Error("Failed to send notification email", map[string]interface{}{"error": err.Error(), "notification_id": id.String()})
			continue
		}
		if _, err := s.db.Exec(ctx, "UPDATE notifications SET email_sent_at = NOW() WHERE id = $1", id); err != nil {
			logging.Error("Failed to mark notification email sent", map[string]interface{}{"error": err.Error(), "notification_id": id.String()})
		}
	}
}

func (s *NotificationService) buildNotificationEmail(nType models.NotificationType, actorName *string, cardTitle *string, cardYear *int, bingoCount *int) (string, string, string) {
	actor := "A friend"
	if actorName != nil && *actorName != "" {
		actor = *actorName
	}
	cardName := cardDisplayName(cardTitle, cardYear)

	var subject string
	var message string
	switch nType {
	case models.NotificationTypeFriendRequestReceived:
		subject = "New friend request"
		message = fmt.Sprintf("%s sent you a friend request.", actor)
	case models.NotificationTypeFriendRequestAccepted:
		subject = "Friend request accepted"
		message = fmt.Sprintf("%s accepted your friend request.", actor)
	case models.NotificationTypeFriendBingo:
		subject = "Your friend got a bingo!"
		if bingoCount != nil && *bingoCount > 0 {
			message = fmt.Sprintf("%s got a bingo on %s (%d total).", actor, cardName, *bingoCount)
		} else {
			message = fmt.Sprintf("%s got a bingo on %s.", actor, cardName)
		}
	case models.NotificationTypeFriendNewCard:
		subject = "Your friend created a new bingo card"
		message = fmt.Sprintf("%s created a new card: %s.", actor, cardName)
	default:
		subject = "New notification"
		message = "You have a new notification."
	}

	viewURL := fmt.Sprintf("%s/#notifications", s.baseURL)
	friendsURL := fmt.Sprintf("%s/#friends", s.baseURL)
	settingsURL := fmt.Sprintf("%s/#profile", s.baseURL)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h1 style="color: #333; font-size: 24px;">Year of Bingo</h1>

  <p style="font-size: 16px;">%s</p>

  <p>
    <a href="%s" style="display: inline-block; background: #4F46E5; color: white; padding: 10px 18px; text-decoration: none; border-radius: 6px; margin: 12px 0;">
      View Notifications
    </a>
  </p>

  <p style="color: #666; font-size: 14px;">
    Friends page: <a href="%s">%s</a>
  </p>

  <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
  <p style="color: #666; font-size: 14px;">Manage notification settings: <a href="%s">%s</a></p>
  <p style="color: #999; font-size: 12px;">Year of Bingo - yearofbingo.com</p>
</body>
</html>`,
		templateEscape(message),
		viewURL,
		friendsURL,
		friendsURL,
		settingsURL,
		settingsURL,
	)

	text := fmt.Sprintf(`%s

View notifications: %s
Friends page: %s
Manage notification settings: %s

--
Year of Bingo
yearofbingo.com`, message, viewURL, friendsURL, settingsURL)

	return subject, html, text
}

func (s *NotificationService) ensureSettingsRow(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		"INSERT INTO notification_settings (user_id) VALUES ($1) ON CONFLICT DO NOTHING",
		userID,
	)
	if err != nil {
		return fmt.Errorf("ensure notification settings: %w", err)
	}
	return nil
}

func (s *NotificationService) loadSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error) {
	settings := &models.NotificationSettings{}
	err := s.db.QueryRow(ctx,
		`SELECT user_id, in_app_enabled, in_app_friend_request_received, in_app_friend_request_accepted,
		        in_app_friend_bingo, in_app_friend_new_card, email_enabled, email_friend_request_received,
		        email_friend_request_accepted, email_friend_bingo, email_friend_new_card, created_at, updated_at
		 FROM notification_settings WHERE user_id = $1`,
		userID,
	).Scan(
		&settings.UserID,
		&settings.InAppEnabled,
		&settings.InAppFriendRequestReceived,
		&settings.InAppFriendRequestAccepted,
		&settings.InAppFriendBingo,
		&settings.InAppFriendNewCard,
		&settings.EmailEnabled,
		&settings.EmailFriendRequestReceived,
		&settings.EmailFriendRequestAccepted,
		&settings.EmailFriendBingo,
		&settings.EmailFriendNewCard,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("load notification settings: %w", err)
	}

	return settings, nil
}

func (s *NotificationService) isEmailVerified(ctx context.Context, userID uuid.UUID) (bool, error) {
	var verified bool
	if err := s.db.QueryRow(ctx, "SELECT email_verified FROM users WHERE id = $1", userID).Scan(&verified); err != nil {
		return false, fmt.Errorf("load email verification: %w", err)
	}
	return verified, nil
}

type insertedNotifications struct {
	emailIDs []uuid.UUID
}

func collectInserted(rows Rows) insertedNotifications {
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var userID uuid.UUID
		var emailDelivered bool
		if err := rows.Scan(&id, &userID, &emailDelivered); err != nil {
			continue
		}
		if emailDelivered {
			ids = append(ids, id)
		}
	}
	return insertedNotifications{emailIDs: ids}
}

func notificationScenarioColumns(nType models.NotificationType) (string, string, error) {
	switch nType {
	case models.NotificationTypeFriendRequestReceived:
		return "in_app_friend_request_received", "email_friend_request_received", nil
	case models.NotificationTypeFriendRequestAccepted:
		return "in_app_friend_request_accepted", "email_friend_request_accepted", nil
	case models.NotificationTypeFriendBingo:
		return "in_app_friend_bingo", "email_friend_bingo", nil
	case models.NotificationTypeFriendNewCard:
		return "in_app_friend_new_card", "email_friend_new_card", nil
	default:
		return "", "", fmt.Errorf("unsupported notification type: %s", nType)
	}
}

func cardDisplayName(title *string, year *int) string {
	if title != nil && *title != "" {
		return *title
	}
	if year != nil {
		return fmt.Sprintf("%d Bingo Card", *year)
	}
	return "a bingo card"
}

func enablesEmail(patch models.NotificationSettingsPatch) bool {
	return (patch.EmailEnabled != nil && *patch.EmailEnabled) ||
		(patch.EmailFriendRequestReceived != nil && *patch.EmailFriendRequestReceived) ||
		(patch.EmailFriendRequestAccepted != nil && *patch.EmailFriendRequestAccepted) ||
		(patch.EmailFriendBingo != nil && *patch.EmailFriendBingo) ||
		(patch.EmailFriendNewCard != nil && *patch.EmailFriendNewCard)
}

func templateEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}
