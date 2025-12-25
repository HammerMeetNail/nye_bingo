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

func TestCardHandler_Create_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards", nil)
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_Create_InvalidBody(t *testing.T) {
	handler := NewCardHandler(&mockCardService{
		CreateFunc: func(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error) {
			t.Fatal("Create should not be called for invalid body")
			return nil, nil
		},
	})

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}

func TestCardHandler_Create_InvalidYear(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}

	tests := []struct {
		name string
		year int
	}{
		{"year too old", 2019},
		{"year too far future", 2030},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := CreateCardRequest{Year: tt.year}
			bodyBytes, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewBuffer(bodyBytes))
			ctx := SetUserInContext(req.Context(), user)
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()

			handler.Create(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", rr.Code)
			}

			var response ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if response.Error != "Year must be between 2020 and next year" {
				t.Errorf("expected year validation error, got %q", response.Error)
			}
		})
	}
}

func TestCardHandler_Create_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	createdCard := &models.BingoCard{ID: uuid.New(), UserID: user.ID, Year: time.Now().Year()}
	mockCard := &mockCardService{
		CreateFunc: func(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error) {
			if params.UserID != user.ID {
				t.Fatalf("unexpected user id: %s", params.UserID)
			}
			return createdCard, nil
		},
	}

	handler := NewCardHandler(mockCard)

	body := CreateCardRequest{Year: createdCard.Year, GridSize: ptrToInt(models.MaxGridSize)}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}

	var response CardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response.Card == nil || response.Card.ID != createdCard.ID {
		t.Fatalf("expected card %s", createdCard.ID)
	}
}

func ptrToInt(i int) *int {
	return &i
}

func ptrToBool(b bool) *bool {
	return &b
}

func ptrToString(s string) *string {
	return &s
}

func TestCardHandler_List_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cards", nil)
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_Get_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_Get_InvalidCardID(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodGet, "/api/cards/invalid-uuid", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid card ID")
}

func TestCardHandler_Delete_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/cards/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_AddItem_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/items", nil)
	rr := httptest.NewRecorder()

	handler.AddItem(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_AddItem_InvalidBody(t *testing.T) {
	handler := NewCardHandler(&mockCardService{
		AddItemFunc: func(ctx context.Context, userID uuid.UUID, params models.AddItemParams) (*models.BingoItem, error) {
			t.Fatal("AddItem should not be called for invalid body")
			return nil, nil
		},
	})

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/items", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddItem(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}

func TestCardHandler_AddItem_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()
	createdItem := &models.BingoItem{ID: uuid.New(), CardID: cardID, Position: 1, Content: "Test"}
	mockCard := &mockCardService{
		AddItemFunc: func(ctx context.Context, userID uuid.UUID, params models.AddItemParams) (*models.BingoItem, error) {
			if userID != user.ID {
				t.Fatalf("unexpected user id: %s", userID)
			}
			if params.CardID != cardID {
				t.Fatalf("unexpected card id: %s", params.CardID)
			}
			return createdItem, nil
		},
	}

	handler := NewCardHandler(mockCard)

	body := AddItemRequest{Content: "Test item"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/items", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddItem(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}

	var response CardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Item == nil || response.Item.ID != createdItem.ID {
		t.Fatalf("expected item %s", createdItem.ID)
	}
}

func TestCardHandler_AddItem_EmptyContent(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	body := AddItemRequest{Content: "   "}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/items", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddItem(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Content is required" {
		t.Errorf("expected 'Content is required', got %q", response.Error)
	}
}

func TestCardHandler_AddItem_ContentTooLong(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	body := AddItemRequest{Content: string(make([]byte, 501))}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/items", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddItem(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Content must be 500 characters or less" {
		t.Errorf("expected content length error, got %q", response.Error)
	}
}

func TestCardHandler_UpdateItem_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/0", nil)
	rr := httptest.NewRecorder()

	handler.UpdateItem(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_UpdateItem_InvalidPosition(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/invalid", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.UpdateItem(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCardHandler_UpdateItem_EmptyContent(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	emptyContent := "   "
	body := UpdateItemRequest{Content: &emptyContent}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/0", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.UpdateItem(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Content cannot be empty" {
		t.Errorf("expected 'Content cannot be empty', got %q", response.Error)
	}
}

func TestCardHandler_UpdateItem_ContentTooLong(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	longContent := string(make([]byte, 501))
	body := UpdateItemRequest{Content: &longContent}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/0", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.UpdateItem(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCardHandler_RemoveItem_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/cards/"+uuid.New().String()+"/items/0", nil)
	rr := httptest.NewRecorder()

	handler.RemoveItem(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_Shuffle_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/shuffle", nil)
	rr := httptest.NewRecorder()

	handler.Shuffle(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_SwapItems_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/swap", nil)
	rr := httptest.NewRecorder()

	handler.SwapItems(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_SwapItems_InvalidBody(t *testing.T) {
	handler := NewCardHandler(&mockCardService{
		SwapItemsFunc: func(ctx context.Context, userID, cardID uuid.UUID, fromPos, toPos int) error {
			t.Fatal("SwapItems should not be called for invalid body")
			return nil
		},
	})

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/swap", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.SwapItems(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}

func TestCardHandler_Finalize_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/finalize", nil)
	rr := httptest.NewRecorder()

	handler.Finalize(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_CompleteItem_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/0/complete", nil)
	rr := httptest.NewRecorder()

	handler.CompleteItem(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_UncompleteItem_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/0/uncomplete", nil)
	rr := httptest.NewRecorder()

	handler.UncompleteItem(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_UpdateNotes_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/0/notes", nil)
	rr := httptest.NewRecorder()

	handler.UpdateNotes(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_Archive_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/archive", nil)
	rr := httptest.NewRecorder()

	handler.Archive(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_Archive_ServiceError(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockCard := &mockCardService{
		GetArchiveFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return nil, errors.New("archive error")
		},
	}
	handler := NewCardHandler(mockCard)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/archive", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Archive(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestCardHandler_Stats_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/"+uuid.New().String()+"/stats", nil)
	rr := httptest.NewRecorder()

	handler.Stats(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_UpdateVisibility_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/visibility", nil)
	rr := httptest.NewRecorder()

	handler.UpdateVisibility(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_BulkUpdateVisibility_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/visibility/bulk", nil)
	rr := httptest.NewRecorder()

	handler.BulkUpdateVisibility(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_BulkUpdateVisibility_EmptyCardIDs(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	body := BulkUpdateVisibilityRequest{CardIDs: []string{}}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/visibility/bulk", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.BulkUpdateVisibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "At least one card ID is required" {
		t.Errorf("expected error about card IDs, got %q", response.Error)
	}
}

func TestCardHandler_BulkUpdateVisibility_InvalidCardID(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	body := BulkUpdateVisibilityRequest{CardIDs: []string{"invalid-uuid"}}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/visibility/bulk", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.BulkUpdateVisibility(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCardHandler_BulkUpdateVisibility_ServiceError(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockCard := &mockCardService{
		BulkUpdateVisibilityFunc: func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, visibleToFriends bool) (int, error) {
			return 0, errors.New("bulk visibility error")
		},
	}
	handler := NewCardHandler(mockCard)

	body := BulkUpdateVisibilityRequest{CardIDs: []string{uuid.New().String()}, VisibleToFriends: true}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/cards/visibility/bulk", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.BulkUpdateVisibility(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestCardHandler_BulkDelete_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/cards/bulk", nil)
	rr := httptest.NewRecorder()

	handler.BulkDelete(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_BulkDelete_EmptyCardIDs(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	body := BulkDeleteRequest{CardIDs: []string{}}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodDelete, "/api/cards/bulk", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.BulkDelete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCardHandler_BulkDelete_ServiceError(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockCard := &mockCardService{
		BulkDeleteFunc: func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID) (int, error) {
			return 0, errors.New("bulk delete error")
		},
	}
	handler := NewCardHandler(mockCard)

	body := BulkDeleteRequest{CardIDs: []string{uuid.New().String()}}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodDelete, "/api/cards/bulk", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.BulkDelete(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestCardHandler_BulkUpdateArchive_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/archive/bulk", nil)
	rr := httptest.NewRecorder()

	handler.BulkUpdateArchive(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_BulkUpdateArchive_EmptyCardIDs(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	body := BulkUpdateArchiveRequest{CardIDs: []string{}}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/archive/bulk", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.BulkUpdateArchive(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCardHandler_BulkUpdateArchive_ServiceError(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockCard := &mockCardService{
		BulkUpdateArchiveFunc: func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, isArchived bool) (int, error) {
			return 0, errors.New("bulk archive error")
		},
	}
	handler := NewCardHandler(mockCard)

	body := BulkUpdateArchiveRequest{CardIDs: []string{uuid.New().String()}, IsArchived: true}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/cards/archive/bulk", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.BulkUpdateArchive(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestCardHandler_GetCategories(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/categories", nil)
	rr := httptest.NewRecorder()

	handler.GetCategories(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response CategoriesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response.Categories) == 0 {
		t.Error("expected categories to be returned")
	}

	// Verify categories have both ID and Name
	for _, cat := range response.Categories {
		if cat.ID == "" {
			t.Error("category ID should not be empty")
		}
		if cat.Name == "" {
			t.Error("category Name should not be empty")
		}
	}
}

func TestCardHandler_Import_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/import", nil)
	rr := httptest.NewRecorder()

	handler.Import(rr, req)

	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestCardHandler_Import_InvalidYear(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	body := ImportCardRequest{
		Year:  2019,
		Items: []ImportCardItem{{Position: 0, Content: "test"}},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/import", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Import(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCardHandler_Import_NoItems(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	body := ImportCardRequest{
		Year:  2024,
		Items: []ImportCardItem{},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/import", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Import(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "At least one item is required" {
		t.Errorf("expected items required error, got %q", response.Error)
	}
}

func TestCardHandler_Import_TooManyItems(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	items := make([]ImportCardItem, 25)
	for i := range items {
		items[i] = ImportCardItem{Position: i, Content: "test"}
	}
	body := ImportCardRequest{
		Year:  2024,
		Items: items,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/import", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Import(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Cannot import more than 24 items for a 5x5 card" {
		t.Errorf("expected too many items error, got %q", response.Error)
	}
}

func TestCardHandler_Import_FinalizeWithoutFullItems(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	body := ImportCardRequest{
		Year:     2024,
		Items:    []ImportCardItem{{Position: 0, Content: "test"}},
		Finalize: true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/import", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Import(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Card must have exactly 24 items to finalize" {
		t.Errorf("expected finalize item count error, got %q", response.Error)
	}
}

func TestCardHandler_UpdateConfig_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	var gotParams models.UpdateCardConfigParams
	mockCard := &mockCardService{
		UpdateConfigFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params models.UpdateCardConfigParams) (*models.BingoCard, error) {
			if userID != user.ID {
				t.Fatalf("unexpected user id: %s", userID)
			}
			if gotCardID != cardID {
				t.Fatalf("unexpected card id: %s", gotCardID)
			}
			gotParams = params
			return &models.BingoCard{ID: cardID, UserID: user.ID}, nil
		},
	}
	handler := NewCardHandler(mockCard)

	bodyBytes, _ := json.Marshal(UpdateCardConfigRequest{
		HeaderText:   ptrToString("  Header  "),
		HasFreeSpace: ptrToBool(false),
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/config", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.UpdateConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if gotParams.HeaderText == nil || *gotParams.HeaderText != "Header" {
		t.Fatalf("expected header to be trimmed")
	}
	if gotParams.HasFreeSpace == nil || *gotParams.HasFreeSpace != false {
		t.Fatalf("expected has_free_space to be forwarded")
	}
}

func TestCardHandler_Clone_SuccessAndTruncationMessage(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	mockCard := &mockCardService{
		CloneFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params services.CloneParams) (*services.CloneResult, error) {
			if userID != user.ID {
				t.Fatalf("unexpected user id: %s", userID)
			}
			if gotCardID != cardID {
				t.Fatalf("unexpected card id: %s", gotCardID)
			}
			if params.Title == nil || *params.Title != "New Title" {
				t.Fatalf("expected title to be trimmed")
			}
			if params.HeaderText != "Header" {
				t.Fatalf("expected header text to be trimmed into string")
			}
			return &services.CloneResult{
				Card:               &models.BingoCard{ID: uuid.New(), UserID: user.ID},
				TruncatedItemCount: 1,
			}, nil
		},
	}
	handler := NewCardHandler(mockCard)

	bodyBytes, _ := json.Marshal(CloneCardRequest{
		Title:      ptrToString("  New Title  "),
		HeaderText: ptrToString("  Header  "),
		GridSize:   4,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/clone", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Clone(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}

	var resp CardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Message == "" {
		t.Fatalf("expected message to be set")
	}
}

func TestCardHandler_UpdateMeta_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	mockCard := &mockCardService{
		UpdateMetaFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params models.UpdateCardMetaParams) (*models.BingoCard, error) {
			if params.Title == nil || *params.Title != "Title" {
				t.Fatalf("expected title to be trimmed")
			}
			return &models.BingoCard{ID: cardID, UserID: userID}, nil
		},
	}
	handler := NewCardHandler(mockCard)

	bodyBytes, _ := json.Marshal(UpdateCardMetaRequest{Title: ptrToString("  Title  ")})
	req := httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/meta", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.UpdateMeta(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestCardHandler_ListExportable_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}

	mockCard := &mockCardService{
		ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return []*models.BingoCard{{ID: uuid.New(), UserID: userID}}, nil
		},
		GetArchiveFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return []*models.BingoCard{{ID: uuid.New(), UserID: userID}}, nil
		},
	}
	handler := NewCardHandler(mockCard)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/exportable", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.ListExportable(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp CardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(resp.Cards))
	}
}

func TestCardHandler_ListExportable_ListError(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockCard := &mockCardService{
		ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return nil, errors.New("list error")
		},
	}
	handler := NewCardHandler(mockCard)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/exportable", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.ListExportable(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestCardHandler_ListExportable_ArchiveError(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockCard := &mockCardService{
		ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return []*models.BingoCard{{ID: uuid.New(), UserID: userID}}, nil
		},
		GetArchiveFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return nil, errors.New("archive error")
		},
	}
	handler := NewCardHandler(mockCard)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/exportable", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.ListExportable(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestCardHandler_CompleteUncompleteAndNotes_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	mockCard := &mockCardService{
		CompleteItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int, params models.CompleteItemParams) (*models.BingoItem, error) {
			return &models.BingoItem{CardID: gotCardID, Position: position}, nil
		},
		UncompleteItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int) (*models.BingoItem, error) {
			return &models.BingoItem{CardID: gotCardID, Position: position}, nil
		},
		UpdateItemNotesFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int, notes, proofURL *string) (*models.BingoItem, error) {
			return &models.BingoItem{CardID: gotCardID, Position: position}, nil
		},
	}
	handler := NewCardHandler(mockCard)

	bodyBytes, _ := json.Marshal(CompleteItemRequest{Notes: ptrToString("notes")})
	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/items/3/complete", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()
	handler.CompleteItem(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/items/3/uncomplete", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.UncompleteItem(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	bodyBytes, _ = json.Marshal(UpdateNotesRequest{Notes: ptrToString("notes")})
	req = httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/items/3/notes", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.UpdateNotes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestCardHandler_StatsAndUpdateVisibility_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	mockCard := &mockCardService{
		GetStatsFunc: func(ctx context.Context, userID, gotCardID uuid.UUID) (*models.CardStats, error) {
			return &models.CardStats{TotalItems: 10, CompletedItems: 3}, nil
		},
		UpdateVisibilityFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, visibleToFriends bool) (*models.BingoCard, error) {
			return &models.BingoCard{ID: gotCardID, UserID: userID, VisibleToFriends: visibleToFriends}, nil
		},
	}
	handler := NewCardHandler(mockCard)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/"+cardID.String()+"/stats", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()
	handler.Stats(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	bodyBytes, _ := json.Marshal(UpdateVisibilityRequest{VisibleToFriends: true})
	req = httptest.NewRequest(http.MethodPatch, "/api/cards/"+cardID.String()+"/visibility", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.UpdateVisibility(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestCardHandler_ListGetDeleteAndOtherEndpoints_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	mockCard := &mockCardService{
		ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return nil, nil
		},
		GetByIDFunc: func(ctx context.Context, gotCardID uuid.UUID) (*models.BingoCard, error) {
			return &models.BingoCard{ID: gotCardID, UserID: user.ID}, nil
		},
		DeleteFunc: func(ctx context.Context, userID, gotCardID uuid.UUID) error {
			return nil
		},
		RemoveItemFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, position int) error {
			return nil
		},
		ShuffleFunc: func(ctx context.Context, userID, gotCardID uuid.UUID) (*models.BingoCard, error) {
			return &models.BingoCard{ID: gotCardID, UserID: userID}, nil
		},
		SwapItemsFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, pos1, pos2 int) error {
			if pos1 != 1 || pos2 != 2 {
				t.Fatalf("unexpected swap positions: %d,%d", pos1, pos2)
			}
			return nil
		},
		FinalizeFunc: func(ctx context.Context, userID, gotCardID uuid.UUID, params *services.FinalizeParams) (*models.BingoCard, error) {
			if params == nil || params.VisibleToFriends == nil || *params.VisibleToFriends != true {
				t.Fatalf("expected visible_to_friends param")
			}
			return &models.BingoCard{ID: gotCardID, UserID: userID, IsFinalized: true}, nil
		},
		GetArchiveFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return nil, nil
		},
		BulkUpdateVisibilityFunc: func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, visibleToFriends bool) (int, error) {
			return len(cardIDs), nil
		},
		BulkDeleteFunc: func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID) (int, error) {
			return len(cardIDs), nil
		},
		BulkUpdateArchiveFunc: func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, isArchived bool) (int, error) {
			if !isArchived {
				t.Fatalf("expected isArchived true")
			}
			return len(cardIDs), nil
		},
	}
	handler := NewCardHandler(mockCard)

	req := httptest.NewRequest(http.MethodGet, "/api/cards", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()
	handler.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/cards/"+cardID.String(), nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/cards/"+cardID.String(), nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.Delete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected delete 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/cards/"+cardID.String()+"/items/2", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.RemoveItem(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected remove item 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/shuffle", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.Shuffle(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected shuffle 200, got %d", rr.Code)
	}

	swapBody, _ := json.Marshal(SwapRequest{Position1: 1, Position2: 2})
	req = httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/swap", bytes.NewBuffer(swapBody))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.SwapItems(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected swap 200, got %d", rr.Code)
	}

	finalizeBody, _ := json.Marshal(FinalizeRequest{VisibleToFriends: ptrToBool(true)})
	req = httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/finalize", bytes.NewBuffer(finalizeBody))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.Finalize(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected finalize 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/cards/archive", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.Archive(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected archive 200, got %d", rr.Code)
	}

	id1 := uuid.New()
	id2 := uuid.New()

	bodyBytes, _ := json.Marshal(BulkUpdateVisibilityRequest{CardIDs: []string{id1.String(), id2.String()}, VisibleToFriends: true})
	req = httptest.NewRequest(http.MethodPost, "/api/cards/visibility/bulk", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.BulkUpdateVisibility(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected bulk visibility 200, got %d", rr.Code)
	}

	bodyBytes, _ = json.Marshal(BulkDeleteRequest{CardIDs: []string{id1.String(), id2.String()}})
	req = httptest.NewRequest(http.MethodPost, "/api/cards/bulk-delete", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.BulkDelete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected bulk delete 200, got %d", rr.Code)
	}

	bodyBytes, _ = json.Marshal(BulkUpdateArchiveRequest{CardIDs: []string{id1.String(), id2.String()}, IsArchived: true})
	req = httptest.NewRequest(http.MethodPost, "/api/cards/archive/bulk", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr = httptest.NewRecorder()
	handler.BulkUpdateArchive(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected bulk archive 200, got %d", rr.Code)
	}
}

func TestCardHandler_Import_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	year := time.Now().Year()

	cardID := uuid.New()
	mockCard := &mockCardService{
		CheckForConflictFunc: func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
			return nil, services.ErrCardNotFound
		},
		ImportFunc: func(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error) {
			if params.UserID != user.ID {
				t.Fatalf("unexpected user id: %s", params.UserID)
			}
			if params.Year != year {
				t.Fatalf("unexpected year: %d", params.Year)
			}
			if params.GridSize != 2 {
				t.Fatalf("unexpected grid size: %d", params.GridSize)
			}
			if params.HasFreeSpace != true {
				t.Fatalf("expected free space default true")
			}
			return &models.BingoCard{ID: cardID, UserID: user.ID, Year: params.Year}, nil
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

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
}

func TestCardHandler_Import_ServiceError(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	year := time.Now().Year()
	mockCard := &mockCardService{
		CheckForConflictFunc: func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
			return nil, services.ErrCardNotFound
		},
		ImportFunc: func(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error) {
			return nil, errors.New("import error")
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

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestParseCardID(t *testing.T) {
	validID := uuid.New()

	tests := []struct {
		name    string
		path    string
		wantID  uuid.UUID
		wantErr bool
	}{
		{
			name:    "valid card ID",
			path:    "/api/cards/" + validID.String(),
			wantID:  validID,
			wantErr: false,
		},
		{
			name:    "invalid card ID",
			path:    "/api/cards/invalid",
			wantErr: true,
		},
		{
			name:    "missing card ID",
			path:    "/api/cards",
			wantErr: true,
		},
		{
			name:    "card ID with extra path",
			path:    "/api/cards/" + validID.String() + "/items",
			wantID:  validID,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			id, err := parseCardID(req)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if id != tt.wantID {
					t.Errorf("expected ID %v, got %v", tt.wantID, id)
				}
			}
		})
	}
}

func TestParsePosition(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantPos int
		wantErr bool
	}{
		{
			name:    "valid position",
			path:    "/api/cards/abc/items/5",
			wantPos: 5,
			wantErr: false,
		},
		{
			name:    "position zero",
			path:    "/api/cards/abc/items/0",
			wantPos: 0,
			wantErr: false,
		},
		{
			name:    "invalid position",
			path:    "/api/cards/abc/items/invalid",
			wantErr: true,
		},
		{
			name:    "missing position",
			path:    "/api/cards/abc/items",
			wantErr: true,
		},
		{
			name:    "position with extra path",
			path:    "/api/cards/abc/items/10/complete",
			wantPos: 10,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			pos, err := parsePosition(req)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if pos != tt.wantPos {
					t.Errorf("expected position %d, got %d", tt.wantPos, pos)
				}
			}
		})
	}
}
