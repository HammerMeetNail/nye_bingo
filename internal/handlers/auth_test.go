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

func TestAuthHandler_Register_Success(t *testing.T) {
	createdUser := &models.User{ID: uuid.New(), Email: "test@example.com", Username: "testuser"}
	mockUser := &mockUserService{
		CreateFunc: func(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
			if params.Email != createdUser.Email {
				t.Fatalf("unexpected email: %s", params.Email)
			}
			if params.PasswordHash != "hashed_password" {
				t.Fatalf("unexpected password hash: %s", params.PasswordHash)
			}
			return createdUser, nil
		},
	}
	mockAuth := &mockAuthService{
		HashPasswordFunc: func(password string) (string, error) {
			if password != "SecurePass123" {
				t.Fatalf("unexpected password: %s", password)
			}
			return "hashed_password", nil
		},
		CreateSessionFunc: func(ctx context.Context, userID uuid.UUID) (string, error) {
			if userID != createdUser.ID {
				t.Fatalf("unexpected user id: %s", userID)
			}
			return "session-token", nil
		},
	}

	handler := NewAuthHandler(mockUser, mockAuth, &mockEmailService{}, false)

	body := RegisterRequest{Email: "test@example.com", Password: "SecurePass123", Username: "testuser", Searchable: true}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(bodyBytes))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}

	var response AuthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.User == nil || response.User.ID != createdUser.ID {
		t.Fatalf("expected returned user %s", createdUser.ID)
	}

	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			found = true
			if c.Value != "session-token" {
				t.Fatalf("expected session token cookie, got %s", c.Value)
			}
		}
	}
	if !found {
		t.Fatal("expected session cookie to be set")
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
		CreateFunc: func(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
			return nil, services.ErrEmailAlreadyExists
		},
	}

	handler := NewAuthHandler(mockUser, &mockAuthService{}, nil, false)

	body := RegisterRequest{Email: "test@example.com", Password: "SecurePass123", Username: "testuser"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(bodyBytes))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Email already registered" {
		t.Fatalf("unexpected error message: %s", response.Error)
	}
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

func TestAuthHandler_Login_Success(t *testing.T) {
	user := &models.User{ID: uuid.New(), Email: "test@example.com", PasswordHash: "stored-hash"}
	mockUser := &mockUserService{
		GetByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			if email != user.Email {
				t.Fatalf("unexpected email lookup: %s", email)
			}
			return user, nil
		},
	}
	mockAuth := &mockAuthService{
		VerifyPasswordFunc: func(hash, password string) bool {
			return hash == "stored-hash" && password == "SecurePass123"
		},
		CreateSessionFunc: func(ctx context.Context, userID uuid.UUID) (string, error) {
			if userID != user.ID {
				t.Fatalf("unexpected session user id: %s", userID)
			}
			return "login-session", nil
		},
	}

	handler := NewAuthHandler(mockUser, mockAuth, nil, false)

	body := LoginRequest{Email: "test@example.com", Password: "SecurePass123"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(bodyBytes))
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var response AuthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response.User == nil || response.User.ID != user.ID {
		t.Fatalf("expected user %s", user.ID)
	}
}

func TestAuthHandler_Login_InvalidPassword(t *testing.T) {
	user := &models.User{ID: uuid.New(), Email: "test@example.com", PasswordHash: "stored-hash"}
	mockUser := &mockUserService{
		GetByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return user, nil
		},
	}
	mockAuth := &mockAuthService{
		VerifyPasswordFunc: func(hash, password string) bool { return false },
	}

	handler := NewAuthHandler(mockUser, mockAuth, nil, false)

	body := LoginRequest{Email: "test@example.com", Password: "wrong"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(bodyBytes))
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestAuthHandler_Login_UserNotFound(t *testing.T) {
	mockUser := &mockUserService{
		GetByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return nil, services.ErrUserNotFound
		},
	}

	handler := NewAuthHandler(mockUser, &mockAuthService{}, nil, false)

	body := LoginRequest{Email: "missing@example.com", Password: "SecurePass123"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(bodyBytes))
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
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
		return
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
		return
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
