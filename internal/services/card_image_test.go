package services

import (
	"bytes"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

func TestClampLines_TruncatesWithValidUTF8(t *testing.T) {
	face := basicfont.Face7x13
	d := &font.Drawer{Face: face}

	original := "こんにちは世界😀😀😀"
	maxWidth := d.MeasureString("こんにちは...").Ceil()

	lines := []string{original, "unused"}
	out := clampLines(face, lines, 1, maxWidth)
	if len(out) != 1 {
		t.Fatalf("expected 1 line, got %d", len(out))
	}
	if !strings.HasSuffix(out[0], "...") {
		t.Fatalf("expected ellipsis suffix, got %q", out[0])
	}
	if !utf8.ValidString(out[0]) {
		t.Fatalf("expected valid UTF-8, got %q", out[0])
	}
	if !strings.Contains(out[0], "こ") {
		t.Fatalf("expected some original content preserved, got %q", out[0])
	}
}

func TestRenderReminderPNG_RendersAndTogglesCompletionColors(t *testing.T) {
	title := "Test Card"
	freePos := 4
	card := models.BingoCard{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		Year:         2025,
		Title:        &title,
		GridSize:     3,
		HasFreeSpace: true,
		FreeSpacePos: &freePos,
	}

	items := []models.BingoItem{
		{ID: uuid.New(), CardID: card.ID, Position: 0, Content: "First", IsCompleted: false},
		{ID: uuid.New(), CardID: card.ID, Position: 1, Content: "Completed", IsCompleted: true},
		{ID: uuid.New(), CardID: card.ID, Position: 2, Content: "Some long text that should wrap in the cell", IsCompleted: false},
	}

	pngNoCompletions, err := RenderReminderPNG(card, items, RenderOptions{ShowCompletions: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	imgNo, err := png.Decode(bytes.NewReader(pngNoCompletions))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if imgNo.Bounds().Dx() != 1200 || imgNo.Bounds().Dy() != 630 {
		t.Fatalf("expected 1200x630 image, got %dx%d", imgNo.Bounds().Dx(), imgNo.Bounds().Dy())
	}

	pngWithCompletions, err := RenderReminderPNG(card, items, RenderOptions{ShowCompletions: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	imgYes, err := png.Decode(bytes.NewReader(pngWithCompletions))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	// Sample the top-left pixel inside the FREE cell. When ShowCompletions=true,
	// completed cells use the green completion background.
	const (
		padding      = 40
		headerHeight = 80
		borderWidth  = 2
	)
	gridAvailableWidth := 1200 - padding*2
	gridAvailableHeight := 630 - headerHeight - padding*2
	cellSize := minInt(gridAvailableWidth/3, gridAvailableHeight/3)
	gridWidth := cellSize * 3
	gridLeft := (1200 - gridWidth) / 2
	gridTop := headerHeight + padding
	freeRow := 1
	freeCol := 1
	sampleX := gridLeft + freeCol*cellSize + borderWidth + 1
	sampleY := gridTop + freeRow*cellSize + borderWidth + 1

	toRGBA := func(c color.Color) color.RGBA {
		r, g, b, a := c.RGBA()
		return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}

	gotNo := toRGBA(imgNo.At(sampleX, sampleY))
	gotYes := toRGBA(imgYes.At(sampleX, sampleY))

	wantNo := color.RGBA{R: 0xF1, G: 0xF0, B: 0xEB, A: 0xFF}
	wantYes := color.RGBA{R: 0xD7, G: 0xF3, B: 0xE3, A: 0xFF}
	if gotNo != wantNo {
		t.Fatalf("expected FREE cell background %v when completions hidden, got %v", wantNo, gotNo)
	}
	if gotYes != wantYes {
		t.Fatalf("expected FREE cell background %v when completions shown, got %v", wantYes, gotYes)
	}
}

func TestRenderReminderPNG_InvalidGridSizeFallsBackToMax(t *testing.T) {
	title := "Invalid Grid"
	card := models.BingoCard{
		ID:       uuid.New(),
		UserID:   uuid.New(),
		Year:     2025,
		Title:    &title,
		GridSize: 0,
	}

	out, err := RenderReminderPNG(card, nil, RenderOptions{ShowCompletions: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 {
		t.Fatalf("expected 1200x630 image, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}
