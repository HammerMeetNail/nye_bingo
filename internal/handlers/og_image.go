package handlers

import (
	"net/http"
	"sync"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type OGImageHandler struct {
	once     sync.Once
	pngBytes []byte
	err      error
}

func NewOGImageHandler() *OGImageHandler {
	return &OGImageHandler{}
}

func (h *OGImageHandler) Default(w http.ResponseWriter, r *http.Request) {
	h.once.Do(func() {
		title := "Year of Bingo"
		freePos := 12
		card := models.BingoCard{
			Title:        &title,
			GridSize:     5,
			HasFreeSpace: true,
			FreeSpacePos: &freePos,
		}

		items := []models.BingoItem{
			{Position: 0, Content: "Start a new habit", IsCompleted: true},
			{Position: 1, Content: "Try a new recipe", IsCompleted: false},
			{Position: 2, Content: "Take a day trip", IsCompleted: true},
			{Position: 4, Content: "Read 12 books", IsCompleted: false},
			{Position: 5, Content: "Volunteer once", IsCompleted: true},
			{Position: 7, Content: "Declutter a room", IsCompleted: true},
			{Position: 8, Content: "Learn a new skill", IsCompleted: false},
			{Position: 10, Content: "Call a friend", IsCompleted: true},
			{Position: 13, Content: "Cook at home", IsCompleted: false},
			{Position: 14, Content: "Try a new hobby", IsCompleted: true},
			{Position: 16, Content: "Walk 10k steps", IsCompleted: false},
			{Position: 18, Content: "Plan a weekend", IsCompleted: true},
			{Position: 20, Content: "Do a small project", IsCompleted: true},
			{Position: 22, Content: "Write something", IsCompleted: false},
			{Position: 24, Content: "Celebrate a win", IsCompleted: true},
		}

		h.pngBytes, h.err = services.RenderReminderPNG(card, items, services.RenderOptions{ShowCompletions: true})
	})

	if h.err != nil {
		http.Error(w, "Failed to render image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.pngBytes)
}
