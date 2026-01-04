package services

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

type reminderStats struct {
	Completed int
	Total     int
	Bingos    int
}

type checkinEmailParams struct {
	Card            *models.BingoCard
	Stats           reminderStats
	Recommendations []models.BingoItem
	BaseURL         string
	ImageURL        string
	UnsubscribeURL  string
	IsTest          bool
}

type goalReminderEmailParams struct {
	CardID         uuid.UUID
	ItemID         uuid.UUID
	CardTitle      *string
	CardYear       int
	GoalText       string
	BaseURL        string
	UnsubscribeURL string
}

func buildCheckinEmail(params checkinEmailParams) (string, string, string) {
	cardName := params.Card.DisplayName()
	progress := fmt.Sprintf("%d/%d complete - %s", params.Stats.Completed, params.Stats.Total, pluralizeBingo(params.Stats.Bingos))
	manageURL := fmt.Sprintf("%s/#profile", params.BaseURL)
	cardURL := fmt.Sprintf("%s/#card/%s", params.BaseURL, params.Card.ID)
	unsubscribe := params.UnsubscribeURL
	safeManageURL := templateEscape(manageURL)
	safeCardURL := templateEscape(cardURL)
	safeUnsubscribe := templateEscape(unsubscribe)

	subject := "Your Year of Bingo check-in"
	if params.IsTest {
		subject = "Your Year of Bingo check-in (test)"
	}

	recommendationHTML := ""
	recommendationText := ""
	if len(params.Recommendations) > 0 {
		items := make([]string, 0, len(params.Recommendations))
		for _, item := range params.Recommendations {
			items = append(items, fmt.Sprintf("<li>%s</li>", templateEscape(item.Content)))
		}
		recommendationHTML = fmt.Sprintf("<h3 style=\"margin-top: 24px;\">Suggested next goals</h3><ul style=\"padding-left: 20px;\">%s</ul>", strings.Join(items, ""))

		textItems := make([]string, 0, len(params.Recommendations))
		for _, item := range params.Recommendations {
			textItems = append(textItems, fmt.Sprintf("- %s", item.Content))
		}
		recommendationText = fmt.Sprintf("Suggested next goals:\n%s\n\n", strings.Join(textItems, "\n"))
	}

	imageBlock := ""
	if params.ImageURL != "" {
		safeImageURL := templateEscape(params.ImageURL)
		imageBlock = fmt.Sprintf("<p><img src=\"%s\" alt=\"%s\" style=\"max-width: 100%%; border-radius: 8px; border: 1px solid #eee;\"></p>",
			safeImageURL,
			templateEscape(cardName),
		)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 640px; margin: 0 auto; padding: 24px;">
  <h1 style="color: #333; font-size: 24px;">Year of Bingo</h1>
  <p style="font-size: 18px; margin-bottom: 4px;"><strong>%s</strong></p>
  <p style="color: #666; margin-top: 0;">%s</p>
  %s
  <p>
    <a href="%s" style="display: inline-block; background: #0f6f62; color: white; padding: 10px 18px; text-decoration: none; border-radius: 6px; margin: 12px 0;">Open my card</a>
  </p>
  %s
  <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
  <p style="color: #666; font-size: 14px;">Manage reminders: <a href="%s">%s</a></p>
  <p style="color: #666; font-size: 14px;">Unsubscribe: <a href="%s">%s</a></p>
  <p style="color: #999; font-size: 12px;">Year of Bingo - yearofbingo.com</p>
</body>
</html>`,
		templateEscape(cardName),
		templateEscape(progress),
		imageBlock,
		safeCardURL,
		recommendationHTML,
		safeManageURL,
		safeManageURL,
		safeUnsubscribe,
		safeUnsubscribe,
	)

	text := fmt.Sprintf(`%s
%s

Open my card: %s

%sManage reminders: %s
Unsubscribe: %s

--
Year of Bingo
yearofbingo.com`,
		cardName,
		progress,
		cardURL,
		recommendationText,
		manageURL,
		unsubscribe,
	)

	return subject, html, text
}

func buildGoalReminderEmail(params goalReminderEmailParams) (string, string, string) {
	cardName := cardDisplayName(params.CardTitle, &params.CardYear)
	goalText := templateEscape(params.GoalText)
	manageURL := fmt.Sprintf("%s/#profile", params.BaseURL)
	goalURL := fmt.Sprintf("%s/#card/%s?item=%s", params.BaseURL, params.CardID, params.ItemID)
	unsubscribe := params.UnsubscribeURL
	safeManageURL := templateEscape(manageURL)
	safeGoalURL := templateEscape(goalURL)
	safeUnsubscribe := templateEscape(unsubscribe)

	subject := sanitizeSubject(fmt.Sprintf("Reminder: %s", params.GoalText))

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 640px; margin: 0 auto; padding: 24px;">
  <h1 style="color: #333; font-size: 24px;">Year of Bingo</h1>
  <p style="font-size: 16px;">%s</p>
  <p style="color: #666;">Card: %s</p>
  <p>
    <a href="%s" style="display: inline-block; background: #0f6f62; color: white; padding: 10px 18px; text-decoration: none; border-radius: 6px; margin: 12px 0;">Open this goal</a>
  </p>
  <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
  <p style="color: #666; font-size: 14px;">Manage reminders: <a href="%s">%s</a></p>
  <p style="color: #666; font-size: 14px;">Unsubscribe: <a href="%s">%s</a></p>
  <p style="color: #999; font-size: 12px;">Year of Bingo - yearofbingo.com</p>
</body>
</html>`,
		goalText,
		templateEscape(cardName),
		safeGoalURL,
		safeManageURL,
		safeManageURL,
		safeUnsubscribe,
		safeUnsubscribe,
	)

	text := fmt.Sprintf(`%s
Card: %s

Open this goal: %s

Manage reminders: %s
Unsubscribe: %s

--
Year of Bingo
yearofbingo.com`,
		params.GoalText,
		cardName,
		goalURL,
		manageURL,
		unsubscribe,
	)

	return subject, html, text
}

func pluralizeBingo(count int) string {
	if count == 1 {
		return "1 bingo"
	}
	return fmt.Sprintf("%d bingos", count)
}

func sanitizeSubject(subject string) string {
	cleaned := strings.ReplaceAll(subject, "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\r", " ")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) > 120 {
		cleaned = cleaned[:117] + "..."
	}
	return cleaned
}
