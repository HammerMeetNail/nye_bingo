package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type mockInviteService struct {
	CreateInviteFunc func(ctx context.Context, inviterID uuid.UUID, expiresInDays int) (*models.FriendInvite, string, error)
	ListInvitesFunc  func(ctx context.Context, inviterID uuid.UUID) ([]models.FriendInvite, error)
	RevokeInviteFunc func(ctx context.Context, inviterID, inviteID uuid.UUID) error
	AcceptInviteFunc func(ctx context.Context, recipientID uuid.UUID, token string) (*models.UserSearchResult, error)
}

func (m *mockInviteService) CreateInvite(ctx context.Context, inviterID uuid.UUID, expiresInDays int) (*models.FriendInvite, string, error) {
	if m.CreateInviteFunc != nil {
		return m.CreateInviteFunc(ctx, inviterID, expiresInDays)
	}
	return &models.FriendInvite{ID: uuid.New(), InviterUserID: inviterID, CreatedAt: time.Now()}, "token", nil
}

func (m *mockInviteService) ListInvites(ctx context.Context, inviterID uuid.UUID) ([]models.FriendInvite, error) {
	if m.ListInvitesFunc != nil {
		return m.ListInvitesFunc(ctx, inviterID)
	}
	return []models.FriendInvite{}, nil
}

func (m *mockInviteService) RevokeInvite(ctx context.Context, inviterID, inviteID uuid.UUID) error {
	if m.RevokeInviteFunc != nil {
		return m.RevokeInviteFunc(ctx, inviterID, inviteID)
	}
	return nil
}

func (m *mockInviteService) AcceptInvite(ctx context.Context, recipientID uuid.UUID, token string) (*models.UserSearchResult, error) {
	if m.AcceptInviteFunc != nil {
		return m.AcceptInviteFunc(ctx, recipientID, token)
	}
	return &models.UserSearchResult{ID: uuid.New(), Username: "inviter"}, nil
}

func TestFriendInviteHandler_Create_InvalidBody(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{})
	req := httptest.NewRequest(http.MethodPost, "/api/friends/invites", bytes.NewBufferString("{"))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Create(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}

func TestFriendInviteHandler_Create_Success(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{})
	req := httptest.NewRequest(http.MethodPost, "/api/friends/invites", bytes.NewBufferString(`{"expires_in_days":7}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Create(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestFriendInviteHandler_Create_InvalidExpiresInDays(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{
		CreateInviteFunc: func(ctx context.Context, inviterID uuid.UUID, expiresInDays int) (*models.FriendInvite, string, error) {
			t.Fatal("CreateInvite should not be called for invalid expires_in_days")
			return nil, "", nil
		},
	})

	payload := fmt.Sprintf(`{"expires_in_days":%d}`, services.InviteExpiryMaxDays+1)
	req := httptest.NewRequest(http.MethodPost, "/api/friends/invites", bytes.NewBufferString(payload))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Create(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, fmt.Sprintf("expires_in_days must be between %d and %d", services.InviteExpiryMinDays, services.InviteExpiryMaxDays))
}

func TestFriendInviteHandler_Create_LimitReached(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{
		CreateInviteFunc: func(ctx context.Context, inviterID uuid.UUID, expiresInDays int) (*models.FriendInvite, string, error) {
			return nil, "", services.ErrInviteLimitReached
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/friends/invites", bytes.NewBufferString(`{"expires_in_days":7}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Create(rr, req)
	assertErrorResponse(t, rr, http.StatusConflict, fmt.Sprintf("Invite limit reached (max %d active)", services.InviteMaxActive))
}

func TestFriendInviteHandler_List_Success(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{
		ListInvitesFunc: func(ctx context.Context, inviterID uuid.UUID) ([]models.FriendInvite, error) {
			return []models.FriendInvite{{ID: uuid.New(), InviterUserID: inviterID}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/friends/invites", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestFriendInviteHandler_Revoke_InvalidID(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/friends/invites/not-a-uuid/revoke", nil)
	req.SetPathValue("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Revoke(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid invite ID")
}

func TestFriendInviteHandler_Revoke_NotFound(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{
		RevokeInviteFunc: func(ctx context.Context, inviterID, inviteID uuid.UUID) error {
			return services.ErrInviteNotFound
		},
	})

	inviteID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/api/friends/invites/"+inviteID+"/revoke", nil)
	req.SetPathValue("id", inviteID)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Revoke(rr, req)
	assertErrorResponse(t, rr, http.StatusNotFound, "Invite not found")
}

func TestFriendInviteHandler_Revoke_DoesNotReturnInvite(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{})
	inviteID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/api/friends/invites/"+inviteID+"/revoke", nil)
	req.SetPathValue("id", inviteID)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Revoke(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "\"invite\"") {
		t.Fatalf("expected revoke response to omit invite, got: %s", rr.Body.String())
	}
}

func TestFriendInviteHandler_Accept_InvalidBody(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{})
	req := httptest.NewRequest(http.MethodPost, "/api/friends/invites/accept", bytes.NewBufferString(`{}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Accept(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}

func TestFriendInviteHandler_Accept_NotFound(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{
		AcceptInviteFunc: func(ctx context.Context, recipientID uuid.UUID, token string) (*models.UserSearchResult, error) {
			return nil, services.ErrInviteNotFound
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/friends/invites/accept", bytes.NewBufferString(`{"token":"abc"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Accept(rr, req)
	assertErrorResponse(t, rr, http.StatusNotFound, "Invite not found or expired")
}

func TestFriendInviteHandler_Accept_Blocked(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{
		AcceptInviteFunc: func(ctx context.Context, recipientID uuid.UUID, token string) (*models.UserSearchResult, error) {
			return nil, services.ErrUserBlocked
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/friends/invites/accept", bytes.NewBufferString(`{"token":"abc"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Accept(rr, req)
	assertErrorResponse(t, rr, http.StatusForbidden, "Cannot accept invite")
}

func TestFriendInviteHandler_Accept_Error(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{
		AcceptInviteFunc: func(ctx context.Context, recipientID uuid.UUID, token string) (*models.UserSearchResult, error) {
			return nil, errors.New("boom")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/friends/invites/accept", bytes.NewBufferString(`{"token":"abc"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Accept(rr, req)
	assertErrorResponse(t, rr, http.StatusInternalServerError, "Internal server error")
}

func TestFriendInviteHandler_Accept_Success(t *testing.T) {
	handler := NewFriendInviteHandler(&mockInviteService{})
	req := httptest.NewRequest(http.MethodPost, "/api/friends/invites/accept", bytes.NewBufferString(`{"token":"abc"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Accept(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
