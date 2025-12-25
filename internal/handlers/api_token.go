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

type ApiTokenHandler struct {
	apiTokenService services.ApiTokenServiceInterface
}

func NewApiTokenHandler(apiTokenService services.ApiTokenServiceInterface) *ApiTokenHandler {
	return &ApiTokenHandler{apiTokenService: apiTokenService}
}

type CreateApiTokenRequest struct {
	Name          string               `json:"name"`
	Scope         models.ApiTokenScope `json:"scope"`
	ExpiresInDays int                  `json:"expires_in_days"`
}

type CreateApiTokenResponse struct {
	Token    *models.ApiToken `json:"token_metadata"`
	RawToken string           `json:"token"` // The actual token, only shown once
	Warning  string           `json:"warning"`
}

type ListApiTokensResponse struct {
	Tokens []models.ApiToken `json:"tokens"`
}

func (h *ApiTokenHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req CreateApiTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Validate scope
	if req.Scope != models.ScopeRead && req.Scope != models.ScopeWrite && req.Scope != models.ScopeReadWrite {
		writeError(w, http.StatusBadRequest, "Invalid scope")
		return
	}

	token, rawToken, err := h.apiTokenService.Create(r.Context(), user.ID, req.Name, req.Scope, req.ExpiresInDays)
	if err != nil {
		log.Printf("Error creating api token: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, CreateApiTokenResponse{
		Token:    token,
		RawToken: rawToken,
		Warning:  "Save this token now. You won't be able to see it again.",
	})
}

func (h *ApiTokenHandler) List(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	tokens, err := h.apiTokenService.List(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error listing api tokens: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if tokens == nil {
		tokens = []models.ApiToken{}
	}

	writeJSON(w, http.StatusOK, ListApiTokensResponse{Tokens: tokens})
}

func (h *ApiTokenHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Extract token ID from path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	// /api/tokens/{id}
	if len(parts) < 4 {
		writeError(w, http.StatusBadRequest, "Invalid token ID")
		return
	}
	tokenID, err := uuid.Parse(parts[3])
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid token ID")
		return
	}

	err = h.apiTokenService.Delete(r.Context(), user.ID, tokenID)
	if errors.Is(err, services.ErrTokenNotFound) {
		writeError(w, http.StatusNotFound, "Token not found")
		return
	}
	if err != nil {
		log.Printf("Error deleting api token: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *ApiTokenHandler) DeleteAll(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	err := h.apiTokenService.DeleteAll(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error deleting all api tokens: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.WriteHeader(http.StatusOK)
}
