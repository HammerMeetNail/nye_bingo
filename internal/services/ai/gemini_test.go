package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
)

func TestGenerateGoals(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL
		if !strings.Contains(r.URL.Path, geminiModel) {
			t.Errorf("expected URL to contain model name, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "test-key" {
			t.Errorf("expected API key 'test-key', got %s", r.URL.Query().Get("key"))
		}

		// Verify request body
		var req geminiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}

		if len(req.Contents) == 0 {
			t.Error("expected contents")
		}

		text := req.Contents[0].Parts[0].Text
		if !strings.Contains(text, "Hobbies goals") {
			t.Errorf("expected category in prompt, got %s", text)
		}
		if !strings.Contains(text, "<user_focus>\nCooking\n</user_focus>") {
			t.Errorf("expected focus block in prompt, got %s", text)
		}

		// Send mock response
		resp := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{
							{Text: `["Goal 1", "Goal 2", "Goal 3"]`},
						},
					},
					FinishReason: "STOP",
				},
			},
			Usage: geminiUsage{
				PromptTokenCount:     100,
				CandidatesTokenCount: 50,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// Override URL
	oldURL := geminiBaseURL
	geminiBaseURL = ts.URL
	defer func() { geminiBaseURL = oldURL }()

	// Create service
	cfg := &config.Config{
		AI: config.AIConfig{GeminiAPIKey: "test-key"},
	}
	// Pass nil DB
	service := NewService(cfg, nil)

	// Call GenerateGoals
	prompt := GoalPrompt{
		Category:   "Hobbies",
		Focus:      "Cooking",
		Difficulty: "medium",
		Frequency:  "weekly",
		Context:    "test context",
	}

	goals, stats, err := service.GenerateGoals(context.Background(), uuid.New(), prompt)
	if err != nil {
		t.Fatalf("GenerateGoals failed: %v", err)
	}

	if len(goals) != 3 {
		t.Errorf("expected 3 goals, got %d", len(goals))
	}
	if goals[0] != "Goal 1" {
		t.Errorf("expected Goal 1, got %s", goals[0])
	}
	if stats.TokensInput != 100 {
		t.Errorf("expected 100 input tokens, got %d", stats.TokensInput)
	}
}
