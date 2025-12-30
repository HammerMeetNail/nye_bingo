package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func TestFriendHandler_Search_ShortQuery(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{SearchUsersFunc: func(ctx context.Context, userID uuid.UUID, query string) ([]models.UserSearchResult, error) {
		t.Fatal("SearchUsers should not be called for short queries")
		return nil, nil
	}}, &mockCardService{})

	req := httptest.NewRequest(http.MethodGet, "/api/friends/search?q=a", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Search(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected short query to return 200, got %d", rr.Code)
	}
}

func TestFriendHandler_Search_ServiceError(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{SearchUsersFunc: func(ctx context.Context, userID uuid.UUID, query string) ([]models.UserSearchResult, error) {
		return nil, errors.New("boom")
	}}, &mockCardService{})

	req := httptest.NewRequest(http.MethodGet, "/api/friends/search?q=abc", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Search(rr, req)
	assertErrorResponse(t, rr, http.StatusInternalServerError, "Internal server error")
}

func TestFriendHandler_SendRequest_InvalidBody(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{SendRequestFunc: func(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
		t.Fatal("SendRequest should not be called for invalid body")
		return nil, nil
	}}, &mockCardService{})

	req := httptest.NewRequest(http.MethodPost, "/api/friends/requests", bytes.NewBufferString("{"))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.SendRequest(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid request body")
}

func TestFriendHandler_SendRequest_Self(t *testing.T) {
	friendID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{SendRequestFunc: func(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
		if friendID == userID {
			return nil, services.ErrCannotFriendSelf
		}
		return &models.Friendship{}, nil
	}}, &mockCardService{})

	payload := []byte(`{"friend_id":"` + friendID.String() + `"}`)
	user := &models.User{ID: friendID}
	req := httptest.NewRequest(http.MethodPost, "/api/friends/requests", bytes.NewBuffer(payload))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr := httptest.NewRecorder()
	handler.SendRequest(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Cannot send friend request to yourself")
}

func TestFriendHandler_SendRequest_Success(t *testing.T) {
	friendID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{SendRequestFunc: func(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
		return &models.Friendship{}, nil
	}}, &mockCardService{})

	payload := []byte(`{"friend_id":"` + friendID.String() + `"}`)
	user := &models.User{ID: uuid.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/friends/requests", bytes.NewBuffer(payload))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr := httptest.NewRecorder()
	handler.SendRequest(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected created, got %d", rr.Code)
	}
}

func TestFriendHandler_SendRequest_InvalidFriendID(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{SendRequestFunc: func(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
		t.Fatal("SendRequest should not be called for invalid friend ID")
		return nil, nil
	}}, &mockCardService{})
	req := httptest.NewRequest(http.MethodPost, "/api/friends/requests", bytes.NewBufferString(`{"friend_id":"not-a-uuid"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.SendRequest(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Invalid friend ID")
}

func TestFriendHandler_SendRequest_Conflict(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{
		SendRequestFunc: func(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
			return nil, services.ErrFriendshipExists
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodPost, "/api/friends/requests", bytes.NewBufferString(`{"friend_id":"`+uuid.New().String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.SendRequest(rr, req)
	assertErrorResponse(t, rr, http.StatusConflict, "Friend request already exists")
}

func TestFriendHandler_SendRequest_Blocked(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{
		SendRequestFunc: func(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
			return nil, services.ErrUserBlocked
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodPost, "/api/friends/requests", bytes.NewBufferString(`{"friend_id":"`+uuid.New().String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.SendRequest(rr, req)
	assertErrorResponse(t, rr, http.StatusForbidden, "Cannot send friend request")
}

func TestFriendHandler_SendRequest_Error(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{
		SendRequestFunc: func(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
			return nil, errors.New("boom")
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodPost, "/api/friends/requests", bytes.NewBufferString(`{"friend_id":"`+uuid.New().String()+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.SendRequest(rr, req)
	assertErrorResponse(t, rr, http.StatusInternalServerError, "Internal server error")
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
	req := httptest.NewRequest(http.MethodPut, "/api/friends/requests/"+friendshipID.String()+"/accept", nil)
	req.SetPathValue("id", friendshipID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr := httptest.NewRecorder()
	handler.AcceptRequest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// reject
	req = httptest.NewRequest(http.MethodPut, "/api/friends/requests/"+friendshipID.String()+"/reject", nil)
	req.SetPathValue("id", friendshipID.String())
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr = httptest.NewRecorder()
	handler.RejectRequest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestFriendHandler_AcceptRequest_NotRecipient(t *testing.T) {
	friendshipID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{
		AcceptRequestFunc: func(ctx context.Context, userID, id uuid.UUID) (*models.Friendship, error) {
			return nil, services.ErrNotFriendshipRecipient
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodPut, "/api/friends/requests/"+friendshipID.String()+"/accept", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.AcceptRequest(rr, req)
	assertErrorResponse(t, rr, http.StatusForbidden, "Only the recipient can accept this request")
}

func TestFriendHandler_RejectRequest_NotRecipient(t *testing.T) {
	friendshipID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{
		RejectRequestFunc: func(ctx context.Context, userID, id uuid.UUID) error {
			return services.ErrNotFriendshipRecipient
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodPut, "/api/friends/requests/"+friendshipID.String()+"/reject", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.RejectRequest(rr, req)
	assertErrorResponse(t, rr, http.StatusForbidden, "Only the recipient can reject this request")
}

func TestFriendHandler_RemoveAndCancel(t *testing.T) {
	friendshipID := uuid.New()
	user := &models.User{ID: uuid.New()}

	handler := NewFriendHandler(&mockFriendService{
		RemoveFriendFunc: func(ctx context.Context, userID, id uuid.UUID) error {
			if userID != user.ID {
				t.Fatalf("unexpected user id: %s", userID)
			}
			if id != friendshipID {
				t.Fatalf("unexpected friendship id: %s", id)
			}
			return nil
		},
		CancelRequestFunc: func(ctx context.Context, userID, id uuid.UUID) error {
			if userID != user.ID {
				t.Fatalf("unexpected user id: %s", userID)
			}
			if id != friendshipID {
				t.Fatalf("unexpected friendship id: %s", id)
			}
			return nil
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/friends/"+friendshipID.String(), nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr := httptest.NewRecorder()
	handler.Remove(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/friends/requests/"+friendshipID.String()+"/cancel", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr = httptest.NewRecorder()
	handler.CancelRequest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestFriendHandler_Remove_Unauthenticated(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{}, &mockCardService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/friends/123", nil)
	rr := httptest.NewRecorder()
	handler.Remove(rr, req)
	assertErrorResponse(t, rr, http.StatusUnauthorized, "Authentication required")
}

func TestFriendHandler_Remove_InvalidID(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{}, &mockCardService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/friends/invalid", nil)
	req.SetPathValue("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Remove(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestFriendHandler_Remove_NotFound(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{
		RemoveFriendFunc: func(ctx context.Context, userID, id uuid.UUID) error {
			return services.ErrFriendshipNotFound
		},
	}, &mockCardService{})

	friendshipID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/api/friends/"+friendshipID, nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Remove(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestFriendHandler_Remove_Error(t *testing.T) {
	handler := NewFriendHandler(&mockFriendService{
		RemoveFriendFunc: func(ctx context.Context, userID, id uuid.UUID) error {
			return errors.New("boom")
		},
	}, &mockCardService{})

	friendshipID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/api/friends/"+friendshipID, nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()
	handler.Remove(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestFriendHandler_List_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	handler := NewFriendHandler(&mockFriendService{
		ListFriendsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
			return []models.FriendWithUser{}, nil
		},
		ListPendingRequestsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendRequest, error) {
			return []models.FriendRequest{}, nil
		},
		ListSentRequestsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
			return []models.FriendWithUser{}, nil
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodGet, "/api/friends", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestFriendHandler_List_DoesNotExposeEmails(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	friendshipID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{
		ListFriendsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
			return []models.FriendWithUser{
				{Friendship: models.Friendship{ID: friendshipID, UserID: userID, FriendID: uuid.New()}, FriendUsername: "friend"},
			}, nil
		},
		ListPendingRequestsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendRequest, error) {
			return []models.FriendRequest{
				{Friendship: models.Friendship{ID: uuid.New(), UserID: uuid.New(), FriendID: userID}, RequesterUsername: "requester"},
			}, nil
		},
		ListSentRequestsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
			return []models.FriendWithUser{
				{Friendship: models.Friendship{ID: uuid.New(), UserID: userID, FriendID: uuid.New()}, FriendUsername: "sentfriend"},
			}, nil
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodGet, "/api/friends", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "email") {
		t.Fatalf("expected response to omit emails, got: %s", rr.Body.String())
	}
}

func TestFriendHandler_GetFriendCardAndCards(t *testing.T) {
	currentUser := &models.User{ID: uuid.New()}
	friendshipID := uuid.New()
	friendUserID := uuid.New()

	mockFriend := &mockFriendService{
		GetFriendUserIDFunc: func(ctx context.Context, currentUserID, friendshipID uuid.UUID) (uuid.UUID, error) {
			return friendUserID, nil
		},
		ListFriendsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
			return []models.FriendWithUser{
				{Friendship: models.Friendship{UserID: currentUser.ID, FriendID: friendUserID}, FriendUsername: "friend1"},
			}, nil
		},
	}

	visibleOld := &models.BingoCard{ID: uuid.New(), UserID: friendUserID, Year: 2023, IsFinalized: true, VisibleToFriends: true}
	visibleNew := &models.BingoCard{ID: uuid.New(), UserID: friendUserID, Year: 2024, IsFinalized: true, VisibleToFriends: true}
	hidden := &models.BingoCard{ID: uuid.New(), UserID: friendUserID, Year: 2025, IsFinalized: true, VisibleToFriends: false}

	mockCard := &mockCardService{
		ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return []*models.BingoCard{visibleOld, hidden, visibleNew}, nil
		},
	}

	handler := NewFriendHandler(mockFriend, mockCard)

	req := httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/card", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, currentUser))
	rr := httptest.NewRecorder()
	handler.GetFriendCard(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/cards", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, currentUser))
	rr = httptest.NewRecorder()
	handler.GetFriendCards(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestFriendHandler_GetFriendCards_FriendshipNotFound(t *testing.T) {
	friendshipID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{
		GetFriendUserIDFunc: func(ctx context.Context, userID, friendshipID uuid.UUID) (uuid.UUID, error) {
			return uuid.Nil, services.ErrFriendshipNotFound
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/cards", nil)
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.GetFriendCards(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestFriendHandler_GetFriendCards_NotFriend(t *testing.T) {
	friendshipID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{
		GetFriendUserIDFunc: func(ctx context.Context, userID, friendshipID uuid.UUID) (uuid.UUID, error) {
			return uuid.Nil, services.ErrNotFriend
		},
	}, &mockCardService{})

	req := httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/cards", nil)
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.GetFriendCards(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestFriendHandler_GetFriendCards_ListCardsError(t *testing.T) {
	friendshipID := uuid.New()
	friendID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{
		GetFriendUserIDFunc: func(ctx context.Context, userID, friendshipID uuid.UUID) (uuid.UUID, error) {
			return friendID, nil
		},
	}, &mockCardService{
		ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return nil, errors.New("boom")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/cards", nil)
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.GetFriendCards(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestFriendHandler_GetFriendCards_ListFriendsError(t *testing.T) {
	friendshipID := uuid.New()
	friendID := uuid.New()
	handler := NewFriendHandler(&mockFriendService{
		GetFriendUserIDFunc: func(ctx context.Context, userID, friendshipID uuid.UUID) (uuid.UUID, error) {
			return friendID, nil
		},
		ListFriendsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
			return nil, errors.New("boom")
		},
	}, &mockCardService{
		ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
			return []*models.BingoCard{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/cards", nil)
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.GetFriendCards(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestFriendHandler_ErrorMappings(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	friendshipID := uuid.New()

	t.Run("accept mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"not found", services.ErrFriendshipNotFound, http.StatusNotFound},
			{"not recipient", services.ErrNotFriendshipRecipient, http.StatusForbidden},
			{"not pending", services.ErrFriendshipNotPending, http.StatusBadRequest},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				handler := NewFriendHandler(&mockFriendService{
					AcceptRequestFunc: func(ctx context.Context, userID, id uuid.UUID) (*models.Friendship, error) {
						return nil, tt.serviceErr
					},
				}, &mockCardService{})

				req := httptest.NewRequest(http.MethodPut, "/api/friends/requests/"+friendshipID.String()+"/accept", nil)
				req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
				rr := httptest.NewRecorder()
				handler.AcceptRequest(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})

	t.Run("reject mappings", func(t *testing.T) {
		tests := []struct {
			name       string
			serviceErr error
			wantStatus int
		}{
			{"not found", services.ErrFriendshipNotFound, http.StatusNotFound},
			{"not recipient", services.ErrNotFriendshipRecipient, http.StatusForbidden},
			{"not pending", services.ErrFriendshipNotPending, http.StatusBadRequest},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				handler := NewFriendHandler(&mockFriendService{
					RejectRequestFunc: func(ctx context.Context, userID, id uuid.UUID) error {
						return tt.serviceErr
					},
				}, &mockCardService{})

				req := httptest.NewRequest(http.MethodPut, "/api/friends/requests/"+friendshipID.String()+"/reject", nil)
				req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
				rr := httptest.NewRecorder()
				handler.RejectRequest(rr, req)
				if rr.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rr.Code)
				}
			})
		}
	})

	t.Run("remove not found", func(t *testing.T) {
		handler := NewFriendHandler(&mockFriendService{
			RemoveFriendFunc: func(ctx context.Context, userID, id uuid.UUID) error {
				return services.ErrFriendshipNotFound
			},
		}, &mockCardService{})

		req := httptest.NewRequest(http.MethodDelete, "/api/friends/"+friendshipID.String(), nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.Remove(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("cancel not pending", func(t *testing.T) {
		handler := NewFriendHandler(&mockFriendService{
			CancelRequestFunc: func(ctx context.Context, userID, id uuid.UUID) error {
				return services.ErrFriendshipNotPending
			},
		}, &mockCardService{})

		req := httptest.NewRequest(http.MethodDelete, "/api/friends/requests/"+friendshipID.String()+"/cancel", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.CancelRequest(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("cancel not found", func(t *testing.T) {
		handler := NewFriendHandler(&mockFriendService{
			CancelRequestFunc: func(ctx context.Context, userID, id uuid.UUID) error {
				return services.ErrFriendshipNotFound
			},
		}, &mockCardService{})

		req := httptest.NewRequest(http.MethodDelete, "/api/friends/requests/"+friendshipID.String()+"/cancel", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.CancelRequest(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("cancel internal error", func(t *testing.T) {
		handler := NewFriendHandler(&mockFriendService{
			CancelRequestFunc: func(ctx context.Context, userID, id uuid.UUID) error {
				return errors.New("boom")
			},
		}, &mockCardService{})

		req := httptest.NewRequest(http.MethodDelete, "/api/friends/requests/"+friendshipID.String()+"/cancel", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.CancelRequest(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})

	t.Run("list friends error", func(t *testing.T) {
		handler := NewFriendHandler(&mockFriendService{
			ListFriendsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
				return nil, errors.New("boom")
			},
		}, &mockCardService{})

		req := httptest.NewRequest(http.MethodGet, "/api/friends", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.List(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})

	t.Run("list pending error", func(t *testing.T) {
		handler := NewFriendHandler(&mockFriendService{
			ListFriendsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
				return []models.FriendWithUser{}, nil
			},
			ListPendingRequestsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendRequest, error) {
				return nil, errors.New("boom")
			},
		}, &mockCardService{})

		req := httptest.NewRequest(http.MethodGet, "/api/friends", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.List(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})

	t.Run("list sent error", func(t *testing.T) {
		handler := NewFriendHandler(&mockFriendService{
			ListFriendsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
				return []models.FriendWithUser{}, nil
			},
			ListPendingRequestsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendRequest, error) {
				return []models.FriendRequest{}, nil
			},
			ListSentRequestsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
				return nil, errors.New("boom")
			},
		}, &mockCardService{})

		req := httptest.NewRequest(http.MethodGet, "/api/friends", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.List(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})

	t.Run("get friend card no visible cards", func(t *testing.T) {
		friendUserID := uuid.New()
		handler := NewFriendHandler(&mockFriendService{
			GetFriendUserIDFunc: func(ctx context.Context, currentUserID, friendshipID uuid.UUID) (uuid.UUID, error) {
				return friendUserID, nil
			},
			ListFriendsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
				return []models.FriendWithUser{}, nil
			},
		}, &mockCardService{
			ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
				return []*models.BingoCard{{ID: uuid.New(), UserID: friendUserID, IsFinalized: false, VisibleToFriends: true}}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/card", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.GetFriendCard(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("get friend card not friend", func(t *testing.T) {
		handler := NewFriendHandler(&mockFriendService{
			GetFriendUserIDFunc: func(ctx context.Context, currentUserID, friendshipID uuid.UUID) (uuid.UUID, error) {
				return uuid.Nil, services.ErrNotFriend
			},
		}, &mockCardService{})

		req := httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/card", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.GetFriendCard(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("get friend card list cards error", func(t *testing.T) {
		friendUserID := uuid.New()
		handler := NewFriendHandler(&mockFriendService{
			GetFriendUserIDFunc: func(ctx context.Context, currentUserID, friendshipID uuid.UUID) (uuid.UUID, error) {
				return friendUserID, nil
			},
		}, &mockCardService{
			ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
				return nil, errors.New("boom")
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/card", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.GetFriendCard(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})

	t.Run("get friend card list friends error", func(t *testing.T) {
		friendUserID := uuid.New()
		handler := NewFriendHandler(&mockFriendService{
			GetFriendUserIDFunc: func(ctx context.Context, currentUserID, friendshipID uuid.UUID) (uuid.UUID, error) {
				return friendUserID, nil
			},
			ListFriendsFunc: func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
				return nil, errors.New("boom")
			},
		}, &mockCardService{
			ListByUserFunc: func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
				return []*models.BingoCard{{ID: uuid.New(), UserID: friendUserID, IsFinalized: true, VisibleToFriends: true}}, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/friends/"+friendshipID.String()+"/card", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		rr := httptest.NewRecorder()
		handler.GetFriendCard(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})
}
