package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestSuggestionService_GetAll(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if !strings.Contains(sql, "FROM suggestions") || !strings.Contains(sql, "WHERE is_active = true") {
				t.Fatalf("unexpected get all sql: %q", sql)
			}
			if len(args) != 0 {
				t.Fatalf("unexpected get all args: %v", args)
			}
			return &fakeRows{rows: [][]any{
				{uuid.New(), "health", "Walk 10k steps", true},
				{uuid.New(), "travel", "Visit a new city", true},
			}}, nil
		},
	}

	svc := NewSuggestionService(db)
	suggestions, err := svc.GetAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(suggestions))
	}
}

func TestSuggestionService_GetAll_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, context.Canceled
		},
	}

	svc := NewSuggestionService(db)
	_, err := svc.GetAll(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSuggestionService_GetAll_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}

	svc := NewSuggestionService(db)
	_, err := svc.GetAll(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSuggestionService_GetGroupedByCategory(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{uuid.New(), models.SuggestionCategories[0], "First", true},
				{uuid.New(), models.SuggestionCategories[1], "Second", true},
				{uuid.New(), models.SuggestionCategories[0], "Third", true},
			}}, nil
		},
	}

	svc := NewSuggestionService(db)
	grouped, err := svc.GetGroupedByCategory(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(grouped) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(grouped))
	}
	if grouped[0].Category != models.SuggestionCategories[0] {
		t.Fatalf("expected first category %s, got %s", models.SuggestionCategories[0], grouped[0].Category)
	}
	if len(grouped[0].Suggestions) != 2 {
		t.Fatalf("expected 2 suggestions in first group, got %d", len(grouped[0].Suggestions))
	}
}

func TestSuggestionService_GetByCategory(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if !strings.Contains(sql, "FROM suggestions") || !strings.Contains(sql, "WHERE is_active = true AND category = $1") {
				t.Fatalf("unexpected get by category sql: %q", sql)
			}
			if len(args) != 1 || args[0] != "health" {
				t.Fatalf("unexpected get by category args: %v", args)
			}
			return &fakeRows{rows: [][]any{
				{uuid.New(), "health", "Run", true},
			}}, nil
		},
	}

	svc := NewSuggestionService(db)
	suggestions, err := svc.GetByCategory(context.Background(), "health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
}

func TestSuggestionService_GetByCategory_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, context.Canceled
		},
	}

	svc := NewSuggestionService(db)
	_, err := svc.GetByCategory(context.Background(), "health")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSuggestionService_GetByCategory_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}

	svc := NewSuggestionService(db)
	_, err := svc.GetByCategory(context.Background(), "health")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSuggestionService_GetCategories(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if !strings.Contains(sql, "SELECT DISTINCT category") || !strings.Contains(sql, "FROM suggestions") || !strings.Contains(sql, "WHERE is_active = true") {
				t.Fatalf("unexpected get categories sql: %q", sql)
			}
			if len(args) != 0 {
				t.Fatalf("unexpected get categories args: %v", args)
			}
			return &fakeRows{rows: [][]any{
				{"health"},
				{"travel"},
			}}, nil
		},
	}

	svc := NewSuggestionService(db)
	categories, err := svc.GetCategories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(categories))
	}
}

func TestSuggestionService_GetCategories_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, context.Canceled
		},
	}

	svc := NewSuggestionService(db)
	_, err := svc.GetCategories(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSuggestionService_GetCategories_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{struct{}{}}}}, nil
		},
	}

	svc := NewSuggestionService(db)
	_, err := svc.GetCategories(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSuggestionService_GetGroupedByCategory_Error(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, context.Canceled
		},
	}

	svc := NewSuggestionService(db)
	_, err := svc.GetGroupedByCategory(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
