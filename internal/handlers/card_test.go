package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestCardHandler_Create_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards", nil)
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_Create_InvalidBody(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
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

func TestCardHandler_List_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cards", nil)
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_Get_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_Get_InvalidCardID(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodGet, "/api/cards/invalid-uuid", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCardHandler_Delete_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/cards/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_AddItem_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/items", nil)
	rr := httptest.NewRecorder()

	handler.AddItem(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_AddItem_InvalidBody(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/items", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddItem(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
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

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
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

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_Shuffle_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/shuffle", nil)
	rr := httptest.NewRecorder()

	handler.Shuffle(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_SwapItems_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/swap", nil)
	rr := httptest.NewRecorder()

	handler.SwapItems(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_SwapItems_InvalidBody(t *testing.T) {
	handler := NewCardHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/swap", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.SwapItems(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCardHandler_Finalize_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+uuid.New().String()+"/finalize", nil)
	rr := httptest.NewRecorder()

	handler.Finalize(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_CompleteItem_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/0/complete", nil)
	rr := httptest.NewRecorder()

	handler.CompleteItem(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_UncompleteItem_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/0/uncomplete", nil)
	rr := httptest.NewRecorder()

	handler.UncompleteItem(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_UpdateNotes_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/items/0/notes", nil)
	rr := httptest.NewRecorder()

	handler.UpdateNotes(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_Archive_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/archive", nil)
	rr := httptest.NewRecorder()

	handler.Archive(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_Stats_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cards/"+uuid.New().String()+"/stats", nil)
	rr := httptest.NewRecorder()

	handler.Stats(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_UpdateVisibility_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/"+uuid.New().String()+"/visibility", nil)
	rr := httptest.NewRecorder()

	handler.UpdateVisibility(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCardHandler_BulkUpdateVisibility_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/visibility/bulk", nil)
	rr := httptest.NewRecorder()

	handler.BulkUpdateVisibility(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
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

func TestCardHandler_BulkDelete_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/cards/bulk", nil)
	rr := httptest.NewRecorder()

	handler.BulkDelete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
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

func TestCardHandler_BulkUpdateArchive_Unauthenticated(t *testing.T) {
	handler := NewCardHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/cards/archive/bulk", nil)
	rr := httptest.NewRecorder()

	handler.BulkUpdateArchive(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
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

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
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

	if response.Error != "Cannot import more than 24 items" {
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
		name        string
		path        string
		wantPos     int
		wantErr     bool
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
