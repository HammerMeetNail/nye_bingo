package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type ShareCardRequest struct {
	ExpiresInDays *int `json:"expires_in_days,omitempty"`
}

type ShareStatusResponse struct {
	Enabled        bool       `json:"enabled"`
	Expired        bool       `json:"expired,omitempty"`
	URL            string     `json:"url,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	AccessCount    int        `json:"access_count"`
	Message        string     `json:"message,omitempty"`
}

func (h *CardHandler) CreateShare(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cardID, err := parseCardID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	var req ShareCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresInDays != nil {
		days := *req.ExpiresInDays
		if days < 0 {
			writeError(w, http.StatusBadRequest, "expires_in_days must be zero or positive")
			return
		}
		if days != 0 {
			if days < services.ShareExpiryMinDays || days > services.ShareExpiryMaxDays {
				writeError(w, http.StatusBadRequest, "expires_in_days is out of range")
				return
			}
			t := time.Now().Add(time.Duration(days) * 24 * time.Hour)
			expiresAt = &t
		}
	}

	share, err := h.cardService.CreateOrRotateShare(r.Context(), user.ID, cardID, expiresAt)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if errors.Is(err, services.ErrCardNotFinalized) {
		writeError(w, http.StatusBadRequest, "Card must be finalized first")
		return
	}
	if err != nil {
		log.Printf("Error creating share: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	url := share.Token
	writeJSON(w, http.StatusCreated, ShareStatusResponse{
		Enabled:        true,
		URL:            url,
		CreatedAt:      &share.CreatedAt,
		ExpiresAt:      share.ExpiresAt,
		LastAccessedAt: share.LastAccessedAt,
		AccessCount:    share.AccessCount,
	})
}

func (h *CardHandler) GetShareStatus(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cardID, err := parseCardID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	share, err := h.cardService.GetShareStatus(r.Context(), user.ID, cardID)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if errors.Is(err, services.ErrShareNotFound) {
		writeJSON(w, http.StatusOK, ShareStatusResponse{Enabled: false})
		return
	}
	if err != nil {
		log.Printf("Error loading share status: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	expired := share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now())
	url := share.Token
	if expired {
		url = ""
	}

	writeJSON(w, http.StatusOK, ShareStatusResponse{
		Enabled:        true,
		Expired:        expired,
		URL:            url,
		CreatedAt:      &share.CreatedAt,
		ExpiresAt:      share.ExpiresAt,
		LastAccessedAt: share.LastAccessedAt,
		AccessCount:    share.AccessCount,
	})
}

func (h *CardHandler) RevokeShare(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cardID, err := parseCardID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	err = h.cardService.RevokeShare(r.Context(), user.ID, cardID)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if err != nil {
		log.Printf("Error revoking share: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ShareStatusResponse{
		Enabled: false,
		Message: "Share revoked",
	})
}

func (h *CardHandler) GetSharedCard(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "Invalid share token")
		return
	}

	shared, err := h.cardService.GetSharedCardByToken(r.Context(), token)
	if errors.Is(err, services.ErrShareNotFound) {
		writeError(w, http.StatusNotFound, "Share link not found")
		return
	}
	if err != nil {
		log.Printf("Error loading shared card: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	writeJSON(w, http.StatusOK, shared)
}
