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
	"github.com/HammerMeetNail/yearofbingo/internal/services"
	"github.com/HammerMeetNail/yearofbingo/internal/services/billing"
)

func TestCardHandler_EditFinalized_ErrorMappings(t *testing.T) {
	prev := billing.GlobalFeatureSwitches()
	billing.SetGlobalFeatureSwitches(billing.FeatureEntitlements{
		Templates:         true,
		EditAfterFinalize: true,
	})
	t.Cleanup(func() {
		billing.SetGlobalFeatureSwitches(prev)
	})

	user := &models.User{ID: uuid.New(), BillingPlan: "premium"}
	cardID := uuid.New()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantMsg    string
	}{
		{"not found", services.ErrCardNotFound, http.StatusNotFound, "Card not found"},
		{"not owner", services.ErrNotCardOwner, http.StatusForbidden, "Access denied"},
		{"not finalized", services.ErrCardNotFinalized, http.StatusBadRequest, "Card must be finalized first"},
		{"title too long", services.ErrTitleTooLong, http.StatusBadRequest, "Title must be 100 characters or less"},
		{"title exists", services.ErrCardTitleExists, http.StatusConflict, "You already have a card with this title for this year"},
		{"already exists", services.ErrCardAlreadyExists, http.StatusConflict, "You already have a card with this title for this year"},
		{"internal", errors.New("boom"), http.StatusInternalServerError, "Internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCardHandler(&mockCardService{
				EditFinalizedFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params services.EditFinalizedCardParams) (*models.BingoCard, error) {
					return nil, tt.serviceErr
				},
			})

			req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/edit", bytes.NewBufferString(`{}`))
			req = req.WithContext(SetUserInContext(req.Context(), user))
			rr := httptest.NewRecorder()

			handler.EditFinalized(rr, req)
			assertErrorResponse(t, rr, tt.wantStatus, tt.wantMsg)
		})
	}
}

func TestCardHandler_BulkDeleteAndArchive_InvalidCardID(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	handler := NewCardHandler(nil)

	t.Run("bulk delete invalid id", func(t *testing.T) {
		bodyBytes, _ := json.Marshal(BulkDeleteRequest{CardIDs: []string{"invalid-uuid"}})
		req := httptest.NewRequest(http.MethodDelete, "/api/cards/bulk", bytes.NewBuffer(bodyBytes))
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()

		handler.BulkDelete(rr, req)
		assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid card ID: invalid-uuid")
	})

	t.Run("bulk archive invalid id", func(t *testing.T) {
		bodyBytes, _ := json.Marshal(BulkUpdateArchiveRequest{CardIDs: []string{"invalid-uuid"}, IsArchived: true})
		req := httptest.NewRequest(http.MethodPut, "/api/cards/archive/bulk", bytes.NewBuffer(bodyBytes))
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()

		handler.BulkUpdateArchive(rr, req)
		assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid card ID: invalid-uuid")
	})
}

func TestCardHandler_EditFinalized_InvalidJSONBody(t *testing.T) {
	prev := billing.GlobalFeatureSwitches()
	billing.SetGlobalFeatureSwitches(billing.FeatureEntitlements{
		Templates:         true,
		EditAfterFinalize: true,
	})
	t.Cleanup(func() {
		billing.SetGlobalFeatureSwitches(prev)
	})

	user := &models.User{ID: uuid.New(), BillingPlan: "premium"}
	handler := NewCardHandler(&mockCardService{
		EditFinalizedFunc: func(ctx context.Context, userID, cardID uuid.UUID, params services.EditFinalizedCardParams) (*models.BingoCard, error) {
			t.Fatal("service should not be called for invalid JSON")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/edit", bytes.NewBufferString("{invalid"))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.EditFinalized(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}
