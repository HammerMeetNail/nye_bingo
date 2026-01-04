package handlers

import (
	"errors"
	"html"
	"net/http"
	"strings"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type ReminderPublicHandler struct {
	reminderService services.ReminderServiceInterface
}

func NewReminderPublicHandler(reminderService services.ReminderServiceInterface) *ReminderPublicHandler {
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

func (h *ReminderPublicHandler) UnsubscribeConfirm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "Missing token")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	escaped := html.EscapeString(token)
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Unsubscribe</title>
  <link rel="stylesheet" href="/static/css/styles.css">
</head>
<body>
  <main class="container main-content">
    <div class="card">
      <h2>Unsubscribe</h2>
      <p>Disable reminder emails for your account?</p>
      <form method="POST" action="/r/unsubscribe">
        <input type="hidden" name="token" value="` + escaped + `">
        <div class="profile-actions">
          <button type="submit" class="btn btn-danger-outline">Disable reminders</button>
          <a class="btn btn-ghost" href="/#home">Cancel</a>
        </div>
      </form>
    </div>
  </main>
</body>
</html>`))
}

func (h *ReminderPublicHandler) UnsubscribeSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid form")
		return
	}
	token := r.Form.Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "Missing token")
		return
	}

	alreadyDisabled, err := h.reminderService.UnsubscribeByToken(r.Context(), token)
	if errors.Is(err, services.ErrReminderNotFound) {
		writeError(w, http.StatusNotFound, "Unsubscribe link expired")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	status := "Reminders disabled"
	if alreadyDisabled {
		status = "Reminders were already disabled"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Unsubscribed</title>
  <link rel="stylesheet" href="/static/css/styles.css">
</head>
<body>
  <main class="container main-content">
    <div class="card">
      <h2>Unsubscribed</h2>
      <p>` + html.EscapeString(status) + `</p>
      <div class="profile-actions">
        <a class="btn btn-secondary" href="/#profile">Manage settings</a>
      </div>
    </div>
  </main>
</body>
</html>`))
}
