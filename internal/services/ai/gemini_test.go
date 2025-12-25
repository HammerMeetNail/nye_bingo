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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type fakeRow struct {
	scanFunc func(dest ...any) error
}

func (f fakeRow) Scan(dest ...any) error {
	return f.scanFunc(dest...)
}

type fakeDB struct {
	execFunc     func(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
	queryRowFunc func(ctx context.Context, sql string, args ...any) services.Row
}

func (f *fakeDB) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	if f.execFunc != nil {
		return f.execFunc(ctx, sql, args...)
	}
	return nil, nil
}

func (f *fakeDB) Query(ctx context.Context, sql string, args ...any) (services.Rows, error) {
	return nil, nil
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...any) services.Row {
	if f.queryRowFunc != nil {
		return f.queryRowFunc(ctx, sql, args...)
	}
	return fakeRow{scanFunc: func(dest ...any) error { return nil }}
}

func TestGenerateGoals(t *testing.T) {
	makeGoals := func(n int) []string {
		goals := make([]string, 0, n)
		for i := 0; i < n; i++ {
			goals = append(goals, fmt.Sprintf("Goal %d", i+1))
		}
		return goals
	}

	tests := []struct {
		name        string
		roundTrip   func(r *http.Request) (*http.Response, error)
		promptCount int
		wantGoals   int
		wantErrIs   error
		wantTokens  int
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
				if !strings.Contains(text, "BUDGET CONSTRAINT: The goals must be completely free or very low cost (under $20).") {
					t.Errorf("expected budget instruction in prompt, got %q", text)
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
			name: "success-trims-extra-goals",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				resp := geminiResponse{
					Candidates: []geminiCandidate{
						{
							Content: geminiContent{
								Parts: []geminiPart{
									{Text: mustJSON(t, makeGoals(30))},
								},
							},
							FinishReason: "STOP",
						},
					},
					Usage: geminiUsage{},
				}
				return jsonHTTPResponse(t, http.StatusOK, resp), nil
			},
			wantGoals: 24,
		},
		{
			name: "success-custom-count",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				var req geminiRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("failed to decode request: %v", err)
					return nil, fmt.Errorf("failed to decode request")
				}
				text := req.Contents[0].Parts[0].Text
				if !strings.Contains(text, "Generate a list of 5 distinct") {
					t.Errorf("expected count in prompt, got %q", text)
				}

				resp := geminiResponse{
					Candidates: []geminiCandidate{
						{
							Content: geminiContent{
								Parts: []geminiPart{
									{Text: mustJSON(t, makeGoals(5))},
								},
							},
							FinishReason: "STOP",
						},
					},
					Usage: geminiUsage{
						PromptTokenCount:     5,
						CandidatesTokenCount: 5,
					},
				}
				return jsonHTTPResponse(t, http.StatusOK, resp), nil
			},
			promptCount: 5,
			wantGoals:   5,
			wantTokens:  5,
		},
		{
			name: "invalid-count",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				t.Fatal("expected request to be rejected before making provider call")
				return nil, nil
			},
			promptCount: 25,
			wantErrIs:   ErrAIProviderUnavailable,
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
			name: "provider-error",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
			wantErrIs: ErrAIProviderUnavailable,
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
			name: "invalid-response-body",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader("not-json")),
				}, nil
			},
			wantErrIs: ErrAIProviderUnavailable,
		},
		{
			name: "empty-content-parts",
			roundTrip: func(r *http.Request) (*http.Response, error) {
				resp := geminiResponse{
					Candidates: []geminiCandidate{
						{
							Content:      geminiContent{Parts: []geminiPart{}},
							FinishReason: "STOP",
						},
					},
					Usage: geminiUsage{},
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
				Count:      tt.promptCount,
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

func TestNewService(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.GeminiAPIKey = "test-key"
	svc := NewService(cfg, nil)
	if svc == nil {
		t.Fatal("expected service")
	}
	if svc.apiKey != "test-key" {
		t.Fatalf("expected api key, got %s", svc.apiKey)
	}
}

func TestConsumeUnverifiedFreeGeneration_NoDB(t *testing.T) {
	svc := &Service{}
	_, err := svc.ConsumeUnverifiedFreeGeneration(context.Background(), uuid.New())
	if !errors.Is(err, ErrAIUsageTrackingUnavailable) {
		t.Fatalf("expected tracking unavailable, got %v", err)
	}
}

func TestConsumeUnverifiedFreeGeneration_RequiresVerification(t *testing.T) {
	db := &fakeDB{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	svc := &Service{db: db}
	_, err := svc.ConsumeUnverifiedFreeGeneration(context.Background(), uuid.New())
	if !errors.Is(err, ErrEmailVerificationRequired) {
		t.Fatalf("expected verification required, got %v", err)
	}
}

func TestConsumeUnverifiedFreeGeneration_DBError(t *testing.T) {
	db := &fakeDB{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}
	svc := &Service{db: db}
	_, err := svc.ConsumeUnverifiedFreeGeneration(context.Background(), uuid.New())
	if !errors.Is(err, ErrAIUsageTrackingUnavailable) {
		t.Fatalf("expected ErrAIUsageTrackingUnavailable, got %v", err)
	}
}

func TestConsumeUnverifiedFreeGeneration_Success(t *testing.T) {
	db := &fakeDB{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				*(dest[0].(*int)) = 3
				return nil
			}}
		},
	}
	svc := &Service{db: db}
	remaining, err := svc.ConsumeUnverifiedFreeGeneration(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remaining != 2 {
		t.Fatalf("expected 2 remaining, got %d", remaining)
	}
}

func TestLogUsage_NoDB(t *testing.T) {
	svc := &Service{}
	svc.logUsage(context.Background(), uuid.New(), UsageStats{}, "success")
}

func TestLogUsage_WritesToDB(t *testing.T) {
	called := false
	db := &fakeDB{
		execFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			called = true
			return nil, nil
		},
	}
	svc := &Service{db: db}
	svc.logUsage(context.Background(), uuid.New(), UsageStats{Model: "m", TokensInput: 1, TokensOutput: 2, Duration: time.Second}, "success")
	if !called {
		t.Fatal("expected log usage insert")
	}
}

func TestLogUsageWithTimeout_NoDB(t *testing.T) {
	svc := &Service{}
	svc.logUsageWithTimeout(uuid.New(), UsageStats{}, "success")
}

func TestLogUsageWithTimeout_WritesToDB(t *testing.T) {
	called := false
	db := &fakeDB{
		execFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			called = true
			return nil, nil
		},
	}
	svc := &Service{db: db}
	svc.logUsageWithTimeout(uuid.New(), UsageStats{Model: "m", TokensInput: 1, TokensOutput: 2, Duration: time.Second}, "success")
	if !called {
		t.Fatal("expected log usage insert")
	}
}

func TestStripMarkdownCodeBlock(t *testing.T) {
	input := "```json\n[\"A\"]\n```"
	if got := stripMarkdownCodeBlock(input); got != "[\"A\"]" {
		t.Fatalf("unexpected strip result: %q", got)
	}
}

func TestSanitizeInput_Truncates(t *testing.T) {
	long := strings.Repeat("a", 600)
	got := sanitizeInput(long)
	if len([]rune(got)) != 500 {
		t.Fatalf("expected 500 chars, got %d", len([]rune(got)))
	}
}

func TestGenerateGoals_EscapesAngleBracketsInUserInput(t *testing.T) {
	cfg := &config.Config{AI: config.AIConfig{GeminiAPIKey: "test-key"}}
	service := &Service{
		apiKey: cfg.AI.GeminiAPIKey,
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			var req geminiRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			text := req.Contents[0].Parts[0].Text

			start := strings.Index(text, "<user_focus>")
			end := strings.Index(text, "</user_focus>")
			if start == -1 || end == -1 || end <= start {
				t.Fatalf("missing user_focus block in prompt")
			}
			block := text[start+len("<user_focus>") : end]
			if strings.Contains(block, "<") || strings.Contains(block, ">") {
				t.Fatalf("expected user_focus block to have escaped angle brackets, got %q", block)
			}

			resp := geminiResponse{
				Candidates: []geminiCandidate{
					{
						Content: geminiContent{
							Parts: []geminiPart{
								{Text: mustJSON(t, []string{
									"Goal 1", "Goal 2", "Goal 3", "Goal 4", "Goal 5", "Goal 6",
									"Goal 7", "Goal 8", "Goal 9", "Goal 10", "Goal 11", "Goal 12",
									"Goal 13", "Goal 14", "Goal 15", "Goal 16", "Goal 17", "Goal 18",
									"Goal 19", "Goal 20", "Goal 21", "Goal 22", "Goal 23", "Goal 24",
								})},
							},
						},
						FinishReason: "STOP",
					},
				},
				Usage: geminiUsage{},
			}
			return jsonHTTPResponse(t, http.StatusOK, resp), nil
		})},
		db: nil,
	}

	_, _, err := service.GenerateGoals(context.Background(), uuid.New(), GoalPrompt{
		Category:   "hobbies",
		Focus:      "<b>Cooking</b>",
		Difficulty: "medium",
		Budget:     "free",
		Context:    "test context",
	})
	if err != nil {
		t.Fatalf("GenerateGoals failed: %v", err)
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
