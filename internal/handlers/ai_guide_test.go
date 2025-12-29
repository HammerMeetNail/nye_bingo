package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
)

func TestGuide(t *testing.T) {
	validRefineBody := map[string]any{
		"mode":         "refine",
		"current_goal": "Visit a local farmer's market",
		"hint":         "make it budget friendly",
	}
	validNewBody := map[string]any{
		"mode": "new",
		"hint": "weekends",
	}

	tests := []struct {
		name           string
		requestBody    interface{}
		user           *models.User
		mockSetup      func(t *testing.T) *MockAIService
		expectedStatus int
		expectedError  string
		expectedGoals  []string
		freeRemaining  *int
		guideCalls     int
		consumeCalls   int
		refundCalls    int
	}{
		{
			name:        "Success (count defaults by mode)",
			requestBody: validRefineBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: false},
			mockSetup: func(t *testing.T) *MockAIService {
				return &MockAIService{
					ConsumeFunc: func(ctx context.Context, userID uuid.UUID) (int, error) {
						return 4, nil
					},
					GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
						if prompt.Count != 3 {
							t.Fatalf("expected count 3, got %d", prompt.Count)
						}
						if prompt.Mode != "refine" {
							t.Fatalf("expected mode refine, got %q", prompt.Mode)
						}
						return []string{"Goal 1", "Goal 2", "Goal 3"}, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedGoals:  []string{"Goal 1", "Goal 2", "Goal 3"},
			freeRemaining:  ptrToIntValue(4),
			guideCalls:     1,
			consumeCalls:   1,
		},
		{
			name:        "Success (new mode defaults count to 5)",
			requestBody: validNewBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: false},
			mockSetup: func(t *testing.T) *MockAIService {
				return &MockAIService{
					ConsumeFunc: func(ctx context.Context, userID uuid.UUID) (int, error) {
						return 4, nil
					},
					GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
						if prompt.Count != 5 {
							t.Fatalf("expected count 5, got %d", prompt.Count)
						}
						if prompt.Mode != "new" {
							t.Fatalf("expected mode new, got %q", prompt.Mode)
						}
						return []string{"Goal 1", "Goal 2", "Goal 3", "Goal 4", "Goal 5"}, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedGoals:  []string{"Goal 1", "Goal 2", "Goal 3", "Goal 4", "Goal 5"},
			freeRemaining:  ptrToIntValue(4),
			guideCalls:     1,
			consumeCalls:   1,
		},
		{
			name:        "Invalid body",
			requestBody: "{nope",
			user:        &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func(t *testing.T) *MockAIService {
				return &MockAIService{
					GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGuideGoals should not be called when body is invalid")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name: "Invalid mode",
			requestBody: map[string]any{
				"mode": "nope",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func(t *testing.T) *MockAIService {
				return &MockAIService{
					GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGuideGoals should not be called when mode is invalid")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid mode",
		},
		{
			name: "Missing current goal for refine",
			requestBody: map[string]any{
				"mode": "refine",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func(t *testing.T) *MockAIService {
				return &MockAIService{
					GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGuideGoals should not be called when current_goal is missing")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Current goal is required",
		},
		{
			name: "Count out of bounds",
			requestBody: map[string]any{
				"mode":         "new",
				"current_goal": "",
				"count":        6,
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func(t *testing.T) *MockAIService {
				return &MockAIService{
					GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGuideGoals should not be called when count is invalid")
						return nil, ai.UsageStats{}, nil
					},
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Count must be between 1 and 5",
		},
		{
			name:        "Unauthorized",
			requestBody: validRefineBody,
			user:        nil,
			mockSetup: func(t *testing.T) *MockAIService {
				return &MockAIService{
					GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGuideGoals should not be called when unauthorized")
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
			name:        "Unverified blocked when quota exhausted",
			requestBody: validRefineBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: false},
			mockSetup: func(t *testing.T) *MockAIService {
				return &MockAIService{
					ConsumeFunc: func(ctx context.Context, userID uuid.UUID) (int, error) {
						return 0, ai.ErrEmailVerificationRequired
					},
					GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
						t.Fatal("GenerateGuideGoals should not be called when quota is exhausted")
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
			name:        "Service error refunds quota",
			requestBody: validRefineBody,
			user:        &models.User{ID: uuid.New(), EmailVerified: false},
			mockSetup: func(t *testing.T) *MockAIService {
				return &MockAIService{
					ConsumeFunc: func(ctx context.Context, userID uuid.UUID) (int, error) {
						return 4, nil
					},
					RefundFunc: func(ctx context.Context, userID uuid.UUID) (bool, error) {
						return true, nil
					},
					GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
						return nil, ai.UsageStats{}, ai.ErrAIProviderUnavailable
					},
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedError:  "The AI service is currently down. Please try again later.",
			freeRemaining:  ptrToIntValue(5),
			guideCalls:     1,
			consumeCalls:   1,
			refundCalls:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := tt.mockSetup(t)
			handler := NewAIHandler(mockService)

			var bodyBytes []byte
			switch v := tt.requestBody.(type) {
			case string:
				bodyBytes = []byte(v)
			default:
				bodyBytes, _ = json.Marshal(v)
			}
			req := httptest.NewRequest("POST", "/api/ai/guide", bytes.NewBuffer(bodyBytes))

			if tt.user != nil {
				req = req.WithContext(SetUserInContext(req.Context(), tt.user))
			}

			w := httptest.NewRecorder()
			handler.Guide(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if ct := w.Result().Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("expected content type application/json, got %q", ct)
			}

			if len(tt.expectedGoals) > 0 {
				var response GuideResponse
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
					t.Fatalf("expected error %q, got %q", tt.expectedError, response["error"])
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

			if tt.guideCalls > 0 && mockService.GenerateGuideCalls != tt.guideCalls {
				t.Fatalf("expected GenerateGuideGoals calls %d, got %d", tt.guideCalls, mockService.GenerateGuideCalls)
			}
			if tt.consumeCalls > 0 && mockService.ConsumeCalls != tt.consumeCalls {
				t.Fatalf("expected ConsumeUnverifiedFreeGeneration calls %d, got %d", tt.consumeCalls, mockService.ConsumeCalls)
			}
			if tt.refundCalls > 0 && mockService.RefundCalls != tt.refundCalls {
				t.Fatalf("expected RefundUnverifiedFreeGeneration calls %d, got %d", tt.refundCalls, mockService.RefundCalls)
			}
			if tt.guideCalls == 0 && mockService.GenerateGuideCalls != 0 {
				t.Fatalf("expected no GenerateGuideGoals calls, got %d", mockService.GenerateGuideCalls)
			}
			if tt.consumeCalls == 0 && mockService.ConsumeCalls != 0 {
				t.Fatalf("expected no ConsumeUnverifiedFreeGeneration calls, got %d", mockService.ConsumeCalls)
			}
			if tt.refundCalls == 0 && mockService.RefundCalls != 0 {
				t.Fatalf("expected no RefundUnverifiedFreeGeneration calls, got %d", mockService.RefundCalls)
			}
		})
	}
}

func TestGuide_ErrorMappingInvalidInput(t *testing.T) {
	mockService := &MockAIService{
		GenerateGuideFunc: func(ctx context.Context, userID uuid.UUID, prompt ai.GuidePrompt) ([]string, ai.UsageStats, error) {
			return nil, ai.UsageStats{}, ai.ErrInvalidInput
		},
	}
	handler := NewAIHandler(mockService)

	bodyBytes, _ := json.Marshal(map[string]any{
		"mode":         "refine",
		"current_goal": "Test",
		"count":        3,
	})
	req := httptest.NewRequest("POST", "/api/ai/guide", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), EmailVerified: true}))

	w := httptest.NewRecorder()
	handler.Guide(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response["error"] != "Invalid AI request." {
		t.Fatalf("expected error %q, got %q", "Invalid AI request.", response["error"])
	}
	if mockService.GenerateGuideCalls != 1 {
		t.Fatalf("expected GenerateGuideGoals to be called once, got %d", mockService.GenerateGuideCalls)
	}
}
