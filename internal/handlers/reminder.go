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

type ReminderHandler struct {
	reminderService services.ReminderServiceInterface
}

func NewReminderHandler(reminderService services.ReminderServiceInterface) *ReminderHandler {
	return &ReminderHandler{reminderService: reminderService}
}

type ReminderSettingsResponse struct {
	Settings *models.ReminderSettings `json:"settings"`
}

type ReminderCardListResponse struct {
	Cards []models.CardCheckinSummary `json:"cards"`
}

type ReminderCheckinResponse struct {
	Checkin *models.CardCheckinReminder `json:"checkin"`
}

type ReminderGoalListResponse struct {
	Reminders []models.GoalReminderSummary `json:"reminders"`
}

type ReminderGoalResponse struct {
	Reminder *models.GoalReminder `json:"reminder"`
}

type ReminderMessageResponse struct {
	Message string `json:"message,omitempty"`
}

type ReminderTestRequest struct {
	CardID uuid.UUID `json:"card_id"`
}

func (h *ReminderHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	settings, err := h.reminderService.GetSettings(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error getting reminder settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReminderSettingsResponse{Settings: settings})
}

func (h *ReminderHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var patch models.ReminderSettingsPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	settings, err := h.reminderService.UpdateSettings(r.Context(), user.ID, patch)
	if errors.Is(err, services.ErrEmailNotVerified) {
		writeError(w, http.StatusForbidden, "Verify your email to enable reminder emails")
		return
	}
	if err != nil {
		log.Printf("Error updating reminder settings: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReminderSettingsResponse{Settings: settings})
}

func (h *ReminderHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cards, err := h.reminderService.ListCardCheckins(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error listing card reminders: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReminderCardListResponse{Cards: cards})
}

func (h *ReminderHandler) UpsertCardCheckin(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cardIDStr := r.PathValue("cardId")
	cardID, err := uuid.Parse(cardIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	var input models.CardCheckinScheduleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	checkin, err := h.reminderService.UpsertCardCheckin(r.Context(), user.ID, cardID, input)
	if errors.Is(err, services.ErrInvalidSchedule) {
		writeError(w, http.StatusBadRequest, "Invalid reminder schedule")
		return
	}
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrCardNotEligible) {
		writeError(w, http.StatusBadRequest, "Card must be finalized and not archived")
		return
	}
	if err != nil {
		log.Printf("Error updating card reminder: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReminderCheckinResponse{Checkin: checkin})
}

func (h *ReminderHandler) DeleteCardCheckin(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cardIDStr := r.PathValue("cardId")
	cardID, err := uuid.Parse(cardIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	if err := h.reminderService.DeleteCardCheckin(r.Context(), user.ID, cardID); errors.Is(err, services.ErrReminderNotFound) {
		writeError(w, http.StatusNotFound, "Reminder not found")
		return
	} else if err != nil {
		log.Printf("Error deleting card reminder: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReminderMessageResponse{Message: "Reminder deleted"})
}

func (h *ReminderHandler) ListGoals(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var cardID *uuid.UUID
	if cardIDStr := r.URL.Query().Get("card_id"); cardIDStr != "" {
		parsed, err := uuid.Parse(cardIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid card ID")
			return
		}
		cardID = &parsed
	}

	reminders, err := h.reminderService.ListGoalReminders(r.Context(), user.ID, cardID)
	if err != nil {
		log.Printf("Error listing goal reminders: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReminderGoalListResponse{Reminders: reminders})
}

func (h *ReminderHandler) UpsertGoalReminder(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input models.GoalReminderInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.ItemID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "Invalid goal ID")
		return
	}

	reminder, err := h.reminderService.UpsertGoalReminder(r.Context(), user.ID, input)
	if errors.Is(err, services.ErrInvalidSchedule) {
		writeError(w, http.StatusBadRequest, "Invalid reminder schedule")
		return
	}
	if errors.Is(err, services.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "Goal not found")
		return
	}
	if errors.Is(err, services.ErrGoalCompleted) {
		writeError(w, http.StatusBadRequest, "Goal already completed")
		return
	}
	if errors.Is(err, services.ErrCardNotEligible) {
		writeError(w, http.StatusBadRequest, "Card must be finalized and not archived")
		return
	}
	if err != nil {
		log.Printf("Error updating goal reminder: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReminderGoalResponse{Reminder: reminder})
}

func (h *ReminderHandler) DeleteGoalReminder(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	reminderIDStr := r.PathValue("id")
	reminderID, err := uuid.Parse(reminderIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid reminder ID")
		return
	}

	if err := h.reminderService.DeleteGoalReminder(r.Context(), user.ID, reminderID); errors.Is(err, services.ErrReminderNotFound) {
		writeError(w, http.StatusNotFound, "Reminder not found")
		return
	} else if err != nil {
		log.Printf("Error deleting goal reminder: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReminderMessageResponse{Message: "Reminder deleted"})
}

func (h *ReminderHandler) SendTest(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req ReminderTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.CardID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "Card ID is required")
		return
	}

	if err := h.reminderService.SendTestEmail(r.Context(), user.ID, req.CardID); errors.Is(err, services.ErrEmailNotVerified) {
		writeError(w, http.StatusForbidden, "Verify your email to send test reminders")
		return
	} else if errors.Is(err, services.ErrRemindersDisabled) {
		writeError(w, http.StatusBadRequest, "Enable reminders before sending a test email")
		return
	} else if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	} else if errors.Is(err, services.ErrCardNotEligible) {
		writeError(w, http.StatusBadRequest, "Card must be finalized and not archived")
		return
	} else if err != nil {
		log.Printf("Error sending test reminder: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReminderMessageResponse{Message: "Test email sent"})
}
