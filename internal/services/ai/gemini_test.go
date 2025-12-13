package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
)

func TestGenerateGoals(t *testing.T) {
	makeGoals := func(n int) []string {
		goals := make([]string, 0, n)
		for i := 0; i < n; i++ {
			goals = append(goals, fmt.Sprintf("Goal %d", i+1))
		}
		return goals
	}

	tests := []struct {
		name       string
		roundTrip  func(r *http.Request) (*http.Response, error)
		wantGoals  int
		wantErrIs  error
		wantTokens int
	}{
		{
			name: "success",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				if !strings.Contains(r.URL.Path, geminiModel) {
					t.Errorf("expected URL to contain model name, got %s", r.URL.Path)
				}
				if got := r.Header.Get("x-goog-api-key"); got != "test-key" {
					t.Errorf("expected x-goog-api-key 'test-key', got %q", got)
				}

				var req geminiRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("failed to decode request: %v", err)
					return nil, fmt.Errorf("failed to decode request")
				}
				if len(req.Contents) == 0 || len(req.Contents[0].Parts) == 0 {
					t.Error("expected prompt contents")
					return nil, fmt.Errorf("missing prompt contents")
				}

				text := req.Contents[0].Parts[0].Text
				if !strings.Contains(text, "medium-difficulty hobbies goals") {
					t.Errorf("expected category+difficulty in prompt, got %q", text)
				}
				if !strings.Contains(text, "<user_focus>\nCooking\n</user_focus>") {
					t.Errorf("expected focus block in prompt, got %q", text)
				}
				if strings.Contains(text, "Goals must be budget-friendly ($20-$100) and public") {
					t.Error("found conflicting hardcoded budget rule in prompt")
				}

				resp := geminiResponse{
					Candidates: []geminiCandidate{
						{
							Content: geminiContent{
								Parts: []geminiPart{
									{Text: mustJSON(t, makeGoals(24))},
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
				return jsonHTTPResponse(t, http.StatusOK, resp), nil
			},
			wantGoals:  24,
			wantTokens: 100,
		},
		{
			name: "markdown-wrapped-json",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				resp := geminiResponse{
					Candidates: []geminiCandidate{
						{
							Content: geminiContent{
								Parts: []geminiPart{
									{Text: "```json\n" + mustJSON(t, makeGoals(24)) + "\n```"},
								},
							},
							FinishReason: "STOP",
						},
					},
					Usage: geminiUsage{
						PromptTokenCount:     1,
						CandidatesTokenCount: 1,
					},
				}
				return jsonHTTPResponse(t, http.StatusOK, resp), nil
			},
			wantGoals:  24,
			wantTokens: 1,
		},
		{
			name: "safety-finish-reason",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				resp := geminiResponse{
					Candidates: []geminiCandidate{
						{
							Content: geminiContent{
								Parts: []geminiPart{{Text: mustJSON(t, makeGoals(24))}},
							},
							FinishReason: "SAFETY",
						},
					},
					Usage: geminiUsage{},
				}
				return jsonHTTPResponse(t, http.StatusOK, resp), nil
			},
			wantErrIs: ErrSafetyViolation,
		},
		{
			name: "empty-candidates",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				resp := geminiResponse{
					Candidates: []geminiCandidate{},
					Usage:      geminiUsage{},
				}
				return jsonHTTPResponse(t, http.StatusOK, resp), nil
			},
			wantErrIs: ErrSafetyViolation,
		},
		{
			name: "provider-rate-limit",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"error":"rate limit"}`)),
				}, nil
			},
			wantErrIs: ErrRateLimitExceeded,
		},
		{
			name: "invalid-json",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				resp := geminiResponse{
					Candidates: []geminiCandidate{
						{
							Content: geminiContent{
								Parts: []geminiPart{{Text: "not-json"}},
							},
							FinishReason: "STOP",
						},
					},
				}
				return jsonHTTPResponse(t, http.StatusOK, resp), nil
			},
			wantErrIs: ErrAIProviderUnavailable,
		},
		{
			name: "wrong-goal-count",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				resp := geminiResponse{
					Candidates: []geminiCandidate{
						{
							Content: geminiContent{
								Parts: []geminiPart{{Text: mustJSON(t, makeGoals(3))}},
							},
							FinishReason: "STOP",
						},
					},
				}
				return jsonHTTPResponse(t, http.StatusOK, resp), nil
			},
			wantErrIs: ErrAIProviderUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{AI: config.AIConfig{GeminiAPIKey: "test-key"}}
			service := &Service{
				apiKey: cfg.AI.GeminiAPIKey,
				client: &http.Client{Transport: roundTripperFunc(tt.roundTrip)},
				db:     nil,
			}

			prompt := GoalPrompt{
				Category:   "hobbies",
				Focus:      "Cooking",
				Difficulty: "medium",
				Budget:     "free",
				Context:    "test context",
			}

			goals, stats, err := service.GenerateGoals(context.Background(), uuid.New(), prompt)
			if tt.wantErrIs != nil {
				if err == nil || !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("expected error %v, got %v", tt.wantErrIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateGoals failed: %v", err)
			}

			if len(goals) != tt.wantGoals {
				t.Fatalf("expected %d goals, got %d", tt.wantGoals, len(goals))
			}
			if tt.wantTokens != 0 && stats.TokensInput != tt.wantTokens {
				t.Fatalf("expected %d input tokens, got %d", tt.wantTokens, stats.TokensInput)
			}
		})
	}
}

func TestGenerateGoals_NotConfigured(t *testing.T) {
	service := &Service{apiKey: ""}
	_, _, err := service.GenerateGoals(context.Background(), uuid.New(), GoalPrompt{
		Category:   "hobbies",
		Focus:      "Cooking",
		Difficulty: "medium",
		Budget:     "free",
		Context:    "test context",
	})
	if err == nil || !errors.Is(err, ErrAINotConfigured) {
		t.Fatalf("expected error %v, got %v", ErrAINotConfigured, err)
	}
}

type roundTripperFunc func(r *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return string(b)
}

func jsonHTTPResponse(t *testing.T, status int, v any) *http.Response {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}
