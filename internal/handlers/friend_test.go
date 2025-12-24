package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func TestFriendHandlerSearchValidationAndError(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{SearchUsersFunc: func(ctx context.Context, userID uuid.UUID, query string) ([]models.UserSearchResult, error) {
		return nil, errors.New("boom")
	}}, &mockCardService{})

	req := httptest.NewRequest(http.MethodGet, "/api/friends/search?q=a", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Search(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected short query to return 200")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/friends/search?q=abc", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr = httptest.NewRecorder()
	handler.Search(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected error path, got %d", rr.Code)
	}
}

func TestFriendHandlerSendRequestScenarios(t *testing.T) {
	friendID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{SendRequestFunc: func(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
		if friendID == userID {
			return nil, services.ErrCannotFriendSelf
		}
		return &models.Friendship{}, nil
	}}, &mockCardService{})

	// invalid body
	req := httptest.NewRequest(http.MethodPost, "/api/friends/request", bytes.NewBufferString("{"))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.SendRequest(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid json")
	}

	// self request
	payload := []byte(`{"friend_id":"` + friendID.String() + `"}`)
	user := &models.User{ID: friendID}
	req = httptest.NewRequest(http.MethodPost, "/api/friends/request", bytes.NewBuffer(payload))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr = httptest.NewRecorder()
	handler.SendRequest(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for self friend")
	}

	// success
	user.ID = uuid.New()
	req = httptest.NewRequest(http.MethodPost, "/api/friends/request", bytes.NewBuffer(payload))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr = httptest.NewRecorder()
	handler.SendRequest(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected created, got %d", rr.Code)
	}
}

func TestFriendHandlerAcceptAndReject(t *testing.T) {
	friendshipID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{
		AcceptRequestFunc: func(ctx context.Context, userID, id uuid.UUID) (*models.Friendship, error) {
			return &models.Friendship{}, nil
		},
		RejectRequestFunc: func(ctx context.Context, userID, id uuid.UUID) error {
			return nil
		},
	}, &mockCardService{})

	user := &models.User{ID: uuid.New()}

	// accept
	req := httptest.NewRequest(http.MethodPut, "/api/friends/"+friendshipID.String()+"/accept", nil)
	req.SetPathValue("id", friendshipID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr := httptest.NewRecorder()
	handler.AcceptRequest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// reject
	req = httptest.NewRequest(http.MethodPut, "/api/friends/"+friendshipID.String()+"/reject", nil)
	req.SetPathValue("id", friendshipID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr = httptest.NewRecorder()
	handler.RejectRequest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
