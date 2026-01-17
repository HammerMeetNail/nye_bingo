package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type SharePublicHandler struct {
	templates   *template.Template
	cardService services.CardServiceInterface
}

type SharePageData struct {
	Found        bool
	PageTitle    string
	RedirectPath string
	ErrorMessage string

	OGTitle       string
	OGDescription string
	OGURL         string
	OGImage       string
	OGImageAlt    string
}

func NewSharePublicHandler(templatesDir string, cardService services.CardServiceInterface) (*SharePublicHandler, error) {
	templates, err := template.ParseFiles(filepath.Join(templatesDir, "share.html"))
	if err != nil {
		return nil, err
	}
	return &SharePublicHandler{
		templates:   templates,
		cardService: cardService,
	}, nil
}

func (h *SharePublicHandler) Serve(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" || !isValidShareToken(token) {
		h.render(w, r, http.StatusNotFound, SharePageData{
			Found:        false,
			PageTitle:    "Invalid Share Link - Year of Bingo",
			RedirectPath: "/",
			ErrorMessage: "This share link is missing or malformed.",
		})
		return
	}

	redirectPath := "/share/" + token

	shared, err := h.cardService.GetSharedCardByToken(r.Context(), token)
	if err != nil {
		if !errors.Is(err, services.ErrShareNotFound) {
			h.render(w, r, http.StatusInternalServerError, SharePageData{
				Found:        false,
				PageTitle:    "Error - Year of Bingo",
				RedirectPath: redirectPath,
				ErrorMessage: "An unexpected error occurred.",
			})
			return
		}
		h.render(w, r, http.StatusNotFound, SharePageData{
			Found:        false,
			PageTitle:    "Share Link Not Found - Year of Bingo",
			RedirectPath: redirectPath,
			ErrorMessage: "This share link may have expired or been revoked.",
		})
		return
	}

	baseURL := resolveBaseURL(r)
	displayName := shareCardDisplayName(shared.Card)

	completed, total := shareCompletionStats(shared)
	description := fmt.Sprintf("%d/%d complete — View shared card", completed, total)

	state := shareCompletionState(shared)
	version := shareVersion(state)

	h.render(w, r, http.StatusOK, SharePageData{
		Found:         true,
		PageTitle:     displayName + " - Year of Bingo",
		RedirectPath:  redirectPath,
		OGTitle:       displayName,
		OGDescription: description,
		OGURL:         baseURL + "/s/" + token,
		OGImage:       baseURL + "/og/share/" + token + ".png?v=" + version,
		OGImageAlt:    "Bingo card preview for " + displayName,
	})
}

func (h *SharePublicHandler) render(w http.ResponseWriter, r *http.Request, status int, data SharePageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(status)
	_ = h.templates.ExecuteTemplate(w, "share.html", data)
}

func isValidShareToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}

func shareCardDisplayName(card models.PublicBingoCard) string {
	if card.Title != nil && strings.TrimSpace(*card.Title) != "" {
		return strings.TrimSpace(*card.Title)
	}
	if card.Year > 0 {
		return fmt.Sprintf("%d Bingo Card", card.Year)
	}
	return "Year of Bingo"
}

func shareCompletionStats(shared *models.SharedCard) (completed int, total int) {
	gridSize := shared.Card.GridSize
	if !models.IsValidGridSize(gridSize) {
		gridSize = models.MaxGridSize
	}
	total = gridSize * gridSize
	if shared.Card.HasFreeSpace {
		total--
	}
	for _, item := range shared.Items {
		if item.IsCompleted {
			completed++
		}
	}
	return completed, total
}

func shareCompletionState(shared *models.SharedCard) []byte {
	gridSize := shared.Card.GridSize
	if !models.IsValidGridSize(gridSize) {
		gridSize = models.MaxGridSize
	}
	total := gridSize * gridSize

	completedByPos := make(map[int]bool, len(shared.Items))
	for _, item := range shared.Items {
		completedByPos[item.Position] = item.IsCompleted
	}

	freePos := -1
	if shared.Card.HasFreeSpace && shared.Card.FreeSpacePos != nil {
		freePos = *shared.Card.FreeSpacePos
	}

	bits := make([]byte, (total+7)/8)
	for pos := 0; pos < total; pos++ {
		isCompleted := completedByPos[pos]
		if pos == freePos {
			isCompleted = true
		}
		if !isCompleted {
			continue
		}
		byteIndex := pos / 8
		bits[byteIndex] |= byte(1 << (pos % 8))
	}
	return bits
}

func shareVersion(state []byte) string {
	sum := sha256.Sum256(state)
	return hex.EncodeToString(sum[:8])
}
