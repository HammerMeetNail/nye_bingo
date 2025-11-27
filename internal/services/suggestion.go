package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

type SuggestionService struct {
	db *pgxpool.Pool
}

func NewSuggestionService(db *pgxpool.Pool) *SuggestionService {
	return &SuggestionService{db: db}
}

func (s *SuggestionService) GetAll(ctx context.Context) ([]*models.Suggestion, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, category, content, is_active
		 FROM suggestions
		 WHERE is_active = true
		 ORDER BY category, content`,
	)
	if err != nil {
		return nil, fmt.Errorf("getting suggestions: %w", err)
	}
	defer rows.Close()

	var suggestions []*models.Suggestion
	for rows.Next() {
		suggestion := &models.Suggestion{}
		if err := rows.Scan(&suggestion.ID, &suggestion.Category, &suggestion.Content, &suggestion.IsActive); err != nil {
			return nil, fmt.Errorf("scanning suggestion: %w", err)
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

func (s *SuggestionService) GetByCategory(ctx context.Context, category string) ([]*models.Suggestion, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, category, content, is_active
		 FROM suggestions
		 WHERE is_active = true AND category = $1
		 ORDER BY content`,
		category,
	)
	if err != nil {
		return nil, fmt.Errorf("getting suggestions by category: %w", err)
	}
	defer rows.Close()

	var suggestions []*models.Suggestion
	for rows.Next() {
		suggestion := &models.Suggestion{}
		if err := rows.Scan(&suggestion.ID, &suggestion.Category, &suggestion.Content, &suggestion.IsActive); err != nil {
			return nil, fmt.Errorf("scanning suggestion: %w", err)
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

func (s *SuggestionService) GetCategories(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT DISTINCT category FROM suggestions WHERE is_active = true ORDER BY category`,
	)
	if err != nil {
		return nil, fmt.Errorf("getting categories: %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		categories = append(categories, category)
	}

	return categories, nil
}

type SuggestionsByCategory struct {
	Category    string               `json:"category"`
	Suggestions []*models.Suggestion `json:"suggestions"`
}

func (s *SuggestionService) GetGroupedByCategory(ctx context.Context) ([]SuggestionsByCategory, error) {
	suggestions, err := s.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	categoryMap := make(map[string][]*models.Suggestion)
	for _, suggestion := range suggestions {
		categoryMap[suggestion.Category] = append(categoryMap[suggestion.Category], suggestion)
	}

	var result []SuggestionsByCategory
	for _, category := range models.SuggestionCategories {
		if items, ok := categoryMap[category]; ok {
			result = append(result, SuggestionsByCategory{
				Category:    category,
				Suggestions: items,
			})
		}
	}

	return result, nil
}
