package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type AccountHandler struct {
	accountService services.AccountServiceInterface
	authService    services.AuthServiceInterface
	secure         bool
}

func NewAccountHandler(accountService services.AccountServiceInterface, authService services.AuthServiceInterface, secure bool) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
		authService:    authService,
		secure:         secure,
	}
}

type AccountDeleteRequest struct {
	ConfirmUsername string `json:"confirm_username"`
	Password        string `json:"password"`
	Confirm         bool   `json:"confirm"`
}

type AccountMessageResponse struct {
	Message string `json:"message"`
}

func (h *AccountHandler) Export(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	data, err := h.accountService.BuildExportZip(r.Context(), user.ID)
	if errors.Is(err, services.ErrUserNotFound) {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if err != nil {
		log.Printf("Error building account export: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	filename := "yearofbingo_account_export_" + time.Now().UTC().Format("2006-01-02") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		log.Printf("Error writing account export: %v", err)
	}
}

func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req AccountDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	confirmUsername := strings.TrimSpace(req.ConfirmUsername)
	if confirmUsername == "" || req.Password == "" || !req.Confirm {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if confirmUsername != user.Username {
		writeError(w, http.StatusBadRequest, "Username confirmation does not match")
		return
	}
	if !h.authService.VerifyPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "Invalid password")
		return
	}

	if err := h.accountService.Delete(r.Context(), user.ID); err != nil {
		log.Printf("Error deleting account: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		_ = h.authService.DeleteSession(r.Context(), cookie.Value)
	}

	h.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, AccountMessageResponse{Message: "Account deleted"})
}

func (h *AccountHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
	})
}
