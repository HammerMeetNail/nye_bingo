package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func TestReminderHandler_GetSettings_RequiresAuth(t *testing.T) {
	handler := NewReminderHandler(&mockReminderService{})
	req := httptest.NewRequest(http.MethodGet, "/api/reminders/settings", nil)
	rr := httptest.NewRecorder()

	handler.GetSettings(rr, req)
	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestReminderHandler_UpdateSettings_InvalidBody(t *testing.T) {
	handler := NewReminderHandler(&mockReminderService{})
	req := httptest.NewRequest(http.MethodPut, "/api/reminders/settings", bytes.NewBufferString("{"))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.UpdateSettings(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}

func TestReminderHandler_UpdateSettings_EmailNotVerified(t *testing.T) {
	userID := uuid.New()
	handler := NewReminderHandler(&mockReminderService{
		UpdateSettingsFunc: func(ctx context.Context, gotUserID uuid.UUID, patch models.ReminderSettingsPatch) (*models.ReminderSettings, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			return nil, services.ErrEmailNotVerified
		},
	})

	payload := `{"email_enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/reminders/settings", bytes.NewBufferString(payload))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.UpdateSettings(rr, req)
	assertErrorResponse(t, rr, http.StatusForbidden, "Verify your email to enable reminder emails")
}

func TestReminderHandler_UpsertCardCheckin_InvalidSchedule(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	var gotCardID uuid.UUID
	var gotUserID uuid.UUID

	handler := NewReminderHandler(&mockReminderService{
		UpsertCardCheckinFunc: func(ctx context.Context, userIDArg, cardIDArg uuid.UUID, schedule models.CardCheckinScheduleInput) (*models.CardCheckinReminder, error) {
			gotUserID = userIDArg
			gotCardID = cardIDArg
			return nil, services.ErrInvalidSchedule
		},
	})

	payload := `{"frequency":"monthly","schedule":{"day_of_month":0,"time":"25:00"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/reminders/cards/"+cardID.String(), bytes.NewBufferString(payload))
	req.SetPathValue("cardId", cardID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.UpsertCardCheckin(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid reminder schedule")
	if gotUserID != userID {
		t.Fatalf("expected userID %v, got %v", userID, gotUserID)
	}
	if gotCardID != cardID {
		t.Fatalf("expected cardID %v, got %v", cardID, gotCardID)
	}
}
