package services

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

type memState struct {
	templates map[uuid.UUID]models.CardTemplate
	items     map[uuid.UUID][]models.CardTemplateItem
}

func newMemState() *memState {
	return &memState{
		templates: map[uuid.UUID]models.CardTemplate{},
		items:     map[uuid.UUID][]models.CardTemplateItem{},
	}
}

func (s *memState) clone() *memState {
	out := newMemState()
	for id, tpl := range s.templates {
		tplCopy := tpl
		if tplCopy.Category != nil {
			v := *tplCopy.Category
			tplCopy.Category = &v
		}
		out.templates[id] = tplCopy
	}
	for tplID, items := range s.items {
		copied := make([]models.CardTemplateItem, len(items))
		copy(copied, items)
		out.items[tplID] = copied
	}
	return out
}

type memDB struct {
	state *memState
}

func newMemDB() *memDB {
	return &memDB{state: newMemState()}
}

func (db *memDB) Begin(ctx context.Context) (Tx, error) {
	return &memTx{parent: db, state: db.state.clone()}, nil
}

func (db *memDB) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	return execMem(db.state, sql, args...)
}

func (db *memDB) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return queryMem(db.state, sql, args...)
}

func (db *memDB) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return queryRowMem(db.state, sql, args...)
}

type memTx struct {
	parent *memDB
	state  *memState
}

func (tx *memTx) Commit(ctx context.Context) error {
	tx.parent.state = tx.state
	return nil
}

func (tx *memTx) Rollback(ctx context.Context) error {
	return nil
}

func (tx *memTx) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	return execMem(tx.state, sql, args...)
}

func (tx *memTx) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return queryMem(tx.state, sql, args...)
}

func (tx *memTx) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return queryRowMem(tx.state, sql, args...)
}

type memCommandTag struct {
	affected int64
}

func (t memCommandTag) RowsAffected() int64 { return t.affected }

type memRow struct {
	err    error
	values []any
}

func (r memRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan mismatch: dest=%d values=%d", len(dest), len(r.values))
	}
	for i := range dest {
		if err := setScanDest(dest[i], r.values[i]); err != nil {
			return fmt.Errorf("scan %d: %w", i, err)
		}
	}
	return nil
}

func setScanDest(dest any, value any) error {
	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Pointer || dv.IsNil() {
		return fmt.Errorf("dest must be a non-nil pointer, got %T", dest)
	}
	ev := dv.Elem()
	if !ev.CanSet() {
		return fmt.Errorf("dest cannot be set: %T", dest)
	}

	if value == nil {
		ev.Set(reflect.Zero(ev.Type()))
		return nil
	}

	vv := reflect.ValueOf(value)
	if vv.Type().AssignableTo(ev.Type()) {
		ev.Set(vv)
		return nil
	}
	if vv.Type().ConvertibleTo(ev.Type()) {
		ev.Set(vv.Convert(ev.Type()))
		return nil
	}
	return fmt.Errorf("cannot assign %T to %T", value, dest)
}

type memRows struct {
	rows [][]any
	idx  int
}

func (r *memRows) Close() {}
func (r *memRows) Err() error {
	return nil
}
func (r *memRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *memRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return fmt.Errorf("scan called out of sequence")
	}
	row := memRow{values: r.rows[r.idx-1]}
	return row.Scan(dest...)
}

func templateValues(t models.CardTemplate) []any {
	return []any{
		t.ID,
		t.UserID,
		t.Name,
		t.Category,
		t.GridSize,
		t.HeaderText,
		t.HasFreeSpace,
		t.DefaultVisibleToFriends,
		t.CreatedAt,
		t.UpdatedAt,
	}
}

func templateItemValues(it models.CardTemplateItem) []any {
	return []any{
		it.ID,
		it.TemplateID,
		it.SortOrder,
		it.Content,
		it.CreatedAt,
	}
}

func queryRowMem(state *memState, sql string, args ...any) Row {
	switch {
	case strings.Contains(sql, "SELECT COUNT(*) FROM card_templates"):
		userID := args[0].(uuid.UUID)
		count := 0
		for _, tpl := range state.templates {
			if tpl.UserID == userID {
				count++
			}
		}
		return memRow{values: []any{count}}

	case strings.Contains(sql, "FROM card_templates") && strings.Contains(sql, "WHERE id = $1 AND user_id = $2"):
		templateID := args[0].(uuid.UUID)
		userID := args[1].(uuid.UUID)
		tpl, ok := state.templates[templateID]
		if !ok || tpl.UserID != userID {
			return memRow{err: pgx.ErrNoRows}
		}
		return memRow{values: templateValues(tpl)}

	case strings.Contains(sql, "UPDATE card_templates") && strings.Contains(sql, "RETURNING"):
		name := args[0].(string)
		var category *string
		if args[1] != nil {
			category, _ = args[1].(*string)
		}
		gridSize := args[2].(int)
		headerText := args[3].(string)
		hasFreeSpace := args[4].(bool)
		defaultVisibleToFriends := args[5].(bool)
		templateID := args[6].(uuid.UUID)
		userID := args[7].(uuid.UUID)

		tpl, ok := state.templates[templateID]
		if !ok || tpl.UserID != userID {
			return memRow{err: pgx.ErrNoRows}
		}

		tpl.Name = name
		tpl.Category = category
		tpl.GridSize = gridSize
		tpl.HeaderText = headerText
		tpl.HasFreeSpace = hasFreeSpace
		tpl.DefaultVisibleToFriends = defaultVisibleToFriends
		tpl.UpdatedAt = time.Now().UTC()
		state.templates[templateID] = tpl

		return memRow{values: templateValues(tpl)}

	case strings.Contains(sql, "INSERT INTO card_templates") && strings.Contains(sql, "RETURNING"):
		userID := args[0].(uuid.UUID)
		name := args[1].(string)
		var category *string
		if args[2] != nil {
			category, _ = args[2].(*string)
		}
		gridSize := args[3].(int)
		headerText := args[4].(string)
		hasFreeSpace := args[5].(bool)
		defaultVisibleToFriends := args[6].(bool)

		now := time.Now().UTC()
		tpl := models.CardTemplate{
			ID:                      uuid.New(),
			UserID:                  userID,
			Name:                    name,
			Category:                category,
			GridSize:                gridSize,
			HeaderText:              headerText,
			HasFreeSpace:            hasFreeSpace,
			DefaultVisibleToFriends: defaultVisibleToFriends,
			CreatedAt:               now,
			UpdatedAt:               now,
		}
		state.templates[tpl.ID] = tpl
		state.items[tpl.ID] = []models.CardTemplateItem{}

		return memRow{values: templateValues(tpl)}

	case strings.Contains(sql, "INSERT INTO card_template_items") && strings.Contains(sql, "RETURNING"):
		templateID := args[0].(uuid.UUID)
		sortOrder := args[1].(int)
		content := args[2].(string)

		if _, ok := state.templates[templateID]; !ok {
			return memRow{err: fmt.Errorf("template does not exist")}
		}

		it := models.CardTemplateItem{
			ID:         uuid.New(),
			TemplateID: templateID,
			SortOrder:  sortOrder,
			Content:    content,
			CreatedAt:  time.Now().UTC(),
		}
		state.items[templateID] = append(state.items[templateID], it)
		return memRow{values: templateItemValues(it)}

	default:
		return memRow{err: fmt.Errorf("unhandled queryRow: %q", strings.TrimSpace(sql))}
	}
}

func queryMem(state *memState, sql string, args ...any) (Rows, error) {
	switch {
	case strings.Contains(sql, "FROM card_templates") && strings.Contains(sql, "ORDER BY updated_at DESC"):
		userID := args[0].(uuid.UUID)
		var list []models.CardTemplate
		for _, tpl := range state.templates {
			if tpl.UserID == userID {
				list = append(list, tpl)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt.After(list[j].UpdatedAt) })
		out := &memRows{rows: make([][]any, 0, len(list))}
		for _, tpl := range list {
			out.rows = append(out.rows, templateValues(tpl))
		}
		return out, nil

	case strings.Contains(sql, "FROM card_template_items"):
		templateID := args[0].(uuid.UUID)
		items := state.items[templateID]
		sort.Slice(items, func(i, j int) bool { return items[i].SortOrder < items[j].SortOrder })
		out := &memRows{rows: make([][]any, 0, len(items))}
		for _, it := range items {
			out.rows = append(out.rows, templateItemValues(it))
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unhandled query: %q", strings.TrimSpace(sql))
	}
}

func execMem(state *memState, sql string, args ...any) (CommandTag, error) {
	switch {
	case strings.Contains(sql, "DELETE FROM card_templates"):
		templateID := args[0].(uuid.UUID)
		userID := args[1].(uuid.UUID)
		tpl, ok := state.templates[templateID]
		if !ok || tpl.UserID != userID {
			return memCommandTag{affected: 0}, nil
		}
		delete(state.templates, templateID)
		delete(state.items, templateID)
		return memCommandTag{affected: 1}, nil

	case strings.Contains(sql, "DELETE FROM card_template_items"):
		templateID := args[0].(uuid.UUID)
		affected := int64(len(state.items[templateID]))
		state.items[templateID] = []models.CardTemplateItem{}
		return memCommandTag{affected: affected}, nil

	case strings.Contains(sql, "UPDATE card_templates SET updated_at = updated_at"):
		templateID := args[0].(uuid.UUID)
		tpl, ok := state.templates[templateID]
		if !ok {
			return memCommandTag{affected: 0}, nil
		}
		tpl.UpdatedAt = time.Now().UTC()
		state.templates[templateID] = tpl
		return memCommandTag{affected: 1}, nil

	default:
		return nil, fmt.Errorf("unhandled exec: %q", strings.TrimSpace(sql))
	}
}

type mockCardServiceForTemplates struct {
	CheckForConflictFunc func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error)
	GetByIDFunc          func(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error)
	ImportFunc           func(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error)

	ImportCalls []models.ImportCardParams
}

func (m *mockCardServiceForTemplates) CheckForConflict(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
	if m.CheckForConflictFunc == nil {
		return nil, ErrCardNotFound
	}
	return m.CheckForConflictFunc(ctx, userID, year, title)
}

func (m *mockCardServiceForTemplates) GetByID(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error) {
	if m.GetByIDFunc == nil {
		return nil, ErrCardNotFound
	}
	return m.GetByIDFunc(ctx, cardID)
}

func (m *mockCardServiceForTemplates) Import(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error) {
	m.ImportCalls = append(m.ImportCalls, params)
	if m.ImportFunc == nil {
		return &models.BingoCard{ID: uuid.New()}, nil
	}
	return m.ImportFunc(ctx, params)
}

func TestTemplateService_Create_List_Get(t *testing.T) {
	t.Parallel()

	db := newMemDB()
	cardSvc := &mockCardServiceForTemplates{}
	svc := NewTemplateService(db, cardSvc)
	userID := uuid.New()

	tpl, err := svc.Create(context.Background(), userID, models.CreateTemplateParams{
		Name:                    "  My Template  ",
		Category:                ptrString("personal"),
		GridSize:                5,
		HeaderText:              "bingo",
		HasFreeSpace:            true,
		DefaultVisibleToFriends: true,
		Items:                   []string{" a ", "b"},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if tpl.Template.UserID != userID {
		t.Fatalf("unexpected user id: %v", tpl.Template.UserID)
	}
	if tpl.Template.Name != "My Template" {
		t.Fatalf("unexpected name: %q", tpl.Template.Name)
	}
	if tpl.Template.HeaderText != "BINGO" {
		t.Fatalf("unexpected header: %q", tpl.Template.HeaderText)
	}
	if len(tpl.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(tpl.Items))
	}
	if tpl.Items[0].SortOrder != 0 || tpl.Items[1].SortOrder != 1 {
		t.Fatalf("unexpected sort order: %#v", tpl.Items)
	}

	list, err := svc.List(context.Background(), userID)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 1 || list[0].ID != tpl.Template.ID {
		t.Fatalf("unexpected list: %#v", list)
	}

	got, err := svc.Get(context.Background(), userID, tpl.Template.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.Template.ID != tpl.Template.ID || len(got.Items) != 2 {
		t.Fatalf("unexpected get: %#v", got)
	}
}

func TestTemplateService_Create_EnforcesLimit(t *testing.T) {
	t.Parallel()

	db := newMemDB()
	userID := uuid.New()
	for i := 0; i < 50; i++ {
		tpl := models.CardTemplate{
			ID:         uuid.New(),
			UserID:     userID,
			Name:       fmt.Sprintf("t%d", i),
			GridSize:   5,
			HeaderText: "BINGO",
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
		db.state.templates[tpl.ID] = tpl
		db.state.items[tpl.ID] = []models.CardTemplateItem{}
	}

	svc := NewTemplateService(db, &mockCardServiceForTemplates{})
	_, err := svc.Create(context.Background(), userID, models.CreateTemplateParams{Name: "x"})
	if !errors.Is(err, ErrTemplateLimitReached) {
		t.Fatalf("expected ErrTemplateLimitReached, got %v", err)
	}
}

func TestTemplateService_Get_NotFound(t *testing.T) {
	t.Parallel()

	db := newMemDB()
	svc := NewTemplateService(db, &mockCardServiceForTemplates{})
	_, err := svc.Get(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestTemplateService_Update_ValidatesCapacity(t *testing.T) {
	t.Parallel()

	db := newMemDB()
	svc := NewTemplateService(db, &mockCardServiceForTemplates{})
	userID := uuid.New()

	create, err := svc.Create(context.Background(), userID, models.CreateTemplateParams{
		Name:         "big",
		GridSize:     5,
		HasFreeSpace: true,
		Items:        makeNItems(24),
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	_, err = svc.Update(context.Background(), userID, create.Template.ID, models.UpdateTemplateParams{
		Name:         "big",
		GridSize:     2,
		HeaderText:   "BI",
		HasFreeSpace: true,
	})
	var vErr *TemplateValidationError
	if err == nil || !errors.As(err, &vErr) {
		t.Fatalf("expected TemplateValidationError, got %v", err)
	}
}

func TestTemplateService_ReplaceItems(t *testing.T) {
	t.Parallel()

	db := newMemDB()
	svc := NewTemplateService(db, &mockCardServiceForTemplates{})
	userID := uuid.New()

	tpl, err := svc.Create(context.Background(), userID, models.CreateTemplateParams{
		Name:         "small",
		GridSize:     2,
		HeaderText:   "BI",
		HasFreeSpace: true,
		Items:        []string{"a"},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	before := db.state.templates[tpl.Template.ID].UpdatedAt

	updated, err := svc.ReplaceItems(context.Background(), userID, tpl.Template.ID, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("ReplaceItems error: %v", err)
	}
	if len(updated.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(updated.Items))
	}
	after := db.state.templates[tpl.Template.ID].UpdatedAt
	if !after.After(before) {
		t.Fatalf("expected updated_at to increase")
	}

	_, err = svc.ReplaceItems(context.Background(), userID, tpl.Template.ID, []string{"a", "b", "c", "d"})
	var vErr *TemplateValidationError
	if err == nil || !errors.As(err, &vErr) {
		t.Fatalf("expected TemplateValidationError, got %v", err)
	}

	got, _ := svc.Get(context.Background(), userID, tpl.Template.ID)
	if len(got.Items) != 3 {
		t.Fatalf("expected items unchanged after over-capacity error, got %d", len(got.Items))
	}
}

func TestTemplateService_Delete(t *testing.T) {
	t.Parallel()

	db := newMemDB()
	svc := NewTemplateService(db, &mockCardServiceForTemplates{})
	userID := uuid.New()

	tpl, err := svc.Create(context.Background(), userID, models.CreateTemplateParams{Name: "x"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if err := svc.Delete(context.Background(), userID, tpl.Template.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if err := svc.Delete(context.Background(), userID, tpl.Template.ID); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestTemplateService_CreateCardFromTemplate_UsesDeterministicFreeSpace(t *testing.T) {
	t.Parallel()

	db := newMemDB()
	cardSvc := &mockCardServiceForTemplates{
		CheckForConflictFunc: func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
			return nil, ErrCardNotFound
		},
	}
	svc := NewTemplateService(db, cardSvc)
	userID := uuid.New()

	tpl, err := svc.Create(context.Background(), userID, models.CreateTemplateParams{
		Name:         "tpl",
		GridSize:     5,
		HasFreeSpace: true,
		Items:        []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	_, err = svc.CreateCardFromTemplate(context.Background(), userID, tpl.Template.ID, models.TemplateCreateCardParams{
		Year:          2026,
		ShuffleLayout: false,
	})
	if err != nil {
		t.Fatalf("CreateCardFromTemplate error: %v", err)
	}
	if len(cardSvc.ImportCalls) != 1 {
		t.Fatalf("expected Import to be called once")
	}
	call := cardSvc.ImportCalls[0]
	if call.FreeSpacePos == nil || *call.FreeSpacePos != 12 {
		t.Fatalf("expected FREE to be at 12, got %v", call.FreeSpacePos)
	}
	for _, it := range call.Items {
		if it.Position == 12 {
			t.Fatalf("expected no item at FREE position")
		}
	}
}

func TestTemplateService_CreateCardFromTemplate_ReturnsConflict(t *testing.T) {
	t.Parallel()

	db := newMemDB()
	cardSvc := &mockCardServiceForTemplates{
		CheckForConflictFunc: func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
			if title != nil && *title == "2026 Bingo Card" {
				return &models.BingoCard{ID: uuid.New()}, nil
			}
			return nil, ErrCardNotFound
		},
	}
	svc := NewTemplateService(db, cardSvc)
	userID := uuid.New()

	tpl, err := svc.Create(context.Background(), userID, models.CreateTemplateParams{
		Name:     "tpl",
		GridSize: 5,
		Items:    []string{"a"},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	_, err = svc.CreateCardFromTemplate(context.Background(), userID, tpl.Template.ID, models.TemplateCreateCardParams{
		Year:          2026,
		ShuffleLayout: false,
	})
	var conflict *CardConflictError
	if err == nil || !errors.As(err, &conflict) {
		t.Fatalf("expected CardConflictError, got %v", err)
	}
	if conflict.SuggestedTitle == "" {
		t.Fatalf("expected a suggested title")
	}
}

func TestTemplateService_RolloverCard_IncompleteOnlyPreservesPositions(t *testing.T) {
	t.Parallel()

	sourceID := uuid.New()
	userID := uuid.New()
	cardSvc := &mockCardServiceForTemplates{
		CheckForConflictFunc: func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
			return nil, ErrCardNotFound
		},
		GetByIDFunc: func(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error) {
			return &models.BingoCard{
				ID:           sourceID,
				UserID:       userID,
				Year:         2025,
				Title:        ptrString("My Card"),
				GridSize:     5,
				HeaderText:   "BINGO",
				HasFreeSpace: true,
				FreeSpacePos: ptrInt(12),
				Items: []models.BingoItem{
					{Position: 0, Content: "a", IsCompleted: true},
					{Position: 1, Content: "b", IsCompleted: false},
					{Position: 3, Content: "c", IsCompleted: false},
				},
			}, nil
		},
	}

	db := newMemDB()
	svc := NewTemplateService(db, cardSvc)

	_, err := svc.RolloverCard(context.Background(), userID, sourceID, models.RolloverParams{
		Year:          2026,
		CarryOver:     "incomplete_only",
		ShuffleLayout: false,
	})
	if err != nil {
		t.Fatalf("RolloverCard error: %v", err)
	}
	if len(cardSvc.ImportCalls) != 1 {
		t.Fatalf("expected Import to be called once")
	}
	call := cardSvc.ImportCalls[0]
	if len(call.Items) != 2 {
		t.Fatalf("expected 2 carried items, got %d", len(call.Items))
	}
	if call.Items[0].Position != 1 || call.Items[1].Position != 3 {
		t.Fatalf("unexpected positions: %#v", call.Items)
	}
}

func ptrString(s string) *string { return &s }
func ptrInt(n int) *int          { return &n }

func makeNItems(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("item %d", i))
	}
	return out
}
