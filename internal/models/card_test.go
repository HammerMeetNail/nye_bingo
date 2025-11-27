package models

import (
	"testing"
)

func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      int
		expected int
	}{
		{"GridSize", GridSize, 5},
		{"TotalSquares", TotalSquares, 25},
		{"FreeSpacePos", FreeSpacePos, 12},
		{"ItemsRequired", ItemsRequired, 24},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s: expected %d, got %d", tt.name, tt.expected, tt.got)
			}
		})
	}
}

func TestFreeSpacePosition(t *testing.T) {
	// Free space should be in the center of a 5x5 grid
	// Row 2 (0-indexed), Column 2 = position 12
	row := FreeSpacePos / GridSize
	col := FreeSpacePos % GridSize

	if row != 2 {
		t.Errorf("free space should be in row 2, got row %d", row)
	}
	if col != 2 {
		t.Errorf("free space should be in column 2, got column %d", col)
	}
}

func TestGridPositionCalculations(t *testing.T) {
	// Test that we can correctly calculate row/col from position
	tests := []struct {
		position int
		row      int
		col      int
	}{
		{0, 0, 0},  // Top-left
		{4, 0, 4},  // Top-right
		{12, 2, 2}, // Center (free space)
		{20, 4, 0}, // Bottom-left
		{24, 4, 4}, // Bottom-right
		{6, 1, 1},  // Second row, second col
		{18, 3, 3}, // Fourth row, fourth col
	}

	for _, tt := range tests {
		t.Run("position_"+string(rune('0'+tt.position)), func(t *testing.T) {
			row := tt.position / GridSize
			col := tt.position % GridSize

			if row != tt.row {
				t.Errorf("position %d: expected row %d, got %d", tt.position, tt.row, row)
			}
			if col != tt.col {
				t.Errorf("position %d: expected col %d, got %d", tt.position, tt.col, col)
			}
		})
	}
}

func TestBingoLinePositions(t *testing.T) {
	// Test that bingo line positions are correct
	t.Run("rows", func(t *testing.T) {
		expectedRows := [][]int{
			{0, 1, 2, 3, 4},      // Row 0
			{5, 6, 7, 8, 9},      // Row 1
			{10, 11, 12, 13, 14}, // Row 2 (contains free space)
			{15, 16, 17, 18, 19}, // Row 3
			{20, 21, 22, 23, 24}, // Row 4
		}

		for rowNum, expected := range expectedRows {
			for colNum, expectedPos := range expected {
				actualPos := rowNum*GridSize + colNum
				if actualPos != expectedPos {
					t.Errorf("row %d, col %d: expected position %d, got %d",
						rowNum, colNum, expectedPos, actualPos)
				}
			}
		}
	})

	t.Run("columns", func(t *testing.T) {
		expectedCols := [][]int{
			{0, 5, 10, 15, 20}, // Column 0 (B)
			{1, 6, 11, 16, 21}, // Column 1 (I)
			{2, 7, 12, 17, 22}, // Column 2 (N, contains free space)
			{3, 8, 13, 18, 23}, // Column 3 (G)
			{4, 9, 14, 19, 24}, // Column 4 (O)
		}

		for colNum, expected := range expectedCols {
			for rowNum, expectedPos := range expected {
				actualPos := rowNum*GridSize + colNum
				if actualPos != expectedPos {
					t.Errorf("col %d, row %d: expected position %d, got %d",
						colNum, rowNum, expectedPos, actualPos)
				}
			}
		}
	})

	t.Run("diagonals", func(t *testing.T) {
		// Top-left to bottom-right
		diagonal1 := []int{0, 6, 12, 18, 24}
		for i, pos := range diagonal1 {
			expected := i*GridSize + i
			if pos != expected {
				t.Errorf("diagonal1[%d]: expected %d, got %d", i, expected, pos)
			}
		}

		// Top-right to bottom-left
		diagonal2 := []int{4, 8, 12, 16, 20}
		for i, pos := range diagonal2 {
			expected := i*GridSize + (GridSize - 1 - i)
			if pos != expected {
				t.Errorf("diagonal2[%d]: expected %d, got %d", i, expected, pos)
			}
		}
	})
}

func TestValidPositions(t *testing.T) {
	// All positions except free space (12) should be valid for items
	validCount := 0
	for i := 0; i < TotalSquares; i++ {
		if i != FreeSpacePos {
			validCount++
		}
	}

	if validCount != ItemsRequired {
		t.Errorf("expected %d valid positions, got %d", ItemsRequired, validCount)
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
