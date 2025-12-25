package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func TestCardHandler_Create_ConflictAndServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}

	t.Run("conflict response", func(t *testing.T) {
		existingID := uuid.New()
		existingTitle := "Existing"
		mockCard := &mockCardService{
			CheckForConflictFunc: func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
				return &models.BingoCard{ID: existingID, UserID: user.ID, Year: year, Title: &existingTitle}, nil
			},
		}
		handler := NewCardHandler(mockCard)

		bodyBytes, _ := json.Marshal(CreateCardRequest{Year: time.Now().Year()})
		req := httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewBuffer(bodyBytes))
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()

		handler.Create(rr, req)

		if rr.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rr.Code)
		}

		var resp ImportCardResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Error != "card_exists" {
			t.Fatalf("expected card_exists, got %q", resp.Error)
		}
		if resp.ExistingCard == nil || resp.ExistingCard.ID != existingID.String() {
			t.Fatalf("expected existing card info")
		}
	})

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantMsg    string
	}{
		{"already exists", services.ErrCardAlreadyExists, http.StatusConflict, "You already have a card for this year. Give your new card a unique title."},
		{"title exists", services.ErrCardTitleExists, http.StatusConflict, "You already have a card with this title for this year"},
		{"invalid category", services.ErrInvalidCategory, http.StatusBadRequest, "Invalid category"},
		{"title too long", services.ErrTitleTooLong, http.StatusBadRequest, "Title must be 100 characters or less"},
		{"invalid grid size", services.ErrInvalidGridSize, http.StatusBadRequest, "Grid size must be 2, 3, 4, or 5"},
		{"invalid header", services.ErrInvalidHeaderText, http.StatusBadRequest, "Invalid header text"},
		{"internal error", errors.New("boom"), http.StatusInternalServerError, "Internal server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCard := &mockCardService{
				CheckForConflictFunc: func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
					return nil, services.ErrCardNotFound
				},
				CreateFunc: func(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error) {
					return nil, tt.serviceErr
				},
			}
			handler := NewCardHandler(mockCard)

			bodyBytes, _ := json.Marshal(CreateCardRequest{Year: time.Now().Year(), GridSize: ptrToInt(models.MaxGridSize)})
			req := httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewBuffer(bodyBytes))
			req = req.WithContext(SetUserInContext(req.Context(), user))
			rr := httptest.NewRecorder()

			handler.Create(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			var resp ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Error != tt.wantMsg {
				t.Fatalf("expected error %q, got %q", tt.wantMsg, resp.Error)
			}
		})
	}
}

func TestCardHandler_UpdateItem_ServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{"card not found", services.ErrCardNotFound, http.StatusNotFound},
		{"item not found", services.ErrItemNotFound, http.StatusNotFound},
		{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
		{"finalized", services.ErrCardFinalized, http.StatusBadRequest},
		{"occupied", services.ErrPositionOccupied, http.StatusConflict},
		{"invalid position", services.ErrInvalidPosition, http.StatusBadRequest},
		{"internal error", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCard := &mockCardService{
				UpdateItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int, params models.UpdateItemParams) (*models.BingoItem, error) {
					return nil, tt.serviceErr
				},
			}
			handler := NewCardHandler(mockCard)

			bodyBytes, _ := json.Marshal(UpdateItemRequest{Content: ptrToString("content")})
			req := httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/items/1", bytes.NewBuffer(bodyBytes))
			req = req.WithContext(SetUserInContext(req.Context(), user))
			rr := httptest.NewRecorder()

			handler.UpdateItem(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}

	t.Run("success", func(t *testing.T) {
		mockCard := &mockCardService{
			UpdateItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int, params models.UpdateItemParams) (*models.BingoItem, error) {
				return &models.BingoItem{CardID: gotCardID, Position: position}, nil
			},
		}
		handler := NewCardHandler(mockCard)

		bodyBytes, _ := json.Marshal(UpdateItemRequest{Content: ptrToString("content")})
		req := httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/items/1", bytes.NewBuffer(bodyBytes))
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()

		handler.UpdateItem(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
	})
}

func TestCardHandler_RemoveItem_ServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{"card not found", services.ErrCardNotFound, http.StatusNotFound},
		{"item not found", services.ErrItemNotFound, http.StatusNotFound},
		{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
		{"finalized", services.ErrCardFinalized, http.StatusBadRequest},
		{"internal error", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCard := &mockCardService{
				RemoveItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int) error {
					return tt.serviceErr
				},
			}
			handler := NewCardHandler(mockCard)

			req := httptest.NewRequest(http.MethodDelete, "/api/cards/"+cardID.String()+"/items/1", nil)
			req = req.WithContext(SetUserInContext(req.Context(), user))
			rr := httptest.NewRecorder()

			handler.RemoveItem(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestCardHandler_ShuffleAndSwapItems_ServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	t.Run("shuffle errors", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"card not found", services.ErrCardNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"finalized", services.ErrCardFinalized, http.StatusBadRequest},
			{"internal error", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					ShuffleFunc: func(ctx context.Context, userID, gotCardID uuid.UUID) (*models.BingoCard, error) {
						return nil, tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/shuffle", nil)
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.Shuffle(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})

	t.Run("swap errors", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"card not found", services.ErrCardNotFound, http.StatusNotFound},
			{"item not found", services.ErrItemNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"finalized", services.ErrCardFinalized, http.StatusBadRequest},
			{"invalid position", services.ErrInvalidPosition, http.StatusBadRequest},
			{"no space for free", services.ErrNoSpaceForFree, http.StatusBadRequest},
			{"internal error", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					SwapItemsFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, pos1, pos2 int) error {
						return tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				bodyBytes, _ := json.Marshal(SwapRequest{Position1: 1, Position2: 2})
				req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/swap", bytes.NewBuffer(bodyBytes))
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.SwapItems(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})
}

func TestCardHandler_FinalizeCompleteUncompleteNotes_ServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	t.Run("finalize errors", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"card not found", services.ErrCardNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"generic error -> bad request", errors.New("boom"), http.StatusBadRequest},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					FinalizeFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params *services.FinalizeParams) (*models.BingoCard, error) {
						return nil, tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/finalize", nil)
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.Finalize(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})

	t.Run("complete/uncomplete/not finalized", func(t *testing.T) {
		mockCard := &mockCardService{
			CompleteItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int, params models.CompleteItemParams) (*models.BingoItem, error) {
				return nil, services.ErrCardNotFinalized
			},
			UncompleteItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int) (*models.BingoItem, error) {
				return nil, services.ErrCardNotFinalized
			},
			UpdateItemNotesFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int, notes, proofURL *string) (*models.BingoItem, error) {
				return nil, services.ErrItemNotFound
			},
		}
		handler := NewCardHandler(mockCard)

		bodyBytes, _ := json.Marshal(CompleteItemRequest{Notes: ptrToString("notes")})
		req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/items/1/complete", bytes.NewBuffer(bodyBytes))
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()
		handler.CompleteItem(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/items/1/uncomplete", nil)
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr = httptest.NewRecorder()
		handler.UncompleteItem(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}

		bodyBytes, _ = json.Marshal(UpdateNotesRequest{Notes: ptrToString("notes")})
		req = httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/items/1/notes", bytes.NewBuffer(bodyBytes))
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr = httptest.NewRecorder()
		handler.UpdateNotes(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})
}

func TestCardHandler_GetDeleteStatsAndVisibility_ServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	t.Run("get not owner", func(t *testing.T) {
		mockCard := &mockCardService{
			GetByIDFunc: func(ctx context.Context, gotCardID uuid.UUID) (*models.BingoCard, error) {
				return &models.BingoCard{ID: gotCardID, UserID: uuid.New()}, nil
			},
		}
		handler := NewCardHandler(mockCard)

		req := httptest.NewRequest(http.MethodGet, "/api/cards/"+cardID.String(), nil)
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()

		handler.Get(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("get not found", func(t *testing.T) {
		mockCard := &mockCardService{
			GetByIDFunc: func(ctx context.Context, gotCardID uuid.UUID) (*models.BingoCard, error) {
				return nil, services.ErrCardNotFound
			},
		}
		handler := NewCardHandler(mockCard)

		req := httptest.NewRequest(http.MethodGet, "/api/cards/"+cardID.String(), nil)
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()

		handler.Get(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("delete mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"not found", services.ErrCardNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"internal", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					DeleteFunc: func(ctx context.Context, userID, gotCardID uuid.UUID) error {
						return tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				req := httptest.NewRequest(http.MethodDelete, "/api/cards/"+cardID.String(), nil)
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.Delete(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})

	t.Run("stats mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"not found", services.ErrCardNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"internal", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					GetStatsFunc: func(ctx context.Context, userID, gotCardID uuid.UUID) (*models.CardStats, error) {
						return nil, tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				req := httptest.NewRequest(http.MethodGet, "/api/cards/"+cardID.String()+"/stats", nil)
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.Stats(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})

	t.Run("update visibility mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"not found", services.ErrCardNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"internal", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					UpdateVisibilityFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, visibleToFriends bool) (*models.BingoCard, error) {
						return nil, tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				bodyBytes, _ := json.Marshal(UpdateVisibilityRequest{VisibleToFriends: true})
				req := httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/visibility", bytes.NewBuffer(bodyBytes))
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.UpdateVisibility(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})
}

func TestCardHandler_UpdateConfigAndMeta_ServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	t.Run("update config mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"not found", services.ErrCardNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"finalized", services.ErrCardFinalized, http.StatusBadRequest},
			{"invalid header", services.ErrInvalidHeaderText, http.StatusBadRequest},
			{"no space for free", services.ErrNoSpaceForFree, http.StatusBadRequest},
			{"internal", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					UpdateConfigFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params models.UpdateCardConfigParams) (*models.BingoCard, error) {
						return nil, tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				bodyBytes, _ := json.Marshal(UpdateCardConfigRequest{HeaderText: ptrToString("Header")})
				req := httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/config", bytes.NewBuffer(bodyBytes))
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.UpdateConfig(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})

	t.Run("update meta mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"not found", services.ErrCardNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"title exists", services.ErrCardTitleExists, http.StatusConflict},
			{"invalid category", services.ErrInvalidCategory, http.StatusBadRequest},
			{"title too long", services.ErrTitleTooLong, http.StatusBadRequest},
			{"internal", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					UpdateMetaFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params models.UpdateCardMetaParams) (*models.BingoCard, error) {
						return nil, tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				bodyBytes, _ := json.Marshal(UpdateCardMetaRequest{Title: ptrToString("Title")})
				req := httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/meta", bytes.NewBuffer(bodyBytes))
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.UpdateMeta(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})
}

func TestCardHandler_ListExportable_Errors(t *testing.T) {
	user := &models.User{ID: uuid.New()}

	t.Run("current cards error", func(t *testing.T) {
		mockCard := &mockCardService{
			ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
				return nil, errors.New("boom")
			},
		}
		handler := NewCardHandler(mockCard)

		req := httptest.NewRequest(http.MethodGet, "/api/cards/exportable", nil)
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()

		handler.ListExportable(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})

	t.Run("archive cards error", func(t *testing.T) {
		mockCard := &mockCardService{
			ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
				return []*models.BingoCard{}, nil
			},
			GetArchiveFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
				return nil, errors.New("boom")
			},
		}
		handler := NewCardHandler(mockCard)

		req := httptest.NewRequest(http.MethodGet, "/api/cards/exportable", nil)
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()

		handler.ListExportable(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})
}

func TestCardHandler_AddItem_MapsServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{"card not found", services.ErrCardNotFound, http.StatusNotFound},
		{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
		{"finalized", services.ErrCardFinalized, http.StatusBadRequest},
		{"card full", services.ErrCardFull, http.StatusBadRequest},
		{"position occupied", services.ErrPositionOccupied, http.StatusConflict},
		{"invalid position", services.ErrInvalidPosition, http.StatusBadRequest},
		{"internal", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCard := &mockCardService{
				AddItemFunc: func(ctx context.Context, userID uuid.UUID, params models.AddItemParams) (*models.BingoItem, error) {
					return nil, tt.serviceErr
				},
			}
			handler := NewCardHandler(mockCard)

			bodyBytes, _ := json.Marshal(AddItemRequest{Content: "item"})
			req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/items", bytes.NewBuffer(bodyBytes))
			req = req.WithContext(SetUserInContext(req.Context(), user))
			rr := httptest.NewRecorder()

			handler.AddItem(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestCardHandler_Clone_MapsServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{"card not found", services.ErrCardNotFound, http.StatusNotFound},
		{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
		{"invalid grid", services.ErrInvalidGridSize, http.StatusBadRequest},
		{"invalid header", services.ErrInvalidHeaderText, http.StatusBadRequest},
		{"title exists", services.ErrCardTitleExists, http.StatusConflict},
		{"already exists", services.ErrCardAlreadyExists, http.StatusConflict},
		{"internal", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCard := &mockCardService{
				CloneFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params services.CloneParams) (*services.CloneResult, error) {
					return nil, tt.serviceErr
				},
			}
			handler := NewCardHandler(mockCard)

			bodyBytes, _ := json.Marshal(CloneCardRequest{Title: ptrToString("New"), GridSize: 5})
			req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/clone", bytes.NewBuffer(bodyBytes))
			req = req.WithContext(SetUserInContext(req.Context(), user))
			rr := httptest.NewRecorder()

			handler.Clone(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}

	t.Run("success without truncation message", func(t *testing.T) {
		mockCard := &mockCardService{
			CloneFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params services.CloneParams) (*services.CloneResult, error) {
				return &services.CloneResult{Card: &models.BingoCard{ID: uuid.New(), UserID: user.ID}}, nil
			},
		}
		handler := NewCardHandler(mockCard)

		req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/clone", nil)
		req = req.WithContext(SetUserInContext(req.Context(), user))
		rr := httptest.NewRecorder()

		handler.Clone(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rr.Code)
		}
	})
}

func TestCardHandler_CompleteUncompleteAndNotes_MapsServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	t.Run("complete mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"card not found", services.ErrCardNotFound, http.StatusNotFound},
			{"item not found", services.ErrItemNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"not finalized", services.ErrCardNotFinalized, http.StatusBadRequest},
			{"internal", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					CompleteItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int, params models.CompleteItemParams) (*models.BingoItem, error) {
						return nil, tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				bodyBytes, _ := json.Marshal(CompleteItemRequest{Notes: ptrToString("notes")})
				req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/items/1/complete", bytes.NewBuffer(bodyBytes))
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.CompleteItem(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})

	t.Run("uncomplete mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"card not found", services.ErrCardNotFound, http.StatusNotFound},
			{"item not found", services.ErrItemNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"not finalized", services.ErrCardNotFinalized, http.StatusBadRequest},
			{"internal", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					UncompleteItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int) (*models.BingoItem, error) {
						return nil, tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/items/1/uncomplete", nil)
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.UncompleteItem(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})

	t.Run("notes mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"card not found", services.ErrCardNotFound, http.StatusNotFound},
			{"item not found", services.ErrItemNotFound, http.StatusNotFound},
			{"not owner", services.ErrNotCardOwner, http.StatusForbidden},
			{"internal", errors.New("boom"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockCard := &mockCardService{
					UpdateItemNotesFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int, notes, proofURL *string) (*models.BingoItem, error) {
						return nil, tt.serviceErr
					},
				}
				handler := NewCardHandler(mockCard)

				bodyBytes, _ := json.Marshal(UpdateNotesRequest{Notes: ptrToString("notes")})
				req := httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/items/1/notes", bytes.NewBuffer(bodyBytes))
				req = req.WithContext(SetUserInContext(req.Context(), user))
				rr := httptest.NewRecorder()

				handler.UpdateNotes(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})
}

func TestCardHandler_Import_MapsServiceErrors(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	year := time.Now().Year()

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{"invalid category", services.ErrInvalidCategory, http.StatusBadRequest},
		{"title too long", services.ErrTitleTooLong, http.StatusBadRequest},
		{"invalid position", services.ErrInvalidPosition, http.StatusBadRequest},
		{"invalid grid size", services.ErrInvalidGridSize, http.StatusBadRequest},
		{"invalid header", services.ErrInvalidHeaderText, http.StatusBadRequest},
		{"internal", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCard := &mockCardService{
				CheckForConflictFunc: func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
					return nil, services.ErrCardNotFound
				},
				ImportFunc: func(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error) {
					return nil, tt.serviceErr
				},
			}
			handler := NewCardHandler(mockCard)

			bodyBytes, _ := json.Marshal(ImportCardRequest{
				Year:     year,
				GridSize: 2,
				Items: []ImportCardItem{
					{Position: 0, Content: "a"},
					{Position: 1, Content: "b"},
					{Position: 2, Content: "c"},
				},
			})
			req := httptest.NewRequest(http.MethodPost, "/api/cards/import", bytes.NewBuffer(bodyBytes))
			req = req.WithContext(SetUserInContext(req.Context(), user))
			rr := httptest.NewRecorder()

			handler.Import(rr, req)
			if rr.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}
