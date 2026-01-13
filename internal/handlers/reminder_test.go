package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestReminderHandler_ListCards_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	title := "Card"
	handler := NewReminderHandler(&mockReminderService{
		ListCardCheckinsFunc: func(ctx context.Context, gotUserID uuid.UUID) ([]models.CardCheckinSummary, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			return []models.CardCheckinSummary{
				{CardID: cardID, CardTitle: &title, CardYear: 2025, IsFinalized: true, IsArchived: false, GridSize: 3},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/reminders/cards", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.ListCards(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload ReminderCardListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Cards) != 1 || payload.Cards[0].CardID != cardID {
		t.Fatalf("unexpected cards response: %#v", payload.Cards)
	}
}

func TestReminderHandler_DeleteCardCheckin_NotFound(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	handler := NewReminderHandler(&mockReminderService{
		DeleteCardCheckinFunc: func(ctx context.Context, gotUserID, gotCardID uuid.UUID) error {
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			if gotCardID != cardID {
				t.Fatalf("expected cardID %v, got %v", cardID, gotCardID)
			}
			return services.ErrReminderNotFound
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/reminders/cards/"+cardID.String(), nil)
	req.SetPathValue("cardId", cardID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.DeleteCardCheckin(rr, req)
	assertErrorResponse(t, rr, http.StatusNotFound, "Reminder not found")
}

func TestReminderHandler_ListGoals_ParsesCardIDAndReturnsResults(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	handler := NewReminderHandler(&mockReminderService{
		ListGoalRemindersFunc: func(ctx context.Context, gotUserID uuid.UUID, gotCardID *uuid.UUID) ([]models.GoalReminderSummary, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			if gotCardID == nil || *gotCardID != cardID {
				t.Fatalf("expected cardID %v, got %v", cardID, gotCardID)
			}
			return []models.GoalReminderSummary{{ID: uuid.New(), CardID: cardID, ItemID: uuid.New(), ItemText: "Goal", CardYear: 2025}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/reminders/goals?card_id="+cardID.String(), nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.ListGoals(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var payload ReminderGoalListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Reminders) != 1 || payload.Reminders[0].CardID != cardID {
		t.Fatalf("unexpected reminders response: %#v", payload.Reminders)
	}
}

func TestReminderHandler_UpsertGoalReminder_ErrorsAndSuccess(t *testing.T) {
	userID := uuid.New()
	itemID := uuid.New()

	t.Run("invalid-body", func(t *testing.T) {
		handler := NewReminderHandler(&mockReminderService{})
		req := httptest.NewRequest(http.MethodPut, "/api/reminders/goals", bytes.NewBufferString("{"))
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
		rr := httptest.NewRecorder()

		handler.UpsertGoalReminder(rr, req)
		assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
	})

	t.Run("missing-goal-id", func(t *testing.T) {
		handler := NewReminderHandler(&mockReminderService{})
		req := httptest.NewRequest(http.MethodPut, "/api/reminders/goals", bytes.NewBufferString(`{}`))
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
		rr := httptest.NewRecorder()

		handler.UpsertGoalReminder(rr, req)
		assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid goal ID")
	})

	t.Run("item-not-found", func(t *testing.T) {
		handler := NewReminderHandler(&mockReminderService{
			UpsertGoalReminderFunc: func(ctx context.Context, gotUserID uuid.UUID, input models.GoalReminderInput) (*models.GoalReminder, error) {
				return nil, services.ErrItemNotFound
			},
		})
		req := httptest.NewRequest(http.MethodPut, "/api/reminders/goals", bytes.NewBufferString(`{"item_id":"`+itemID.String()+`","kind":"one_time","schedule":{"send_at":"2030-01-02T15:04:05Z"}}`))
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
		rr := httptest.NewRecorder()

		handler.UpsertGoalReminder(rr, req)
		assertErrorResponse(t, rr, http.StatusNotFound, "Goal not found")
	})

	t.Run("success", func(t *testing.T) {
		got := false
		handler := NewReminderHandler(&mockReminderService{
			UpsertGoalReminderFunc: func(ctx context.Context, gotUserID uuid.UUID, input models.GoalReminderInput) (*models.GoalReminder, error) {
				got = true
				return &models.GoalReminder{ID: uuid.New(), UserID: gotUserID, ItemID: input.ItemID}, nil
			},
		})
		req := httptest.NewRequest(http.MethodPut, "/api/reminders/goals", bytes.NewBufferString(`{"item_id":"`+itemID.String()+`","kind":"one_time","schedule":{"send_at":"2030-01-02T15:04:05Z"}}`))
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
		rr := httptest.NewRecorder()

		handler.UpsertGoalReminder(rr, req)
		if !got {
			t.Fatal("expected service to be called")
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
	})
}

func TestReminderHandler_DeleteGoalReminder_InvalidID(t *testing.T) {
	handler := NewReminderHandler(&mockReminderService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/reminders/goals/bad", nil)
	req.SetPathValue("id", "bad")
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.DeleteGoalReminder(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid reminder ID")
}

func TestReminderHandler_SendTest_ErrorMapping(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	handler := NewReminderHandler(&mockReminderService{
		SendTestEmailFunc: func(ctx context.Context, gotUserID, gotCardID uuid.UUID) error {
			return services.ErrEmailNotVerified
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/reminders/test", bytes.NewBufferString(`{"card_id":"`+cardID.String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.SendTest(rr, req)
	assertErrorResponse(t, rr, http.StatusForbidden, "Verify your email to send test reminders")
}

func TestReminderHandler_DeleteCardCheckin_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	handler := NewReminderHandler(&mockReminderService{
		DeleteCardCheckinFunc: func(ctx context.Context, gotUserID, gotCardID uuid.UUID) error {
			if gotUserID != userID || gotCardID != cardID {
				t.Fatalf("unexpected args user=%v card=%v", gotUserID, gotCardID)
			}
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/reminders/cards/"+cardID.String(), nil)
	req.SetPathValue("cardId", cardID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.DeleteCardCheckin(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Reminder deleted") {
		t.Fatalf("expected success message, got %q", rr.Body.String())
	}
}

func TestReminderHandler_ListGoals_InvalidCardID(t *testing.T) {
	handler := NewReminderHandler(&mockReminderService{})
	req := httptest.NewRequest(http.MethodGet, "/api/reminders/goals?card_id=bad", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.ListGoals(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid card ID")
}

func TestReminderHandler_DeleteGoalReminder_NotFound(t *testing.T) {
	userID := uuid.New()
	reminderID := uuid.New()
	handler := NewReminderHandler(&mockReminderService{
		DeleteGoalReminderFunc: func(ctx context.Context, gotUserID, gotReminderID uuid.UUID) error {
			return services.ErrReminderNotFound
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/reminders/goals/"+reminderID.String(), nil)
	req.SetPathValue("id", reminderID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()

	handler.DeleteGoalReminder(rr, req)
	assertErrorResponse(t, rr, http.StatusNotFound, "Reminder not found")
}

func TestReminderHandler_SendTest_RequiresCardID(t *testing.T) {
	handler := NewReminderHandler(&mockReminderService{})
	req := httptest.NewRequest(http.MethodPost, "/api/reminders/test", bytes.NewBufferString(`{}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.SendTest(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Card ID is required")
}

func TestReminderHandler_GetSettings_SuccessAndError(t *testing.T) {
	userID := uuid.New()
	handler := NewReminderHandler(&mockReminderService{
		GetSettingsFunc: func(ctx context.Context, gotUserID uuid.UUID) (*models.ReminderSettings, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			return &models.ReminderSettings{UserID: userID, EmailEnabled: true}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/reminders/settings", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()
	handler.GetSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	handler = NewReminderHandler(&mockReminderService{
		GetSettingsFunc: func(ctx context.Context, gotUserID uuid.UUID) (*models.ReminderSettings, error) {
			return nil, errors.New("boom")
		},
	})
	rr = httptest.NewRecorder()
	handler.GetSettings(rr, req)
	assertErrorResponse(t, rr, http.StatusInternalServerError, "Internal server error")
}

func TestReminderHandler_UpdateSettings_Success(t *testing.T) {
	userID := uuid.New()
	handler := NewReminderHandler(&mockReminderService{
		UpdateSettingsFunc: func(ctx context.Context, gotUserID uuid.UUID, patch models.ReminderSettingsPatch) (*models.ReminderSettings, error) {
			return &models.ReminderSettings{UserID: gotUserID, EmailEnabled: false}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/reminders/settings", bytes.NewBufferString(`{"email_enabled":false}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()
	handler.UpdateSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestReminderHandler_ListCards_ServiceError(t *testing.T) {
	handler := NewReminderHandler(&mockReminderService{
		ListCardCheckinsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.CardCheckinSummary, error) {
			return nil, errors.New("boom")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/reminders/cards", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.ListCards(rr, req)
	assertErrorResponse(t, rr, http.StatusInternalServerError, "Internal server error")
}

func TestReminderHandler_UpsertCardCheckin_MappingsAndSuccess(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()

	t.Run("invalid-card-id", func(t *testing.T) {
		handler := NewReminderHandler(&mockReminderService{})
		req := httptest.NewRequest(http.MethodPut, "/api/reminders/cards/bad", bytes.NewBufferString(`{}`))
		req.SetPathValue("cardId", "bad")
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
		rr := httptest.NewRecorder()

		handler.UpsertCardCheckin(rr, req)
		assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid card ID")
	})

	t.Run("invalid-body", func(t *testing.T) {
		handler := NewReminderHandler(&mockReminderService{})
		req := httptest.NewRequest(http.MethodPut, "/api/reminders/cards/"+cardID.String(), bytes.NewBufferString("{"))
		req.SetPathValue("cardId", cardID.String())
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
		rr := httptest.NewRecorder()

		handler.UpsertCardCheckin(rr, req)
		assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
	})

	cases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantMsg    string
	}{
		{name: "card-not-found", serviceErr: services.ErrCardNotFound, wantStatus: http.StatusNotFound, wantMsg: "Card not found"},
		{name: "card-not-eligible", serviceErr: services.ErrCardNotEligible, wantStatus: http.StatusBadRequest, wantMsg: "Card must be finalized and not archived"},
		{name: "internal", serviceErr: errors.New("boom"), wantStatus: http.StatusInternalServerError, wantMsg: "Internal server error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewReminderHandler(&mockReminderService{
				UpsertCardCheckinFunc: func(ctx context.Context, userIDArg, cardIDArg uuid.UUID, schedule models.CardCheckinScheduleInput) (*models.CardCheckinReminder, error) {
					return nil, tc.serviceErr
				},
			})
			req := httptest.NewRequest(http.MethodPut, "/api/reminders/cards/"+cardID.String(), bytes.NewBufferString(`{"frequency":"monthly","schedule":{"day_of_month":28,"time":"09:00"}}`))
			req.SetPathValue("cardId", cardID.String())
			req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
			rr := httptest.NewRecorder()

			handler.UpsertCardCheckin(rr, req)
			assertErrorResponse(t, rr, tc.wantStatus, tc.wantMsg)
		})
	}

	handler := NewReminderHandler(&mockReminderService{
		UpsertCardCheckinFunc: func(ctx context.Context, userIDArg, cardIDArg uuid.UUID, schedule models.CardCheckinScheduleInput) (*models.CardCheckinReminder, error) {
			return &models.CardCheckinReminder{ID: uuid.New(), UserID: userIDArg, CardID: cardIDArg, Enabled: true}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/reminders/cards/"+cardID.String(), bytes.NewBufferString(`{"frequency":"monthly","schedule":{"day_of_month":28,"time":"09:00"}}`))
	req.SetPathValue("cardId", cardID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()
	handler.UpsertCardCheckin(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestReminderHandler_DeleteCardCheckin_InvalidIDAndInternal(t *testing.T) {
	userID := uuid.New()

	handler := NewReminderHandler(&mockReminderService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/reminders/cards/bad", nil)
	req.SetPathValue("cardId", "bad")
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()
	handler.DeleteCardCheckin(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid card ID")

	handler = NewReminderHandler(&mockReminderService{
		DeleteCardCheckinFunc: func(ctx context.Context, userID, cardID uuid.UUID) error {
			return errors.New("boom")
		},
	})
	req = httptest.NewRequest(http.MethodDelete, "/api/reminders/cards/"+uuid.New().String(), nil)
	req.SetPathValue("cardId", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr = httptest.NewRecorder()
	handler.DeleteCardCheckin(rr, req)
	assertErrorResponse(t, rr, http.StatusInternalServerError, "Internal server error")
}

func TestReminderHandler_ListGoals_NoFilterAndServiceError(t *testing.T) {
	userID := uuid.New()
	handler := NewReminderHandler(&mockReminderService{
		ListGoalRemindersFunc: func(ctx context.Context, gotUserID uuid.UUID, cardID *uuid.UUID) ([]models.GoalReminderSummary, error) {
			if cardID != nil {
				t.Fatalf("expected nil card filter, got %v", *cardID)
			}
			return []models.GoalReminderSummary{}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/reminders/goals", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()
	handler.ListGoals(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	handler = NewReminderHandler(&mockReminderService{
		ListGoalRemindersFunc: func(ctx context.Context, gotUserID uuid.UUID, cardID *uuid.UUID) ([]models.GoalReminderSummary, error) {
			return nil, errors.New("boom")
		},
	})
	rr = httptest.NewRecorder()
	handler.ListGoals(rr, req)
	assertErrorResponse(t, rr, http.StatusInternalServerError, "Internal server error")
}

func TestReminderHandler_UpsertGoalReminder_Mappings(t *testing.T) {
	userID := uuid.New()
	itemID := uuid.New()
	body := `{"item_id":"` + itemID.String() + `","kind":"one_time","schedule":{"send_at":"2030-01-02T15:04:05Z"}}`

	cases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantMsg    string
	}{
		{name: "invalid-schedule", serviceErr: services.ErrInvalidSchedule, wantStatus: http.StatusBadRequest, wantMsg: "Invalid reminder schedule"},
		{name: "goal-completed", serviceErr: services.ErrGoalCompleted, wantStatus: http.StatusBadRequest, wantMsg: "Goal already completed"},
		{name: "card-not-eligible", serviceErr: services.ErrCardNotEligible, wantStatus: http.StatusBadRequest, wantMsg: "Card must be finalized and not archived"},
		{name: "internal", serviceErr: errors.New("boom"), wantStatus: http.StatusInternalServerError, wantMsg: "Internal server error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewReminderHandler(&mockReminderService{
				UpsertGoalReminderFunc: func(ctx context.Context, gotUserID uuid.UUID, input models.GoalReminderInput) (*models.GoalReminder, error) {
					return nil, tc.serviceErr
				},
			})
			req := httptest.NewRequest(http.MethodPut, "/api/reminders/goals", bytes.NewBufferString(body))
			req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
			rr := httptest.NewRecorder()
			handler.UpsertGoalReminder(rr, req)
			assertErrorResponse(t, rr, tc.wantStatus, tc.wantMsg)
		})
	}
}

func TestReminderHandler_DeleteGoalReminder_SuccessAndInternal(t *testing.T) {
	userID := uuid.New()
	reminderID := uuid.New()

	handler := NewReminderHandler(&mockReminderService{
		DeleteGoalReminderFunc: func(ctx context.Context, gotUserID, gotReminderID uuid.UUID) error {
			return nil
		},
	})
	req := httptest.NewRequest(http.MethodDelete, "/api/reminders/goals/"+reminderID.String(), nil)
	req.SetPathValue("id", reminderID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()
	handler.DeleteGoalReminder(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	handler = NewReminderHandler(&mockReminderService{
		DeleteGoalReminderFunc: func(ctx context.Context, gotUserID, gotReminderID uuid.UUID) error {
			return errors.New("boom")
		},
	})
	rr = httptest.NewRecorder()
	handler.DeleteGoalReminder(rr, req)
	assertErrorResponse(t, rr, http.StatusInternalServerError, "Internal server error")
}

func TestReminderHandler_SendTest_MappingsAndSuccess(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()

	handler := NewReminderHandler(&mockReminderService{})
	req := httptest.NewRequest(http.MethodPost, "/api/reminders/test", bytes.NewBufferString("{"))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr := httptest.NewRecorder()
	handler.SendTest(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")

	cases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantMsg    string
	}{
		{name: "disabled", serviceErr: services.ErrRemindersDisabled, wantStatus: http.StatusBadRequest, wantMsg: "Enable reminders before sending a test email"},
		{name: "card-not-found", serviceErr: services.ErrCardNotFound, wantStatus: http.StatusNotFound, wantMsg: "Card not found"},
		{name: "card-not-eligible", serviceErr: services.ErrCardNotEligible, wantStatus: http.StatusBadRequest, wantMsg: "Card must be finalized and not archived"},
		{name: "internal", serviceErr: errors.New("boom"), wantStatus: http.StatusInternalServerError, wantMsg: "Internal server error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewReminderHandler(&mockReminderService{
				SendTestEmailFunc: func(ctx context.Context, gotUserID, gotCardID uuid.UUID) error {
					return tc.serviceErr
				},
			})
			req := httptest.NewRequest(http.MethodPost, "/api/reminders/test", bytes.NewBufferString(`{"card_id":"`+cardID.String()+`"}`))
			req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
			rr := httptest.NewRecorder()
			handler.SendTest(rr, req)
			assertErrorResponse(t, rr, tc.wantStatus, tc.wantMsg)
		})
	}

	handler = NewReminderHandler(&mockReminderService{
		SendTestEmailFunc: func(ctx context.Context, gotUserID, gotCardID uuid.UUID) error {
			return nil
		},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/reminders/test", bytes.NewBufferString(`{"card_id":"`+cardID.String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: userID}))
	rr = httptest.NewRecorder()
	handler.SendTest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}
