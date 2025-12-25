package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
)

// MockAIService implements AIService interface
type MockAIService struct {
	GenerateGoalsFunc func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error)
	ConsumeFunc       func(ctx context.Context, userID uuid.UUID) (int, error)
	GenerateCalls     int
	ConsumeCalls      int
}

func (m *MockAIService) GenerateGoals(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
	m.GenerateCalls++
	if m.GenerateGoalsFunc == nil {
		return nil, ai.UsageStats{}, errors.New("GenerateGoalsFunc not set")
	}
	return m.GenerateGoalsFunc(ctx, userID, prompt)
}

func (m *MockAIService) ConsumeUnverifiedFreeGeneration(ctx context.Context, userID uuid.UUID) (int, error) {
	m.ConsumeCalls++
	if m.ConsumeFunc == nil {
		return 0, nil
	}
	return m.ConsumeFunc(ctx, userID)
}

func TestGenerate(t *testing.T) {
	// Setup common variables
	validBody := map[string]any{
		"category":   "hobbies",
		"focus":      "cooking",
		"difficulty": "medium",
		"budget":     "low",
		"context":    "none",
	}

	tests := []struct {
		name           string
		requestBody    interface{}
		user           *models.User
		mockSetup      func() *MockAIService
		expectedStatus int
		expectedError  string
		expectedGoals  []string
		freeRemaining  *int
		generateCalls  int
		consumeCalls   int
	}{
		{
			name:        "Success",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						return []string{"Goal 1", "Goal 2"}, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedGoals:  []string{"Goal 1", "Goal 2"},
			generateCalls:  1,
		},
		{
			name:        "Success (Count defaults to 24)",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						if prompt.Count != 24 {
							t.Fatalf("expected count 24, got %d", prompt.Count)
						}
						return []string{"Goal 1", "Goal 2"}, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedGoals:  []string{"Goal 1", "Goal 2"},
			generateCalls:  1,
		},
		{
			name: "Success (Count passed through)",
			requestBody: map[string]any{
				"category":   "hobbies",
				"focus":      "cooking",
				"difficulty": "medium",
				"budget":     "low",
				"context":    "none",
				"count":      5,
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						if prompt.Count != 5 {
							t.Fatalf("expected count 5, got %d", prompt.Count)
						}
						return []string{"Goal 1", "Goal 2"}, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedGoals:  []string{"Goal 1", "Goal 2"},
			generateCalls:  1,
		},
		{
			name: "Invalid Input - Count",
			requestBody: map[string]any{
				"category":   "hobbies",
				"difficulty": "medium",
				"budget":     "low",
				"count":      25,
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when count is invalid")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Count must be between 1 and 24",
		},
		{
			name:        "Success (Unverified consumes quota)",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: false},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					ConsumeFunc: func(ctx context.Context, userID uuid.UUID) (int, error) {
						return 4, nil
					},
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						return []string{"Goal 1", "Goal 2"}, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedGoals:  []string{"Goal 1", "Goal 2"},
			freeRemaining:  ptrToIntValue(4),
			generateCalls:  1,
			consumeCalls:   1,
		},
		{
			name:        "Unverified blocked when quota exhausted",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: false},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					ConsumeFunc: func(ctx context.Context, userID uuid.UUID) (int, error) {
						return 0, ai.ErrEmailVerificationRequired
					},
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when quota is exhausted")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "You've used your 5 free AI generations. Verify your email to keep using AI.",
			freeRemaining:  ptrToIntValue(0),
			consumeCalls:   1,
		},
		{
			name:        "Unauthorized",
			requestBody: validBody,
			user:        nil,
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when unauthorized")
						return nil, ai.UsageStats{}, nil
					},
					ConsumeFunc: func(ctx context.Context, userID uuid.UUID) (int, error) {
						t.Fatal("ConsumeUnverifiedFreeGeneration should not be called when unauthorized")
						return 0, nil
					},
				}
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name: "Invalid Input - Category",
			requestBody: map[string]any{
				"category":   "invalid",
				"difficulty": "medium",
				"budget":     "low",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when category is invalid")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid category",
		},
		{
			name: "Invalid Input - Missing Difficulty",
			requestBody: map[string]any{
				"category": "hobbies",
				"budget":   "low",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when required fields are missing")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Missing required fields",
		},
		{
			name: "Invalid Input - Difficulty",
			requestBody: map[string]any{
				"category":   "hobbies",
				"difficulty": "expert",
				"budget":     "low",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when difficulty is invalid")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid difficulty",
		},
		{
			name: "Invalid Input - Budget",
			requestBody: map[string]any{
				"category":   "hobbies",
				"difficulty": "medium",
				"budget":     "ultra",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when budget is invalid")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid budget",
		},
		{
			name: "Invalid Input - Focus Too Long",
			requestBody: map[string]any{
				"category":   "hobbies",
				"focus":      strings.Repeat("a", 101),
				"difficulty": "medium",
				"budget":     "low",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when focus is too long")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Focus is too long (max 100 chars)",
		},
		{
			name: "Invalid Input - Context Too Long",
			requestBody: map[string]any{
				"category":   "hobbies",
				"context":    strings.Repeat("a", 501),
				"difficulty": "medium",
				"budget":     "low",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when context is too long")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Context is too long (max 500 chars)",
		},
		{
			name:        "Invalid Input - Unknown Field",
			requestBody: `{"category":"hobbies","difficulty":"medium","budget":"low","unknown":true}`,
			user:        &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when body has unknown fields")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name:        "Service Error - Safety",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						return nil, ai.UsageStats{}, ai.ErrSafetyViolation
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "We couldn't generate safe goals for that topic. Please try rephrasing.",
			generateCalls:  1,
		},
		{
			name:        "Service Error - Rate Limit",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						return nil, ai.UsageStats{}, ai.ErrRateLimitExceeded
					},
				}
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedError:  "AI provider rate limit exceeded.",
			generateCalls:  1,
		},
		{
			name:        "Service Error - Unavailable",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						return nil, ai.UsageStats{}, ai.ErrAIProviderUnavailable
					},
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "The AI service is currently down. Please try again later.",
			generateCalls:  1,
		},
		{
			name:        "Service Error - Not Configured",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						return nil, ai.UsageStats{}, ai.ErrAINotConfigured
					},
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "AI is not configured on this server. Please try again later.",
			generateCalls:  1,
		},
		{
			name:        "Service Error - Generic",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						return nil, ai.UsageStats{}, errors.New("unknown error")
					},
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "An unexpected error occurred.",
			generateCalls:  1,
		},
		{
			name:        "Unverified Usage Tracking Unavailable",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: false},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					ConsumeFunc: func(ctx context.Context, userID uuid.UUID) (int, error) {
						return 0, ai.ErrAIUsageTrackingUnavailable
					},
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when tracking is unavailable")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "AI usage tracking is temporarily unavailable. Please try again later.",
			consumeCalls:   1,
		},
		{
			name:        "Unverified Usage Tracking Generic Error",
			requestBody: validBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: false},
			mockSetup: func() *MockAIService {
				return &MockAIService{
					ConsumeFunc: func(ctx context.Context, userID uuid.UUID) (int, error) {
						return 0, errors.New("boom")
					},
					GenerateGoalsFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGoals should not be called when tracking fails")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "AI usage tracking is temporarily unavailable. Please try again later.",
			consumeCalls:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := tt.mockSetup()
			handler := NewAIHandler(mockService)

			var bodyBytes []byte
			switch v := tt.requestBody.(type) {
			case string:
				bodyBytes = []byte(v)
			default:
				bodyBytes, _ = json.Marshal(v)
			}
			req := httptest.NewRequest("POST", "/api/ai/generate", bytes.NewBuffer(bodyBytes))

			// Mock context with user if provided
			if tt.user != nil {
				ctx := SetUserInContext(req.Context(), tt.user)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler.Generate(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if ct := w.Result().Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("expected content type application/json, got %q", ct)
			}

			if len(tt.expectedGoals) > 0 {
				var response GenerateResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				if len(response.Goals) != len(tt.expectedGoals) {
					t.Fatalf("expected %d goals, got %d", len(tt.expectedGoals), len(response.Goals))
				}
				for i, goal := range tt.expectedGoals {
					if response.Goals[i] != goal {
						t.Fatalf("expected goal %d to be %q, got %q", i, goal, response.Goals[i])
					}
				}
				if tt.freeRemaining == nil && response.FreeRemaining != nil {
					t.Fatalf("expected free_remaining to be omitted, got %d", *response.FreeRemaining)
				}
				if tt.freeRemaining != nil {
					if response.FreeRemaining == nil {
						t.Fatal("expected free_remaining to be set")
					}
					if *response.FreeRemaining != *tt.freeRemaining {
						t.Fatalf("expected free_remaining %d, got %d", *tt.freeRemaining, *response.FreeRemaining)
					}
				}
			}

			if tt.expectedError != "" {
				var response map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if response["error"] != tt.expectedError {
					t.Fatalf("expected error %q, got %v", tt.expectedError, response["error"])
				}
				if tt.freeRemaining == nil {
					if _, ok := response["free_remaining"]; ok {
						t.Fatalf("expected free_remaining to be omitted, got %v", response["free_remaining"])
					}
				} else {
					got, ok := response["free_remaining"]
					if !ok {
						t.Fatal("expected free_remaining in response")
					}
					if int(got.(float64)) != *tt.freeRemaining {
						t.Fatalf("expected free_remaining %d, got %v", *tt.freeRemaining, got)
					}
				}
			}

			if tt.generateCalls > 0 && mockService.GenerateCalls != tt.generateCalls {
				t.Fatalf("expected GenerateGoals calls %d, got %d", tt.generateCalls, mockService.GenerateCalls)
			}
			if tt.consumeCalls > 0 && mockService.ConsumeCalls != tt.consumeCalls {
				t.Fatalf("expected ConsumeUnverifiedFreeGeneration calls %d, got %d", tt.consumeCalls, mockService.ConsumeCalls)
			}
			if tt.generateCalls == 0 && mockService.GenerateCalls != 0 {
				t.Fatalf("expected no GenerateGoals calls, got %d", mockService.GenerateCalls)
			}
			if tt.consumeCalls == 0 && mockService.ConsumeCalls != 0 {
				t.Fatalf("expected no ConsumeUnverifiedFreeGeneration calls, got %d", mockService.ConsumeCalls)
			}
		})
	}
}

func ptrToIntValue(value int) *int {
	return &value
}
