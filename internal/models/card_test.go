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

func TestCardStats_ZeroValues(t *testing.T) {
	stats := CardStats{}

	if stats.TotalItems != 0 {
		t.Errorf("expected TotalItems to be 0, got %d", stats.TotalItems)
	}
	if stats.CompletedItems != 0 {
		t.Errorf("expected CompletedItems to be 0, got %d", stats.CompletedItems)
	}
	if stats.CompletionRate != 0 {
		t.Errorf("expected CompletionRate to be 0, got %f", stats.CompletionRate)
	}
	if stats.BingosAchieved != 0 {
		t.Errorf("expected BingosAchieved to be 0, got %d", stats.BingosAchieved)
	}
	if stats.FirstCompletion != nil {
		t.Error("expected FirstCompletion to be nil")
	}
	if stats.LastCompletion != nil {
		t.Error("expected LastCompletion to be nil")
	}
}
