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

func TestReactionHandler_AddReaction_Unauthenticated(t *testing.T) {
	handler := NewReactionHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/items/"+uuid.New().String()+"/react", nil)
	rr := httptest.NewRecorder()

	handler.AddReaction(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestReactionHandler_AddReaction_InvalidItemID(t *testing.T) {
	handler := NewReactionHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/items/invalid-uuid/react", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddReaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Invalid item ID" {
		t.Errorf("expected 'Invalid item ID', got %q", response.Error)
	}
}

func TestReactionHandler_AddReaction_InvalidBody(t *testing.T) {
	handler := NewReactionHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/items/"+uuid.New().String()+"/react", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AddReaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestReactionHandler_RemoveReaction_Unauthenticated(t *testing.T) {
	handler := NewReactionHandler(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/items/"+uuid.New().String()+"/react", nil)
	rr := httptest.NewRecorder()

	handler.RemoveReaction(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestReactionHandler_RemoveReaction_InvalidItemID(t *testing.T) {
	handler := NewReactionHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodDelete, "/api/items/invalid-uuid/react", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.RemoveReaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestReactionHandler_GetReactions_Unauthenticated(t *testing.T) {
	handler := NewReactionHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/items/"+uuid.New().String()+"/reactions", nil)
	rr := httptest.NewRecorder()

	handler.GetReactions(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestReactionHandler_GetReactions_InvalidItemID(t *testing.T) {
	handler := NewReactionHandler(nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodGet, "/api/items/invalid-uuid/reactions", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.GetReactions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestReactionHandler_GetAllowedEmojis(t *testing.T) {
	handler := NewReactionHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/reactions/emojis", nil)
	rr := httptest.NewRecorder()

	handler.GetAllowedEmojis(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response AllowedEmojisResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response.Emojis) == 0 {
		t.Error("expected emojis to be returned")
	}

	// Verify common emojis are present (based on AllowedEmojis in models/reaction.go)
	expectedEmojis := []string{"🎉", "👏", "🔥", "❤️", "⭐"}

	for _, emoji := range expectedEmojis {
		found := false
		for _, e := range response.Emojis {
			if e == emoji {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected emoji %s to be in allowed emojis", emoji)
		}
	}
}

func TestParseItemID(t *testing.T) {
	validID := uuid.New()

	tests := []struct {
		name    string
		path    string
		wantID  uuid.UUID
		wantErr bool
	}{
		{
			name:    "valid item ID",
			path:    "/api/items/" + validID.String() + "/react",
			wantID:  validID,
			wantErr: false,
		},
		{
			name:    "invalid item ID",
			path:    "/api/items/invalid/react",
			wantErr: true,
		},
		{
			name:    "missing item ID",
			path:    "/api/items",
			wantErr: true,
		},
		{
			name:    "item ID with reactions path",
			path:    "/api/items/" + validID.String() + "/reactions",
			wantID:  validID,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			id, err := parseItemID(req)

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
