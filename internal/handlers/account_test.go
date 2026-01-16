package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type mockAccountService struct {
	services.AccountServiceInterface
	BuildExportZipFunc func(ctx context.Context, userID uuid.UUID) ([]byte, error)
	DeleteFunc         func(ctx context.Context, userID uuid.UUID) error
}

func (m *mockAccountService) BuildExportZip(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	return m.BuildExportZipFunc(ctx, userID)
}

func (m *mockAccountService) Delete(ctx context.Context, userID uuid.UUID) error {
	return m.DeleteFunc(ctx, userID)
}

type mockAccountAuthService struct {
	services.AuthServiceInterface
	VerifyPasswordFunc func(hash *string, password string) bool
	DeleteSessionFunc  func(ctx context.Context, token string) error
}

func (m *mockAccountAuthService) VerifyPassword(hash *string, password string) bool {
	return m.VerifyPasswordFunc(hash, password)
}

func (m *mockAccountAuthService) DeleteSession(ctx context.Context, token string) error {
	return m.DeleteSessionFunc(ctx, token)
}

func TestAccountHandler_Export_Unauthorized(t *testing.T) {
	handler := NewAccountHandler(&mockAccountService{}, &mockAccountAuthService{}, false)
	req := httptest.NewRequest(http.MethodGet, "/api/account/export", nil)
	rr := httptest.NewRecorder()

	handler.Export(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestAccountHandler_Export_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	handler := NewAccountHandler(&mockAccountService{
		BuildExportZipFunc: func(ctx context.Context, userID uuid.UUID) ([]byte, error) {
			return []byte("PK\x03\x04test"), nil
		},
	}, &mockAccountAuthService{}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/account/export", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Export(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("expected content-type application/zip, got %q", ct)
	}
	if !bytes.HasPrefix(rr.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Fatalf("expected zip signature, got %q", rr.Body.Bytes())
	}
}

func TestAccountHandler_Delete_Unauthorized(t *testing.T) {
	handler := NewAccountHandler(&mockAccountService{}, &mockAccountAuthService{}, false)
	req := httptest.NewRequest(http.MethodDelete, "/api/account", nil)
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestAccountHandler_Delete_Validation(t *testing.T) {
	hash := "hash"
	user := &models.User{ID: uuid.New(), Username: "tester", PasswordHash: &hash}
	handler := NewAccountHandler(&mockAccountService{}, &mockAccountAuthService{
		VerifyPasswordFunc: func(hash *string, password string) bool {
			return true
		},
	}, false)

	reqBody := `{"confirm_username":"wrong","password":"pass","confirm":true}`
	req := httptest.NewRequest(http.MethodDelete, "/api/account", bytes.NewBufferString(reqBody))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestAccountHandler_Delete_WrongPassword(t *testing.T) {
	hash := "hash"
	user := &models.User{ID: uuid.New(), Username: "tester", PasswordHash: &hash}
	handler := NewAccountHandler(&mockAccountService{}, &mockAccountAuthService{
		VerifyPasswordFunc: func(hash *string, password string) bool {
			return false
		},
	}, false)

	body := map[string]any{
		"confirm_username": "tester",
		"password":         "badpass",
		"confirm":          true,
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodDelete, "/api/account", bytes.NewBuffer(payload))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestAccountHandler_Delete_Success(t *testing.T) {
	hash := "hash"
	user := &models.User{ID: uuid.New(), Username: "tester", PasswordHash: &hash}
	handler := NewAccountHandler(&mockAccountService{
		DeleteFunc: func(ctx context.Context, userID uuid.UUID) error {
			return nil
		},
	}, &mockAccountAuthService{
		VerifyPasswordFunc: func(hash *string, password string) bool {
			return true
		},
		DeleteSessionFunc: func(ctx context.Context, token string) error {
			return nil
		},
	}, false)

	body := map[string]any{
		"confirm_username": "tester",
		"password":         "password",
		"confirm":          true,
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodDelete, "/api/account", bytes.NewBuffer(payload))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "token"})
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	cookies := rr.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.MaxAge != -1 {
		t.Fatalf("expected cleared session cookie, got %+v", sessionCookie)
	}
}
