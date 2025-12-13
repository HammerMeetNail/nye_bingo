package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
)

// MockAIService implements AIService interface
type MockAIService struct {
	GenerateGoalsFunc func(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error)
	ConsumeFunc       func(ctx context.Context, userID uuid.UUID) (int, error)
}

func (m *MockAIService) GenerateGoals(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error) {
	return m.GenerateGoalsFunc(ctx, userID, prompt)
}

func (m *MockAIService) ConsumeUnverifiedFreeGeneration(ctx context.Context, userID uuid.UUID) (int, error) {
	if m.ConsumeFunc == nil {
		return 0, nil
	}
	return m.ConsumeFunc(ctx, userID)
}

func TestGenerate(t *testing.T) {
	// Setup common variables
	validBody := map[string]string{
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
		},
		{
			name:        "Unauthorized",
			requestBody: validBody,
			user:        nil,
			mockSetup: func() *MockAIService {
				return &MockAIService{}
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Invalid Input - Category",
			requestBody: map[string]string{
				"category":   "invalid",
				"difficulty": "medium",
				"budget":     "low",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{}
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Invalid Input - Missing Difficulty",
			requestBody: map[string]string{
				"category": "hobbies",
				"budget":   "low",
			},
			user: &models.User{ID: uuid.New(), EmailVerified: true},
			mockSetup: func() *MockAIService {
				return &MockAIService{}
			},
			expectedStatus: http.StatusBadRequest,
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := tt.mockSetup()
			handler := NewAIHandler(mockService)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/ai/generate", bytes.NewBuffer(body))

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
		})
	}
}
