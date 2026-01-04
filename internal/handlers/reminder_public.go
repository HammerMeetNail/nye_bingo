package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type ReminderPublicHandler struct {
	reminderService *services.ReminderService
}

func NewReminderPublicHandler(reminderService *services.ReminderService) *ReminderPublicHandler {
	return &ReminderPublicHandler{reminderService: reminderService}
}

func (h *ReminderPublicHandler) ServeImage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "Missing token")
		return
	}
	token = strings.TrimSuffix(token, ".png")

	pngBytes, err := h.reminderService.RenderImageByToken(r.Context(), token)
	if errors.Is(err, services.ErrReminderNotFound) {
		writeError(w, http.StatusNotFound, "Image not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pngBytes)
}

func (h *ReminderPublicHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "Missing token")
		return
	}

	active, err := h.reminderService.UnsubscribeByToken(r.Context(), token)
	if errors.Is(err, services.ErrReminderNotFound) {
		writeError(w, http.StatusNotFound, "Unsubscribe link expired")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	status := "Reminders disabled"
	if active {
		status = "Reminders were already disabled"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>Unsubscribed</title></head><body style=\"font-family: sans-serif; padding: 2rem;\"><h1>Year of Bingo</h1><p>" + status + "</p></body></html>"))
}
