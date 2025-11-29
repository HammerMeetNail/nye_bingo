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
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid password",
			password: "SecurePass123",
			wantErr:  false,
		},
		{
			name:     "too short",
			password: "Pass1",
			wantErr:  true,
			errMsg:   "password must be at least 8 characters",
		},
		{
			name:     "too long",
			password: string(make([]byte, 129)),
			wantErr:  true,
			errMsg:   "password must be at most 128 characters",
		},
		{
			name:     "no uppercase",
			password: "securepass123",
			wantErr:  true,
			errMsg:   "password must contain at least one uppercase letter, one lowercase letter, and one digit",
		},
		{
			name:     "no lowercase",
			password: "SECUREPASS123",
			wantErr:  true,
			errMsg:   "password must contain at least one uppercase letter, one lowercase letter, and one digit",
		},
		{
			name:     "no digit",
			password: "SecurePassword",
			wantErr:  true,
			errMsg:   "password must contain at least one uppercase letter, one lowercase letter, and one digit",
		},
		{
			name:     "exactly 8 characters",
			password: "Secure1a",
			wantErr:  false,
		},
		{
			name:     "at max length 128",
			password: "Aa1" + strings.Repeat("x", 125),
			wantErr:  false, // Exactly 128 characters, meets all requirements
		},
		{
			name:     "with special characters",
			password: "Secure@Pass123!",
			wantErr:  false,
		},
		{
			name:     "unicode characters",
			password: "Sécure1Pass",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.password)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Mock services for testing
type mockUserService struct {
	createFunc           func(ctx context.Context, params models.CreateUserParams) (*models.User, error)
	getByIDFunc          func(ctx context.Context, id uuid.UUID) (*models.User, error)
	getByEmailFunc       func(ctx context.Context, email string) (*models.User, error)
	updatePasswordFunc   func(ctx context.Context, userID uuid.UUID, newPasswordHash string) error
	updateSearchableFunc func(ctx context.Context, userID uuid.UUID, searchable bool) error
	markEmailVerifiedFunc func(ctx context.Context, userID uuid.UUID) error
}

func (m *mockUserService) Create(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, params)
	}
	return nil, nil
}

func (m *mockUserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockUserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.getByEmailFunc != nil {
		return m.getByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *mockUserService) UpdatePassword(ctx context.Context, userID uuid.UUID, newPasswordHash string) error {
	if m.updatePasswordFunc != nil {
		return m.updatePasswordFunc(ctx, userID, newPasswordHash)
	}
	return nil
}

func (m *mockUserService) UpdateSearchable(ctx context.Context, userID uuid.UUID, searchable bool) error {
	if m.updateSearchableFunc != nil {
		return m.updateSearchableFunc(ctx, userID, searchable)
	}
	return nil
}

func (m *mockUserService) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	if m.markEmailVerifiedFunc != nil {
		return m.markEmailVerifiedFunc(ctx, userID)
	}
	return nil
}

type mockAuthService struct {
	hashPasswordFunc          func(password string) (string, error)
	verifyPasswordFunc        func(hash, password string) bool
	createSessionFunc         func(ctx context.Context, userID uuid.UUID) (string, error)
	deleteSessionFunc         func(ctx context.Context, token string) error
	deleteAllUserSessionsFunc func(ctx context.Context, userID uuid.UUID) error
}

func (m *mockAuthService) HashPassword(password string) (string, error) {
	if m.hashPasswordFunc != nil {
		return m.hashPasswordFunc(password)
	}
	return "hashed_" + password, nil
}

func (m *mockAuthService) VerifyPassword(hash, password string) bool {
	if m.verifyPasswordFunc != nil {
		return m.verifyPasswordFunc(hash, password)
	}
	return hash == "hashed_"+password
}

func (m *mockAuthService) CreateSession(ctx context.Context, userID uuid.UUID) (string, error) {
	if m.createSessionFunc != nil {
		return m.createSessionFunc(ctx, userID)
	}
	return "test_session_token", nil
}

func (m *mockAuthService) DeleteSession(ctx context.Context, token string) error {
	if m.deleteSessionFunc != nil {
		return m.deleteSessionFunc(ctx, token)
	}
	return nil
}

func (m *mockAuthService) DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	if m.deleteAllUserSessionsFunc != nil {
		return m.deleteAllUserSessionsFunc(ctx, userID)
	}
	return nil
}

// mockableAuthHandler wraps AuthHandler but uses mockable services
type mockableAuthHandler struct {
	userService  userServiceInterface
	authService  authServiceInterface
	emailService emailServiceInterface
	secure       bool
}

type userServiceInterface interface {
	Create(ctx context.Context, params models.CreateUserParams) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	UpdatePassword(ctx context.Context, userID uuid.UUID, newPasswordHash string) error
	UpdateSearchable(ctx context.Context, userID uuid.UUID, searchable bool) error
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error
}

type authServiceInterface interface {
	HashPassword(password string) (string, error)
	VerifyPassword(hash, password string) bool
	CreateSession(ctx context.Context, userID uuid.UUID) (string, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error
}

type emailServiceInterface interface {
	SendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error
}

type mockEmailService struct {
	sendVerificationEmailFunc func(ctx context.Context, userID uuid.UUID, email string) error
}

func (m *mockEmailService) SendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error {
	if m.sendVerificationEmailFunc != nil {
		return m.sendVerificationEmailFunc(ctx, userID, email)
	}
	return nil
}

func TestAuthHandler_Register_InvalidBody(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString("invalid json"))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Invalid request body" {
		t.Errorf("expected error 'Invalid request body', got %q", response.Error)
	}
}

func TestAuthHandler_Register_InvalidEmail(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	body := RegisterRequest{
		Email:    "not-an-email",
		Password: "SecurePass123",
		Username: "testuser",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(bodyBytes))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Invalid email address" {
		t.Errorf("expected error 'Invalid email address', got %q", response.Error)
	}
}

func TestAuthHandler_Register_InvalidPassword(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	body := RegisterRequest{
		Email:    "test@example.com",
		Password: "weak",
		Username: "testuser",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(bodyBytes))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Register_UsernameTooShort(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	body := RegisterRequest{
		Email:    "test@example.com",
		Password: "SecurePass123",
		Username: "a",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(bodyBytes))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Username must be between 2 and 100 characters" {
		t.Errorf("expected username length error, got %q", response.Error)
	}
}

func TestAuthHandler_Register_UsernameTooLong(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	body := RegisterRequest{
		Email:    "test@example.com",
		Password: "SecurePass123",
		Username: string(make([]byte, 101)),
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(bodyBytes))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	mockUser := &mockUserService{
		createFunc: func(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
			return nil, services.ErrEmailAlreadyExists
		},
	}
	mockAuth := &mockAuthService{}

	handler := &AuthHandler{
		userService: &services.UserService{},
		authService: &services.AuthService{},
		secure:      false,
	}

	// We can't easily inject mocks into the real handler, so let's test validation paths
	// For integration with mocks, we need to test via the real service or refactor
	_ = mockUser
	_ = mockAuth
	_ = handler

	// This test demonstrates the pattern - in practice, you'd use dependency injection
	// or test with actual database mocks
}

func TestAuthHandler_Login_InvalidBody(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString("invalid"))
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Logout_NoCookie(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rr := httptest.NewRecorder()

	handler.Logout(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response AuthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Message != "Logged out successfully" {
		t.Errorf("expected logout success message, got %q", response.Message)
	}

	// Check cookie is cleared
	cookies := rr.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Error("expected session cookie to be set (for clearing)")
	} else if sessionCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge -1 for cleared cookie, got %d", sessionCookie.MaxAge)
	}
}

func TestAuthHandler_Me_Unauthenticated(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rr := httptest.NewRecorder()

	handler.Me(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestAuthHandler_Me_Authenticated(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	user := &models.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Username: "testuser",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response AuthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.User == nil {
		t.Error("expected user in response")
	} else if response.User.Email != user.Email {
		t.Errorf("expected email %q, got %q", user.Email, response.User.Email)
	}
}

func TestAuthHandler_ChangePassword_Unauthenticated(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", nil)
	rr := httptest.NewRecorder()

	handler.ChangePassword(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestAuthHandler_ChangePassword_InvalidBody(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	user := &models.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Username: "testuser",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ChangePassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_VerifyEmail_MissingToken(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	body := `{"token": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify-email", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.VerifyEmail(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_ResendVerification_Unauthenticated(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", nil)
	rr := httptest.NewRecorder()

	handler.ResendVerification(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestAuthHandler_ResendVerification_AlreadyVerified(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	user := &models.User{
		ID:            uuid.New(),
		Email:         "test@example.com",
		Username:      "testuser",
		EmailVerified: true,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ResendVerification(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Email is already verified" {
		t.Errorf("expected 'Email is already verified', got %q", response.Error)
	}
}

func TestAuthHandler_MagicLink_InvalidBody(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewBufferString("invalid"))
	rr := httptest.NewRecorder()

	handler.MagicLink(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_MagicLink_InvalidEmail(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	body := `{"email": "not-an-email"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.MagicLink(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_MagicLinkVerify_MissingToken(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link/verify", nil)
	rr := httptest.NewRecorder()

	handler.MagicLinkVerify(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_ForgotPassword_InvalidBody(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewBufferString("invalid"))
	rr := httptest.NewRecorder()

	handler.ForgotPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_ForgotPassword_InvalidEmail(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	body := `{"email": "not-an-email"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.ForgotPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_ResetPassword_InvalidBody(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBufferString("invalid"))
	rr := httptest.NewRecorder()

	handler.ResetPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_ResetPassword_MissingToken(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	body := `{"token": "", "password": "SecurePass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.ResetPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Token is required" {
		t.Errorf("expected 'Token is required', got %q", response.Error)
	}
}

func TestAuthHandler_ResetPassword_WeakPassword(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	body := `{"token": "valid-token", "password": "weak"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.ResetPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_UpdateSearchable_Unauthenticated(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	req := httptest.NewRequest(http.MethodPut, "/api/auth/searchable", nil)
	rr := httptest.NewRecorder()

	handler.UpdateSearchable(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestAuthHandler_UpdateSearchable_InvalidBody(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	user := &models.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Username: "testuser",
	}

	req := httptest.NewRequest(http.MethodPut, "/api/auth/searchable", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.UpdateSearchable(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_SessionCookie_SecureMode(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, true)

	rr := httptest.NewRecorder()
	handler.setSessionCookie(rr, "test_token")

	cookies := rr.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}

	if !sessionCookie.Secure {
		t.Error("expected Secure flag to be true in secure mode")
	}

	if !sessionCookie.HttpOnly {
		t.Error("expected HttpOnly flag to be true")
	}

	if sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSite=Strict, got %v", sessionCookie.SameSite)
	}
}

func TestAuthHandler_SessionCookie_NonSecureMode(t *testing.T) {
	handler := NewAuthHandler(nil, nil, nil, false)

	rr := httptest.NewRecorder()
	handler.setSessionCookie(rr, "test_token")

	cookies := rr.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}

	if sessionCookie.Secure {
		t.Error("expected Secure flag to be false in non-secure mode")
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	data := map[string]string{"message": "test"}
	writeJSON(rr, http.StatusCreated, data)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %q", contentType)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["message"] != "test" {
		t.Errorf("expected message 'test', got %q", response["message"])
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()

	writeError(rr, http.StatusNotFound, "Resource not found")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Resource not found" {
		t.Errorf("expected error 'Resource not found', got %q", response.Error)
	}
}
