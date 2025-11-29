package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type FriendHandler struct {
	friendService *services.FriendService
	cardService   *services.CardService
}

func NewFriendHandler(friendService *services.FriendService, cardService *services.CardService) *FriendHandler {
	return &FriendHandler{
		friendService: friendService,
		cardService:   cardService,
	}
}

type SendRequestRequest struct {
	FriendID string `json:"friend_id"`
}

type FriendListResponse struct {
	Friends  []models.FriendWithUser `json:"friends,omitempty"`
	Requests []models.FriendRequest  `json:"requests,omitempty"`
	Sent     []models.FriendWithUser `json:"sent,omitempty"`
	Message  string                  `json:"message,omitempty"`
}

type UserSearchResponse struct {
	Users []models.UserSearchResult `json:"users"`
}

type FriendCardResponse struct {
	Card    *models.BingoCard `json:"card,omitempty"`
	Owner   *FriendOwner      `json:"owner,omitempty"`
	Message string            `json:"message,omitempty"`
}

type FriendCardsResponse struct {
	Cards []*models.BingoCard `json:"cards,omitempty"`
	Owner *FriendOwner        `json:"owner,omitempty"`
}

type FriendOwner struct {
	Username string `json:"username"`
}

func (h *FriendHandler) Search(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	query := r.URL.Query().Get("q")
	if len(strings.TrimSpace(query)) < 2 {
		writeJSON(w, http.StatusOK, UserSearchResponse{Users: []models.UserSearchResult{}})
		return
	}

	users, err := h.friendService.SearchUsers(r.Context(), user.ID, query)
	if err != nil {
		log.Printf("Error searching users: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, UserSearchResponse{Users: users})
}

func (h *FriendHandler) SendRequest(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req SendRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	friendID, err := uuid.Parse(req.FriendID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid friend ID")
		return
	}

	_, err = h.friendService.SendRequest(r.Context(), user.ID, friendID)
	if errors.Is(err, services.ErrCannotFriendSelf) {
		writeError(w, http.StatusBadRequest, "Cannot send friend request to yourself")
		return
	}
	if errors.Is(err, services.ErrFriendshipExists) {
		writeError(w, http.StatusConflict, "Friend request already exists")
		return
	}
	if err != nil {
		log.Printf("Error sending friend request: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, FriendListResponse{Message: "Friend request sent"})
}

func (h *FriendHandler) AcceptRequest(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	friendshipID, err := parseFriendshipID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid friendship ID")
		return
	}

	_, err = h.friendService.AcceptRequest(r.Context(), user.ID, friendshipID)
	if errors.Is(err, services.ErrFriendshipNotFound) {
		writeError(w, http.StatusNotFound, "Friend request not found")
		return
	}
	if errors.Is(err, services.ErrNotFriendshipRecipient) {
		writeError(w, http.StatusForbidden, "Only the recipient can accept this request")
		return
	}
	if errors.Is(err, services.ErrFriendshipNotPending) {
		writeError(w, http.StatusBadRequest, "Request is not pending")
		return
	}
	if err != nil {
		log.Printf("Error accepting friend request: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, FriendListResponse{Message: "Friend request accepted"})
}

func (h *FriendHandler) RejectRequest(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	friendshipID, err := parseFriendshipID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid friendship ID")
		return
	}

	err = h.friendService.RejectRequest(r.Context(), user.ID, friendshipID)
	if errors.Is(err, services.ErrFriendshipNotFound) {
		writeError(w, http.StatusNotFound, "Friend request not found")
		return
	}
	if errors.Is(err, services.ErrNotFriendshipRecipient) {
		writeError(w, http.StatusForbidden, "Only the recipient can reject this request")
		return
	}
	if errors.Is(err, services.ErrFriendshipNotPending) {
		writeError(w, http.StatusBadRequest, "Request is not pending")
		return
	}
	if err != nil {
		log.Printf("Error rejecting friend request: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, FriendListResponse{Message: "Friend request rejected"})
}

func (h *FriendHandler) Remove(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	friendshipID, err := parseFriendshipID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid friendship ID")
		return
	}

	err = h.friendService.RemoveFriend(r.Context(), user.ID, friendshipID)
	if errors.Is(err, services.ErrFriendshipNotFound) {
		writeError(w, http.StatusNotFound, "Friendship not found")
		return
	}
	if err != nil {
		log.Printf("Error removing friend: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, FriendListResponse{Message: "Friend removed"})
}

func (h *FriendHandler) CancelRequest(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	friendshipID, err := parseFriendshipID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid friendship ID")
		return
	}

	err = h.friendService.CancelRequest(r.Context(), user.ID, friendshipID)
	if errors.Is(err, services.ErrFriendshipNotFound) {
		writeError(w, http.StatusNotFound, "Friend request not found")
		return
	}
	if errors.Is(err, services.ErrFriendshipNotPending) {
		writeError(w, http.StatusBadRequest, "Request is not pending")
		return
	}
	if err != nil {
		log.Printf("Error canceling friend request: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, FriendListResponse{Message: "Friend request canceled"})
}

func (h *FriendHandler) List(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	friends, err := h.friendService.ListFriends(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error listing friends: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	requests, err := h.friendService.ListPendingRequests(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error listing pending requests: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	sent, err := h.friendService.ListSentRequests(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error listing sent requests: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, FriendListResponse{
		Friends:  friends,
		Requests: requests,
		Sent:     sent,
	})
}

func (h *FriendHandler) GetFriendCard(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	friendshipID, err := parseFriendshipID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid friendship ID")
		return
	}

	// Get the friend's user ID
	friendUserID, err := h.friendService.GetFriendUserID(r.Context(), user.ID, friendshipID)
	if errors.Is(err, services.ErrFriendshipNotFound) {
		writeError(w, http.StatusNotFound, "Friendship not found")
		return
	}
	if errors.Is(err, services.ErrNotFriend) {
		writeError(w, http.StatusForbidden, "You are not friends with this user")
		return
	}
	if err != nil {
		log.Printf("Error getting friend user ID: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Get the friend's cards
	cards, err := h.cardService.ListByUser(r.Context(), friendUserID)
	if err != nil {
		log.Printf("Error listing friend cards: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Find the active/current year finalized card that is visible to friends
	var activeCard *models.BingoCard
	for _, card := range cards {
		if card.IsFinalized && card.VisibleToFriends {
			if activeCard == nil || card.Year > activeCard.Year {
				activeCard = card
			}
		}
	}

	if activeCard == nil {
		writeJSON(w, http.StatusOK, FriendCardResponse{Message: "Friend has no visible cards"})
		return
	}

	// Get friend's username from the friendship list
	friends, err := h.friendService.ListFriends(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error getting friends list: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	var ownerName string
	for _, f := range friends {
		friendID := f.FriendID
		if f.UserID != user.ID {
			friendID = f.UserID
		}
		if friendID == friendUserID {
			ownerName = f.FriendUsername
			break
		}
	}

	writeJSON(w, http.StatusOK, FriendCardResponse{
		Card:  activeCard,
		Owner: &FriendOwner{Username: ownerName},
	})
}

func (h *FriendHandler) GetFriendCards(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	friendshipID, err := parseFriendshipID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid friendship ID")
		return
	}

	// Get the friend's user ID
	friendUserID, err := h.friendService.GetFriendUserID(r.Context(), user.ID, friendshipID)
	if errors.Is(err, services.ErrFriendshipNotFound) {
		writeError(w, http.StatusNotFound, "Friendship not found")
		return
	}
	if errors.Is(err, services.ErrNotFriend) {
		writeError(w, http.StatusForbidden, "You are not friends with this user")
		return
	}
	if err != nil {
		log.Printf("Error getting friend user ID: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Get all friend's cards
	cards, err := h.cardService.ListByUser(r.Context(), friendUserID)
	if err != nil {
		log.Printf("Error listing friend cards: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Filter to only finalized cards that are visible to friends
	var finalizedCards []*models.BingoCard
	for _, card := range cards {
		if card.IsFinalized && card.VisibleToFriends {
			finalizedCards = append(finalizedCards, card)
		}
	}

	// Get friend's username
	friends, err := h.friendService.ListFriends(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error getting friends list: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	var ownerName string
	for _, f := range friends {
		friendID := f.FriendID
		if f.UserID != user.ID {
			friendID = f.UserID
		}
		if friendID == friendUserID {
			ownerName = f.FriendUsername
			break
		}
	}

	writeJSON(w, http.StatusOK, FriendCardsResponse{
		Cards: finalizedCards,
		Owner: &FriendOwner{Username: ownerName},
	})
}

func parseFriendshipID(r *http.Request) (uuid.UUID, error) {
	path := r.URL.Path
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "friends" && i+1 < len(parts) {
			return uuid.Parse(parts[i+1])
		}
	}
	return uuid.Nil, errors.New("friendship ID not found in path")
}
