package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type ShareOGImageHandler struct {
	cardService services.CardServiceInterface
}

func NewShareOGImageHandler(cardService services.CardServiceInterface) *ShareOGImageHandler {
	return &ShareOGImageHandler{cardService: cardService}
}

func (h *ShareOGImageHandler) Serve(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	token = strings.TrimSuffix(token, ".png")
	if token == "" || !isValidShareToken(token) {
		http.NotFound(w, r)
		return
	}

	shared, err := h.cardService.GetSharedCardByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, services.ErrShareNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	state := shareCompletionState(shared)
	etag := `W/"` + shareVersion(state) + `"`
	if inm := r.Header.Get("If-None-Match"); inm != "" && strings.Contains(inm, etag) {
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	pngBytes, err := renderSharedCardPNG(shared)
	if err != nil {
		http.Error(w, "Failed to render image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=300, must-revalidate")
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pngBytes)
}

func renderSharedCardPNG(shared *models.SharedCard) ([]byte, error) {
	card := models.BingoCard{
		Year:         shared.Card.Year,
		Category:     shared.Card.Category,
		Title:        shared.Card.Title,
		GridSize:     shared.Card.GridSize,
		HeaderText:   shared.Card.HeaderText,
		HasFreeSpace: shared.Card.HasFreeSpace,
		FreeSpacePos: shared.Card.FreeSpacePos,
		IsFinalized:  true,
	}
	items := make([]models.BingoItem, 0, len(shared.Items))
	for _, item := range shared.Items {
		items = append(items, models.BingoItem{
			Position:    item.Position,
			Content:     item.Content,
			IsCompleted: item.IsCompleted,
		})
	}
	return services.RenderReminderPNG(card, items, services.RenderOptions{ShowCompletions: true})
}
