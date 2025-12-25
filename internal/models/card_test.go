package models

import (
	"testing"
)

func TestIsValidGridSize(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{
		{1, false},
		{2, true},
		{3, true},
		{4, true},
		{5, true},
		{6, false},
	}

	for _, tt := range tests {
		if got := IsValidGridSize(tt.n); got != tt.want {
			t.Errorf("IsValidGridSize(%d): expected %v, got %v", tt.n, tt.want, got)
		}
	}
}

func TestDefaultHeaderText(t *testing.T) {
	tests := []struct {
		gridSize int
		want     string
	}{
		{2, "BI"},
		{3, "BIN"},
		{4, "BING"},
		{5, "BINGO"},
	}

	for _, tt := range tests {
		if got := DefaultHeaderText(tt.gridSize); got != tt.want {
			t.Errorf("DefaultHeaderText(%d): expected %q, got %q", tt.gridSize, tt.want, got)
		}
	}
}

func TestValidateHeaderText(t *testing.T) {
	if err := ValidateHeaderText("BINGO", 5); err != nil {
		t.Fatalf("expected valid header, got error: %v", err)
	}
	if err := ValidateHeaderText("", 5); err == nil {
		t.Fatal("expected error for empty header")
	}
	if err := ValidateHeaderText("TOOLONG", 5); err == nil {
		t.Fatal("expected error for header exceeding grid size")
	}
}

func TestBingoCardCapacityAndFree(t *testing.T) {
	freePos := 12
	card := BingoCard{
		GridSize:     5,
		HasFreeSpace: true,
		FreeSpacePos: &freePos,
	}
	if card.TotalSquares() != 25 {
		t.Errorf("TotalSquares: expected 25, got %d", card.TotalSquares())
	}
	if card.Capacity() != 24 {
		t.Errorf("Capacity (FREE on): expected 24, got %d", card.Capacity())
	}
	if !card.IsFreeSpacePosition(12) {
		t.Error("expected position 12 to be free space")
	}
	if card.IsValidItemPosition(12) {
		t.Error("expected position 12 to be invalid for items when FREE enabled")
	}

	card.HasFreeSpace = false
	card.FreeSpacePos = nil
	if card.Capacity() != 25 {
		t.Errorf("Capacity (FREE off): expected 25, got %d", card.Capacity())
	}
	if !card.IsValidItemPosition(12) {
		t.Error("expected position 12 to be valid for items when FREE disabled")
	}
}

func TestDefaultFreeSpacePosition(t *testing.T) {
	card := BingoCard{GridSize: 5}
	if pos := card.DefaultFreeSpacePosition(); pos != 12 {
		t.Fatalf("expected center position 12, got %d", pos)
	}

	card.GridSize = 0
	if pos := card.DefaultFreeSpacePosition(); pos != 12 {
		t.Fatalf("expected fallback center position 12, got %d", pos)
	}

	card.GridSize = 4
	pos := card.DefaultFreeSpacePosition()
	if pos < 0 || pos >= 16 {
		t.Fatalf("expected position within range, got %d", pos)
	}
}

func TestDisplayName(t *testing.T) {
	title := "My Card"
	card := BingoCard{Title: &title, Year: 2024}
	if name := card.DisplayName(); name != "My Card" {
		t.Fatalf("expected title display name, got %s", name)
	}

	card.Title = nil
	if name := card.DisplayName(); name != "2024 Bingo Card" {
		t.Fatalf("expected year display name, got %s", name)
	}
}

func TestIsValidCategory(t *testing.T) {
	if !IsValidCategory("health") {
		t.Fatal("expected valid category")
	}
	if IsValidCategory("invalid") {
		t.Fatal("expected invalid category")
	}
}
