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

func TestFriendHandler_Search_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/friends/search?q=test", nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestFriendHandler_Search_ShortQuery(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodGet, "/api/friends/search?q=a", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response UserSearchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response.Users) != 0 {
		t.Errorf("expected empty users list for short query, got %d users", len(response.Users))
	}
}

func TestFriendHandler_Search_EmptyQuery(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodGet, "/api/friends/search?q=", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response UserSearchResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response.Users) != 0 {
		t.Errorf("expected empty users list, got %d users", len(response.Users))
	}
}

func TestFriendHandler_SendRequest_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/friends/request", nil)
	rr := httptest.NewRecorder()

	handler.SendRequest(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestFriendHandler_SendRequest_InvalidBody(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/friends/request", bytes.NewBufferString("invalid"))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.SendRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestFriendHandler_SendRequest_InvalidFriendID(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	user := &models.User{ID: uuid.New()}
	body := SendRequestRequest{FriendID: "invalid-uuid"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/friends/request", bytes.NewBuffer(bodyBytes))
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.SendRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Error != "Invalid friend ID" {
		t.Errorf("expected 'Invalid friend ID', got %q", response.Error)
	}
}

func TestFriendHandler_AcceptRequest_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/friends/"+uuid.New().String()+"/accept", nil)
	rr := httptest.NewRecorder()

	handler.AcceptRequest(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestFriendHandler_AcceptRequest_InvalidFriendshipID(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPut, "/api/friends/invalid-uuid/accept", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.AcceptRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestFriendHandler_RejectRequest_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/friends/"+uuid.New().String()+"/reject", nil)
	rr := httptest.NewRecorder()

	handler.RejectRequest(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestFriendHandler_Remove_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/friends/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	handler.Remove(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestFriendHandler_CancelRequest_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/friends/"+uuid.New().String()+"/cancel", nil)
	rr := httptest.NewRecorder()

	handler.CancelRequest(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestFriendHandler_List_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/friends", nil)
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestFriendHandler_GetFriendCard_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/friends/"+uuid.New().String()+"/card", nil)
	rr := httptest.NewRecorder()

	handler.GetFriendCard(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestFriendHandler_GetFriendCard_InvalidFriendshipID(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodGet, "/api/friends/invalid-uuid/card", nil)
	ctx := SetUserInContext(req.Context(), user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.GetFriendCard(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestFriendHandler_GetFriendCards_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/friends/"+uuid.New().String()+"/cards", nil)
	rr := httptest.NewRecorder()

	handler.GetFriendCards(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestParseFriendshipID(t *testing.T) {
	validID := uuid.New()

	tests := []struct {
		name    string
		path    string
		wantID  uuid.UUID
		wantErr bool
	}{
		{
			name:    "valid friendship ID",
			path:    "/api/friends/" + validID.String(),
			wantID:  validID,
			wantErr: false,
		},
		{
			name:    "invalid friendship ID",
			path:    "/api/friends/invalid",
			wantErr: true,
		},
		{
			name:    "missing friendship ID",
			path:    "/api/friends",
			wantErr: true,
		},
		{
			name:    "friendship ID with extra path",
			path:    "/api/friends/" + validID.String() + "/accept",
			wantID:  validID,
			wantErr: false,
		},
		{
			name:    "friendship ID with card path",
			path:    "/api/friends/" + validID.String() + "/card",
			wantID:  validID,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			id, err := parseFriendshipID(req)

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
