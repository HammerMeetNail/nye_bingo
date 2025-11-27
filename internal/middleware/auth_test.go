package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestAuthMiddleware_RequireAuth_NoUser(t *testing.T) {
	// Create a mock AuthMiddleware with nil authService
	// In practice this tests the RequireAuth behavior
	am := &AuthMiddleware{authService: nil}

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	rr := httptest.NewRecorder()

	am.RequireAuth(handler).ServeHTTP(rr, req)

	if handlerCalled {
		t.Error("handler should not be called without authenticated user")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}

	// Check response body
	expected := `{"error":"Authentication required"}`
	if got := rr.Body.String(); got != expected {
		t.Errorf("expected body %q, got %q", expected, got)
	}
}

func TestAuthMiddleware_RequireAuth_WithUser(t *testing.T) {
	am := &AuthMiddleware{authService: nil}

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// Verify user is in context
		user := handlers.GetUserFromContext(r.Context())
		if user == nil {
			t.Error("expected user in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	// Create a request with user in context
	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	ctx := handlers.SetUserInContext(req.Context(), &models.User{
		ID:          uuid.New(),
		Email:       "test@example.com",
		DisplayName: "Test User",
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	am.RequireAuth(handler).ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler should be called with authenticated user")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_Authenticate_NoCookie(t *testing.T) {
	am := &AuthMiddleware{authService: nil}

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// User should be nil since no cookie provided
		user := handlers.GetUserFromContext(r.Context())
		if user != nil {
			t.Error("expected no user in context when no cookie")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/public", nil)
	rr := httptest.NewRecorder()

	am.Authenticate(handler).ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler should be called even without authentication")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_Authenticate_EmptyCookie(t *testing.T) {
	am := &AuthMiddleware{authService: nil}

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		user := handlers.GetUserFromContext(r.Context())
		if user != nil {
			t.Error("expected no user in context when empty cookie")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/public", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ""})
	rr := httptest.NewRecorder()

	am.Authenticate(handler).ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler should be called even with empty cookie")
	}
}

func TestAuthMiddleware_ContentType(t *testing.T) {
	am := &AuthMiddleware{authService: nil}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	rr := httptest.NewRecorder()

	am.RequireAuth(handler).ServeHTTP(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %q", contentType)
	}
}
