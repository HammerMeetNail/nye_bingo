package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestGenerateGuideGoalsWithFeature_Validation(t *testing.T) {
	svc := &Service{stub: true}
	userID := uuid.New()

	_, _, err := svc.generateGuideGoalsWithFeature(context.Background(), userID, GuidePrompt{
		Mode: "bad",
	}, featureGenerate)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad mode, got %v", err)
	}

	_, _, err = svc.generateGuideGoalsWithFeature(context.Background(), userID, GuidePrompt{
		Mode:  "new",
		Count: 6,
	}, featureGenerate)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for out-of-range count, got %v", err)
	}

	_, _, err = svc.generateGuideGoalsWithFeature(context.Background(), userID, GuidePrompt{
		Mode: "refine",
	}, featureGenerate)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty refine goal, got %v", err)
	}
}

func TestGenerateGuideGoals_InvalidMode(t *testing.T) {
	svc := &Service{}
	_, _, err := svc.GenerateGuideGoals(context.Background(), uuid.New(), GuidePrompt{Mode: "nope"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGenerateGuideGoals_RefineRequiresCurrentGoal(t *testing.T) {
	svc := &Service{}
	_, _, err := svc.GenerateGuideGoals(context.Background(), uuid.New(), GuidePrompt{Mode: "refine"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGenerateGuideGoals_StubDeterministic(t *testing.T) {
	svc := &Service{stub: true}
	prompt := GuidePrompt{Mode: "new", Hint: "Local adventures", Count: 3}

	first, _, err := svc.GenerateGuideGoals(context.Background(), uuid.New(), prompt)
	if err != nil {
		t.Fatalf("GenerateGuideGoals failed: %v", err)
	}
	second, _, err := svc.GenerateGuideGoals(context.Background(), uuid.New(), prompt)
	if err != nil {
		t.Fatalf("GenerateGuideGoals failed: %v", err)
	}

	if len(first) != 3 {
		t.Fatalf("expected 3 goals, got %d", len(first))
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected deterministic stub output, got %v vs %v", first, second)
	}
}

func TestGenerateGuideGoalsWithFeature_StubDefaults(t *testing.T) {
	svc := &Service{stub: true}
	userID := uuid.New()

	refined, statsRefine, err := svc.generateGuideGoalsWithFeature(context.Background(), userID, GuidePrompt{
		Mode:        "refine",
		CurrentGoal: "Read one chapter",
		Count:       0,
	}, featureGenerate)
	if err != nil {
		t.Fatalf("unexpected refine error: %v", err)
	}
	if len(refined) != 3 {
		t.Fatalf("expected default refine count 3, got %d", len(refined))
	}
	if statsRefine.Model != "stub" {
		t.Fatalf("expected stub model stats, got %q", statsRefine.Model)
	}

	fresh, _, err := svc.generateGuideGoalsWithFeature(context.Background(), userID, GuidePrompt{
		Mode: "new",
		Hint: "Local nature",
	}, featureGenerate)
	if err != nil {
		t.Fatalf("unexpected new-mode error: %v", err)
	}
	if len(fresh) != 5 {
		t.Fatalf("expected default new count 5, got %d", len(fresh))
	}
}

func TestGenerateGuideGoals_TrimsExtraGoals(t *testing.T) {
	svc := &Service{
		apiKey: "test-key",
		model:  "test-model",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp := geminiResponse{
				Candidates: []geminiCandidate{
					{
						Content: geminiContent{
							Parts: []geminiPart{
								{Text: mustJSON(t, []string{"Goal 1", "Goal 2", "Goal 3", "Goal 4"})},
							},
						},
						FinishReason: "STOP",
					},
				},
				Usage: geminiUsage{},
			}
			return jsonHTTPResponse(t, http.StatusOK, resp), nil
		})},
	}

	goals, _, err := svc.GenerateGuideGoals(context.Background(), uuid.New(), GuidePrompt{
		Mode:  "new",
		Hint:  "Weekends",
		Count: 3,
		Avoid: []string{"Goal 99"},
	})
	if err != nil {
		t.Fatalf("GenerateGuideGoals failed: %v", err)
	}
	if len(goals) != 3 {
		t.Fatalf("expected 3 goals, got %d", len(goals))
	}
	if goals[0] != "Goal 1" || goals[2] != "Goal 3" {
		t.Fatalf("unexpected goals: %v", goals)
	}
}

func TestGenerateGuideGoals_ErrorsOnShortResponse(t *testing.T) {
	svc := &Service{
		apiKey: "test-key",
		model:  "test-model",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			var req geminiRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			resp := geminiResponse{
				Candidates: []geminiCandidate{
					{
						Content: geminiContent{
							Parts: []geminiPart{
								{Text: mustJSON(t, []string{"Goal 1", "Goal 2"})},
							},
						},
						FinishReason: "STOP",
					},
				},
				Usage: geminiUsage{},
			}
			return jsonHTTPResponse(t, http.StatusOK, resp), nil
		})},
	}

	_, _, err := svc.GenerateGuideGoals(context.Background(), uuid.New(), GuidePrompt{
		Mode:        "refine",
		CurrentGoal: "Visit a local farmer's market",
		Count:       3,
	})
	if !errors.Is(err, ErrAIProviderUnavailable) {
		t.Fatalf("expected ErrAIProviderUnavailable, got %v", err)
	}
}

func TestSanitizeGuideAvoidList(t *testing.T) {
	input := make([]string, 0, 30)
	input = append(input, "", "   ", "<b>alpha</b>", strings.Repeat("x", 130))
	for i := 0; i < 30; i++ {
		input = append(input, "item "+strings.Repeat("z", i))
	}

	got := sanitizeGuideAvoidList(input)
	if len(got) == 0 {
		t.Fatal("expected non-empty sanitized list")
	}
	if len(got) > 24 {
		t.Fatalf("expected max 24 entries, got %d", len(got))
	}
	if strings.Contains(got[0], "<") || strings.Contains(got[0], ">") {
		t.Fatalf("expected XML-safe escaped content, got %q", got[0])
	}
	for _, item := range got {
		if len([]rune(item)) > 100 {
			t.Fatalf("expected item length <=100, got %d", len([]rune(item)))
		}
	}
}

func TestTruncateGuideRunes(t *testing.T) {
	if got := truncateGuideRunes("abc", 0); got != "" {
		t.Fatalf("expected empty string for non-positive max, got %q", got)
	}
	if got := truncateGuideRunes("abc", 10); got != "abc" {
		t.Fatalf("expected unmodified short string, got %q", got)
	}
	if got := truncateGuideRunes("abcdef", 3); got != "abc" {
		t.Fatalf("expected truncated string, got %q", got)
	}
}
