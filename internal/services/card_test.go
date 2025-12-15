package services

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestCountBingos_NoCompletions(t *testing.T) {
	svc := &CardService{}

	items := []models.BingoItem{}
	freePos := 12
	count := svc.countBingos(items, 5, &freePos)

	if count != 0 {
		t.Errorf("expected 0 bingos, got %d", count)
	}
}

func TestCountBingos_FirstRowComplete(t *testing.T) {
	svc := &CardService{}

	// First row: positions 0, 1, 2, 3, 4
	items := []models.BingoItem{
		{Position: 0, IsCompleted: true},
		{Position: 1, IsCompleted: true},
		{Position: 2, IsCompleted: true},
		{Position: 3, IsCompleted: true},
		{Position: 4, IsCompleted: true},
	}

	freePos := 12
	count := svc.countBingos(items, 5, &freePos)
	if count != 1 {
		t.Errorf("expected 1 bingo for first row, got %d", count)
	}
}

func TestCountBingos_MiddleRowWithFreeSpace(t *testing.T) {
	svc := &CardService{}

	// Middle row: positions 10, 11, 12 (free), 13, 14
	// Only need 10, 11, 13, 14 completed (12 is free space, counted as complete)
	items := []models.BingoItem{
		{Position: 10, IsCompleted: true},
		{Position: 11, IsCompleted: true},
		// Position 12 is free space - no item needed
		{Position: 13, IsCompleted: true},
		{Position: 14, IsCompleted: true},
	}

	freePos := 12
	count := svc.countBingos(items, 5, &freePos)
	if count != 1 {
		t.Errorf("expected 1 bingo for middle row with free space, got %d", count)
	}
}

func TestCountBingos_FirstColumnComplete(t *testing.T) {
	svc := &CardService{}

	// First column (B): positions 0, 5, 10, 15, 20
	items := []models.BingoItem{
		{Position: 0, IsCompleted: true},
		{Position: 5, IsCompleted: true},
		{Position: 10, IsCompleted: true},
		{Position: 15, IsCompleted: true},
		{Position: 20, IsCompleted: true},
	}

	freePos := 12
	count := svc.countBingos(items, 5, &freePos)
	if count != 1 {
		t.Errorf("expected 1 bingo for first column, got %d", count)
	}
}

func TestCountBingos_MiddleColumnWithFreeSpace(t *testing.T) {
	svc := &CardService{}

	// Middle column (N): positions 2, 7, 12 (free), 17, 22
	items := []models.BingoItem{
		{Position: 2, IsCompleted: true},
		{Position: 7, IsCompleted: true},
		// Position 12 is free space
		{Position: 17, IsCompleted: true},
		{Position: 22, IsCompleted: true},
	}

	freePos := 12
	count := svc.countBingos(items, 5, &freePos)
	if count != 1 {
		t.Errorf("expected 1 bingo for middle column with free space, got %d", count)
	}
}

func TestCountBingos_Diagonal1(t *testing.T) {
	svc := &CardService{}

	// Diagonal: 0, 6, 12 (free), 18, 24
	items := []models.BingoItem{
		{Position: 0, IsCompleted: true},
		{Position: 6, IsCompleted: true},
		// Position 12 is free space
		{Position: 18, IsCompleted: true},
		{Position: 24, IsCompleted: true},
	}

	freePos := 12
	count := svc.countBingos(items, 5, &freePos)
	if count != 1 {
		t.Errorf("expected 1 bingo for diagonal, got %d", count)
	}
}

func TestCountBingos_Diagonal2(t *testing.T) {
	svc := &CardService{}

	// Diagonal: 4, 8, 12 (free), 16, 20
	items := []models.BingoItem{
		{Position: 4, IsCompleted: true},
		{Position: 8, IsCompleted: true},
		// Position 12 is free space
		{Position: 16, IsCompleted: true},
		{Position: 20, IsCompleted: true},
	}

	freePos := 12
	count := svc.countBingos(items, 5, &freePos)
	if count != 1 {
		t.Errorf("expected 1 bingo for anti-diagonal, got %d", count)
	}
}

func TestResolveCloneHasFreeSpace_InheritsWhenOmitted(t *testing.T) {
	override := (*bool)(nil)

	if got := resolveCloneHasFreeSpace(false, override); got != false {
		t.Fatalf("expected false when omitted and source is false, got %v", got)
	}
	if got := resolveCloneHasFreeSpace(true, override); got != true {
		t.Fatalf("expected true when omitted and source is true, got %v", got)
	}
}

func TestResolveCloneHasFreeSpace_OverrideWins(t *testing.T) {
	tval := true
	fval := false

	if got := resolveCloneHasFreeSpace(false, &tval); got != true {
		t.Fatalf("expected true override, got %v", got)
	}
	if got := resolveCloneHasFreeSpace(true, &fval); got != false {
		t.Fatalf("expected false override, got %v", got)
	}
}

func TestCountBingos_MultipleBingos(t *testing.T) {
	svc := &CardService{}

	// Complete first row (0-4) and first column (0,5,10,15,20)
	items := []models.BingoItem{
		{Position: 0, IsCompleted: true},
		{Position: 1, IsCompleted: true},
		{Position: 2, IsCompleted: true},
		{Position: 3, IsCompleted: true},
		{Position: 4, IsCompleted: true},
		{Position: 5, IsCompleted: true},
		{Position: 10, IsCompleted: true},
		{Position: 15, IsCompleted: true},
		{Position: 20, IsCompleted: true},
	}

	freePos := 12
	count := svc.countBingos(items, 5, &freePos)
	if count != 2 {
		t.Errorf("expected 2 bingos (row + column), got %d", count)
	}
}

func TestCountBingos_AllComplete(t *testing.T) {
	svc := &CardService{}

	// All 24 items completed
	items := make([]models.BingoItem, 0, 24)
	for i := 0; i < 25; i++ {
		if i != 12 { // Skip free space
			items = append(items, models.BingoItem{Position: i, IsCompleted: true})
		}
	}

	freePos := 12
	count := svc.countBingos(items, 5, &freePos)
	// 5 rows + 5 columns + 2 diagonals = 12
	if count != 12 {
		t.Errorf("expected 12 bingos when all complete, got %d", count)
	}
}

func TestCountBingos_PartialRow(t *testing.T) {
	svc := &CardService{}

	// First row almost complete, missing one
	items := []models.BingoItem{
		{Position: 0, IsCompleted: true},
		{Position: 1, IsCompleted: true},
		{Position: 2, IsCompleted: true},
		{Position: 3, IsCompleted: true},
		// Position 4 not completed
	}

	freePos := 12
	count := svc.countBingos(items, 5, &freePos)
	if count != 0 {
		t.Errorf("expected 0 bingos for partial row, got %d", count)
	}
}

func TestFindRandomPosition_EmptyCard(t *testing.T) {
	svc := &CardService{}

	freePos := 12
	card := &models.BingoCard{GridSize: 5, HasFreeSpace: true, FreeSpacePos: &freePos, Items: []models.BingoItem{}}
	pos, err := svc.findRandomPosition(card)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pos < 0 || pos >= 25 {
		t.Errorf("position out of range: %d", pos)
	}
	if pos == 12 {
		t.Error("position should not be free space (12)")
	}
}

func TestFindRandomPosition_PartiallyFilled(t *testing.T) {
	svc := &CardService{}

	existingItems := []models.BingoItem{
		{Position: 0},
		{Position: 1},
		{Position: 2},
	}

	freePos := 12
	card := &models.BingoCard{GridSize: 5, HasFreeSpace: true, FreeSpacePos: &freePos, Items: existingItems}
	pos, err := svc.findRandomPosition(card)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not return occupied positions or free space
	for _, item := range existingItems {
		if pos == item.Position {
			t.Errorf("returned occupied position: %d", pos)
		}
	}
	if pos == 12 {
		t.Error("returned free space position")
	}
}

func TestFindRandomPosition_AlmostFull(t *testing.T) {
	svc := &CardService{}

	// Fill all positions except 24
	existingItems := make([]models.BingoItem, 0, 23)
	for i := 0; i < 24; i++ {
		if i != 12 { // Skip free space
			existingItems = append(existingItems, models.BingoItem{Position: i})
		}
	}

	freePos := 12
	card := &models.BingoCard{GridSize: 5, HasFreeSpace: true, FreeSpacePos: &freePos, Items: existingItems}
	pos, err := svc.findRandomPosition(card)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pos != 24 {
		t.Errorf("expected position 24 (only available), got %d", pos)
	}
}

func TestFindRandomPosition_Full(t *testing.T) {
	svc := &CardService{}

	// Fill all positions
	existingItems := make([]models.BingoItem, 0, 24)
	for i := 0; i < 25; i++ {
		if i != 12 { // Skip free space
			existingItems = append(existingItems, models.BingoItem{Position: i})
		}
	}

	freePos := 12
	card := &models.BingoCard{GridSize: 5, HasFreeSpace: true, FreeSpacePos: &freePos, Items: existingItems}
	_, err := svc.findRandomPosition(card)
	if err != ErrCardFull {
		t.Errorf("expected ErrCardFull, got %v", err)
	}
}

func TestFindRandomPosition_Randomness(t *testing.T) {
	svc := &CardService{}

	positions := make(map[int]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		freePos := 12
		card := &models.BingoCard{GridSize: 5, HasFreeSpace: true, FreeSpacePos: &freePos, Items: []models.BingoItem{}}
		pos, _ := svc.findRandomPosition(card)
		positions[pos]++
	}

	// Check that we got variety (at least 10 different positions)
	if len(positions) < 10 {
		t.Errorf("expected more variety in positions, only got %d unique positions", len(positions))
	}

	// Check free space never returned
	if positions[12] > 0 {
		t.Error("free space (12) should never be returned")
	}
}

func TestCardServiceErrors(t *testing.T) {
	// Verify error constants are defined correctly
	errors := []error{
		ErrCardNotFound,
		ErrCardAlreadyExists,
		ErrCardFinalized,
		ErrCardNotFinalized,
		ErrCardFull,
		ErrItemNotFound,
		ErrPositionOccupied,
		ErrInvalidPosition,
		ErrNotCardOwner,
		ErrInvalidGridSize,
		ErrInvalidHeaderText,
		ErrNoSpaceForFree,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("error constant should not be nil")
		}
		if err.Error() == "" {
			t.Error("error message should not be empty")
		}
	}
}

func TestCardStats_Calculation(t *testing.T) {
	// This tests the logic that would be used in GetStats
	items := []models.BingoItem{
		{Position: 0, IsCompleted: true, CompletedAt: timePtr(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))},
		{Position: 1, IsCompleted: true, CompletedAt: timePtr(time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC))},
		{Position: 2, IsCompleted: false},
		{Position: 3, IsCompleted: true, CompletedAt: timePtr(time.Date(2024, 2, 10, 0, 0, 0, 0, time.UTC))},
	}

	totalItems := len(items)
	completedItems := 0
	var firstCompletion, lastCompletion *time.Time

	for _, item := range items {
		if item.IsCompleted {
			completedItems++
			if item.CompletedAt != nil {
				if firstCompletion == nil || item.CompletedAt.Before(*firstCompletion) {
					firstCompletion = item.CompletedAt
				}
				if lastCompletion == nil || item.CompletedAt.After(*lastCompletion) {
					lastCompletion = item.CompletedAt
				}
			}
		}
	}

	if totalItems != 4 {
		t.Errorf("expected 4 total items, got %d", totalItems)
	}
	if completedItems != 3 {
		t.Errorf("expected 3 completed items, got %d", completedItems)
	}

	expectedRate := float64(completedItems) / float64(totalItems) * 100
	if expectedRate != 75.0 {
		t.Errorf("expected 75%% completion rate, got %f", expectedRate)
	}

	if firstCompletion == nil || firstCompletion.Month() != time.January {
		t.Error("first completion should be in January")
	}
	if lastCompletion == nil || lastCompletion.Month() != time.March {
		t.Error("last completion should be in March")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestValidItemPosition_Default5x5WithFree(t *testing.T) {
	freePos := 12
	card := models.BingoCard{GridSize: 5, HasFreeSpace: true, FreeSpacePos: &freePos}

	tests := []struct {
		position int
		valid    bool
	}{
		{-1, false},
		{0, true},
		{11, true},
		{12, false}, // Free space
		{13, true},
		{24, true},
		{25, false},
		{100, false},
	}

	for _, tt := range tests {
		if got := card.IsValidItemPosition(tt.position); got != tt.valid {
			t.Errorf("position %d: expected valid=%v, got %v", tt.position, tt.valid, got)
		}
	}
}

func TestCardID_UUID(t *testing.T) {
	cardID := uuid.New()
	userID := uuid.New()

	card := &models.BingoCard{
		ID:     cardID,
		UserID: userID,
		Year:   2024,
	}

	if card.ID == uuid.Nil {
		t.Error("card ID should not be nil")
	}
	if card.UserID == uuid.Nil {
		t.Error("user ID should not be nil")
	}
	if card.ID == card.UserID {
		t.Error("card ID and user ID should be different")
	}
}
