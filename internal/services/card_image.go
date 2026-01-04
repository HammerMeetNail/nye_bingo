package services

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

// RenderOptions controls reminder image rendering behavior.
type RenderOptions struct {
	ShowCompletions bool
}

var (
	fontOnce      sync.Once
	parsedGoFont  *opentype.Font
	parsedGoError error
)

// RenderReminderPNG renders a bingo card PNG for reminder emails.
func RenderReminderPNG(card models.BingoCard, items []models.BingoItem, opts RenderOptions) ([]byte, error) {
	const width = 1200
	const height = 630
	const padding = 40
	const headerHeight = 80
	const borderWidth = 2
	const cellPadding = 10

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{0xFA, 0xF9, 0xF7, 0xFF}}, image.Point{}, draw.Src)

	headerFace, err := newFontFace(32)
	if err != nil {
		return nil, err
	}
	defer func() { _ = headerFace.Close() }()

	bodyFace, err := newFontFace(18)
	if err != nil {
		return nil, err
	}
	defer func() { _ = bodyFace.Close() }()

	statsFace, err := newFontFace(16)
	if err != nil {
		return nil, err
	}
	defer func() { _ = statsFace.Close() }()

	cardName := card.DisplayName()
	stats := buildReminderStats(&card, items)
	statsLine := fmt.Sprintf("%d/%d complete - %s", stats.Completed, stats.Total, pluralizeBingo(stats.Bingos))

	drawText(img, headerFace, padding, 44, cardName, color.RGBA{0x2D, 0x2D, 0x2D, 0xFF})
	drawText(img, statsFace, padding, 70, statsLine, color.RGBA{0x6B, 0x6B, 0x6B, 0xFF})

	gridSize := card.GridSize
	if !models.IsValidGridSize(gridSize) {
		gridSize = models.MaxGridSize
	}

	gridAvailableWidth := width - padding*2
	gridAvailableHeight := height - headerHeight - padding*2
	cellSize := minInt(gridAvailableWidth/gridSize, gridAvailableHeight/gridSize)
	gridWidth := cellSize * gridSize
	gridLeft := (width - gridWidth) / 2
	gridTop := headerHeight + padding

	itemByPos := map[int]models.BingoItem{}
	for _, item := range items {
		itemByPos[item.Position] = item
	}

	freePos := -1
	if card.HasFreeSpace && card.FreeSpacePos != nil {
		freePos = *card.FreeSpacePos
	}

	for row := 0; row < gridSize; row++ {
		for col := 0; col < gridSize; col++ {
			pos := row*gridSize + col
			rect := image.Rect(
				gridLeft+col*cellSize,
				gridTop+row*cellSize,
				gridLeft+(col+1)*cellSize,
				gridTop+(row+1)*cellSize,
			)

			bg := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
			textColor := color.RGBA{0x2D, 0x2D, 0x2D, 0xFF}
			content := ""
			completed := false

			if pos == freePos {
				content = "FREE"
				completed = true
				bg = color.RGBA{0xF1, 0xF0, 0xEB, 0xFF}
			} else if item, ok := itemByPos[pos]; ok {
				content = item.Content
				completed = item.IsCompleted
			}

			if completed && opts.ShowCompletions {
				bg = color.RGBA{0xD7, 0xF3, 0xE3, 0xFF}
				textColor = color.RGBA{0x1B, 0x4D, 0x3E, 0xFF}
			}

			draw.Draw(img, rect, &image.Uniform{C: bg}, image.Point{}, draw.Src)
			drawBorder(img, rect, borderWidth, color.RGBA{0x3A, 0x3A, 0x3A, 0xFF})

			if strings.TrimSpace(content) == "" {
				continue
			}

			maxLines := 4
			if gridSize >= 4 {
				maxLines = 3
			}

			textRect := image.Rect(
				rect.Min.X+cellPadding,
				rect.Min.Y+cellPadding,
				rect.Max.X-cellPadding,
				rect.Max.Y-cellPadding,
			)
			lines := wrapText(bodyFace, content, textRect.Dx())
			lines = clampLines(bodyFace, lines, maxLines, textRect.Dx())
			drawWrappedText(img, bodyFace, textRect, lines, textColor)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func newFontFace(size float64) (*opentype.Face, error) {
	fontOnce.Do(func() {
		parsedGoFont, parsedGoError = opentype.Parse(goregular.TTF)
	})
	if parsedGoError != nil {
		return nil, fmt.Errorf("parse font: %w", parsedGoError)
	}
	face, err := opentype.NewFace(parsedGoFont, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("load font face: %w", err)
	}
	otFace, ok := face.(*opentype.Face)
	if !ok {
		return nil, fmt.Errorf("load font face: unexpected type")
	}
	return otFace, nil
}

func drawText(img draw.Image, face font.Face, x, y int, text string, clr color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func drawBorder(img draw.Image, rect image.Rectangle, width int, clr color.Color) {
	border := image.NewUniform(clr)
	draw.Draw(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+width), border, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(rect.Min.X, rect.Max.Y-width, rect.Max.X, rect.Max.Y), border, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+width, rect.Max.Y), border, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(rect.Max.X-width, rect.Min.Y, rect.Max.X, rect.Max.Y), border, image.Point{}, draw.Src)
}

func wrapText(face font.Face, text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	d := &font.Drawer{Face: face}
	lines := []string{}
	current := words[0]

	for _, word := range words[1:] {
		test := current + " " + word
		if d.MeasureString(test).Ceil() <= maxWidth {
			current = test
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func clampLines(face font.Face, lines []string, maxLines int, maxWidth int) []string {
	if len(lines) <= maxLines {
		return lines
	}
	lines = lines[:maxLines]
	last := lines[maxLines-1]
	ellipsis := "..."
	d := &font.Drawer{Face: face}

	runes := []rune(last)
	for d.MeasureString(string(runes)+ellipsis).Ceil() > maxWidth && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	lines[maxLines-1] = strings.TrimSpace(string(runes)) + ellipsis
	return lines
}

func drawWrappedText(img draw.Image, face font.Face, rect image.Rectangle, lines []string, clr color.Color) {
	if len(lines) == 0 {
		return
	}
	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	textHeight := lineHeight * len(lines)
	startY := rect.Min.Y + (rect.Dy()-textHeight)/2 + metrics.Ascent.Ceil()

	for i, line := range lines {
		lineWidth := font.MeasureString(face, line).Ceil()
		x := rect.Min.X + (rect.Dx()-lineWidth)/2
		y := startY + i*lineHeight
		drawText(img, face, x, y, line, clr)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
