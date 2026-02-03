package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrTemplateNotFound      = errors.New("template not found")
	ErrTemplateLimitReached  = errors.New("template limit reached")
	ErrInvalidCarryOverValue = errors.New("invalid carry over value")
)

type TemplateValidationError struct {
	msg string
}

func (e *TemplateValidationError) Error() string {
	if e == nil {
		return "invalid template"
	}
	return e.msg
}

type CardConflictError struct {
	Year           int
	Title          string
	SuggestedTitle string
}

func (e *CardConflictError) Error() string {
	if e == nil {
		return "card conflict"
	}
	return fmt.Sprintf("card conflict: %d %q", e.Year, e.Title)
}

type TemplateService struct {
	db          DB
	cardService CardServiceInterface
}

func NewTemplateService(db DB, cardService CardServiceInterface) *TemplateService {
	return &TemplateService{db: db, cardService: cardService}
}

func (s *TemplateService) List(ctx context.Context, userID uuid.UUID) ([]models.CardTemplate, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, name, category, grid_size, header_text, has_free_space, default_visible_to_friends, created_at, updated_at
		FROM card_templates
		WHERE user_id = $1
		ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}
	defer rows.Close()

	var out []models.CardTemplate
	for rows.Next() {
		var t models.CardTemplate
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Name, &t.Category, &t.GridSize, &t.HeaderText, &t.HasFreeSpace, &t.DefaultVisibleToFriends, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}
	return out, nil
}

func (s *TemplateService) Get(ctx context.Context, userID, templateID uuid.UUID) (*models.TemplateWithItems, error) {
	var t models.CardTemplate
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, name, category, grid_size, header_text, has_free_space, default_visible_to_friends, created_at, updated_at
		FROM card_templates
		WHERE id = $1 AND user_id = $2`, templateID, userID).Scan(
		&t.ID, &t.UserID, &t.Name, &t.Category, &t.GridSize, &t.HeaderText, &t.HasFreeSpace, &t.DefaultVisibleToFriends, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting template: %w", err)
	}

	items, err := s.getItems(ctx, templateID)
	if err != nil {
		return nil, err
	}
	return &models.TemplateWithItems{Template: t, Items: items}, nil
}

func (s *TemplateService) CreateFromCard(ctx context.Context, userID, cardID uuid.UUID, name string) (*models.TemplateWithItems, error) {
	name = strings.TrimSpace(name)
	if err := validateTemplateName(name); err != nil {
		return nil, err
	}
	if err := s.enforceTemplateLimit(ctx, userID); err != nil {
		return nil, err
	}

	card, err := s.cardService.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card.UserID != userID {
		return nil, ErrNotCardOwner
	}

	itemContents := make([]string, 0, len(card.Items))
	sort.Slice(card.Items, func(i, j int) bool { return card.Items[i].Position < card.Items[j].Position })
	for _, it := range card.Items {
		itemContents = append(itemContents, it.Content)
	}
	itemContents, err = normalizeTemplateItems(itemContents)
	if err != nil {
		return nil, err
	}
	if err := validateTemplateCapacity(card.GridSize, card.HasFreeSpace, len(itemContents)); err != nil {
		return nil, err
	}
	if err := validateTemplateConfig(card.GridSize, card.HeaderText); err != nil {
		return nil, err
	}

	return s.create(ctx, userID, models.CreateTemplateParams{
		Name:                    name,
		Category:                card.Category,
		GridSize:                card.GridSize,
		HeaderText:              card.HeaderText,
		HasFreeSpace:            card.HasFreeSpace,
		DefaultVisibleToFriends: card.VisibleToFriends,
		Items:                   itemContents,
	})
}

func (s *TemplateService) Create(ctx context.Context, userID uuid.UUID, params models.CreateTemplateParams) (*models.TemplateWithItems, error) {
	params.Name = strings.TrimSpace(params.Name)
	if err := validateTemplateName(params.Name); err != nil {
		return nil, err
	}
	if err := validateTemplateCategory(params.Category); err != nil {
		return nil, err
	}
	if params.GridSize == 0 {
		params.GridSize = models.MaxGridSize
	}
	if params.HeaderText == "" {
		params.HeaderText = models.DefaultHeaderText(params.GridSize)
	}
	params.HeaderText = models.NormalizeHeaderText(params.HeaderText)
	if err := validateTemplateConfig(params.GridSize, params.HeaderText); err != nil {
		return nil, err
	}

	items, err := normalizeTemplateItems(params.Items)
	if err != nil {
		return nil, err
	}
	if err := validateTemplateCapacity(params.GridSize, params.HasFreeSpace, len(items)); err != nil {
		return nil, err
	}
	if err := s.enforceTemplateLimit(ctx, userID); err != nil {
		return nil, err
	}

	params.Items = items
	return s.create(ctx, userID, params)
}

func (s *TemplateService) Update(ctx context.Context, userID, templateID uuid.UUID, params models.UpdateTemplateParams) (*models.TemplateWithItems, error) {
	params.Name = strings.TrimSpace(params.Name)
	if err := validateTemplateName(params.Name); err != nil {
		return nil, err
	}
	if err := validateTemplateCategory(params.Category); err != nil {
		return nil, err
	}
	if params.GridSize == 0 {
		params.GridSize = models.MaxGridSize
	}
	if params.HeaderText == "" {
		params.HeaderText = models.DefaultHeaderText(params.GridSize)
	}
	params.HeaderText = models.NormalizeHeaderText(params.HeaderText)
	if err := validateTemplateConfig(params.GridSize, params.HeaderText); err != nil {
		return nil, err
	}

	var updated models.CardTemplate
	err := s.db.QueryRow(ctx, `
		UPDATE card_templates
		SET name = $1,
		    category = $2,
		    grid_size = $3,
		    header_text = $4,
		    has_free_space = $5,
		    default_visible_to_friends = $6
		WHERE id = $7 AND user_id = $8
		RETURNING id, user_id, name, category, grid_size, header_text, has_free_space, default_visible_to_friends, created_at, updated_at`,
		params.Name, params.Category, params.GridSize, params.HeaderText, params.HasFreeSpace, params.DefaultVisibleToFriends, templateID, userID,
	).Scan(
		&updated.ID, &updated.UserID, &updated.Name, &updated.Category, &updated.GridSize, &updated.HeaderText, &updated.HasFreeSpace, &updated.DefaultVisibleToFriends, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updating template: %w", err)
	}

	items, err := s.getItems(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if err := validateTemplateCapacity(updated.GridSize, updated.HasFreeSpace, len(items)); err != nil {
		return nil, err
	}
	return &models.TemplateWithItems{Template: updated, Items: items}, nil
}

func (s *TemplateService) ReplaceItems(ctx context.Context, userID, templateID uuid.UUID, items []string) (*models.TemplateWithItems, error) {
	items, err := normalizeTemplateItems(items)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var t models.CardTemplate
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, name, category, grid_size, header_text, has_free_space, default_visible_to_friends, created_at, updated_at
		FROM card_templates
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`, templateID, userID).Scan(
		&t.ID, &t.UserID, &t.Name, &t.Category, &t.GridSize, &t.HeaderText, &t.HasFreeSpace, &t.DefaultVisibleToFriends, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("locking template: %w", err)
	}
	if err := validateTemplateCapacity(t.GridSize, t.HasFreeSpace, len(items)); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, "DELETE FROM card_template_items WHERE template_id = $1", templateID); err != nil {
		return nil, fmt.Errorf("deleting template items: %w", err)
	}

	outItems := make([]models.CardTemplateItem, 0, len(items))
	for i, content := range items {
		var it models.CardTemplateItem
		err := tx.QueryRow(ctx, `
			INSERT INTO card_template_items (template_id, sort_order, content)
			VALUES ($1, $2, $3)
			RETURNING id, template_id, sort_order, content, created_at`,
			templateID, i, content).Scan(&it.ID, &it.TemplateID, &it.SortOrder, &it.Content, &it.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("inserting template item: %w", err)
		}
		outItems = append(outItems, it)
	}

	// Bump template updated_at (item replacements should affect list ordering).
	if _, err := tx.Exec(ctx, "UPDATE card_templates SET updated_at = updated_at WHERE id = $1", templateID); err != nil {
		return nil, fmt.Errorf("updating template timestamp: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	updated, err := s.Get(ctx, userID, templateID)
	if err != nil {
		return nil, err
	}
	updated.Items = outItems
	return updated, nil
}

func (s *TemplateService) Delete(ctx context.Context, userID, templateID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "DELETE FROM card_templates WHERE id = $1 AND user_id = $2", templateID, userID)
	if err != nil {
		return fmt.Errorf("deleting template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

func (s *TemplateService) CreateCardFromTemplate(ctx context.Context, userID, templateID uuid.UUID, params models.TemplateCreateCardParams) (*models.BingoCard, error) {
	tpl, err := s.Get(ctx, userID, templateID)
	if err != nil {
		return nil, err
	}

	year := params.Year
	title := normalizeTitle(params.Title, year)
	category := normalizeOptionalString(params.Category)
	if category == nil {
		category = normalizeOptionalString(tpl.Template.Category)
	}

	visible := tpl.Template.DefaultVisibleToFriends
	if params.VisibleToFriends != nil {
		visible = *params.VisibleToFriends
	}

	itemCount := len(tpl.Items)
	if err := validateTemplateCapacity(tpl.Template.GridSize, tpl.Template.HasFreeSpace, itemCount); err != nil {
		return nil, err
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // Non-crypto shuffle is fine for layout.
	freePos := (*int)(nil)
	if tpl.Template.HasFreeSpace {
		pos := defaultFreePosition(tpl.Template.GridSize, rng)
		freePos = &pos
	}

	positions, err := choosePositions(tpl.Template.GridSize, freePos, itemCount, params.ShuffleLayout, rng)
	if err != nil {
		return nil, err
	}

	if err := s.ensureNoCardConflict(ctx, userID, year, title); err != nil {
		return nil, err
	}

	importItems := make([]models.ImportItem, 0, itemCount)
	for i, it := range tpl.Items {
		importItems = append(importItems, models.ImportItem{
			Position: positions[i],
			Content:  it.Content,
		})
	}

	visibleToFriends := visible
	cardTitle := title
	cardParams := models.ImportCardParams{
		UserID:           userID,
		Year:             year,
		Title:            &cardTitle,
		Category:         category,
		Items:            importItems,
		Finalize:         false,
		VisibleToFriends: &visibleToFriends,
		GridSize:         tpl.Template.GridSize,
		HeaderText:       tpl.Template.HeaderText,
		HasFreeSpace:     tpl.Template.HasFreeSpace,
		FreeSpacePos:     freePos,
	}
	card, err := s.cardService.Import(ctx, cardParams)
	if err != nil {
		if isBingoCardsUniqueViolation(err) {
			if err2 := s.ensureNoCardConflict(ctx, userID, year, title); err2 != nil {
				return nil, err2
			}
		}
		return nil, err
	}
	return card, nil
}

func (s *TemplateService) RolloverCard(ctx context.Context, userID, cardID uuid.UUID, params models.RolloverParams) (*models.BingoCard, error) {
	source, err := s.cardService.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if source.UserID != userID {
		return nil, ErrNotCardOwner
	}
	if len(source.Items) == 0 {
		return nil, &TemplateValidationError{msg: "card must have at least 1 item"}
	}

	targetYear := params.Year
	title := strings.TrimSpace(derefStringPtr(params.Title))
	if title == "" {
		if source.Title != nil && strings.TrimSpace(*source.Title) != "" {
			title = strings.TrimSpace(*source.Title)
		} else {
			title = fmt.Sprintf("%d Bingo Card", targetYear)
		}
	}

	carry := strings.TrimSpace(params.CarryOver)
	if carry == "" {
		carry = "all"
	}
	if carry != "all" && carry != "incomplete_only" {
		return nil, ErrInvalidCarryOverValue
	}

	selected := make([]models.BingoItem, 0, len(source.Items))
	for _, it := range source.Items {
		if carry == "incomplete_only" && it.IsCompleted {
			continue
		}
		selected = append(selected, it)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // Non-crypto shuffle is fine for layout.
	freePos := source.FreeSpacePos
	if source.HasFreeSpace && freePos == nil {
		pos := defaultFreePosition(source.GridSize, rng)
		freePos = &pos
	}

	visibleToFriends := source.VisibleToFriends
	cardTitle := title

	var importItems []models.ImportItem
	if params.ShuffleLayout {
		positions, err := choosePositions(source.GridSize, freePos, len(selected), true, rng)
		if err != nil {
			return nil, err
		}
		importItems = make([]models.ImportItem, 0, len(selected))
		for i, it := range selected {
			importItems = append(importItems, models.ImportItem{
				Position: positions[i],
				Content:  it.Content,
			})
		}
	} else {
		importItems = make([]models.ImportItem, 0, len(selected))
		for _, it := range selected {
			importItems = append(importItems, models.ImportItem{
				Position: it.Position,
				Content:  it.Content,
			})
		}
	}

	if err := s.ensureNoCardConflict(ctx, userID, targetYear, title); err != nil {
		return nil, err
	}

	cardParams := models.ImportCardParams{
		UserID:           userID,
		Year:             targetYear,
		Title:            &cardTitle,
		Category:         source.Category,
		Items:            importItems,
		Finalize:         false,
		VisibleToFriends: &visibleToFriends,
		GridSize:         source.GridSize,
		HeaderText:       source.HeaderText,
		HasFreeSpace:     source.HasFreeSpace,
		FreeSpacePos:     freePos,
	}
	card, err := s.cardService.Import(ctx, cardParams)
	if err != nil {
		if isBingoCardsUniqueViolation(err) {
			if err2 := s.ensureNoCardConflict(ctx, userID, targetYear, title); err2 != nil {
				return nil, err2
			}
		}
		return nil, err
	}
	return card, nil
}

func (s *TemplateService) enforceTemplateLimit(ctx context.Context, userID uuid.UUID) error {
	var count int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM card_templates WHERE user_id = $1", userID).Scan(&count); err != nil {
		return fmt.Errorf("checking template count: %w", err)
	}
	if count >= 50 {
		return ErrTemplateLimitReached
	}
	return nil
}

func (s *TemplateService) create(ctx context.Context, userID uuid.UUID, params models.CreateTemplateParams) (*models.TemplateWithItems, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tpl models.CardTemplate
	err = tx.QueryRow(ctx, `
		INSERT INTO card_templates (user_id, name, category, grid_size, header_text, has_free_space, default_visible_to_friends)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, name, category, grid_size, header_text, has_free_space, default_visible_to_friends, created_at, updated_at`,
		userID, params.Name, params.Category, params.GridSize, params.HeaderText, params.HasFreeSpace, params.DefaultVisibleToFriends,
	).Scan(
		&tpl.ID, &tpl.UserID, &tpl.Name, &tpl.Category, &tpl.GridSize, &tpl.HeaderText, &tpl.HasFreeSpace, &tpl.DefaultVisibleToFriends, &tpl.CreatedAt, &tpl.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating template: %w", err)
	}

	outItems := make([]models.CardTemplateItem, 0, len(params.Items))
	for i, content := range params.Items {
		var it models.CardTemplateItem
		err := tx.QueryRow(ctx, `
			INSERT INTO card_template_items (template_id, sort_order, content)
			VALUES ($1, $2, $3)
			RETURNING id, template_id, sort_order, content, created_at`,
			tpl.ID, i, content).Scan(&it.ID, &it.TemplateID, &it.SortOrder, &it.Content, &it.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating template item: %w", err)
		}
		outItems = append(outItems, it)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &models.TemplateWithItems{Template: tpl, Items: outItems}, nil
}

func (s *TemplateService) getItems(ctx context.Context, templateID uuid.UUID) ([]models.CardTemplateItem, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, template_id, sort_order, content, created_at
		FROM card_template_items
		WHERE template_id = $1
		ORDER BY sort_order ASC`, templateID)
	if err != nil {
		return nil, fmt.Errorf("getting template items: %w", err)
	}
	defer rows.Close()

	var out []models.CardTemplateItem
	for rows.Next() {
		var it models.CardTemplateItem
		if err := rows.Scan(&it.ID, &it.TemplateID, &it.SortOrder, &it.Content, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning template item: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("getting template items: %w", err)
	}
	return out, nil
}

func validateTemplateName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return &TemplateValidationError{msg: "name is required"}
	}
	if len(n) > 100 {
		return &TemplateValidationError{msg: "name must be 100 characters or less"}
	}
	return nil
}

func validateTemplateCategory(category *string) error {
	if category == nil {
		return nil
	}
	c := strings.TrimSpace(*category)
	if c == "" {
		return nil
	}
	if len(c) > 50 {
		return &TemplateValidationError{msg: "category must be 50 characters or less"}
	}
	return nil
}

func validateTemplateConfig(gridSize int, headerText string) error {
	if !models.IsValidGridSize(gridSize) {
		return &TemplateValidationError{msg: "grid size must be 2, 3, 4, or 5"}
	}
	if err := models.ValidateHeaderText(headerText, gridSize); err != nil {
		return &TemplateValidationError{msg: err.Error()}
	}
	return nil
}

func normalizeTemplateItems(items []string) ([]string, error) {
	if items == nil {
		return []string{}, nil
	}
	out := make([]string, 0, len(items))
	for _, raw := range items {
		s := strings.TrimSpace(raw)
		if s == "" {
			return nil, &TemplateValidationError{msg: "items cannot be empty"}
		}
		if len(s) > 500 {
			return nil, &TemplateValidationError{msg: "items must be 500 characters or less"}
		}
		out = append(out, s)
	}
	return out, nil
}

func validateTemplateCapacity(gridSize int, hasFree bool, itemCount int) error {
	total := gridSize * gridSize
	capacity := total
	if hasFree {
		capacity = total - 1
	}
	if itemCount > capacity {
		return &TemplateValidationError{msg: "too many items for this grid size"}
	}
	return nil
}

func defaultFreePosition(gridSize int, rng *rand.Rand) int {
	total := gridSize * gridSize
	if gridSize%2 == 1 {
		return total / 2
	}
	return rng.Intn(total)
}

func choosePositions(gridSize int, freePos *int, itemCount int, shuffle bool, rng *rand.Rand) ([]int, error) {
	total := gridSize * gridSize
	available := make([]int, 0, total)
	for pos := 0; pos < total; pos++ {
		if freePos != nil && pos == *freePos {
			continue
		}
		available = append(available, pos)
	}
	if itemCount > len(available) {
		return nil, &TemplateValidationError{msg: "itemCount exceeds capacity"}
	}
	if shuffle {
		rng.Shuffle(len(available), func(i, j int) { available[i], available[j] = available[j], available[i] })
	}
	return available[:itemCount], nil
}

func normalizeTitle(title *string, year int) string {
	t := strings.TrimSpace(derefStringPtr(title))
	if t != "" {
		return t
	}
	return fmt.Sprintf("%d Bingo Card", year)
}

func normalizeOptionalString(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func derefStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func isBingoCardsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *TemplateService) ensureNoCardConflict(ctx context.Context, userID uuid.UUID, year int, title string) error {
	t := title
	existing, err := s.cardService.CheckForConflict(ctx, userID, year, &t)
	if err != nil && !errors.Is(err, ErrCardNotFound) {
		return fmt.Errorf("checking for card conflict: %w", err)
	}
	if existing == nil {
		return nil
	}

	suggested, err := s.suggestCopyTitle(ctx, userID, year, title)
	if err != nil {
		return err
	}
	return &CardConflictError{
		Year:           year,
		Title:          title,
		SuggestedTitle: suggested,
	}
}

func (s *TemplateService) suggestCopyTitle(ctx context.Context, userID uuid.UUID, year int, baseTitle string) (string, error) {
	baseTitle = strings.TrimSpace(baseTitle)
	if baseTitle == "" {
		baseTitle = fmt.Sprintf("%d Bingo Card", year)
	}

	for i := 1; i <= 25; i++ {
		candidate := withCopySuffix(baseTitle, i)
		t := candidate
		_, err := s.cardService.CheckForConflict(ctx, userID, year, &t)
		if errors.Is(err, ErrCardNotFound) {
			return candidate, nil
		}
		if err != nil && !errors.Is(err, ErrCardNotFound) {
			return "", fmt.Errorf("checking suggested title conflict: %w", err)
		}
	}

	// Last resort: add a random suffix.
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // Used for UX-friendly naming only.
	candidate := withCopySuffix(baseTitle, rng.Intn(9999)+100)
	return candidate, nil
}

func withCopySuffix(base string, n int) string {
	base = strings.TrimSpace(base)
	suffix := " (Copy)"
	if n > 1 {
		suffix = fmt.Sprintf(" (Copy %d)", n)
	}
	maxLen := 100
	if len(suffix) >= maxLen {
		return suffix[:maxLen]
	}
	allowed := maxLen - len(suffix)
	if len(base) > allowed {
		base = base[:allowed]
		base = strings.TrimRight(base, " ")
	}
	if base == "" {
		base = "Card"
		if len(base) > allowed {
			base = base[:allowed]
		}
	}
	return base + suffix
}
