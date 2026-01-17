package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

const (
	sessionCookieName = "session_token"
	cookieMaxAge      = 30 * 24 * 60 * 60 // 30 days in seconds
)

type AuthHandler struct {
	userService  services.UserServiceInterface
	authService  services.AuthServiceInterface
	emailService services.EmailServiceInterface
	secure       bool // Use secure cookies (HTTPS only)
}

func NewAuthHandler(userService services.UserServiceInterface, authService services.AuthServiceInterface, emailService services.EmailServiceInterface, secure bool) *AuthHandler {
	return &AuthHandler{
		userService:  userService,
		authService:  authService,
		emailService: emailService,
		secure:       secure,
	}
}

type RegisterRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Username   string `json:"username"`
	Searchable bool   `json:"searchable"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	User    *models.User `json:"user"`
	Message string       `json:"message,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate email
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid email address")
		return
	}

	// Validate password
	if err := validatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate username
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 2 || len(req.Username) > 100 {
		writeError(w, http.StatusBadRequest, "Username must be between 2 and 100 characters")
		return
	}

	// Hash password
	passwordHash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		if errors.Is(err, services.ErrPasswordTooLong) {
			writeError(w, http.StatusBadRequest, "Password is too long")
			return
		}
		log.Printf("Error hashing password: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Create user
	user, err := h.userService.Create(r.Context(), models.CreateUserParams{
		Email:        req.Email,
		PasswordHash: &passwordHash,
		Username:     req.Username,
		Searchable:   req.Searchable,
	})
	if errors.Is(err, services.ErrEmailAlreadyExists) {
		writeError(w, http.StatusConflict, "Email already registered")
		return
	}
	if errors.Is(err, services.ErrUsernameAlreadyExists) {
		writeError(w, http.StatusConflict, "Username already taken")
		return
	}
	if err != nil {
		log.Printf("Error creating user: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Create session
	token, err := h.authService.CreateSession(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error creating session: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Send verification email (non-blocking, don't fail registration if email fails)
	// Use context.Background() since the request context will be canceled when the response is sent
	if h.emailService != nil {
		go func() {
			if err := h.emailService.SendVerificationEmail(context.Background(), user.ID, user.Email); err != nil {
				log.Printf("Error sending verification email: %v", err)
			}
		}()
	}

	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusCreated, AuthResponse{User: user})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	// Get user by email
	user, err := h.userService.GetByEmail(r.Context(), req.Email)
	if errors.Is(err, services.ErrUserNotFound) {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}
	if err != nil {
		log.Printf("Error getting user: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if user.PasswordHash == nil {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Verify password
	if !h.authService.VerifyPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Create session
	token, err := h.authService.CreateSession(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error creating session: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, AuthResponse{User: user})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = h.authService.DeleteSession(r.Context(), cookie.Value)
	}

	h.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, AuthResponse{Message: "Logged out successfully"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{User: user})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if user.PasswordHash == nil {
		writeError(w, http.StatusBadRequest, "No password set; use password reset to set one.")
		return
	}

	// Verify current password
	if !h.authService.VerifyPassword(user.PasswordHash, req.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "Current password is incorrect")
		return
	}

	// Validate new password
	if err := validatePassword(req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Hash new password
	newHash, err := h.authService.HashPassword(req.NewPassword)
	if err != nil {
		if errors.Is(err, services.ErrPasswordTooLong) {
			writeError(w, http.StatusBadRequest, "Password is too long")
			return
		}
		log.Printf("Error hashing password: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Update password
	if err := h.userService.UpdatePassword(r.Context(), user.ID, newHash); err != nil {
		log.Printf("Error updating password: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Invalidate all other sessions
	_ = h.authService.DeleteAllUserSessions(r.Context(), user.ID)

	// Create new session
	token, err := h.authService.CreateSession(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error creating session: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, AuthResponse{Message: "Password changed successfully"})
}

// VerifyEmail handles email verification via token
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "Token is required")
		return
	}

	if err := h.emailService.VerifyEmail(r.Context(), req.Token); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Email verified successfully"})
}

// ResendVerification resends the verification email
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	if user.EmailVerified {
		writeError(w, http.StatusBadRequest, "Email is already verified")
		return
	}

	if err := h.emailService.SendVerificationEmail(r.Context(), user.ID, user.Email); err != nil {
		log.Printf("Error sending verification email: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to send verification email")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Verification email sent"})
}

// MagicLink sends a magic link for passwordless login
func (h *AuthHandler) MagicLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid email address")
		return
	}

	// Check if user exists - but always return success to prevent email enumeration
	user, err := h.userService.GetByEmail(r.Context(), req.Email)
	if err == nil && user != nil {
		// User exists, send magic link
		if err := h.emailService.SendMagicLinkEmail(r.Context(), req.Email); err != nil {
			log.Printf("Error sending magic link email: %v", err)
		}
	}

	// Always return success to prevent email enumeration
	writeJSON(w, http.StatusOK, map[string]string{"message": "If an account exists, a login link has been sent"})
}

// MagicLinkVerify verifies a magic link token and creates a session
func (h *AuthHandler) MagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "Token is required")
		return
	}

	email, err := h.emailService.VerifyMagicLink(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get or create user
	user, err := h.userService.GetByEmail(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "User not found")
		return
	}

	// Mark email as verified since they clicked a link sent to their email
	if !user.EmailVerified {
		if err := h.userService.MarkEmailVerified(r.Context(), user.ID); err != nil {
			log.Printf("Error marking email verified: %v", err)
		} else {
			// Re-fetch user to get updated verification status
			user, _ = h.userService.GetByID(r.Context(), user.ID)
		}
	}

	// Create session
	sessionToken, err := h.authService.CreateSession(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error creating session: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	h.setSessionCookie(w, sessionToken)
	writeJSON(w, http.StatusOK, AuthResponse{User: user})
}

// ForgotPassword sends a password reset email
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid email address")
		return
	}

	// Check if user exists - but always return success to prevent email enumeration
	user, err := h.userService.GetByEmail(r.Context(), req.Email)
	if err == nil && user != nil {
		// User exists, send reset email
		if err := h.emailService.SendPasswordResetEmail(r.Context(), user.ID, user.Email); err != nil {
			log.Printf("Error sending password reset email: %v", err)
		}
	}

	// Always return success to prevent email enumeration
	writeJSON(w, http.StatusOK, map[string]string{"message": "If an account exists, reset instructions have been sent"})
}

// ResetPassword resets the password using a token
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "Token is required")
		return
	}

	// Validate new password
	if err := validatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Verify token and get user ID
	userID, err := h.emailService.VerifyPasswordResetToken(r.Context(), req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Hash new password
	passwordHash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		if errors.Is(err, services.ErrPasswordTooLong) {
			writeError(w, http.StatusBadRequest, "Password is too long")
			return
		}
		log.Printf("Error hashing password: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Update password
	if err := h.userService.UpdatePassword(r.Context(), userID, passwordHash); err != nil {
		log.Printf("Error updating password: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Mark token as used
	if err := h.emailService.MarkPasswordResetUsed(r.Context(), req.Token); err != nil {
		log.Printf("Error marking reset token as used: %v", err)
	}

	// Invalidate all sessions
	_ = h.authService.DeleteAllUserSessions(r.Context(), userID)

	// Mark email as verified since they clicked a link sent to their email
	if err := h.userService.MarkEmailVerified(r.Context(), userID); err != nil {
		log.Printf("Error marking email verified: %v", err)
	}

	// Get user for response
	user, err := h.userService.GetByID(r.Context(), userID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Create new session
	sessionToken, err := h.authService.CreateSession(r.Context(), userID)
	if err != nil {
		log.Printf("Error creating session: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	h.setSessionCookie(w, sessionToken)
	writeJSON(w, http.StatusOK, AuthResponse{User: user, Message: "Password reset successfully"})
}

type UpdateSearchableRequest struct {
	Searchable bool `json:"searchable"`
}

func (h *AuthHandler) UpdateSearchable(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req UpdateSearchableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.userService.UpdateSearchable(r.Context(), user.ID, req.Searchable); err != nil {
		log.Printf("Error updating searchable: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Fetch updated user
	updatedUser, err := h.userService.GetByID(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{User: updatedUser, Message: "Privacy settings updated"})
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
	})
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if len([]byte(password)) > 72 {
		return errors.New("password must be at most 72 bytes")
	}

	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit {
		return errors.New("password must contain at least one uppercase letter, one lowercase letter, and one digit")
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}
