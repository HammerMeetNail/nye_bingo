package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

const (
	assistReplyMaxRunes = 1200
	assistNotesMaxRunes = 500
)

var validAssistModes = map[string]struct{}{
	"breakdown":  {},
	"next_step":  {},
	"obstacles":  {},
	"schedule":   {},
	"ideas":      {},
	"motivation": {},
}

type AssistPayload struct {
	IsOnGoal bool   `json:"is_on_goal"`
	Reply    string `json:"reply"`
}

func monthStartUTC(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func nextMonthStartUTC(t time.Time) time.Time {
	return monthStartUTC(t).AddDate(0, 1, 0)
}

func (s *Service) premiumMonthlyLimit() int {
	return maxInt(s.premiumEnhancementsPerMonth, defaultPremiumEnhancementsPerMonth)
}

func (s *Service) GetPremiumEnhancementsStatus(ctx context.Context, userID uuid.UUID, now time.Time) (limit int, used int, remaining int, resetsAt time.Time, err error) {
	limit = s.premiumMonthlyLimit()
	resetsAt = nextMonthStartUTC(now)
	if s.db == nil {
		return limit, 0, limit, resetsAt, ErrAIUsageTrackingUnavailable
	}

	monthStart := monthStartUTC(now)
	if err := s.db.QueryRow(ctx, `
		SELECT enhancements_used
		FROM ai_premium_usage_monthly
		WHERE user_id = $1 AND month_start = $2
	`, userID, monthStart).Scan(&used); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			logging.Error("Failed to load premium AI usage", map[string]interface{}{
				"error":   err.Error(),
				"user_id": userID.String(),
			})
			return limit, 0, limit, resetsAt, ErrAIUsageTrackingUnavailable
		}
		used = 0
	}

	remaining = limit - used
	if remaining < 0 {
		remaining = 0
	}
	return limit, used, remaining, resetsAt, nil
}

func (s *Service) ReservePremiumEnhancement(ctx context.Context, userID uuid.UUID, now time.Time) (remaining int, resetsAt time.Time, err error) {
	limit := s.premiumMonthlyLimit()
	resetsAt = nextMonthStartUTC(now)
	if s.db == nil {
		return 0, resetsAt, ErrAIUsageTrackingUnavailable
	}

	monthStart := monthStartUTC(now)
	var used int
	err = s.db.QueryRow(ctx, `
		WITH upsert AS (
		  INSERT INTO ai_premium_usage_monthly (user_id, month_start, enhancements_used)
		  VALUES ($1, $2, 1)
		  ON CONFLICT (user_id, month_start)
		  DO UPDATE SET
		    enhancements_used = ai_premium_usage_monthly.enhancements_used + 1,
		    updated_at = NOW()
		  WHERE ai_premium_usage_monthly.enhancements_used < $3
		  RETURNING enhancements_used
		)
		SELECT enhancements_used FROM upsert
	`, userID, monthStart, limit).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, resetsAt, ErrPremiumEnhancementsExhausted
	}
	if err != nil {
		logging.Error("Failed to reserve premium AI enhancement", map[string]interface{}{
			"error":   err.Error(),
			"user_id": userID.String(),
		})
		return 0, resetsAt, ErrAIUsageTrackingUnavailable
	}

	remaining = limit - used
	if remaining < 0 {
		remaining = 0
	}
	return remaining, resetsAt, nil
}

func (s *Service) RefundPremiumEnhancement(ctx context.Context, userID uuid.UUID, now time.Time) error {
	if s.db == nil {
		return ErrAIUsageTrackingUnavailable
	}

	monthStart := monthStartUTC(now)
	_, err := s.db.Exec(ctx, `
		UPDATE ai_premium_usage_monthly
		SET enhancements_used = GREATEST(enhancements_used - 1, 0),
		    updated_at = NOW()
		WHERE user_id = $1 AND month_start = $2
	`, userID, monthStart)
	if err != nil {
		logging.Error("Failed to refund premium AI enhancement", map[string]interface{}{
			"error":   err.Error(),
			"user_id": userID.String(),
		})
		return ErrAIUsageTrackingUnavailable
	}
	return nil
}

func (s *Service) RegenerateGoal(ctx context.Context, userID uuid.UUID, prompt GoalPrompt, existingGoals []string, replaceIndex int) (string, UsageStats, error) {
	if len(existingGoals) == 0 || replaceIndex < 0 || replaceIndex >= len(existingGoals) {
		return "", UsageStats{}, ErrInvalidInput
	}
	currentGoal := strings.TrimSpace(existingGoals[replaceIndex])
	if currentGoal == "" {
		return "", UsageStats{}, ErrInvalidInput
	}

	hintParts := []string{}
	if v := strings.TrimSpace(prompt.Category); v != "" {
		hintParts = append(hintParts, "Category: "+v)
	}
	if v := strings.TrimSpace(prompt.Difficulty); v != "" {
		hintParts = append(hintParts, "Difficulty: "+v)
	}
	if v := strings.TrimSpace(prompt.Budget); v != "" {
		hintParts = append(hintParts, "Budget: "+v)
	}
	if v := strings.TrimSpace(prompt.Focus); v != "" {
		hintParts = append(hintParts, "Focus: "+v)
	}
	if v := strings.TrimSpace(prompt.Context); v != "" {
		hintParts = append(hintParts, "Context: "+v)
	}

	goals, stats, err := s.generateGuideGoalsWithFeature(ctx, userID, GuidePrompt{
		Mode:        "refine",
		CurrentGoal: currentGoal,
		Hint:        strings.Join(hintParts, " | "),
		Count:       1,
		Avoid:       existingGoals,
	}, featureRegenerate)
	if err != nil {
		return "", stats, err
	}
	if len(goals) != 1 {
		return "", stats, fmt.Errorf("%w: expected 1 goal, got %d", ErrAIProviderUnavailable, len(goals))
	}

	regenerated := strings.TrimSpace(goals[0])
	if regenerated == "" {
		return "", stats, fmt.Errorf("%w: empty regenerated goal", ErrAIProviderUnavailable)
	}
	for _, item := range existingGoals {
		if strings.EqualFold(strings.TrimSpace(item), regenerated) {
			return "", stats, fmt.Errorf("%w: regenerated goal duplicated existing item", ErrAIProviderUnavailable)
		}
	}

	return regenerated, stats, nil
}

func (s *Service) GenerateFillGoals(ctx context.Context, userID uuid.UUID, prompt GoalPrompt, existingGoals []string) ([]string, UsageStats, error) {
	if prompt.Count < 1 || prompt.Count > 24 {
		return nil, UsageStats{}, ErrInvalidInput
	}

	if len(existingGoals) > 0 {
		avoid := sanitizeGuideAvoidList(existingGoals)
		if len(avoid) > 0 {
			var b strings.Builder
			b.WriteString(strings.TrimSpace(prompt.Context))
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("Avoid duplicates of these goals:\n")
			for _, item := range avoid {
				b.WriteString("- ")
				b.WriteString(item)
				b.WriteString("\n")
			}
			prompt.Context = b.String()
		}
	}

	goals, stats, err := s.generateGoalsWithFeature(ctx, userID, prompt, featureFillEmpty)
	if err != nil {
		return nil, stats, err
	}

	seen := make(map[string]struct{}, len(existingGoals)+len(goals))
	for _, item := range existingGoals {
		trimmed := strings.TrimSpace(strings.ToLower(item))
		if trimmed != "" {
			seen[trimmed] = struct{}{}
		}
	}
	for _, item := range goals {
		trimmed := strings.TrimSpace(strings.ToLower(item))
		if trimmed == "" {
			return nil, stats, fmt.Errorf("%w: generated empty goal", ErrAIProviderUnavailable)
		}
		if _, exists := seen[trimmed]; exists {
			return nil, stats, fmt.Errorf("%w: generated duplicate goal", ErrAIProviderUnavailable)
		}
		seen[trimmed] = struct{}{}
	}

	return goals, stats, nil
}

func (s *Service) FillEmptyOnCard(ctx context.Context, userID, cardID uuid.UUID, prompt GoalPrompt) (*models.BingoCard, UsageStats, error) {
	if s.db == nil {
		return nil, UsageStats{}, ErrAIUsageTrackingUnavailable
	}
	dbWithTx, ok := s.db.(services.DB)
	if !ok {
		return nil, UsageStats{}, ErrAIUsageTrackingUnavailable
	}

	cardSnapshot, emptySnapshot, existingGoals, err := s.loadCardState(ctx, s.db, userID, cardID, false)
	if err != nil {
		return nil, UsageStats{}, err
	}
	if cardSnapshot.IsFinalized {
		return nil, UsageStats{}, services.ErrCardFinalized
	}
	if len(emptySnapshot) == 0 {
		return nil, UsageStats{}, services.ErrCardFull
	}

	prompt.Count = len(emptySnapshot)
	goals, stats, err := s.GenerateFillGoals(ctx, userID, prompt, existingGoals)
	if err != nil {
		return nil, stats, err
	}
	if len(goals) != len(emptySnapshot) {
		return nil, stats, fmt.Errorf("%w: expected %d goals, got %d", ErrAIProviderUnavailable, len(emptySnapshot), len(goals))
	}

	tx, err := dbWithTx.Begin(ctx)
	if err != nil {
		return nil, stats, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	card, emptyPositions, _, err := s.loadCardState(ctx, tx, userID, cardID, true)
	if err != nil {
		return nil, stats, err
	}
	if card.IsFinalized {
		return nil, stats, services.ErrCardFinalized
	}
	if len(emptyPositions) != len(goals) {
		return nil, stats, fmt.Errorf("%w: card changed while generating goals", ErrAIProviderUnavailable)
	}

	for i, pos := range emptyPositions {
		content := strings.TrimSpace(goals[i])
		if content == "" {
			return nil, stats, fmt.Errorf("%w: generated empty goal", ErrAIProviderUnavailable)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO bingo_items (card_id, position, content)
			VALUES ($1, $2, $3)
		`, cardID, pos, content); err != nil {
			return nil, stats, fmt.Errorf("insert generated goal: %w", err)
		}
	}

	updatedItems, err := s.loadCardItems(ctx, tx, cardID)
	if err != nil {
		return nil, stats, err
	}
	card.Items = updatedItems

	if err := tx.Commit(ctx); err != nil {
		return nil, stats, fmt.Errorf("commit transaction: %w", err)
	}
	return card, stats, nil
}

func (s *Service) AssistCardGoal(ctx context.Context, userID, cardID uuid.UUID, position int, mode, notes string) (string, UsageStats, error) {
	goalText, err := s.goalTextForAssist(ctx, userID, cardID, position)
	if err != nil {
		return "", UsageStats{}, err
	}
	return s.AssistGoal(ctx, userID, goalText, mode, notes)
}

func (s *Service) AssistGoal(ctx context.Context, userID uuid.UUID, goalText, mode, notes string) (string, UsageStats, error) {
	start := time.Now()

	mode = strings.ToLower(strings.TrimSpace(mode))
	if _, ok := validAssistModes[mode]; !ok {
		return "", UsageStats{}, ErrInvalidInput
	}

	goalText = strings.TrimSpace(goalText)
	if goalText == "" {
		return "", UsageStats{}, ErrInvalidInput
	}
	goalText = escapeXMLTags(sanitizeInput(goalText))
	notes = escapeXMLTags(truncateGuideRunes(sanitizeInput(notes), assistNotesMaxRunes))

	if s.stub {
		reply := fmt.Sprintf("Goal: %s\nMode: %s\n1) Pick one concrete action today.\n2) Keep it realistic and scheduled.\n3) Track progress this week.", goalText, mode)
		stats := UsageStats{
			Model:    "stub",
			Duration: time.Since(start),
		}
		s.logUsageWithTimeout(userID, stats, "success", featureAssist)
		return reply, stats, nil
	}

	if strings.TrimSpace(s.apiKey) == "" {
		logging.Warn("Gemini API key missing; AI assistant unavailable", map[string]interface{}{
			"user_id": userID.String(),
		})
		return "", UsageStats{}, ErrAINotConfigured
	}

	systemPrompt := `You are a strict bingo goal assistant.
You may only help with the selected goal and practical ways to complete it.
If notes are off-topic, refuse and redirect back to the selected goal.
Keep replies concise and actionable.`

	userMessage := fmt.Sprintf(`Selected goal:
<goal>
%s
</goal>

Mode: %s

Notes/constraints:
<notes>
%s
</notes>

Return JSON object:
{
  "is_on_goal": boolean,
  "reply": string
}

Rules:
- "reply" must only discuss completing the selected goal.
- If notes are off-topic, set is_on_goal=false and provide a short refusal that redirects to the selected goal.
- Keep reply under 1200 characters.`, goalText, mode, notes)

	var thinkingConfig *geminiThinkingConfig
	if strings.HasPrefix(s.model, "gemini-3") {
		if s.thinkingLevel != "" {
			thinkingConfig = &geminiThinkingConfig{ThinkingLevel: s.thinkingLevel}
		}
	} else if s.thinkingBudget > 0 {
		thinkingConfig = &geminiThinkingConfig{ThinkingBudget: s.thinkingBudget}
	}

	reqBody := geminiRequest{
		SystemInstruction: &geminiSystemInstruction{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{
				Parts: []geminiPart{{Text: userMessage}},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema: &geminiSchema{
				Type: "object",
				Required: []string{
					"is_on_goal",
					"reply",
				},
				Properties: map[string]*geminiSchema{
					"is_on_goal": {Type: "boolean"},
					"reply":      {Type: "string"},
				},
			},
			Temperature:     s.temperature,
			MaxOutputTokens: s.maxOutputTokens,
			ThinkingConfig:  thinkingConfig,
		},
		SafetySettings: []geminiSafetySetting{
			{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
			{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
			{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
			{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", UsageStats{}, fmt.Errorf("%w: failed to marshal request", ErrAIProviderUnavailable)
	}

	url := fmt.Sprintf("%s/%s:generateContent", geminiBaseURL, s.model)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", UsageStats{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		s.logUsageWithTimeout(userID, UsageStats{Model: s.model, Duration: time.Since(start)}, "error", featureAssist)
		return "", UsageStats{}, fmt.Errorf("%w: %v", ErrAIProviderUnavailable, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		s.logUsageWithTimeout(userID, UsageStats{Model: s.model, Duration: time.Since(start)}, "error", featureAssist)
		if resp.StatusCode == http.StatusTooManyRequests {
			return "", UsageStats{}, fmt.Errorf("%w: status %d", ErrRateLimitExceeded, resp.StatusCode)
		}

		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		if len(bodyBytes) > 0 {
			logging.Error("Gemini non-200 response (assist)", map[string]interface{}{
				"user_id": userID.String(),
				"status":  resp.StatusCode,
				"body":    string(bodyBytes),
			})
		} else if dump, dumpErr := httputil.DumpResponse(resp, false); dumpErr == nil {
			logging.Error("Gemini non-200 response (assist, headers only)", map[string]interface{}{
				"user_id": userID.String(),
				"status":  resp.StatusCode,
				"dump":    string(dump),
			})
		}
		return "", UsageStats{}, fmt.Errorf("%w: status %d", ErrAIProviderUnavailable, resp.StatusCode)
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		s.logUsageWithTimeout(userID, UsageStats{Model: s.model, Duration: time.Since(start)}, "error", featureAssist)
		return "", UsageStats{}, fmt.Errorf("%w: failed to decode response", ErrAIProviderUnavailable)
	}

	stats := UsageStats{
		Model:        s.model,
		TokensInput:  geminiResp.Usage.PromptTokenCount,
		TokensOutput: geminiResp.Usage.CandidatesTokenCount,
		Duration:     time.Since(start),
	}

	if len(geminiResp.Candidates) == 0 {
		s.logUsageWithTimeout(userID, stats, "safety_block", featureAssist)
		return "", stats, ErrSafetyViolation
	}

	candidate := geminiResp.Candidates[0]
	if candidate.FinishReason == "SAFETY" {
		s.logUsageWithTimeout(userID, stats, "safety_block", featureAssist)
		return "", stats, ErrSafetyViolation
	}
	if len(candidate.Content.Parts) == 0 {
		s.logUsageWithTimeout(userID, stats, "error", featureAssist)
		return "", stats, fmt.Errorf("%w: empty content parts", ErrAIProviderUnavailable)
	}

	responseText := stripMarkdownCodeBlock(candidate.Content.Parts[0].Text)
	var payload AssistPayload
	if err := json.Unmarshal([]byte(responseText), &payload); err != nil {
		s.logUsageWithTimeout(userID, stats, "error", featureAssist)
		return "", stats, fmt.Errorf("%w: invalid JSON response", ErrAIProviderUnavailable)
	}

	reply := strings.TrimSpace(payload.Reply)
	if !payload.IsOnGoal {
		reply = "I can only help with the selected goal. Share constraints tied to this goal and I can help plan it."
	}
	reply = truncateGuideRunes(reply, assistReplyMaxRunes)
	if reply == "" {
		s.logUsageWithTimeout(userID, stats, "error", featureAssist)
		return "", stats, fmt.Errorf("%w: empty reply", ErrAIProviderUnavailable)
	}

	s.logUsageWithTimeout(userID, stats, "success", featureAssist)
	return reply, stats, nil
}

func (s *Service) goalTextForAssist(ctx context.Context, userID, cardID uuid.UUID, position int) (string, error) {
	card, err := s.loadOwnedCard(ctx, s.db, userID, cardID, false)
	if err != nil {
		return "", err
	}
	if !card.IsValidItemPosition(position) {
		return "", services.ErrInvalidPosition
	}

	var content string
	if err := s.db.QueryRow(ctx, `
		SELECT content
		FROM bingo_items
		WHERE card_id = $1 AND position = $2
	`, cardID, position).Scan(&content); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", services.ErrItemNotFound
		}
		return "", fmt.Errorf("loading goal for assist: %w", err)
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return "", services.ErrItemNotFound
	}
	return content, nil
}

func (s *Service) loadCardState(ctx context.Context, conn services.DBConn, userID, cardID uuid.UUID, forUpdate bool) (*models.BingoCard, []int, []string, error) {
	card, err := s.loadOwnedCard(ctx, conn, userID, cardID, forUpdate)
	if err != nil {
		return nil, nil, nil, err
	}
	items, err := s.loadCardItems(ctx, conn, cardID)
	if err != nil {
		return nil, nil, nil, err
	}
	card.Items = items

	occupied := make(map[int]struct{}, len(items))
	existingGoals := make([]string, 0, len(items))
	for _, item := range items {
		occupied[item.Position] = struct{}{}
		if text := strings.TrimSpace(item.Content); text != "" {
			existingGoals = append(existingGoals, text)
		}
	}

	emptyPositions := make([]int, 0, card.Capacity()-len(items))
	for pos := 0; pos < card.TotalSquares(); pos++ {
		if !card.IsValidItemPosition(pos) {
			continue
		}
		if _, ok := occupied[pos]; ok {
			continue
		}
		emptyPositions = append(emptyPositions, pos)
	}

	return card, emptyPositions, existingGoals, nil
}

func (s *Service) loadOwnedCard(ctx context.Context, conn services.DBConn, userID, cardID uuid.UUID, forUpdate bool) (*models.BingoCard, error) {
	query := `
		SELECT id, user_id, year, category, title, grid_size, header_text, has_free_space, free_space_position,
		       is_active, is_finalized, visible_to_friends, is_archived, created_at, updated_at
		FROM bingo_cards
		WHERE id = $1 AND user_id = $2
	`
	if forUpdate {
		query += " FOR UPDATE"
	}

	card := &models.BingoCard{}
	err := conn.QueryRow(ctx, query, cardID, userID).Scan(
		&card.ID, &card.UserID, &card.Year, &card.Category, &card.Title,
		&card.GridSize, &card.HeaderText, &card.HasFreeSpace, &card.FreeSpacePos,
		&card.IsActive, &card.IsFinalized, &card.VisibleToFriends, &card.IsArchived, &card.CreatedAt, &card.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, services.ErrCardNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading card: %w", err)
	}
	return card, nil
}

func (s *Service) loadCardItems(ctx context.Context, conn services.DBConn, cardID uuid.UUID) ([]models.BingoItem, error) {
	rows, err := conn.Query(ctx, `
		SELECT id, card_id, position, content, is_completed, completed_at, notes, proof_url, created_at
		FROM bingo_items
		WHERE card_id = $1
		ORDER BY position
	`, cardID)
	if err != nil {
		return nil, fmt.Errorf("loading card items: %w", err)
	}
	defer rows.Close()

	items := make([]models.BingoItem, 0)
	for rows.Next() {
		var item models.BingoItem
		if err := rows.Scan(
			&item.ID, &item.CardID, &item.Position, &item.Content, &item.IsCompleted, &item.CompletedAt, &item.Notes, &item.ProofURL, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning card item: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}
