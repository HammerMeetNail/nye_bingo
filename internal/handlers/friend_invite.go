package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type FriendInviteHandler struct {
	inviteService services.FriendInviteServiceInterface
}

func NewFriendInviteHandler(inviteService services.FriendInviteServiceInterface) *FriendInviteHandler {
	return &FriendInviteHandler{inviteService: inviteService}
}

type CreateInviteRequest struct {
	ExpiresInDays int `json:"expires_in_days"`
}

type AcceptInviteRequest struct {
	Token string `json:"token"`
}

type InviteResponse struct {
	Invite  *models.FriendInvite `json:"invite,omitempty"`
	URL     string               `json:"url,omitempty"`
	Message string               `json:"message,omitempty"`
}

type InviteListResponse struct {
	Invites []models.FriendInvite `json:"invites"`
}

type InviteAcceptResponse struct {
	Inviter models.UserSearchResult `json:"inviter"`
	Message string                  `json:"message,omitempty"`
}

func (h *FriendInviteHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req CreateInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	expiresInDays := req.ExpiresInDays
	if expiresInDays == 0 {
		expiresInDays = services.InviteExpiryDefaultDays
	}
	if expiresInDays < services.InviteExpiryMinDays || expiresInDays > services.InviteExpiryMaxDays {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("expires_in_days must be between %d and %d", services.InviteExpiryMinDays, services.InviteExpiryMaxDays))
		return
	}

	invite, token, err := h.inviteService.CreateInvite(r.Context(), user.ID, expiresInDays)
	if errors.Is(err, services.ErrInviteExpiryOutOfRange) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("expires_in_days must be between %d and %d", services.InviteExpiryMinDays, services.InviteExpiryMaxDays))
		return
	}
	if errors.Is(err, services.ErrInviteLimitReached) {
		writeError(w, http.StatusConflict, fmt.Sprintf("Invite limit reached (max %d active)", services.InviteMaxActive))
		return
	}
	if err != nil {
		log.Printf("Error creating invite: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	url := "#friend-invite/" + token
	writeJSON(w, http.StatusCreated, InviteResponse{Invite: invite, URL: url})
}

func (h *FriendInviteHandler) List(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	invites, err := h.inviteService.ListInvites(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error listing invites: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, InviteListResponse{Invites: invites})
}

func (h *FriendInviteHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	inviteIDStr := r.PathValue("id")
	inviteID, err := uuid.Parse(inviteIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid invite ID")
		return
	}

	err = h.inviteService.RevokeInvite(r.Context(), user.ID, inviteID)
	if errors.Is(err, services.ErrInviteNotFound) {
		writeError(w, http.StatusNotFound, "Invite not found")
		return
	}
	if err != nil {
		log.Printf("Error revoking invite: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, InviteResponse{Message: "Invite revoked"})
}

func (h *FriendInviteHandler) Accept(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req AcceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	inviter, err := h.inviteService.AcceptInvite(r.Context(), user.ID, req.Token)
	if errors.Is(err, services.ErrInviteNotFound) {
		writeError(w, http.StatusNotFound, "Invite not found or expired")
		return
	}
	if errors.Is(err, services.ErrCannotFriendSelf) {
		writeError(w, http.StatusBadRequest, "Cannot accept your own invite")
		return
	}
	if errors.Is(err, services.ErrUserBlocked) {
		writeError(w, http.StatusForbidden, "Cannot accept invite")
		return
	}
	if errors.Is(err, services.ErrFriendshipExists) {
		writeError(w, http.StatusConflict, "Already friends")
		return
	}
	if err != nil {
		log.Printf("Error accepting invite: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, InviteAcceptResponse{Inviter: *inviter, Message: "Invite accepted"})
}
