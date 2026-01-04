package services

import (
	"strings"
	"testing"
	"unicode/utf8"

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
