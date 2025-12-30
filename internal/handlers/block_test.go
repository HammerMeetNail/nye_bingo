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

type mockBlockService struct {
	BlockFunc       func(ctx context.Context, blockerID, blockedID uuid.UUID) error
	UnblockFunc     func(ctx context.Context, blockerID, blockedID uuid.UUID) error
	ListBlockedFunc func(ctx context.Context, blockerID uuid.UUID) ([]models.BlockedUser, error)
}

func (m *mockBlockService) Block(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	if m.BlockFunc != nil {
		return m.BlockFunc(ctx, blockerID, blockedID)
	}
	return nil
}

func (m *mockBlockService) Unblock(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	if m.UnblockFunc != nil {
		return m.UnblockFunc(ctx, blockerID, blockedID)
	}
	return nil
}

func (m *mockBlockService) IsBlocked(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockBlockService) ListBlocked(ctx context.Context, blockerID uuid.UUID) ([]models.BlockedUser, error) {
	if m.ListBlockedFunc != nil {
		return m.ListBlockedFunc(ctx, blockerID)
	}
	return []models.BlockedUser{}, nil
}

func TestBlockHandler_Block_InvalidBody(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{
		BlockFunc: func(ctx context.Context, blockerID, blockedID uuid.UUID) error {
			t.Fatal("Block should not be called for invalid body")
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/blocks", bytes.NewBufferString("{"))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Block(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}

func TestBlockHandler_Block_InvalidUserID(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{})
	req := httptest.NewRequest(http.MethodPost, "/api/blocks", bytes.NewBufferString(`{"user_id":"not-a-uuid"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Block(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid user ID")
}

func TestBlockHandler_Block_Self(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{
		BlockFunc: func(ctx context.Context, blockerID, blockedID uuid.UUID) error {
			return services.ErrCannotBlockSelf
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/blocks", bytes.NewBufferString(`{"user_id":"`+uuid.New().String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Block(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Cannot block yourself")
}

func TestBlockHandler_Block_Exists(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{
		BlockFunc: func(ctx context.Context, blockerID, blockedID uuid.UUID) error {
			return services.ErrBlockExists
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/blocks", bytes.NewBufferString(`{"user_id":"`+uuid.New().String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Block(rr, req)
	assertErrorResponse(t, rr, http.StatusConflict, "User already blocked")
}

func TestBlockHandler_Block_UserNotFound(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{
		BlockFunc: func(ctx context.Context, blockerID, blockedID uuid.UUID) error {
			return services.ErrBlockedUserNotFound
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/blocks", bytes.NewBufferString(`{"user_id":"`+uuid.New().String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Block(rr, req)
	assertErrorResponse(t, rr, http.StatusNotFound, "User not found")
}

func TestBlockHandler_Block_Error(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{
		BlockFunc: func(ctx context.Context, blockerID, blockedID uuid.UUID) error {
			return errors.New("boom")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/blocks", bytes.NewBufferString(`{"user_id":"`+uuid.New().String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Block(rr, req)
	assertErrorResponse(t, rr, http.StatusInternalServerError, "Internal server error")
}

func TestBlockHandler_Block_Success(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{})
	req := httptest.NewRequest(http.MethodPost, "/api/blocks", bytes.NewBufferString(`{"user_id":"`+uuid.New().String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Block(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestBlockHandler_Unblock_InvalidUserID(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/blocks/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Unblock(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid user ID")
}

func TestBlockHandler_Unblock_NotFound(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{
		UnblockFunc: func(ctx context.Context, blockerID, blockedID uuid.UUID) error {
			return services.ErrBlockNotFound
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/blocks/"+uuid.New().String(), nil)
	req.SetPathValue("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Unblock(rr, req)
	assertErrorResponse(t, rr, http.StatusNotFound, "Block not found")
}

func TestBlockHandler_Unblock_Success(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/blocks/"+uuid.New().String(), nil)
	req.SetPathValue("id", uuid.New().String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Unblock(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestBlockHandler_List_Success(t *testing.T) {
	handler := NewBlockHandler(&mockBlockService{
		ListBlockedFunc: func(ctx context.Context, blockerID uuid.UUID) ([]models.BlockedUser, error) {
			return []models.BlockedUser{{ID: uuid.New(), Username: "blocked"}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/blocks", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
