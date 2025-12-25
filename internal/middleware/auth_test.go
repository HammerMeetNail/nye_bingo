package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type middlewareFakeRow struct {
	values []any
}

func (m middlewareFakeRow) Scan(dest ...any) error {
	if len(dest) != len(m.values) {
		return http.ErrAbortHandler
	}
	for i, value := range m.values {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr || dv.IsNil() {
			return http.ErrAbortHandler
		}
		if value == nil {
			dv.Elem().Set(reflect.Zero(dv.Elem().Type()))
			continue
		}
		vv := reflect.ValueOf(value)
		if vv.Type().AssignableTo(dv.Elem().Type()) {
			dv.Elem().Set(vv)
		} else if vv.Type().ConvertibleTo(dv.Elem().Type()) {
			dv.Elem().Set(vv.Convert(dv.Elem().Type()))
		}
	}
	return nil
}

type middlewareFakeDB struct {
	execFunc     func(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
	queryRowFunc func(ctx context.Context, sql string, args ...any) services.Row
}

func (m *middlewareFakeDB) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	if m.execFunc != nil {
		return m.execFunc(ctx, sql, args...)
	}
	return nil, nil
}

func (m *middlewareFakeDB) Query(ctx context.Context, sql string, args ...any) (services.Rows, error) {
	return nil, nil
}

func (m *middlewareFakeDB) QueryRow(ctx context.Context, sql string, args ...any) services.Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(ctx, sql, args...)
	}
	return middlewareFakeRow{values: []any{}}
}

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
		ID:       uuid.New(),
		Email:    "test@example.com",
		Username: "Test User",
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

func TestAuthMiddleware_NewAuthMiddleware(t *testing.T) {
	am := NewAuthMiddleware(nil, nil, nil)
	if am == nil {
		t.Fatal("expected auth middleware instance")
	}
}

func TestAuthMiddleware_RequireScope_Unauthenticated(t *testing.T) {
	am := &AuthMiddleware{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected handler call")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	rr := httptest.NewRecorder()

	am.RequireScope(models.ScopeRead)(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_RequireScope_AllowsSession(t *testing.T) {
	am := &AuthMiddleware{}
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	ctx := handlers.SetUserInContext(req.Context(), &models.User{ID: uuid.New()})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	am.RequireScope(models.ScopeRead)(handler).ServeHTTP(rr, req)
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}
}

func TestAuthMiddleware_RequireScope_AllowsWriteForRead(t *testing.T) {
	am := &AuthMiddleware{}
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	ctx := handlers.SetUserInContext(req.Context(), &models.User{ID: uuid.New()})
	ctx = handlers.SetTokenScopeInContext(ctx, models.ScopeWrite)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	am.RequireScope(models.ScopeRead)(handler).ServeHTTP(rr, req)
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}
}

func TestAuthMiddleware_RequireScope_Forbidden(t *testing.T) {
	am := &AuthMiddleware{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected handler call")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	ctx := handlers.SetUserInContext(req.Context(), &models.User{ID: uuid.New()})
	ctx = handlers.SetTokenScopeInContext(ctx, models.ScopeRead)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	am.RequireScope(models.ScopeWrite)(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestAuthMiddleware_RequireSession_RejectsToken(t *testing.T) {
	am := &AuthMiddleware{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected handler call")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	ctx := handlers.SetTokenScopeInContext(req.Context(), models.ScopeRead)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	am.RequireSession(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestAuthMiddleware_RequireSession_AllowsSession(t *testing.T) {
	am := &AuthMiddleware{}
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	rr := httptest.NewRecorder()

	am.RequireSession(handler).ServeHTTP(rr, req)
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}
}

func TestAuthMiddleware_Authenticate_BearerToken(t *testing.T) {
	userID := uuid.New()
	tokenID := uuid.New()
	now := time.Now()
	db := &middlewareFakeDB{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			if strings.Contains(sql, "FROM api_tokens") {
				return middlewareFakeRow{values: []any{
					tokenID, userID, "token", "yob_", models.ScopeRead, (*time.Time)(nil), (*time.Time)(nil), now,
				}}
			}
			if strings.Contains(sql, "FROM users") {
				return middlewareFakeRow{values: []any{
					userID, "user@example.com", "hash", "user", true, (*time.Time)(nil), 0, true, now, now,
				}}
			}
			return middlewareFakeRow{values: []any{}}
		},
		execFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return nil, nil
		},
	}

	authMiddleware := NewAuthMiddleware(
		nil,
		services.NewUserService(db),
		services.NewApiTokenService(db),
	)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		user := handlers.GetUserFromContext(r.Context())
		if user == nil || user.ID != userID {
			t.Fatalf("expected user in context")
		}
		scope := handlers.GetTokenScopeFromContext(r.Context())
		if scope != models.ScopeRead {
			t.Fatalf("expected scope read, got %q", scope)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := httptest.NewRecorder()

	authMiddleware.Authenticate(handler).ServeHTTP(rr, req)
	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}
}
