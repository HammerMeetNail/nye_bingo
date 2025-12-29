package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestGenerateGuideGoals_InvalidMode(t *testing.T) {
	service := &Service{}
	_, _, err := service.GenerateGuideGoals(context.Background(), uuid.New(), GuidePrompt{Mode: "nope"})
	if err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected error %v, got %v", ErrInvalidInput, err)
	}
}

func TestGenerateGuideGoals_RefineRequiresCurrentGoal(t *testing.T) {
	service := &Service{}
	_, _, err := service.GenerateGuideGoals(context.Background(), uuid.New(), GuidePrompt{Mode: "refine"})
	if err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected error %v, got %v", ErrInvalidInput, err)
	}
}

func TestGenerateGuideGoals_StubDeterministic(t *testing.T) {
	service := &Service{stub: true}
	prompt := GuidePrompt{Mode: "new", Hint: "Local adventures", Count: 3}
	first, _, err := service.GenerateGuideGoals(context.Background(), uuid.New(), prompt)
	if err != nil {
		t.Fatalf("GenerateGuideGoals failed: %v", err)
	}
	second, _, err := service.GenerateGuideGoals(context.Background(), uuid.New(), prompt)
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

func TestGenerateGuideGoals_TrimsExtraGoals(t *testing.T) {
	service := &Service{
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

	goals, _, err := service.GenerateGuideGoals(context.Background(), uuid.New(), GuidePrompt{
		Mode:        "new",
		Hint:        "Weekends",
		Count:       3,
		Avoid:       []string{"Goal 99"},
		CurrentGoal: "",
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
	service := &Service{
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

	_, _, err := service.GenerateGuideGoals(context.Background(), uuid.New(), GuidePrompt{
		Mode:        "refine",
		CurrentGoal: "Visit a local farmer's market",
		Count:       3,
	})
	if err == nil || !errors.Is(err, ErrAIProviderUnavailable) {
		t.Fatalf("expected error %v, got %v", ErrAIProviderUnavailable, err)
	}
}
