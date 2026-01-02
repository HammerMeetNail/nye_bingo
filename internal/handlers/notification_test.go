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

func TestNotificationHandler_List_RequiresAuth(t *testing.T) {
	handler := NewNotificationHandler(&mockNotificationService{})
	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	rr := httptest.NewRecorder()

	handler.List(rr, req)
	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestNotificationHandler_GetSettings_Success(t *testing.T) {
	userID := uuid.New()
	handler := NewNotificationHandler(&mockNotificationService{
		GetSettingsFunc: func(ctx context.Context, gotUserID uuid.UUID) (*models.NotificationSettings, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			return &models.NotificationSettings{UserID: userID, InAppEnabled: true}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/settings", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.GetSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var response NotificationSettingsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Settings == nil || response.Settings.UserID != userID {
		t.Fatalf("unexpected settings response: %+v", response.Settings)
	}
}

func TestNotificationHandler_UpdateSettings_InvalidBody(t *testing.T) {
	handler := NewNotificationHandler(&mockNotificationService{})
	req := httptest.NewRequest(http.MethodPut, "/api/notifications/settings", bytes.NewBufferString("{"))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.UpdateSettings(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}

func TestNotificationHandler_UpdateSettings_Persists(t *testing.T) {
	userID := uuid.New()
	var gotPatch models.NotificationSettingsPatch
	handler := NewNotificationHandler(&mockNotificationService{
		UpdateSettingsFunc: func(ctx context.Context, gotUserID uuid.UUID, patch models.NotificationSettingsPatch) (*models.NotificationSettings, error) {
			gotPatch = patch
			return &models.NotificationSettings{UserID: gotUserID, InAppEnabled: true}, nil
		},
	})

	payload := `{"in_app_enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/notifications/settings", bytes.NewBufferString(payload))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.UpdateSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotPatch.InAppEnabled == nil || !*gotPatch.InAppEnabled {
		t.Fatalf("expected in_app_enabled true, got %+v", gotPatch.InAppEnabled)
	}
}

func TestNotificationHandler_MarkRead_NotOwned(t *testing.T) {
	userID := uuid.New()
	notificationID := uuid.New()
	handler := NewNotificationHandler(&mockNotificationService{
		MarkReadFunc: func(ctx context.Context, gotUserID, gotNotificationID uuid.UUID) error {
			return services.ErrNotificationNotFound
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/"+notificationID.String()+"/read", nil)
	req.SetPathValue("id", notificationID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.MarkRead(rr, req)
	assertErrorResponse(t, rr, http.StatusNotFound, "Notification not found")
}

func TestNotificationHandler_MarkRead_InvalidID(t *testing.T) {
	userID := uuid.New()
	handler := NewNotificationHandler(&mockNotificationService{})

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/bad-id/read", nil)
	req.SetPathValue("id", "bad-id")
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.MarkRead(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid notification ID")
}

func TestNotificationHandler_MarkAllRead_RequiresAuth(t *testing.T) {
	handler := NewNotificationHandler(&mockNotificationService{})
	req := httptest.NewRequest(http.MethodPost, "/api/notifications/read-all", nil)
	rr := httptest.NewRecorder()

	handler.MarkAllRead(rr, req)
	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestNotificationHandler_MarkAllRead_Success(t *testing.T) {
	userID := uuid.New()
	var called bool
	handler := NewNotificationHandler(&mockNotificationService{
		MarkAllReadFunc: func(ctx context.Context, gotUserID uuid.UUID) error {
			called = true
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/read-all", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.MarkAllRead(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !called {
		t.Fatal("expected MarkAllRead to be called")
	}
}

func TestNotificationHandler_Delete_NotOwned(t *testing.T) {
	userID := uuid.New()
	notificationID := uuid.New()
	handler := NewNotificationHandler(&mockNotificationService{
		DeleteFunc: func(ctx context.Context, gotUserID, gotNotificationID uuid.UUID) error {
			return services.ErrNotificationNotFound
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/notifications/"+notificationID.String(), nil)
	req.SetPathValue("id", notificationID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)
	assertErrorResponse(t, rr, http.StatusNotFound, "Notification not found")
}

func TestNotificationHandler_Delete_InvalidID(t *testing.T) {
	userID := uuid.New()
	handler := NewNotificationHandler(&mockNotificationService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/notifications/bad-id", nil)
	req.SetPathValue("id", "bad-id")
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid notification ID")
}

func TestNotificationHandler_DeleteAll_RequiresAuth(t *testing.T) {
	handler := NewNotificationHandler(&mockNotificationService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/notifications", nil)
	rr := httptest.NewRecorder()

	handler.DeleteAll(rr, req)
	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestNotificationHandler_DeleteAll_Success(t *testing.T) {
	userID := uuid.New()
	var called bool
	handler := NewNotificationHandler(&mockNotificationService{
		DeleteAllFunc: func(ctx context.Context, gotUserID uuid.UUID) error {
			called = true
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/notifications", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.DeleteAll(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !called {
		t.Fatal("expected DeleteAll to be called")
	}
}

func TestNotificationHandler_UnreadCount_RequiresAuth(t *testing.T) {
	handler := NewNotificationHandler(&mockNotificationService{})
	req := httptest.NewRequest(http.MethodGet, "/api/notifications/unread-count", nil)
	rr := httptest.NewRecorder()

	handler.UnreadCount(rr, req)
	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestNotificationHandler_UnreadCount_Success(t *testing.T) {
	userID := uuid.New()
	handler := NewNotificationHandler(&mockNotificationService{
		UnreadCountFunc: func(ctx context.Context, gotUserID uuid.UUID) (int, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			return 3, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/unread-count", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.UnreadCount(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var response NotificationUnreadCountResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Count != 3 {
		t.Fatalf("expected count 3, got %d", response.Count)
	}
}

func TestNotificationHandler_List_InvalidLimit(t *testing.T) {
	userID := uuid.New()
	handler := NewNotificationHandler(&mockNotificationService{})
	req := httptest.NewRequest(http.MethodGet, "/api/notifications?limit=abc", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.List(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid limit")
}

func TestNotificationHandler_List_InvalidBefore(t *testing.T) {
	userID := uuid.New()
	handler := NewNotificationHandler(&mockNotificationService{})
	req := httptest.NewRequest(http.MethodGet, "/api/notifications?before=not-a-time", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.List(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid before timestamp")
}

func TestNotificationHandler_UpdateSettings_EmailNotVerified(t *testing.T) {
	userID := uuid.New()
	handler := NewNotificationHandler(&mockNotificationService{
		UpdateSettingsFunc: func(ctx context.Context, gotUserID uuid.UUID, patch models.NotificationSettingsPatch) (*models.NotificationSettings, error) {
			return nil, services.ErrEmailNotVerified
		},
	})

	payload := `{"email_enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/notifications/settings", bytes.NewBufferString(payload))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.UpdateSettings(rr, req)
	assertErrorResponse(t, rr, http.StatusForbidden, "Verify your email to enable email notifications")
}
