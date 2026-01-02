package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type NotificationHandler struct {
	notificationService services.NotificationServiceInterface
}

func NewNotificationHandler(notificationService services.NotificationServiceInterface) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

type NotificationListResponse struct {
	Notifications []models.Notification `json:"notifications"`
}

type NotificationSettingsResponse struct {
	Settings *models.NotificationSettings `json:"settings"`
}

type NotificationUnreadCountResponse struct {
	Count int `json:"count"`
}

type NotificationMessageResponse struct {
	Message string `json:"message,omitempty"`
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	limit := 50
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		parsed, err := strconv.Atoi(limitParam)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "Invalid limit")
			return
		}
		limit = parsed
	}

	var before *time.Time
	if beforeParam := r.URL.Query().Get("before"); beforeParam != "" {
		parsed, err := time.Parse(time.RFC3339, beforeParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid before timestamp")
			return
		}
		before = &parsed
	}

	unreadOnly := r.URL.Query().Get("unread") == "1"

	notifications, err := h.notificationService.List(r.Context(), user.ID, services.NotificationListParams{
		Limit:      limit,
		Before:     before,
		UnreadOnly: unreadOnly,
	})
	if err != nil {
		log.Printf("Error listing notifications: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, NotificationListResponse{Notifications: notifications})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	notificationIDStr := r.PathValue("id")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	err = h.notificationService.MarkRead(r.Context(), user.ID, notificationID)
	if errors.Is(err, services.ErrNotificationNotFound) {
		writeError(w, http.StatusNotFound, "Notification not found")
		return
	}
	if err != nil {
		log.Printf("Error marking notification read: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, NotificationMessageResponse{Message: "Notification marked as read"})
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	if err := h.notificationService.MarkAllRead(r.Context(), user.ID); err != nil {
		log.Printf("Error marking all notifications read: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, NotificationMessageResponse{Message: "Notifications marked as read"})
}

func (h *NotificationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	notificationIDStr := r.PathValue("id")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	err = h.notificationService.Delete(r.Context(), user.ID, notificationID)
	if errors.Is(err, services.ErrNotificationNotFound) {
		writeError(w, http.StatusNotFound, "Notification not found")
		return
	}
	if err != nil {
		log.Printf("Error deleting notification: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, NotificationMessageResponse{Message: "Notification deleted"})
}

func (h *NotificationHandler) DeleteAll(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	if err := h.notificationService.DeleteAll(r.Context(), user.ID); err != nil {
		log.Printf("Error deleting notifications: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, NotificationMessageResponse{Message: "Notifications deleted"})
}

func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	count, err := h.notificationService.UnreadCount(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error counting notifications: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, NotificationUnreadCountResponse{Count: count})
}

func (h *NotificationHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	settings, err := h.notificationService.GetSettings(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error getting notification settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, NotificationSettingsResponse{Settings: settings})
}

func (h *NotificationHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var patch models.NotificationSettingsPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	settings, err := h.notificationService.UpdateSettings(r.Context(), user.ID, patch)
	if errors.Is(err, services.ErrEmailNotVerified) {
		writeError(w, http.StatusForbidden, "Verify your email to enable email notifications")
		return
	}
	if err != nil {
		log.Printf("Error updating notification settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, NotificationSettingsResponse{Settings: settings})
}
