package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func cardRowValues(cardID, userID uuid.UUID, gridSize int, hasFree bool, freePos *int, finalized bool) []any {
	now := time.Now()
	return []any{
		cardID,
		userID,
		2024,
		nil,
		nil,
		gridSize,
		"BINGO",
		hasFree,
		freePos,
		true,
		finalized,
		true,
		false,
		now,
		now,
	}
}

func lockCardRowValues(cardID, userID uuid.UUID, gridSize int, hasFree bool, freePos *int, finalized bool) []any {
	return []any{cardID, userID, gridSize, "BINGO", hasFree, freePos, finalized}
}

func newCardDB(cardID, userID uuid.UUID, gridSize int, hasFree bool, freePos *int, finalized bool, items [][]any) *fakeDB {
	return &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM bingo_cards") {
				return rowFromValues(cardRowValues(cardID, userID, gridSize, hasFree, freePos, finalized)...)
			}
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("unexpected query")
			}}
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if strings.Contains(sql, "FROM bingo_items") {
				return &fakeRows{rows: items}, nil
			}
			return &fakeRows{rows: [][]any{}}, nil
		},
	}
}

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

func TestCardService_Create_InvalidCategory(t *testing.T) {
	svc := &CardService{}
	category := "not-a-category"
	_, err := svc.Create(context.Background(), models.CreateCardParams{
		UserID:   uuid.New(),
		Year:     2024,
		Category: &category,
		GridSize: 5,
		Header:   "BINGO",
		HasFree:  true,
	})
	if !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("expected ErrInvalidCategory, got %v", err)
	}
}

func TestCardService_Create_TitleTooLong(t *testing.T) {
	svc := &CardService{}
	title := strings.Repeat("a", 101)
	_, err := svc.Create(context.Background(), models.CreateCardParams{
		UserID:   uuid.New(),
		Year:     2024,
		Title:    &title,
		GridSize: 5,
		Header:   "BINGO",
		HasFree:  true,
	})
	if !errors.Is(err, ErrTitleTooLong) {
		t.Fatalf("expected ErrTitleTooLong, got %v", err)
	}
}

func TestCardService_Create_InvalidGridSize(t *testing.T) {
	svc := &CardService{}
	_, err := svc.Create(context.Background(), models.CreateCardParams{
		UserID:   uuid.New(),
		Year:     2024,
		GridSize: 7,
		Header:   "BINGO",
		HasFree:  true,
	})
	if !errors.Is(err, ErrInvalidGridSize) {
		t.Fatalf("expected ErrInvalidGridSize, got %v", err)
	}
}

func TestCardService_Create_InvalidHeaderText(t *testing.T) {
	svc := &CardService{}
	_, err := svc.Create(context.Background(), models.CreateCardParams{
		UserID:   uuid.New(),
		Year:     2024,
		GridSize: 2,
		Header:   "BINGO",
		HasFree:  true,
	})
	if !errors.Is(err, ErrInvalidHeaderText) {
		t.Fatalf("expected ErrInvalidHeaderText, got %v", err)
	}
}

func TestCardService_Create_TitleConflict(t *testing.T) {
	userID := uuid.New()
	title := "My Card"
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(true)
		},
	}

	svc := NewCardService(db)
	_, err := svc.Create(context.Background(), models.CreateCardParams{
		UserID:   userID,
		Year:     2024,
		Title:    &title,
		GridSize: 5,
		Header:   "BINGO",
		HasFree:  true,
	})
	if !errors.Is(err, ErrCardTitleExists) {
		t.Fatalf("expected ErrCardTitleExists, got %v", err)
	}
}

func TestCardService_Create_YearConflictWithoutTitle(t *testing.T) {
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(true)
		},
	}

	svc := NewCardService(db)
	_, err := svc.Create(context.Background(), models.CreateCardParams{
		UserID:   userID,
		Year:     2024,
		GridSize: 5,
		Header:   "BINGO",
		HasFree:  true,
	})
	if !errors.Is(err, ErrCardAlreadyExists) {
		t.Fatalf("expected ErrCardAlreadyExists, got %v", err)
	}
}

func TestCardService_Create_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	title := "Card"
	category := models.ValidCategories[0]
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT EXISTS") {
				return rowFromValues(false)
			}
			if strings.Contains(sql, "INSERT INTO bingo_cards") {
				return rowFromValues(cardRowValues(cardID, userID, 5, true, nil, false)...)
			}
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("unexpected query")
			}}
		},
	}

	svc := NewCardService(db)
	card, err := svc.Create(context.Background(), models.CreateCardParams{
		UserID:   userID,
		Year:     2024,
		Title:    &title,
		Category: &category,
		GridSize: 5,
		Header:   "BINGO",
		HasFree:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.ID != cardID {
		t.Fatalf("expected card %v, got %v", cardID, card.ID)
	}
	if len(card.Items) != 0 {
		t.Fatalf("expected empty items, got %d", len(card.Items))
	}
}
func TestCardService_FindRandomPosition_SingleAvailable(t *testing.T) {
	svc := &CardService{}
	free := 2
	card := &models.BingoCard{
		GridSize:     2,
		HasFreeSpace: true,
		FreeSpacePos: &free,
		Items: []models.BingoItem{
			{Position: 0},
			{Position: 1},
		},
	}

	pos, err := svc.findRandomPosition(card)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos != 3 {
		t.Fatalf("expected position 3, got %d", pos)
	}
}

func TestCardService_FindRandomPosition_Full(t *testing.T) {
	svc := &CardService{}
	card := &models.BingoCard{
		GridSize: 2,
		Items: []models.BingoItem{
			{Position: 0},
			{Position: 1},
			{Position: 2},
			{Position: 3},
		},
	}

	_, err := svc.findRandomPosition(card)
	if !errors.Is(err, ErrCardFull) {
		t.Fatalf("expected ErrCardFull, got %v", err)
	}
}

func TestCardService_AddItem_InvalidPosition(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	free := 2
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(cardRowValues(cardID, userID, 2, true, &free, false)...)
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	svc := NewCardService(db)
	pos := 2
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:   cardID,
		Position: &pos,
		Content:  "Test",
	})
	if !errors.Is(err, ErrInvalidPosition) {
		t.Fatalf("expected ErrInvalidPosition, got %v", err)
	}
}

func TestCardService_AddItem_PositionOccupied(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, false, [][]any{
		{uuid.New(), cardID, 0, "Item", false, nil, nil, nil, time.Now()},
	})

	svc := NewCardService(db)
	pos := 0
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:   cardID,
		Position: &pos,
		Content:  "Test",
	})
	if !errors.Is(err, ErrPositionOccupied) {
		t.Fatalf("expected ErrPositionOccupied, got %v", err)
	}
}

func TestCardService_UpdateMeta_TitleConflict(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	title := "Conflicting"
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "SELECT EXISTS") {
				return rowFromValues(true)
			}
			if strings.Contains(sql, "FROM bingo_cards") {
				return rowFromValues(cardRowValues(cardID, userID, 5, true, nil, false)...)
			}
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("unexpected query")
			}}
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			t.Fatal("unexpected exec for conflicting title")
			return fakeCommandTag{}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.UpdateMeta(context.Background(), userID, cardID, models.UpdateCardMetaParams{
		Title: &title,
	})
	if !errors.Is(err, ErrCardTitleExists) {
		t.Fatalf("expected ErrCardTitleExists, got %v", err)
	}
}

func TestCardService_Shuffle_EmptyItems(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return nil, errors.New("begin should not be called")
	}

	svc := NewCardService(db)
	card, err := svc.Shuffle(context.Background(), userID, cardID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.ID != cardID {
		t.Fatalf("expected card ID %v, got %v", cardID, card.ID)
	}
}

func TestCardService_Shuffle_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.Shuffle(context.Background(), userID, cardID)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_UpdateVisibility_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UpdateVisibility(context.Background(), userID, cardID, true)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_BulkUpdateVisibility_Empty(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			t.Fatal("unexpected exec for empty cardIDs")
			return fakeCommandTag{}, nil
		},
	}

	svc := NewCardService(db)
	updated, err := svc.BulkUpdateVisibility(context.Background(), uuid.New(), nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 0 {
		t.Fatalf("expected 0 updated, got %d", updated)
	}
}

func TestCardService_UpdateItemNotes_NotFound(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UpdateItemNotes(context.Background(), userID, cardID, 3, nil, nil)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestCardService_UpdateItemNotes_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UpdateItemNotes(context.Background(), userID, cardID, 3, nil, nil)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_UpdateConfig_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{})
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_UpdateConfig_Finalized(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, true, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{})
	if !errors.Is(err, ErrCardFinalized) {
		t.Fatalf("expected ErrCardFinalized, got %v", err)
	}
}

func TestCardService_UpdateConfig_InvalidHeader(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, true, nil, false, [][]any{})
	header := "BINGO"

	svc := NewCardService(db)
	_, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{
		HeaderText: &header,
	})
	if !errors.Is(err, ErrInvalidHeaderText) {
		t.Fatalf("expected ErrInvalidHeaderText, got %v", err)
	}
}

func TestCardService_CompleteItem_NotFound(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, true, [][]any{})

	svc := NewCardService(db)
	_, err := svc.CompleteItem(context.Background(), userID, cardID, 3, models.CompleteItemParams{})
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestCardService_UncompleteItem_NotFound(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, true, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UncompleteItem(context.Background(), userID, cardID, 3)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestCardService_BulkDelete_Empty(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			t.Fatal("unexpected exec for empty cardIDs")
			return fakeCommandTag{}, nil
		},
	}

	svc := NewCardService(db)
	deleted, err := svc.BulkDelete(context.Background(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deleted, got %d", deleted)
	}
}

func TestCardService_BulkUpdateArchive_Empty(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			t.Fatal("unexpected exec for empty cardIDs")
			return fakeCommandTag{}, nil
		},
	}

	svc := NewCardService(db)
	updated, err := svc.BulkUpdateArchive(context.Background(), uuid.New(), nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 0 {
		t.Fatalf("expected 0 updated, got %d", updated)
	}
}

func TestCardService_BulkUpdateArchive_Count(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 4}, nil
		},
	}

	svc := NewCardService(db)
	updated, err := svc.BulkUpdateArchive(context.Background(), uuid.New(), []uuid.UUID{uuid.New()}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 4 {
		t.Fatalf("expected 4 updated, got %d", updated)
	}
}

func TestCardService_BulkUpdateArchive_Error(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	svc := NewCardService(db)
	_, err := svc.BulkUpdateArchive(context.Background(), uuid.New(), []uuid.UUID{uuid.New()}, true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_AddItem_Random_CardNotFound(t *testing.T) {
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					return fakeRow{scanFunc: func(dest ...any) error {
						return pgx.ErrNoRows
					}}
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.AddItem(context.Background(), uuid.New(), models.AddItemParams{
		CardID:  uuid.New(),
		Content: "Test",
	})
	if !errors.Is(err, ErrCardNotFound) {
		t.Fatalf("expected ErrCardNotFound, got %v", err)
	}
}

func TestCardService_AddItem_Random_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					return rowFromValues(lockCardRowValues(cardID, uuid.New(), 2, false, nil, false)...)
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:  cardID,
		Content: "Test",
	})
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_AddItem_Random_Finalized(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					return rowFromValues(lockCardRowValues(cardID, userID, 2, false, nil, true)...)
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:  cardID,
		Content: "Test",
	})
	if !errors.Is(err, ErrCardFinalized) {
		t.Fatalf("expected ErrCardFinalized, got %v", err)
	}
}

func TestCardService_AddItem_Random_CardFull(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					return rowFromValues(lockCardRowValues(cardID, userID, 2, false, nil, false)...)
				},
				QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
					return &fakeRows{rows: [][]any{{0}, {1}, {2}, {3}}}, nil
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:  cardID,
		Content: "Test",
	})
	if !errors.Is(err, ErrCardFull) {
		t.Fatalf("expected ErrCardFull, got %v", err)
	}
}

func TestCardService_AddItem_Random_RowsError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					return rowFromValues(lockCardRowValues(cardID, userID, 2, false, nil, false)...)
				},
				QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
					return &fakeRows{rows: [][]any{{0}}, err: errors.New("rows error")}, nil
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:  cardID,
		Content: "Test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_AddItem_Random_InsertConflict(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					if strings.Contains(sql, "FOR UPDATE") {
						return rowFromValues(lockCardRowValues(cardID, userID, 2, false, nil, false)...)
					}
					return fakeRow{scanFunc: func(dest ...any) error {
						return &pgconn.PgError{Code: "23505"}
					}}
				},
				QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
					return &fakeRows{rows: [][]any{}}, nil
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:  cardID,
		Content: "Test",
	})
	if !errors.Is(err, ErrPositionOccupied) {
		t.Fatalf("expected ErrPositionOccupied, got %v", err)
	}
}

func TestCardService_AddItem_Random_CommitError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					if strings.Contains(sql, "FOR UPDATE") {
						return rowFromValues(lockCardRowValues(cardID, userID, 2, false, nil, false)...)
					}
					return rowFromValues(
						uuid.New(),
						cardID,
						0,
						"Test",
						false,
						nil,
						nil,
						nil,
						time.Now(),
					)
				},
				QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
					return &fakeRows{rows: [][]any{}}, nil
				},
				CommitFunc: func(ctx context.Context) error { return errors.New("commit error") },
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:  cardID,
		Content: "Test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
func TestCardService_UpdateConfig_NoSpaceForFree(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 2, "C", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 3, "D", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{}, nil
	}

	svc := NewCardService(db)
	enableFree := true
	_, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{
		HasFreeSpace: &enableFree,
	})
	if !errors.Is(err, ErrNoSpaceForFree) {
		t.Fatalf("expected ErrNoSpaceForFree, got %v", err)
	}
}

func TestResolveCloneHasFreeSpace(t *testing.T) {
	if got := resolveCloneHasFreeSpace(true, nil); got != true {
		t.Fatalf("expected default true, got %v", got)
	}
	override := false
	if got := resolveCloneHasFreeSpace(true, &override); got != false {
		t.Fatalf("expected override false, got %v", got)
	}
}

func TestMapBingoCardsUniqueViolationToCardExistsError(t *testing.T) {
	err := mapBingoCardsUniqueViolationToCardExistsError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_bingo_cards_user_year_null_title",
	}, nil)
	if !errors.Is(err, ErrCardAlreadyExists) {
		t.Fatalf("expected ErrCardAlreadyExists, got %v", err)
	}

	title := "Title"
	err = mapBingoCardsUniqueViolationToCardExistsError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "idx_bingo_cards_user_year_title",
	}, &title)
	if !errors.Is(err, ErrCardTitleExists) {
		t.Fatalf("expected ErrCardTitleExists, got %v", err)
	}

	err = mapBingoCardsUniqueViolationToCardExistsError(&pgconn.PgError{
		Code:           "23505",
		ConstraintName: "unknown",
	}, nil)
	if !errors.Is(err, ErrCardAlreadyExists) {
		t.Fatalf("expected ErrCardAlreadyExists fallback, got %v", err)
	}
}

func TestCardService_Clone_InvalidGridSize(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.Clone(context.Background(), userID, cardID, CloneParams{
		GridSize: 7,
	})
	if !errors.Is(err, ErrInvalidGridSize) {
		t.Fatalf("expected ErrInvalidGridSize, got %v", err)
	}
}

func TestCardService_Clone_InvalidHeader(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.Clone(context.Background(), userID, cardID, CloneParams{
		GridSize:   2,
		HeaderText: "BINGO",
	})
	if !errors.Is(err, ErrInvalidHeaderText) {
		t.Fatalf("expected ErrInvalidHeaderText, got %v", err)
	}
}

func TestCardService_Clone_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.Clone(context.Background(), userID, cardID, CloneParams{})
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_Clone_DuplicateTitleMapped(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					return &pgconn.PgError{
						Code:           "23505",
						ConstraintName: "idx_bingo_cards_user_year_title",
					}
				}}
			},
		}, nil
	}

	svc := NewCardService(db)
	_, err := svc.Clone(context.Background(), userID, cardID, CloneParams{})
	if !errors.Is(err, ErrCardTitleExists) {
		t.Fatalf("expected ErrCardTitleExists, got %v", err)
	}
}

func TestCardService_Clone_BeginError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return nil, errors.New("begin error")
	}

	svc := NewCardService(db)
	_, err := svc.Clone(context.Background(), userID, cardID, CloneParams{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Clone_CreateError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					return errors.New("create error")
				}}
			},
		}, nil
	}

	svc := NewCardService(db)
	_, err := svc.Clone(context.Background(), userID, cardID, CloneParams{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Clone_CopyItemError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				return rowFromValues(cardRowValues(uuid.New(), userID, 2, false, nil, false)...)
			},
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{}, errors.New("copy error")
			},
		}, nil
	}

	svc := NewCardService(db)
	_, err := svc.Clone(context.Background(), userID, cardID, CloneParams{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Clone_CommitError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				return rowFromValues(cardRowValues(uuid.New(), userID, 2, false, nil, false)...)
			},
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error {
				return errors.New("commit error")
			},
		}, nil
	}

	svc := NewCardService(db)
	_, err := svc.Clone(context.Background(), userID, cardID, CloneParams{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_GetByID_NotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrCardNotFound) {
		t.Fatalf("expected ErrCardNotFound, got %v", err)
	}
}

func TestCardService_GetByID_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_GetByID_ItemsError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(cardRowValues(cardID, userID, 2, false, nil, false)...)
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("items error")
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetByID(context.Background(), cardID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_GetByUserAndYear_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)

	svc := NewCardService(db)
	card, err := svc.GetByUserAndYear(context.Background(), userID, 2024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.ID != cardID {
		t.Fatalf("expected card %v, got %v", cardID, card.ID)
	}
	if len(card.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(card.Items))
	}
}

func TestCardService_GetByUserAndYear_NotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetByUserAndYear(context.Background(), uuid.New(), 2024)
	if !errors.Is(err, ErrCardNotFound) {
		t.Fatalf("expected ErrCardNotFound, got %v", err)
	}
}

func TestCardService_GetByUserAndYear_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetByUserAndYear(context.Background(), uuid.New(), 2024)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_GetByUserAndYear_ItemsError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(cardRowValues(cardID, userID, 2, false, nil, false)...)
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("items error")
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetByUserAndYear(context.Background(), userID, 2024)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_AddItem_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 2, false, nil, false, [][]any{})

	svc := NewCardService(db)
	pos := 0
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:   cardID,
		Position: &pos,
		Content:  "Test",
	})
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_UpdateMeta_InvalidCategory(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	bad := "invalid"

	svc := NewCardService(db)
	_, err := svc.UpdateMeta(context.Background(), userID, cardID, models.UpdateCardMetaParams{
		Category: &bad,
	})
	if !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("expected ErrInvalidCategory, got %v", err)
	}
}

func TestCardService_ListByUser_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	cardID2 := uuid.New()
	items := map[uuid.UUID][][]any{
		cardID: {
			{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		},
		cardID2: {
			{uuid.New(), cardID2, 1, "B", false, nil, nil, nil, time.Now()},
			{uuid.New(), cardID2, 2, "C", false, nil, nil, nil, time.Now()},
		},
	}

	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if strings.Contains(sql, "FROM bingo_cards") {
				return &fakeRows{rows: [][]any{
					cardRowValues(cardID, userID, 2, false, nil, false),
					cardRowValues(cardID2, userID, 2, false, nil, false),
				}}, nil
			}
			if strings.Contains(sql, "FROM bingo_items") {
				cardID := args[0].(uuid.UUID)
				return &fakeRows{rows: items[cardID]}, nil
			}
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	svc := NewCardService(db)
	cards, err := svc.ListByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}
	if len(cards[1].Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(cards[1].Items))
	}
}

func TestCardService_ListByUser_QueryError(t *testing.T) {
	userID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("boom")
		},
	}

	svc := NewCardService(db)
	_, err := svc.ListByUser(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_ListByUser_ScanError(t *testing.T) {
	userID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.ListByUser(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_ListByUser_ItemsError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if strings.Contains(sql, "FROM bingo_cards") {
				return &fakeRows{rows: [][]any{
					cardRowValues(cardID, userID, 2, false, nil, false),
				}}, nil
			}
			return nil, errors.New("boom")
		},
	}

	svc := NewCardService(db)
	_, err := svc.ListByUser(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_UpdateMeta_TitleTooLong(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	long := strings.Repeat("a", 101)

	svc := NewCardService(db)
	_, err := svc.UpdateMeta(context.Background(), userID, cardID, models.UpdateCardMetaParams{
		Title: &long,
	})
	if !errors.Is(err, ErrTitleTooLong) {
		t.Fatalf("expected ErrTitleTooLong, got %v", err)
	}
}

func TestCardService_UpdateMeta_TitleExists(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.QueryRowFunc = func(ctx context.Context, sql string, args ...any) Row {
		if strings.Contains(sql, "EXISTS") {
			return fakeRow{scanFunc: func(dest ...any) error {
				return assignRow(dest, []any{true})
			}}
		}
		return rowFromValues(cardRowValues(cardID, userID, 5, true, nil, false)...)
	}
	title := "Dup"

	svc := NewCardService(db)
	_, err := svc.UpdateMeta(context.Background(), userID, cardID, models.UpdateCardMetaParams{
		Title: &title,
	})
	if !errors.Is(err, ErrCardTitleExists) {
		t.Fatalf("expected ErrCardTitleExists, got %v", err)
	}
}

func TestCardService_UpdateMeta_TitleCheckError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.QueryRowFunc = func(ctx context.Context, sql string, args ...any) Row {
		if strings.Contains(sql, "EXISTS") {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		}
		return rowFromValues(cardRowValues(cardID, userID, 5, true, nil, false)...)
	}
	title := "Unique"

	svc := NewCardService(db)
	_, err := svc.UpdateMeta(context.Background(), userID, cardID, models.UpdateCardMetaParams{
		Title: &title,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_SwapItems_NoOp(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	if err := svc.SwapItems(context.Background(), userID, cardID, 1, 1); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCardService_SwapItems_InvalidPosition(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, true, nil, false, [][]any{})

	svc := NewCardService(db)
	err := svc.SwapItems(context.Background(), userID, cardID, -1, 2)
	if !errors.Is(err, ErrInvalidPosition) {
		t.Fatalf("expected ErrInvalidPosition, got %v", err)
	}
}

func TestCardService_SwapItems_Finalized(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, true, [][]any{})

	svc := NewCardService(db)
	err := svc.SwapItems(context.Background(), userID, cardID, 0, 1)
	if !errors.Is(err, ErrCardFinalized) {
		t.Fatalf("expected ErrCardFinalized, got %v", err)
	}
}

func TestCardService_SwapItems_ItemsMissing(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	err := svc.SwapItems(context.Background(), userID, cardID, 0, 1)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestCardService_MoveFreeSpace_NoFreeSpace(t *testing.T) {
	card := &models.BingoCard{GridSize: 5, HasFreeSpace: false}
	svc := &CardService{}
	err := svc.moveFreeSpace(context.Background(), card, 0, 1)
	if !errors.Is(err, ErrInvalidPosition) {
		t.Fatalf("expected ErrInvalidPosition, got %v", err)
	}
}

func TestCardService_CompleteItem_NotFinalized(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.CompleteItem(context.Background(), userID, cardID, 0, models.CompleteItemParams{})
	if !errors.Is(err, ErrCardNotFinalized) {
		t.Fatalf("expected ErrCardNotFinalized, got %v", err)
	}
}

func TestCardService_CompleteItem_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 5, true, nil, true, [][]any{})

	svc := NewCardService(db)
	_, err := svc.CompleteItem(context.Background(), userID, cardID, 0, models.CompleteItemParams{})
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_UncompleteItem_NotFinalized(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UncompleteItem(context.Background(), userID, cardID, 0)
	if !errors.Is(err, ErrCardNotFinalized) {
		t.Fatalf("expected ErrCardNotFinalized, got %v", err)
	}
}

func TestCardService_UncompleteItem_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 5, true, nil, true, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UncompleteItem(context.Background(), userID, cardID, 0)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_RemoveItem_NotFound(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 0}, nil
	}

	svc := NewCardService(db)
	err := svc.RemoveItem(context.Background(), userID, cardID, 0)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestCardService_RemoveItem_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	err := svc.RemoveItem(context.Background(), userID, cardID, 0)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_RemoveItem_Finalized(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, true, [][]any{})

	svc := NewCardService(db)
	err := svc.RemoveItem(context.Background(), userID, cardID, 0)
	if !errors.Is(err, ErrCardFinalized) {
		t.Fatalf("expected ErrCardFinalized, got %v", err)
	}
}

func TestCardService_Delete_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 5, true, nil, false, [][]any{})

	svc := NewCardService(db)
	err := svc.Delete(context.Background(), userID, cardID)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_UpdateVisibility_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 1}, nil
	}

	svc := NewCardService(db)
	card, err := svc.UpdateVisibility(context.Background(), userID, cardID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !card.VisibleToFriends {
		t.Fatal("expected VisibleToFriends true")
	}
}

func TestCardService_UpdateVisibility_DBError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{}, errors.New("boom")
	}

	svc := NewCardService(db)
	_, err := svc.UpdateVisibility(context.Background(), userID, cardID, true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_BulkUpdateVisibility_Count(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 3}, nil
		},
	}

	svc := NewCardService(db)
	count, err := svc.BulkUpdateVisibility(context.Background(), uuid.New(), []uuid.UUID{uuid.New(), uuid.New()}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 updated, got %d", count)
	}
}

func TestCardService_BulkUpdateVisibility_Error(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	svc := NewCardService(db)
	_, err := svc.BulkUpdateVisibility(context.Background(), uuid.New(), []uuid.UUID{uuid.New()}, true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_BulkDelete_Count(t *testing.T) {
	call := 0
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			call++
			if call == 1 {
				return fakeCommandTag{rowsAffected: 5}, nil
			}
			return fakeCommandTag{rowsAffected: 2}, nil
		},
	}

	svc := NewCardService(db)
	count, err := svc.BulkDelete(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 deleted, got %d", count)
	}
	if call != 2 {
		t.Fatalf("expected 2 exec calls, got %d", call)
	}
}

func TestCardService_BulkDelete_ItemsError(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("items error")
		},
	}

	svc := NewCardService(db)
	_, err := svc.BulkDelete(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_BulkDelete_CardsError(t *testing.T) {
	call := 0
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			call++
			if call == 1 {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			return fakeCommandTag{}, errors.New("cards error")
		},
	}

	svc := NewCardService(db)
	_, err := svc.BulkDelete(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_MoveFreeSpace_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	free := 0
	card := &models.BingoCard{
		ID:           cardID,
		UserID:       userID,
		GridSize:     2,
		HasFreeSpace: true,
		FreeSpacePos: &free,
		Items: []models.BingoItem{
			{ID: uuid.New(), Position: 2},
		},
	}

	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
					return fakeCommandTag{rowsAffected: 1}, nil
				},
				CommitFunc: func(ctx context.Context) error { return nil },
			}, nil
		},
	}

	svc := NewCardService(db)
	if err := svc.moveFreeSpace(context.Background(), card, 1, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_MoveFreeSpace_UpdateError(t *testing.T) {
	free := 0
	card := &models.BingoCard{
		ID:           uuid.New(),
		GridSize:     2,
		HasFreeSpace: true,
		FreeSpacePos: &free,
		Items:        []models.BingoItem{},
	}

	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
					return fakeCommandTag{}, errors.New("update error")
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	err := svc.moveFreeSpace(context.Background(), card, 0, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_MoveFreeSpace_RelocateError(t *testing.T) {
	free := 0
	card := &models.BingoCard{
		ID:           uuid.New(),
		GridSize:     2,
		HasFreeSpace: true,
		FreeSpacePos: &free,
		Items: []models.BingoItem{
			{ID: uuid.New(), Position: 1},
		},
	}
	call := 0
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
					call++
					if call == 2 {
						return fakeCommandTag{}, errors.New("relocate error")
					}
					return fakeCommandTag{rowsAffected: 1}, nil
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	err := svc.moveFreeSpace(context.Background(), card, 0, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_MoveFreeSpace_CommitError(t *testing.T) {
	free := 0
	card := &models.BingoCard{
		ID:           uuid.New(),
		GridSize:     2,
		HasFreeSpace: true,
		FreeSpacePos: &free,
	}

	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
					return fakeCommandTag{rowsAffected: 1}, nil
				},
				CommitFunc: func(ctx context.Context) error {
					return errors.New("commit error")
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	err := svc.moveFreeSpace(context.Background(), card, 0, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_MoveFreeSpace_NoSpaceForFree(t *testing.T) {
	free := 0
	card := &models.BingoCard{
		ID:           uuid.New(),
		GridSize:     2,
		HasFreeSpace: true,
		FreeSpacePos: &free,
		Items: []models.BingoItem{
			{ID: uuid.New(), Position: 0},
			{ID: uuid.New(), Position: 1},
			{ID: uuid.New(), Position: 2},
			{ID: uuid.New(), Position: 3},
		},
	}

	svc := NewCardService(&fakeDB{})
	err := svc.moveFreeSpace(context.Background(), card, 0, 1)
	if !errors.Is(err, ErrNoSpaceForFree) {
		t.Fatalf("expected ErrNoSpaceForFree, got %v", err)
	}
}

func TestCardService_SwapItems_BothItems(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	svc := NewCardService(db)
	if err := svc.SwapItems(context.Background(), userID, cardID, 0, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_SwapItems_MoveTempError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{}, errors.New("move temp error")
			},
		}, nil
	}

	svc := NewCardService(db)
	err := svc.SwapItems(context.Background(), userID, cardID, 0, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_SwapItems_BeginError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return nil, errors.New("begin error")
	}

	svc := NewCardService(db)
	err := svc.SwapItems(context.Background(), userID, cardID, 0, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_SwapItems_SingleItem(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	svc := NewCardService(db)
	if err := svc.SwapItems(context.Background(), userID, cardID, 0, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_SwapItems_SecondItemOnly(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 1, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	svc := NewCardService(db)
	if err := svc.SwapItems(context.Background(), userID, cardID, 0, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_SwapItems_CommitError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return errors.New("commit error") },
		}, nil
	}

	svc := NewCardService(db)
	err := svc.SwapItems(context.Background(), userID, cardID, 0, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_AddItem_Explicit_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, false, [][]any{})
	db.QueryRowFunc = func(ctx context.Context, sql string, args ...any) Row {
		if strings.Contains(sql, "FROM bingo_cards") {
			return rowFromValues(cardRowValues(cardID, userID, 2, false, nil, false)...)
		}
		return rowFromValues(
			uuid.New(),
			cardID,
			1,
			"Test",
			false,
			nil,
			nil,
			nil,
			time.Now(),
		)
	}

	svc := NewCardService(db)
	pos := 1
	item, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:   cardID,
		Position: &pos,
		Content:  "Test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Position != 1 {
		t.Fatalf("expected position 1, got %d", item.Position)
	}
}

func TestCardService_AddItem_Explicit_InsertConflict(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, false, [][]any{})
	db.QueryRowFunc = func(ctx context.Context, sql string, args ...any) Row {
		if strings.Contains(sql, "FROM bingo_cards") {
			return rowFromValues(cardRowValues(cardID, userID, 2, false, nil, false)...)
		}
		return fakeRow{scanFunc: func(dest ...any) error {
			return &pgconn.PgError{Code: "23505"}
		}}
	}

	svc := NewCardService(db)
	pos := 1
	_, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:   cardID,
		Position: &pos,
		Content:  "Test",
	})
	if !errors.Is(err, ErrPositionOccupied) {
		t.Fatalf("expected ErrPositionOccupied, got %v", err)
	}
}

func TestCardService_UpdateItem_Finalized(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, true, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UpdateItem(context.Background(), userID, cardID, 0, models.UpdateItemParams{})
	if !errors.Is(err, ErrCardFinalized) {
		t.Fatalf("expected ErrCardFinalized, got %v", err)
	}
}

func TestCardService_UpdateItem_ItemNotFound(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UpdateItem(context.Background(), userID, cardID, 0, models.UpdateItemParams{})
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestCardService_UpdateItem_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 2, false, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.UpdateItem(context.Background(), userID, cardID, 0, models.UpdateItemParams{})
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_Shuffle_WithItems(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	svc := NewCardService(db)
	if _, err := svc.Shuffle(context.Background(), userID, cardID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_Shuffle_ClearError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{}, errors.New("clear error")
			},
		}, nil
	}

	svc := NewCardService(db)
	_, err := svc.Shuffle(context.Background(), userID, cardID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Shuffle_AssignError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	call := 0
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				call++
				if call == 3 {
					return fakeCommandTag{}, errors.New("assign error")
				}
				return fakeCommandTag{rowsAffected: 1}, nil
			},
		}, nil
	}

	svc := NewCardService(db)
	_, err := svc.Shuffle(context.Background(), userID, cardID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Shuffle_CommitError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return errors.New("commit error") },
		}, nil
	}

	svc := NewCardService(db)
	_, err := svc.Shuffle(context.Background(), userID, cardID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Shuffle_BeginError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return nil, errors.New("begin error")
	}

	svc := NewCardService(db)
	_, err := svc.Shuffle(context.Background(), userID, cardID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Finalize_MissingItems(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)

	svc := NewCardService(db)
	_, err := svc.Finalize(context.Background(), userID, cardID, nil)
	if err == nil {
		t.Fatal("expected error for missing items")
	}
}

func TestCardService_Finalize_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 2, false, nil, false, [][]any{})

	svc := NewCardService(db)
	_, err := svc.Finalize(context.Background(), userID, cardID, nil)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_Finalize_UpdateError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 2, "C", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 3, "D", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{}, errors.New("update error")
	}

	svc := NewCardService(db)
	_, err := svc.Finalize(context.Background(), userID, cardID, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_CheckForConflict_NotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	svc := NewCardService(db)
	_, err := svc.CheckForConflict(context.Background(), uuid.New(), 2024, nil)
	if !errors.Is(err, ErrCardNotFound) {
		t.Fatalf("expected ErrCardNotFound, got %v", err)
	}
}

func TestCardService_Import_InvalidCategory(t *testing.T) {
	svc := &CardService{}
	bad := "invalid"
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		Category: &bad,
		GridSize: 2,
	})
	if !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("expected ErrInvalidCategory, got %v", err)
	}
}

func TestCardService_Import_TitleTooLong(t *testing.T) {
	svc := &CardService{}
	title := strings.Repeat("a", 101)
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		Title:    &title,
		GridSize: 2,
	})
	if !errors.Is(err, ErrTitleTooLong) {
		t.Fatalf("expected ErrTitleTooLong, got %v", err)
	}
}

func TestCardService_Import_InvalidGridSize(t *testing.T) {
	svc := &CardService{}
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		GridSize: 7,
	})
	if !errors.Is(err, ErrInvalidGridSize) {
		t.Fatalf("expected ErrInvalidGridSize, got %v", err)
	}
}

func TestCardService_Import_InvalidHeader(t *testing.T) {
	svc := &CardService{}
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		GridSize:   2,
		HeaderText: "BINGO",
	})
	if !errors.Is(err, ErrInvalidHeaderText) {
		t.Fatalf("expected ErrInvalidHeaderText, got %v", err)
	}
}

func TestCardService_UpdateConfig_EnableFree_OddGrid(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 3, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	svc := NewCardService(db)
	enable := true
	if _, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{
		HasFreeSpace: &enable,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_UpdateConfig_RelocateError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 4, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 3, false, nil, false, items)
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				if strings.Contains(sql, "UPDATE bingo_items") {
					return fakeCommandTag{}, errors.New("relocate error")
				}
				return fakeCommandTag{rowsAffected: 1}, nil
			},
		}, nil
	}

	svc := NewCardService(db)
	enable := true
	_, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{
		HasFreeSpace: &enable,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_UpdateConfig_UpdateError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, false, [][]any{})
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{}, errors.New("update error")
			},
		}, nil
	}
	header := "BI"

	svc := NewCardService(db)
	_, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{
		HeaderText: &header,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_UpdateConfig_CommitError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, false, [][]any{})
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error {
				return errors.New("commit error")
			},
		}, nil
	}
	header := "BI"

	svc := NewCardService(db)
	_, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{
		HeaderText: &header,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Finalize_AlreadyFinalized(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, true, [][]any{})

	svc := NewCardService(db)
	card, err := svc.Finalize(context.Background(), userID, cardID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !card.IsFinalized {
		t.Fatal("expected card to be finalized")
	}
}

func TestCardService_CheckForConflict_Found(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)

	svc := NewCardService(db)
	title := "Title"
	card, err := svc.CheckForConflict(context.Background(), userID, 2024, &title)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.ID != cardID {
		t.Fatalf("expected card %v, got %v", cardID, card.ID)
	}
	if len(card.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(card.Items))
	}
}

func TestCardService_CheckForConflict_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	svc := NewCardService(db)
	_, err := svc.CheckForConflict(context.Background(), uuid.New(), 2024, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_CheckForConflict_ItemsError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(cardRowValues(cardID, userID, 2, false, nil, false)...)
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("items error")
		},
	}

	svc := NewCardService(db)
	_, err := svc.CheckForConflict(context.Background(), userID, 2024, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_UpdateMeta_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 1}, nil
	}
	category := models.ValidCategories[0]

	svc := NewCardService(db)
	if _, err := svc.UpdateMeta(context.Background(), userID, cardID, models.UpdateCardMetaParams{
		Category: &category,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_UpdateMeta_DBError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{}, errors.New("boom")
	}
	category := models.ValidCategories[0]

	svc := NewCardService(db)
	_, err := svc.UpdateMeta(context.Background(), userID, cardID, models.UpdateCardMetaParams{
		Category: &category,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Create_ExistsQueryError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	svc := NewCardService(db)
	_, err := svc.Create(context.Background(), models.CreateCardParams{
		UserID:   uuid.New(),
		Year:     2024,
		GridSize: 5,
		Header:   "BINGO",
		HasFree:  true,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_GetArchive_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if strings.Contains(sql, "FROM bingo_cards") {
				return &fakeRows{rows: [][]any{
					cardRowValues(cardID, userID, 2, false, nil, true),
				}}, nil
			}
			return &fakeRows{rows: items}, nil
		},
	}

	svc := NewCardService(db)
	archive, err := svc.GetArchive(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(archive) != 1 {
		t.Fatalf("expected 1 card, got %d", len(archive))
	}
	if len(archive[0].Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(archive[0].Items))
	}
}

func TestCardService_GetArchive_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("boom")
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetArchive(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_GetArchive_ScanError(t *testing.T) {
	userID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if strings.Contains(sql, "FROM bingo_cards") {
				return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
			}
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetArchive(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_GetArchive_ItemsError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if strings.Contains(sql, "FROM bingo_cards") {
				return &fakeRows{rows: [][]any{
					cardRowValues(cardID, userID, 2, false, nil, true),
				}}, nil
			}
			return nil, errors.New("items error")
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetArchive(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Import_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	center := 4
	now := time.Now()
	call := 0

	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					call++
					if call == 1 {
						return rowFromValues(
							cardID,
							userID,
							2024,
							nil,
							nil,
							3,
							"BING",
							true,
							&center,
							true,
							false,
							true,
							false,
							now,
							now,
						)
					}
					return rowFromValues(
						uuid.New(),
						cardID,
						0,
						"Item",
						false,
						nil,
						nil,
						nil,
						now,
					)
				},
				CommitFunc: func(ctx context.Context) error { return nil },
			}, nil
		},
	}

	svc := NewCardService(db)
	card, err := svc.Import(context.Background(), models.ImportCardParams{
		UserID:       userID,
		Year:         2024,
		GridSize:     3,
		HeaderText:   "BIN",
		HasFreeSpace: true,
		Items: []models.ImportItem{
			{Position: 0, Content: "Item"},
			{Position: 1, Content: "Item2"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.FreeSpacePos == nil || *card.FreeSpacePos != center {
		t.Fatalf("expected free space %d, got %v", center, card.FreeSpacePos)
	}
	if len(card.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(card.Items))
	}
}

func TestCardService_Import_BeginError(t *testing.T) {
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return nil, errors.New("begin error")
		},
	}

	svc := NewCardService(db)
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		UserID:   uuid.New(),
		Year:     2024,
		GridSize: 2,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Import_CreateCardError(t *testing.T) {
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					return fakeRow{scanFunc: func(dest ...any) error {
						return errors.New("create error")
					}}
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		UserID:   uuid.New(),
		Year:     2024,
		GridSize: 2,
		Items: []models.ImportItem{
			{Position: 0, Content: "A"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Import_CreateItemError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	now := time.Now()
	call := 0
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					call++
					if call == 1 {
						return rowFromValues(
							cardID,
							userID,
							2024,
							nil,
							nil,
							2,
							"BING",
							false,
							nil,
							true,
							false,
							true,
							false,
							now,
							now,
						)
					}
					return fakeRow{scanFunc: func(dest ...any) error {
						return errors.New("item error")
					}}
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		UserID:   userID,
		Year:     2024,
		GridSize: 2,
		Items: []models.ImportItem{
			{Position: 0, Content: "A"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Import_CommitError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	now := time.Now()
	call := 0
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					call++
					if call == 1 {
						return rowFromValues(
							cardID,
							userID,
							2024,
							nil,
							nil,
							2,
							"BING",
							false,
							nil,
							true,
							false,
							true,
							false,
							now,
							now,
						)
					}
					return rowFromValues(
						uuid.New(),
						cardID,
						0,
						"Item",
						false,
						nil,
						nil,
						nil,
						now,
					)
				},
				CommitFunc: func(ctx context.Context) error {
					return errors.New("commit error")
				},
			}, nil
		},
	}

	svc := NewCardService(db)
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		UserID:   userID,
		Year:     2024,
		GridSize: 2,
		Items: []models.ImportItem{
			{Position: 0, Content: "Item"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_GetStats_AllCompleted(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	now := time.Now()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", true, &now, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", true, &now, nil, nil, time.Now()},
		{uuid.New(), cardID, 2, "C", true, &now, nil, nil, time.Now()},
		{uuid.New(), cardID, 3, "D", true, &now, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, true, items)

	svc := NewCardService(db)
	stats, err := svc.GetStats(context.Background(), userID, cardID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.CompletedItems != 4 {
		t.Fatalf("expected 4 completed, got %d", stats.CompletedItems)
	}
	if stats.CompletionRate != 100 {
		t.Fatalf("expected 100 completion rate, got %v", stats.CompletionRate)
	}
	if stats.BingosAchieved != 6 {
		t.Fatalf("expected 6 bingos, got %d", stats.BingosAchieved)
	}
	if stats.FirstCompletion == nil || stats.LastCompletion == nil {
		t.Fatal("expected completion timestamps")
	}
}

func TestCardService_GetStats_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, uuid.New(), 2, false, nil, true, [][]any{})

	svc := NewCardService(db)
	_, err := svc.GetStats(context.Background(), userID, cardID)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_Import_InvalidPosition(t *testing.T) {
	svc := &CardService{}
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		GridSize: 2,
		Items: []models.ImportItem{
			{Position: 10, Content: "Item"},
		},
	})
	if !errors.Is(err, ErrInvalidPosition) {
		t.Fatalf("expected ErrInvalidPosition, got %v", err)
	}
}

func TestCardService_Import_PositionOccupied(t *testing.T) {
	svc := &CardService{}
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		GridSize: 2,
		Items: []models.ImportItem{
			{Position: 0, Content: "Item"},
			{Position: 0, Content: "Item2"},
		},
	})
	if !errors.Is(err, ErrPositionOccupied) {
		t.Fatalf("expected ErrPositionOccupied, got %v", err)
	}
}

func TestCardService_Import_FinalizeMissingItems(t *testing.T) {
	svc := &CardService{}
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		GridSize: 2,
		Finalize: true,
		Items: []models.ImportItem{
			{Position: 0, Content: "Item"},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing items")
	}
}

func TestCardService_AddItem_Random_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					if strings.Contains(sql, "FOR UPDATE") {
						return rowFromValues(lockCardRowValues(cardID, userID, 2, false, nil, false)...)
					}
					return rowFromValues(
						uuid.New(),
						cardID,
						0,
						"Test",
						false,
						nil,
						nil,
						nil,
						time.Now(),
					)
				},
				QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
					return &fakeRows{rows: [][]any{}}, nil
				},
				CommitFunc: func(ctx context.Context) error { return nil },
			}, nil
		},
	}

	svc := NewCardService(db)
	if _, err := svc.AddItem(context.Background(), userID, models.AddItemParams{
		CardID:  cardID,
		Content: "Test",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_Finalize_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 2, "C", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 3, "D", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 1}, nil
	}

	svc := NewCardService(db)
	card, err := svc.Finalize(context.Background(), userID, cardID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !card.IsFinalized {
		t.Fatal("expected card finalized")
	}
}

func TestCardService_Finalize_VisibleOverride(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 2, "C", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 3, "D", false, nil, nil, nil, time.Now()},
	}
	now := time.Now()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(
				cardID,
				userID,
				2024,
				nil,
				nil,
				2,
				"BI",
				false,
				nil,
				true,
				false,
				true,
				false,
				now,
				now,
			)
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: items}, nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if len(args) < 2 || args[1] != false {
				t.Fatalf("expected visibleToFriends=false, got %v", args)
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewCardService(db)
	visible := false
	card, err := svc.Finalize(context.Background(), userID, cardID, &FinalizeParams{
		VisibleToFriends: &visible,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.VisibleToFriends {
		t.Fatal("expected visibleToFriends false")
	}
}

func TestCardService_RemoveItem_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, false, [][]any{})
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 1}, nil
	}

	svc := NewCardService(db)
	if err := svc.RemoveItem(context.Background(), userID, cardID, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_RemoveItem_DBError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 2, false, nil, false, [][]any{})
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{}, errors.New("db error")
	}

	svc := NewCardService(db)
	err := svc.RemoveItem(context.Background(), userID, cardID, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_UpdateItem_ContentSuccess(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "Old", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 1}, nil
	}

	svc := NewCardService(db)
	content := "New"
	item, err := svc.UpdateItem(context.Background(), userID, cardID, 0, models.UpdateItemParams{
		Content: &content,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Content != "New" {
		t.Fatalf("expected updated content, got %s", item.Content)
	}
}

func TestCardService_UpdateItem_ContentError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "Old", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{}, errors.New("update error")
	}

	svc := NewCardService(db)
	content := "New"
	_, err := svc.UpdateItem(context.Background(), userID, cardID, 0, models.UpdateItemParams{
		Content: &content,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_UpdateItem_PositionError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "Old", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{}, errors.New("update error")
	}

	svc := NewCardService(db)
	newPos := 1
	_, err := svc.UpdateItem(context.Background(), userID, cardID, 0, models.UpdateItemParams{
		Position: &newPos,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_UpdateItem_PositionOccupied(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)

	svc := NewCardService(db)
	newPos := 1
	_, err := svc.UpdateItem(context.Background(), userID, cardID, 0, models.UpdateItemParams{
		Position: &newPos,
	})
	if !errors.Is(err, ErrPositionOccupied) {
		t.Fatalf("expected ErrPositionOccupied, got %v", err)
	}
}

func TestCardService_UpdateItem_PositionInvalid(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)

	svc := NewCardService(db)
	newPos := 9
	_, err := svc.UpdateItem(context.Background(), userID, cardID, 0, models.UpdateItemParams{
		Position: &newPos,
	})
	if !errors.Is(err, ErrInvalidPosition) {
		t.Fatalf("expected ErrInvalidPosition, got %v", err)
	}
}

func TestCardService_UpdateItem_PositionSuccess(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 1}, nil
	}

	svc := NewCardService(db)
	newPos := 2
	item, err := svc.UpdateItem(context.Background(), userID, cardID, 0, models.UpdateItemParams{
		Position: &newPos,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Position != 2 {
		t.Fatalf("expected position 2, got %d", item.Position)
	}
}

func TestCardService_CompleteItem_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 2, "C", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 3, "D", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, true, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 1}, nil
	}

	svc := NewCardService(db)
	item, err := svc.CompleteItem(context.Background(), userID, cardID, 0, models.CompleteItemParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !item.IsCompleted || item.CompletedAt == nil {
		t.Fatal("expected item completed")
	}
}

func TestCardService_UncompleteItem_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	now := time.Now()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", true, &now, nil, nil, time.Now()},
		{uuid.New(), cardID, 1, "B", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 2, "C", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 3, "D", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, true, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 1}, nil
	}

	svc := NewCardService(db)
	item, err := svc.UncompleteItem(context.Background(), userID, cardID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.IsCompleted || item.CompletedAt != nil {
		t.Fatal("expected item uncompleted")
	}
}

func TestCardService_UpdateItemNotes_Success(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{rowsAffected: 1}, nil
	}

	note := "note"
	svc := NewCardService(db)
	item, err := svc.UpdateItemNotes(context.Background(), userID, cardID, 0, &note, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Notes == nil || *item.Notes != "note" {
		t.Fatal("expected notes updated")
	}
}

func TestCardService_UpdateItemNotes_DBError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	items := [][]any{
		{uuid.New(), cardID, 0, "A", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, false, nil, false, items)
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{}, errors.New("boom")
	}

	svc := NewCardService(db)
	_, err := svc.UpdateItemNotes(context.Background(), userID, cardID, 0, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_SwapItems_FreeSpaceMoves(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	free := 0
	items := [][]any{
		{uuid.New(), cardID, 1, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), cardID, 2, "B", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 2, true, &free, false, items)
	var movedFree bool
	var movedItem bool
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				if strings.Contains(sql, "UPDATE bingo_cards") {
					movedFree = true
				}
				if strings.Contains(sql, "UPDATE bingo_items") {
					movedItem = true
				}
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	svc := NewCardService(db)
	if err := svc.SwapItems(context.Background(), userID, cardID, 0, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !movedFree {
		t.Fatal("expected free space update")
	}
	if !movedItem {
		t.Fatal("expected displaced item move")
	}
}

func TestCardService_UpdateConfig_CenterOccupiedRelocates(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	free := (*int)(nil)
	items := [][]any{
		{uuid.New(), cardID, 4, "Center", false, nil, nil, nil, time.Now()},
	}
	db := newCardDB(cardID, userID, 3, false, free, false, items)
	var relocated bool
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				if strings.Contains(sql, "UPDATE bingo_items") {
					relocated = true
				}
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	svc := NewCardService(db)
	enable := true
	if _, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{
		HasFreeSpace: &enable,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !relocated {
		t.Fatal("expected occupied center relocation")
	}
}

func TestCardService_UpdateConfig_DisableFree(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	free := 4
	db := newCardDB(cardID, userID, 3, true, &free, false, [][]any{})
	db.BeginFunc = func(ctx context.Context) (Tx, error) {
		return &fakeTx{
			ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
				if len(args) < 3 || args[1] != false {
					t.Fatalf("expected hasFree false, got %v", args)
				}
				if args[2] != nil {
					if ptr, ok := args[2].(*int); !ok || ptr != nil {
						t.Fatalf("expected freePos nil, got %v", args)
					}
				}
				return fakeCommandTag{rowsAffected: 1}, nil
			},
			CommitFunc: func(ctx context.Context) error { return nil },
		}, nil
	}

	svc := NewCardService(db)
	disable := false
	if _, err := svc.UpdateConfig(context.Background(), userID, cardID, models.UpdateCardConfigParams{
		HasFreeSpace: &disable,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCardService_Delete_NotFoundAfterDelete(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	call := 0
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		call++
		if call == 1 {
			return fakeCommandTag{rowsAffected: 2}, nil
		}
		return fakeCommandTag{rowsAffected: 0}, nil
	}

	svc := NewCardService(db)
	err := svc.Delete(context.Background(), userID, cardID)
	if !errors.Is(err, ErrCardNotFound) {
		t.Fatalf("expected ErrCardNotFound, got %v", err)
	}
}

func TestCardService_Delete_ItemsError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		return fakeCommandTag{}, errors.New("delete items error")
	}

	svc := NewCardService(db)
	err := svc.Delete(context.Background(), userID, cardID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCardService_Delete_CardError(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	db := newCardDB(cardID, userID, 5, true, nil, false, [][]any{})
	call := 0
	db.ExecFunc = func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
		call++
		if call == 1 {
			return fakeCommandTag{rowsAffected: 3}, nil
		}
		return fakeCommandTag{}, errors.New("delete card error")
	}

	svc := NewCardService(db)
	err := svc.Delete(context.Background(), userID, cardID)
	if err == nil {
		t.Fatal("expected error")
	}
}
func TestCardService_Import_NoSpaceForFree(t *testing.T) {
	svc := &CardService{}
	_, err := svc.Import(context.Background(), models.ImportCardParams{
		GridSize:     2,
		HasFreeSpace: true,
		Items: []models.ImportItem{
			{Position: 0, Content: "A"},
			{Position: 1, Content: "B"},
			{Position: 2, Content: "C"},
			{Position: 3, Content: "D"},
		},
	})
	if !errors.Is(err, ErrNoSpaceForFree) {
		t.Fatalf("expected ErrNoSpaceForFree, got %v", err)
	}
}

func TestCardService_Clone_TruncatesItems(t *testing.T) {
	userID := uuid.New()
	sourceCardID := uuid.New()
	newCardID := uuid.New()
	free := 4
	fallbackTitle := "2024 Bingo Card (Copy)"
	sourceItems := [][]any{
		{uuid.New(), sourceCardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), sourceCardID, 1, "B", false, nil, nil, nil, time.Now()},
		{uuid.New(), sourceCardID, 2, "C", false, nil, nil, nil, time.Now()},
		{uuid.New(), sourceCardID, 3, "D", false, nil, nil, nil, time.Now()},
		{uuid.New(), sourceCardID, 5, "E", false, nil, nil, nil, time.Now()},
	}
	newItems := [][]any{
		{uuid.New(), newCardID, 0, "A", false, nil, nil, nil, time.Now()},
		{uuid.New(), newCardID, 1, "B", false, nil, nil, nil, time.Now()},
		{uuid.New(), newCardID, 2, "C", false, nil, nil, nil, time.Now()},
	}

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM bingo_cards") {
				if len(args) > 0 && args[0] == sourceCardID {
					return rowFromValues(cardRowValues(sourceCardID, userID, 3, true, &free, false)...)
				}
				if len(args) > 0 && args[0] == newCardID {
					row := cardRowValues(newCardID, userID, 2, true, nil, false)
					row[4] = &fallbackTitle
					return rowFromValues(row...)
				}
			}
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("unexpected query")
			}}
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if len(args) > 0 && args[0] == sourceCardID {
				return &fakeRows{rows: sourceItems}, nil
			}
			if len(args) > 0 && args[0] == newCardID {
				return &fakeRows{rows: newItems}, nil
			}
			return &fakeRows{rows: [][]any{}}, nil
		},
		BeginFunc: func(ctx context.Context) (Tx, error) {
			itemInserts := 0
			return &fakeTx{
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
					return rowFromValues(
						newCardID,
						userID,
						2025,
						nil,
						nil,
						2,
						"BI",
						true,
						nil,
						true,
						false,
						true,
						false,
						time.Now(),
						time.Now(),
					)
				},
				ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
					if strings.Contains(sql, "INSERT INTO bingo_items") {
						itemInserts++
					}
					if itemInserts > 3 {
						t.Fatalf("expected at most 3 item inserts, got %d", itemInserts)
					}
					return fakeCommandTag{rowsAffected: 1}, nil
				},
				CommitFunc: func(ctx context.Context) error { return nil },
			}, nil
		},
	}

	svc := NewCardService(db)
	blankTitle := "  "
	result, err := svc.Clone(context.Background(), userID, sourceCardID, CloneParams{
		GridSize: 2,
		Title:    &blankTitle,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TruncatedItemCount != 2 {
		t.Fatalf("expected 2 truncated items, got %d", result.TruncatedItemCount)
	}
	if result.Card.GridSize != 2 {
		t.Fatalf("expected grid size 2, got %d", result.Card.GridSize)
	}
	if result.Card.Title == nil || *result.Card.Title == "" {
		t.Fatal("expected clone title fallback")
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

func TestMapBingoCardsUniqueViolationToCardExistsError_Indexes(t *testing.T) {
	title := "My Card"

	if got := mapBingoCardsUniqueViolationToCardExistsError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_bingo_cards_user_year_title"}, &title); got != ErrCardTitleExists {
		t.Fatalf("expected ErrCardTitleExists, got %v", got)
	}
	if got := mapBingoCardsUniqueViolationToCardExistsError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_bingo_cards_user_year_null_title"}, nil); got != ErrCardAlreadyExists {
		t.Fatalf("expected ErrCardAlreadyExists, got %v", got)
	}
}

func TestMapBingoCardsUniqueViolationToCardExistsError_Fallback(t *testing.T) {
	unknown := &pgconn.PgError{Code: "23505", ConstraintName: "some_other_index"}
	title := "Copy"
	empty := "   "

	if got := mapBingoCardsUniqueViolationToCardExistsError(unknown, &title); got != ErrCardTitleExists {
		t.Fatalf("expected ErrCardTitleExists for non-empty title, got %v", got)
	}
	if got := mapBingoCardsUniqueViolationToCardExistsError(unknown, &empty); got != ErrCardAlreadyExists {
		t.Fatalf("expected ErrCardAlreadyExists for blank title, got %v", got)
	}
	if got := mapBingoCardsUniqueViolationToCardExistsError(&pgconn.PgError{Code: "99999"}, &title); got != nil {
		t.Fatalf("expected nil for non-unique error, got %v", got)
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
