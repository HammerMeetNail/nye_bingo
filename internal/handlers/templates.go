package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
	"github.com/HammerMeetNail/yearofbingo/internal/services/billing"
)

type TemplateHandler struct {
	templateService services.TemplateServiceInterface
}

func NewTemplateHandler(templateService services.TemplateServiceInterface) *TemplateHandler {
	return &TemplateHandler{templateService: templateService}
}

type TemplatesListResponse struct {
	Templates []models.CardTemplate `json:"templates"`
}

type CreateTemplateRequest struct {
	FromCardID              *string  `json:"from_card_id,omitempty"`
	Name                    string   `json:"name"`
	Category                *string  `json:"category,omitempty"`
	GridSize                int      `json:"grid_size,omitempty"`
	HeaderText              string   `json:"header_text,omitempty"`
	HasFreeSpace            *bool    `json:"has_free_space,omitempty"`
	DefaultVisibleToFriends *bool    `json:"default_visible_to_friends,omitempty"`
	Items                   []string `json:"items,omitempty"`
}

type UpdateTemplateRequest struct {
	Name                    string  `json:"name"`
	Category                *string `json:"category,omitempty"`
	GridSize                int     `json:"grid_size,omitempty"`
	HeaderText              string  `json:"header_text,omitempty"`
	HasFreeSpace            *bool   `json:"has_free_space,omitempty"`
	DefaultVisibleToFriends *bool   `json:"default_visible_to_friends,omitempty"`
}

type ReplaceTemplateItemsRequest struct {
	Items []string `json:"items"`
}

type CreateCardFromTemplateRequest struct {
	Year             int     `json:"year"`
	Title            *string `json:"title,omitempty"`
	Category         *string `json:"category,omitempty"`
	ShuffleLayout    *bool   `json:"shuffle_layout,omitempty"`
	VisibleToFriends *bool   `json:"visible_to_friends,omitempty"`
}

type RolloverCardRequest struct {
	Year          int     `json:"year"`
	CarryOver     string  `json:"carry_over,omitempty"`
	ShuffleLayout *bool   `json:"shuffle_layout,omitempty"`
	Title         *string `json:"title,omitempty"`
}

type CardConflictInfo struct {
	Year  int    `json:"year"`
	Title string `json:"title"`
}

type CardConflictResponse struct {
	Error          string           `json:"error"`
	Conflict       CardConflictInfo `json:"conflict"`
	SuggestedTitle string           `json:"suggested_title"`
}

func (h *TemplateHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureTemplates) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	templates, err := h.templateService.List(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, TemplatesListResponse{Templates: templates})
}

func (h *TemplateHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureTemplates) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	templateID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	tpl, err := h.templateService.Get(r.Context(), user.ID, templateID)
	if errors.Is(err, services.ErrTemplateNotFound) {
		writeError(w, http.StatusNotFound, "Template not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (h *TemplateHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureTemplates) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	var req CreateTemplateRequest
	if err := decodeStrictJSON(w, r, &req, 256*1024); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.FromCardID != nil && *req.FromCardID != "" {
		cardID, err := uuid.Parse(*req.FromCardID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid from_card_id")
			return
		}
		tpl, err := h.templateService.CreateFromCard(r.Context(), user.ID, cardID, req.Name)
		if errors.Is(err, services.ErrTemplateLimitReached) {
			writeError(w, http.StatusBadRequest, "Template limit reached (50)")
			return
		}
		if errors.Is(err, services.ErrNotCardOwner) {
			writeError(w, http.StatusForbidden, "Access denied")
			return
		}
		var vErr *services.TemplateValidationError
		if errors.As(err, &vErr) {
			writeError(w, http.StatusBadRequest, vErr.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		writeJSON(w, http.StatusCreated, tpl)
		return
	}

	hasFree := true
	if req.HasFreeSpace != nil {
		hasFree = *req.HasFreeSpace
	}
	defaultVisible := true
	if req.DefaultVisibleToFriends != nil {
		defaultVisible = *req.DefaultVisibleToFriends
	}

	tpl, err := h.templateService.Create(r.Context(), user.ID, models.CreateTemplateParams{
		Name:                    req.Name,
		Category:                req.Category,
		GridSize:                req.GridSize,
		HeaderText:              req.HeaderText,
		HasFreeSpace:            hasFree,
		DefaultVisibleToFriends: defaultVisible,
		Items:                   req.Items,
	})
	if errors.Is(err, services.ErrTemplateLimitReached) {
		writeError(w, http.StatusBadRequest, "Template limit reached (50)")
		return
	}
	var vErr *services.TemplateValidationError
	if errors.As(err, &vErr) {
		writeError(w, http.StatusBadRequest, vErr.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, tpl)
}

func (h *TemplateHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureTemplates) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	templateID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req UpdateTemplateRequest
	if err := decodeStrictJSON(w, r, &req, 256*1024); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	hasFree := true
	if req.HasFreeSpace != nil {
		hasFree = *req.HasFreeSpace
	}
	defaultVisible := true
	if req.DefaultVisibleToFriends != nil {
		defaultVisible = *req.DefaultVisibleToFriends
	}

	tpl, err := h.templateService.Update(r.Context(), user.ID, templateID, models.UpdateTemplateParams{
		Name:                    req.Name,
		Category:                req.Category,
		GridSize:                req.GridSize,
		HeaderText:              req.HeaderText,
		HasFreeSpace:            hasFree,
		DefaultVisibleToFriends: defaultVisible,
	})
	if errors.Is(err, services.ErrTemplateNotFound) {
		writeError(w, http.StatusNotFound, "Template not found")
		return
	}
	var vErr *services.TemplateValidationError
	if errors.As(err, &vErr) {
		writeError(w, http.StatusBadRequest, vErr.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (h *TemplateHandler) ReplaceTemplateItems(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureTemplates) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	templateID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req ReplaceTemplateItemsRequest
	if err := decodeStrictJSON(w, r, &req, 256*1024); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tpl, err := h.templateService.ReplaceItems(r.Context(), user.ID, templateID, req.Items)
	if errors.Is(err, services.ErrTemplateNotFound) {
		writeError(w, http.StatusNotFound, "Template not found")
		return
	}
	var vErr *services.TemplateValidationError
	if errors.As(err, &vErr) {
		writeError(w, http.StatusBadRequest, vErr.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (h *TemplateHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureTemplates) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	templateID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.templateService.Delete(r.Context(), user.ID, templateID); errors.Is(err, services.ErrTemplateNotFound) {
		writeError(w, http.StatusNotFound, "Template not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Template deleted"})
}

func (h *TemplateHandler) CreateCardFromTemplate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureTemplates) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	templateID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req CreateCardFromTemplateRequest
	if err := decodeStrictJSON(w, r, &req, 64*1024); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateYear(req.Year); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	shuffle := true
	if req.ShuffleLayout != nil {
		shuffle = *req.ShuffleLayout
	}

	card, err := h.templateService.CreateCardFromTemplate(r.Context(), user.ID, templateID, models.TemplateCreateCardParams{
		Year:             req.Year,
		Title:            req.Title,
		Category:         req.Category,
		ShuffleLayout:    shuffle,
		VisibleToFriends: req.VisibleToFriends,
	})
	var conflict *services.CardConflictError
	if errors.As(err, &conflict) {
		writeJSON(w, http.StatusConflict, CardConflictResponse{
			Error:          "Card conflict",
			Conflict:       CardConflictInfo{Year: conflict.Year, Title: conflict.Title},
			SuggestedTitle: conflict.SuggestedTitle,
		})
		return
	}
	if errors.Is(err, services.ErrTemplateNotFound) {
		writeError(w, http.StatusNotFound, "Template not found")
		return
	}
	var vErr *services.TemplateValidationError
	if errors.As(err, &vErr) {
		writeError(w, http.StatusBadRequest, vErr.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, CardResponse{Card: card})
}

func (h *TemplateHandler) RolloverCard(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureTemplates) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	cardID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req RolloverCardRequest
	if err := decodeStrictJSON(w, r, &req, 64*1024); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateYear(req.Year); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	shuffle := true
	if req.ShuffleLayout != nil {
		shuffle = *req.ShuffleLayout
	}
	carry := req.CarryOver
	if carry == "" {
		carry = "all"
	}

	card, err := h.templateService.RolloverCard(r.Context(), user.ID, cardID, models.RolloverParams{
		Year:          req.Year,
		CarryOver:     carry,
		ShuffleLayout: shuffle,
		Title:         req.Title,
	})
	var conflict *services.CardConflictError
	if errors.As(err, &conflict) {
		writeJSON(w, http.StatusConflict, CardConflictResponse{
			Error:          "Card conflict",
			Conflict:       CardConflictInfo{Year: conflict.Year, Title: conflict.Title},
			SuggestedTitle: conflict.SuggestedTitle,
		})
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if errors.Is(err, services.ErrInvalidCarryOverValue) {
		writeError(w, http.StatusBadRequest, "Invalid carry_over")
		return
	}
	var vErr *services.TemplateValidationError
	if errors.As(err, &vErr) {
		writeError(w, http.StatusBadRequest, vErr.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, CardResponse{Card: card})
}

func validateYear(year int) error {
	currentYear := time.Now().Year()
	if year < 2020 || year > currentYear+1 {
		return errors.New("year must be between 2020 and next year")
	}
	return nil
}

func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	idStr := r.PathValue(name)
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "Missing id")
		return uuid.Nil, false
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, dst interface{}, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	// Ensure there are no trailing tokens.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("invalid request body")
	}
	return nil
}
