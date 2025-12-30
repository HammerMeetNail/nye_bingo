package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type BlockHandler struct {
	blockService services.BlockServiceInterface
}

func NewBlockHandler(blockService services.BlockServiceInterface) *BlockHandler {
	return &BlockHandler{blockService: blockService}
}

type BlockRequest struct {
	UserID string `json:"user_id"`
}

type BlockListResponse struct {
	Blocked []models.BlockedUser `json:"blocked"`
	Message string               `json:"message,omitempty"`
}

func (h *BlockHandler) Block(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req BlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	blockedID, err := uuid.Parse(req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	err = h.blockService.Block(r.Context(), user.ID, blockedID)
	if errors.Is(err, services.ErrCannotBlockSelf) {
		writeError(w, http.StatusBadRequest, "Cannot block yourself")
		return
	}
	if errors.Is(err, services.ErrBlockedUserNotFound) {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}
	if errors.Is(err, services.ErrBlockExists) {
		writeError(w, http.StatusConflict, "User already blocked")
		return
	}
	if err != nil {
		log.Printf("Error blocking user: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, BlockListResponse{Message: "User blocked"})
}

func (h *BlockHandler) Unblock(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	blockedIDStr := r.PathValue("id")
	blockedID, err := uuid.Parse(blockedIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	err = h.blockService.Unblock(r.Context(), user.ID, blockedID)
	if errors.Is(err, services.ErrBlockNotFound) {
		writeError(w, http.StatusNotFound, "Block not found")
		return
	}
	if err != nil {
		log.Printf("Error unblocking user: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, BlockListResponse{Message: "User unblocked"})
}

func (h *BlockHandler) List(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	blocked, err := h.blockService.ListBlocked(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error listing blocked users: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, BlockListResponse{Blocked: blocked})
}
