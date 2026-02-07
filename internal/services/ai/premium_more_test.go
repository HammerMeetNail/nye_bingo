package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type fakeCommandTag struct {
	rows int64
}

func (f fakeCommandTag) RowsAffected() int64 { return f.rows }

type rowsFixture struct {
	rows [][]any
	idx  int
	err  error
}

func (r *rowsFixture) Close() {}

func (r *rowsFixture) Err() error { return r.err }

func (r *rowsFixture) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *rowsFixture) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return fmt.Errorf("scan called without next")
	}
	row := r.rows[r.idx-1]
	if len(dest) != len(row) {
		return fmt.Errorf("scan destination mismatch got=%d want=%d", len(dest), len(row))
	}
	for i := range row {
		if err := assignScanValue(dest[i], row[i]); err != nil {
			return err
		}
	}
	return nil
}

func assignScanValue(dest any, value any) error {
	switch d := dest.(type) {
	case *uuid.UUID:
		v, ok := value.(uuid.UUID)
		if !ok {
			return fmt.Errorf("expected uuid.UUID, got %T", value)
		}
		*d = v
	case *int:
		v, ok := value.(int)
		if !ok {
			return fmt.Errorf("expected int, got %T", value)
		}
		*d = v
	case *string:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
		*d = v
	case *bool:
		v, ok := value.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %T", value)
		}
		*d = v
	case *time.Time:
		v, ok := value.(time.Time)
		if !ok {
			return fmt.Errorf("expected time.Time, got %T", value)
		}
		*d = v
	case **time.Time:
		if value == nil {
			*d = nil
			return nil
		}
		v, ok := value.(time.Time)
		if !ok {
			return fmt.Errorf("expected time.Time or nil, got %T", value)
		}
		t := v
		*d = &t
	case **string:
		if value == nil {
			*d = nil
			return nil
		}
		switch v := value.(type) {
		case string:
			s := v
			*d = &s
		case *string:
			if v == nil {
				*d = nil
				return nil
			}
			s := *v
			*d = &s
		default:
			return fmt.Errorf("expected string/*string or nil, got %T", value)
		}
	case **int:
		if value == nil {
			*d = nil
			return nil
		}
		switch v := value.(type) {
		case int:
			i := v
			*d = &i
		case *int:
			if v == nil {
				*d = nil
				return nil
			}
			i := *v
			*d = &i
		default:
			return fmt.Errorf("expected int/*int or nil, got %T", value)
		}
	default:
		return fmt.Errorf("unsupported scan destination %T", dest)
	}
	return nil
}

type txFixture struct {
	execFunc     func(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
	queryFunc    func(ctx context.Context, sql string, args ...any) (services.Rows, error)
	queryRowFunc func(ctx context.Context, sql string, args ...any) services.Row
	commitErr    error
	rollbackErr  error
	commits      int
	rollbacks    int
}

func (t *txFixture) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	if t.execFunc != nil {
		return t.execFunc(ctx, sql, args...)
	}
	return fakeCommandTag{rows: 1}, nil
}

func (t *txFixture) Query(ctx context.Context, sql string, args ...any) (services.Rows, error) {
	if t.queryFunc != nil {
		return t.queryFunc(ctx, sql, args...)
	}
	return &rowsFixture{}, nil
}

func (t *txFixture) QueryRow(ctx context.Context, sql string, args ...any) services.Row {
	if t.queryRowFunc != nil {
		return t.queryRowFunc(ctx, sql, args...)
	}
	return fakeRow{scanFunc: func(dest ...any) error { return nil }}
}

func (t *txFixture) Commit(ctx context.Context) error {
	t.commits++
	return t.commitErr
}

func (t *txFixture) Rollback(ctx context.Context) error {
	t.rollbacks++
	return t.rollbackErr
}

type dbConnOnlyFixture struct {
	queryRowFunc func(ctx context.Context, sql string, args ...any) services.Row
}

func (d *dbConnOnlyFixture) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	return fakeCommandTag{rows: 1}, nil
}

func (d *dbConnOnlyFixture) Query(ctx context.Context, sql string, args ...any) (services.Rows, error) {
	return &rowsFixture{}, nil
}

func (d *dbConnOnlyFixture) QueryRow(ctx context.Context, sql string, args ...any) services.Row {
	if d.queryRowFunc != nil {
		return d.queryRowFunc(ctx, sql, args...)
	}
	return fakeRow{scanFunc: func(dest ...any) error { return nil }}
}

func baseCard(userID, cardID uuid.UUID, gridSize int, hasFree bool, freePos *int, finalized bool) models.BingoCard {
	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	return models.BingoCard{
		ID:               cardID,
		UserID:           userID,
		Year:             2026,
		GridSize:         gridSize,
		HeaderText:       "BINGO",
		HasFreeSpace:     hasFree,
		FreeSpacePos:     freePos,
		IsActive:         true,
		IsFinalized:      finalized,
		VisibleToFriends: true,
		IsArchived:       false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func cardRowValues(card models.BingoCard) []any {
	var category any
	if card.Category != nil {
		category = *card.Category
	}
	var title any
	if card.Title != nil {
		title = *card.Title
	}
	var freeSpacePos any
	if card.FreeSpacePos != nil {
		freeSpacePos = *card.FreeSpacePos
	}
	return []any{
		card.ID,
		card.UserID,
		card.Year,
		category,
		title,
		card.GridSize,
		card.HeaderText,
		card.HasFreeSpace,
		freeSpacePos,
		card.IsActive,
		card.IsFinalized,
		card.VisibleToFriends,
		card.IsArchived,
		card.CreatedAt,
		card.UpdatedAt,
	}
}

func itemRowValues(itemID, cardID uuid.UUID, pos int, content string) []any {
	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	return []any{
		itemID,
		cardID,
		pos,
		content,
		false,
		nil,
		nil,
		nil,
		now,
	}
}

func TestPremiumMonthlyLimit_DefaultFallback(t *testing.T) {
	svc := &Service{premiumEnhancementsPerMonth: 0}
	if got := svc.premiumMonthlyLimit(); got != defaultPremiumEnhancementsPerMonth {
		t.Fatalf("expected default monthly limit %d, got %d", defaultPremiumEnhancementsPerMonth, got)
	}
}

func TestGetPremiumEnhancementsStatus_DBNil(t *testing.T) {
	svc := &Service{premiumEnhancementsPerMonth: 100}
	limit, used, remaining, _, err := svc.GetPremiumEnhancementsStatus(context.Background(), uuid.New(), time.Now())
	if !errors.Is(err, ErrAIUsageTrackingUnavailable) {
		t.Fatalf("expected ErrAIUsageTrackingUnavailable, got %v", err)
	}
	if limit != 100 || used != 0 || remaining != 100 {
		t.Fatalf("unexpected status values limit=%d used=%d remaining=%d", limit, used, remaining)
	}
}

func TestGetPremiumEnhancementsStatus_DBErrorAndRemainingClamp(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC)

	svcErr := &Service{
		db: &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error { return errors.New("boom") }}
			},
		},
		premiumEnhancementsPerMonth: 100,
	}
	if _, _, _, _, err := svcErr.GetPremiumEnhancementsStatus(context.Background(), userID, now); !errors.Is(err, ErrAIUsageTrackingUnavailable) {
		t.Fatalf("expected ErrAIUsageTrackingUnavailable, got %v", err)
	}

	svcClamp := &Service{
		db: &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					*(dest[0].(*int)) = 150
					return nil
				}}
			},
		},
		premiumEnhancementsPerMonth: 100,
	}
	_, _, remaining, _, err := svcClamp.GetPremiumEnhancementsStatus(context.Background(), userID, now)
	if err != nil {
		t.Fatalf("unexpected clamp status error: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected remaining clamped to 0, got %d", remaining)
	}
}

func TestReservePremiumEnhancement_DBNilAndDBError(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC)

	svcNil := &Service{}
	if _, _, err := svcNil.ReservePremiumEnhancement(context.Background(), userID, now); !errors.Is(err, ErrAIUsageTrackingUnavailable) {
		t.Fatalf("expected ErrAIUsageTrackingUnavailable for nil DB, got %v", err)
	}

	svcErr := &Service{
		db: &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error { return errors.New("boom") }}
			},
		},
		premiumEnhancementsPerMonth: 100,
	}
	if _, _, err := svcErr.ReservePremiumEnhancement(context.Background(), userID, now); !errors.Is(err, ErrAIUsageTrackingUnavailable) {
		t.Fatalf("expected ErrAIUsageTrackingUnavailable for DB error, got %v", err)
	}
}

func TestRefundPremiumEnhancement_SuccessUsesMonthStart(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 2, 7, 10, 0, 0, 0, time.UTC)
	var gotMonthStart time.Time

	svc := &Service{
		db: &fakeDB{
			execFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
				ms, ok := args[1].(time.Time)
				if !ok {
					return nil, fmt.Errorf("expected time.Time month_start arg, got %T", args[1])
				}
				gotMonthStart = ms
				return fakeCommandTag{rows: 1}, nil
			},
		},
	}

	if err := svc.RefundPremiumEnhancement(context.Background(), userID, now); err != nil {
		t.Fatalf("unexpected refund error: %v", err)
	}
	if !gotMonthStart.Equal(monthStartUTC(now)) {
		t.Fatalf("expected monthStart %s, got %s", monthStartUTC(now), gotMonthStart)
	}
}

func TestRegenerateGoal_StubSuccessAndDuplicate(t *testing.T) {
	svc := &Service{stub: true}
	userID := uuid.New()
	prompt := GoalPrompt{
		Category:   "travel",
		Difficulty: "medium",
		Budget:     "free",
	}

	goal, _, err := svc.RegenerateGoal(context.Background(), userID, prompt, []string{"Read one chapter", "Take a walk"}, 0)
	if err != nil {
		t.Fatalf("unexpected regenerate error: %v", err)
	}
	if strings.TrimSpace(goal) == "" {
		t.Fatal("expected non-empty regenerated goal")
	}

	_, _, err = svc.RegenerateGoal(context.Background(), userID, prompt, []string{"Read one chapter", "Read one chapter (refined 1)"}, 0)
	if !errors.Is(err, ErrAIProviderUnavailable) {
		t.Fatalf("expected ErrAIProviderUnavailable for duplicate regenerated goal, got %v", err)
	}
}

func TestGenerateFillGoals_StubSuccessAndDuplicate(t *testing.T) {
	svc := &Service{stub: true}
	userID := uuid.New()

	prompt := GoalPrompt{
		Category:   "hobbies",
		Difficulty: "easy",
		Budget:     "free",
		Count:      2,
	}
	goals, _, err := svc.GenerateFillGoals(context.Background(), userID, prompt, []string{"Existing one"})
	if err != nil {
		t.Fatalf("unexpected fill generation error: %v", err)
	}
	if len(goals) != 2 {
		t.Fatalf("expected 2 goals, got %d", len(goals))
	}

	promptDup := GoalPrompt{
		Category:   "hobbies",
		Difficulty: "easy",
		Budget:     "free",
		Count:      1,
	}
	existing := []string{stubGoals(promptDup)[0]}
	_, _, err = svc.GenerateFillGoals(context.Background(), userID, promptDup, existing)
	if !errors.Is(err, ErrAIProviderUnavailable) {
		t.Fatalf("expected ErrAIProviderUnavailable for duplicate generated goal, got %v", err)
	}
}

func TestAssistGoal_StubAndNotConfigured(t *testing.T) {
	userID := uuid.New()

	svcStub := &Service{stub: true}
	reply, stats, err := svcStub.AssistGoal(context.Background(), userID, "Complete a 5k run", "next_step", "I only have 20 minutes")
	if err != nil {
		t.Fatalf("unexpected stub assist error: %v", err)
	}
	if !strings.Contains(reply, "Goal: Complete a 5k run") {
		t.Fatalf("expected stub reply to include goal, got %q", reply)
	}
	if stats.Model != "stub" {
		t.Fatalf("expected stub stats model, got %q", stats.Model)
	}

	svcNoKey := &Service{stub: false, apiKey: "", model: "test-model"}
	_, _, err = svcNoKey.AssistGoal(context.Background(), userID, "Complete a 5k run", "next_step", "")
	if !errors.Is(err, ErrAINotConfigured) {
		t.Fatalf("expected ErrAINotConfigured, got %v", err)
	}
}

func TestAssistGoal_HTTPAndPayloadErrors(t *testing.T) {
	userID := uuid.New()

	svc429 := &Service{
		apiKey: "test-key",
		model:  "test-model",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{"error":"rate limit"}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		})},
	}
	_, _, err := svc429.AssistGoal(context.Background(), userID, "Do one stretch", "next_step", "")
	if !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("expected ErrRateLimitExceeded, got %v", err)
	}

	svcBadJSON := &Service{
		apiKey: "test-key",
		model:  "test-model",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp := geminiResponse{
				Candidates: []geminiCandidate{
					{
						Content:      geminiContent{Parts: []geminiPart{{Text: "not-json"}}},
						FinishReason: "STOP",
					},
				},
			}
			return jsonHTTPResponse(t, http.StatusOK, resp), nil
		})},
	}
	_, _, err = svcBadJSON.AssistGoal(context.Background(), userID, "Do one stretch", "next_step", "")
	if !errors.Is(err, ErrAIProviderUnavailable) {
		t.Fatalf("expected ErrAIProviderUnavailable for bad JSON payload, got %v", err)
	}
}

func TestAssistGoal_OffTopicAndEmptyReply(t *testing.T) {
	userID := uuid.New()

	svcOffTopic := &Service{
		apiKey: "test-key",
		model:  "test-model",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp := geminiResponse{
				Candidates: []geminiCandidate{
					{
						Content:      geminiContent{Parts: []geminiPart{{Text: `{"is_on_goal":false,"reply":"off topic"}`}}},
						FinishReason: "STOP",
					},
				},
			}
			return jsonHTTPResponse(t, http.StatusOK, resp), nil
		})},
	}
	reply, _, err := svcOffTopic.AssistGoal(context.Background(), userID, "Read one chapter", "next_step", "")
	if err != nil {
		t.Fatalf("unexpected off-topic assist error: %v", err)
	}
	wantRefusal := "I can only help with the selected goal. Share constraints tied to this goal and I can help plan it."
	if reply != wantRefusal {
		t.Fatalf("expected refusal reply, got %q", reply)
	}

	svcEmptyReply := &Service{
		apiKey: "test-key",
		model:  "test-model",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp := geminiResponse{
				Candidates: []geminiCandidate{
					{
						Content:      geminiContent{Parts: []geminiPart{{Text: `{"is_on_goal":true,"reply":"   "}`}}},
						FinishReason: "STOP",
					},
				},
			}
			return jsonHTTPResponse(t, http.StatusOK, resp), nil
		})},
	}
	_, _, err = svcEmptyReply.AssistGoal(context.Background(), userID, "Read one chapter", "next_step", "")
	if !errors.Is(err, ErrAIProviderUnavailable) {
		t.Fatalf("expected ErrAIProviderUnavailable for empty reply, got %v", err)
	}
}

func TestAssistCardGoal_SuccessAndLookupErrors(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	freePos := 0
	card := baseCard(userID, cardID, 2, true, &freePos, false)

	t.Run("card not found", func(t *testing.T) {
		svc := &Service{
			db: &fakeDB{
				queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
					return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
				},
			},
			stub: true,
		}

		_, _, err := svc.AssistCardGoal(context.Background(), userID, cardID, 1, "next_step", "")
		if !errors.Is(err, services.ErrCardNotFound) {
			t.Fatalf("expected ErrCardNotFound, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		call := 0
		svc := &Service{
			db: &fakeDB{
				queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
					call++
					if call == 1 {
						return fakeRow{scanFunc: func(dest ...any) error {
							row := cardRowValues(card)
							for i := range row {
								if err := assignScanValue(dest[i], row[i]); err != nil {
									return err
								}
							}
							return nil
						}}
					}
					return fakeRow{scanFunc: func(dest ...any) error {
						*(dest[0].(*string)) = "Walk 10 minutes"
						return nil
					}}
				},
			},
			stub: true,
		}

		reply, stats, err := svc.AssistCardGoal(context.Background(), userID, cardID, 1, "next_step", "after work")
		if err != nil {
			t.Fatalf("unexpected AssistCardGoal error: %v", err)
		}
		if !strings.Contains(reply, "Goal: Walk 10 minutes") {
			t.Fatalf("expected assist reply to reference looked-up goal, got %q", reply)
		}
		if stats.Model != "stub" {
			t.Fatalf("expected stub model stats, got %q", stats.Model)
		}
	})
}

func TestGoalTextForAssist_PositionAndItemLookup(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	freePos := 0
	card := baseCard(userID, cardID, 2, true, &freePos, false)

	t.Run("invalid position", func(t *testing.T) {
		call := 0
		svc := &Service{
			db: &fakeDB{
				queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
					call++
					if call == 1 {
						return fakeRow{scanFunc: func(dest ...any) error {
							row := cardRowValues(card)
							for i := range row {
								if err := assignScanValue(dest[i], row[i]); err != nil {
									return err
								}
							}
							return nil
						}}
					}
					return fakeRow{scanFunc: func(dest ...any) error { return nil }}
				},
			},
		}
		_, err := svc.goalTextForAssist(context.Background(), userID, cardID, 0)
		if !errors.Is(err, services.ErrInvalidPosition) {
			t.Fatalf("expected ErrInvalidPosition, got %v", err)
		}
	})

	t.Run("item not found", func(t *testing.T) {
		call := 0
		svc := &Service{
			db: &fakeDB{
				queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
					call++
					if call == 1 {
						return fakeRow{scanFunc: func(dest ...any) error {
							row := cardRowValues(card)
							for i := range row {
								if err := assignScanValue(dest[i], row[i]); err != nil {
									return err
								}
							}
							return nil
						}}
					}
					return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
				},
			},
		}
		_, err := svc.goalTextForAssist(context.Background(), userID, cardID, 1)
		if !errors.Is(err, services.ErrItemNotFound) {
			t.Fatalf("expected ErrItemNotFound, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		call := 0
		svc := &Service{
			db: &fakeDB{
				queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
					call++
					if call == 1 {
						return fakeRow{scanFunc: func(dest ...any) error {
							row := cardRowValues(card)
							for i := range row {
								if err := assignScanValue(dest[i], row[i]); err != nil {
									return err
								}
							}
							return nil
						}}
					}
					return fakeRow{scanFunc: func(dest ...any) error {
						*(dest[0].(*string)) = "  Walk 10 minutes  "
						return nil
					}}
				},
			},
		}
		text, err := svc.goalTextForAssist(context.Background(), userID, cardID, 1)
		if err != nil {
			t.Fatalf("unexpected goal lookup error: %v", err)
		}
		if text != "Walk 10 minutes" {
			t.Fatalf("expected trimmed goal text, got %q", text)
		}
	})
}

func TestLoadCardStateAndFillEmptyOnCard(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	card := baseCard(userID, cardID, 2, false, nil, false)

	t.Run("loadCardState", func(t *testing.T) {
		db := &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					row := cardRowValues(card)
					for i := range row {
						if err := assignScanValue(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				}}
			},
			queryFunc: func(ctx context.Context, sql string, args ...any) (services.Rows, error) {
				return &rowsFixture{rows: [][]any{
					itemRowValues(uuid.New(), cardID, 0, "Existing goal"),
				}}, nil
			},
		}
		svc := &Service{db: db}
		gotCard, empties, existing, err := svc.loadCardState(context.Background(), db, userID, cardID, false)
		if err != nil {
			t.Fatalf("unexpected loadCardState error: %v", err)
		}
		if gotCard.ID != cardID {
			t.Fatalf("unexpected card id %s", gotCard.ID)
		}
		if len(empties) != 3 {
			t.Fatalf("expected 3 empty positions, got %d", len(empties))
		}
		if len(existing) != 1 || existing[0] != "Existing goal" {
			t.Fatalf("unexpected existing goals: %#v", existing)
		}
	})

	t.Run("fill empty nil/non-tx db", func(t *testing.T) {
		svcNil := &Service{}
		if _, _, err := svcNil.FillEmptyOnCard(context.Background(), userID, cardID, GoalPrompt{}); !errors.Is(err, ErrAIUsageTrackingUnavailable) {
			t.Fatalf("expected ErrAIUsageTrackingUnavailable for nil db, got %v", err)
		}

		svcNoTx := &Service{db: &dbConnOnlyFixture{}}
		if _, _, err := svcNoTx.FillEmptyOnCard(context.Background(), userID, cardID, GoalPrompt{}); !errors.Is(err, ErrAIUsageTrackingUnavailable) {
			t.Fatalf("expected ErrAIUsageTrackingUnavailable for non-transaction db, got %v", err)
		}
	})

	t.Run("fill empty success", func(t *testing.T) {
		tx := &txFixture{}
		insertCalls := 0
		tx.execFunc = func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			insertCalls++
			return fakeCommandTag{rows: 1}, nil
		}

		txQueryCall := 0
		tx.queryFunc = func(ctx context.Context, sql string, args ...any) (services.Rows, error) {
			txQueryCall++
			if txQueryCall == 1 {
				return &rowsFixture{rows: [][]any{
					itemRowValues(uuid.New(), cardID, 0, "Existing goal"),
				}}, nil
			}
			return &rowsFixture{rows: [][]any{
				itemRowValues(uuid.New(), cardID, 0, "Existing goal"),
				itemRowValues(uuid.New(), cardID, 1, "Generated 1"),
				itemRowValues(uuid.New(), cardID, 2, "Generated 2"),
				itemRowValues(uuid.New(), cardID, 3, "Generated 3"),
			}}, nil
		}
		tx.queryRowFunc = func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				row := cardRowValues(card)
				for i := range row {
					if err := assignScanValue(dest[i], row[i]); err != nil {
						return err
					}
				}
				return nil
			}}
		}

		dbQueryCall := 0
		db := &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					row := cardRowValues(card)
					for i := range row {
						if err := assignScanValue(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				}}
			},
			queryFunc: func(ctx context.Context, sql string, args ...any) (services.Rows, error) {
				dbQueryCall++
				if dbQueryCall != 1 {
					return nil, fmt.Errorf("unexpected db query call %d", dbQueryCall)
				}
				return &rowsFixture{rows: [][]any{
					itemRowValues(uuid.New(), cardID, 0, "Existing goal"),
				}}, nil
			},
			beginFunc: func(ctx context.Context) (services.Tx, error) {
				return tx, nil
			},
		}

		svc := &Service{
			db:   db,
			stub: true,
		}
		updated, stats, err := svc.FillEmptyOnCard(context.Background(), userID, cardID, GoalPrompt{
			Category:   "travel",
			Difficulty: "easy",
			Budget:     "free",
		})
		if err != nil {
			t.Fatalf("unexpected FillEmptyOnCard error: %v", err)
		}
		if stats.Model != "stub" {
			t.Fatalf("expected stub stats model, got %q", stats.Model)
		}
		if insertCalls != 3 {
			t.Fatalf("expected 3 inserts for empty positions, got %d", insertCalls)
		}
		if tx.commits != 1 {
			t.Fatalf("expected transaction commit once, got %d", tx.commits)
		}
		if len(updated.Items) != 4 {
			t.Fatalf("expected 4 items after fill, got %d", len(updated.Items))
		}
	})
}

func TestFillEmptyOnCard_BeginErrorAndCardChanged(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	card := baseCard(userID, cardID, 2, false, nil, false)

	t.Run("begin error", func(t *testing.T) {
		db := &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					row := cardRowValues(card)
					for i := range row {
						if err := assignScanValue(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				}}
			},
			queryFunc: func(ctx context.Context, sql string, args ...any) (services.Rows, error) {
				return &rowsFixture{rows: [][]any{
					itemRowValues(uuid.New(), cardID, 0, "Existing goal"),
				}}, nil
			},
			beginFunc: func(ctx context.Context) (services.Tx, error) {
				return nil, errors.New("tx unavailable")
			},
		}
		svc := &Service{db: db, stub: true}

		_, _, err := svc.FillEmptyOnCard(context.Background(), userID, cardID, GoalPrompt{
			Category:   "travel",
			Difficulty: "easy",
			Budget:     "free",
		})
		if err == nil || !strings.Contains(err.Error(), "begin transaction") {
			t.Fatalf("expected begin transaction error, got %v", err)
		}
	})

	t.Run("card changed while generating", func(t *testing.T) {
		tx := &txFixture{}
		tx.queryRowFunc = func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				row := cardRowValues(card)
				for i := range row {
					if err := assignScanValue(dest[i], row[i]); err != nil {
						return err
					}
				}
				return nil
			}}
		}
		tx.queryFunc = func(ctx context.Context, sql string, args ...any) (services.Rows, error) {
			return &rowsFixture{rows: [][]any{
				itemRowValues(uuid.New(), cardID, 0, "Existing goal"),
				itemRowValues(uuid.New(), cardID, 1, "Another goal"),
			}}, nil
		}

		db := &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					row := cardRowValues(card)
					for i := range row {
						if err := assignScanValue(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				}}
			},
			queryFunc: func(ctx context.Context, sql string, args ...any) (services.Rows, error) {
				return &rowsFixture{rows: [][]any{
					itemRowValues(uuid.New(), cardID, 0, "Existing goal"),
				}}, nil
			},
			beginFunc: func(ctx context.Context) (services.Tx, error) {
				return tx, nil
			},
		}
		svc := &Service{db: db, stub: true}

		_, _, err := svc.FillEmptyOnCard(context.Background(), userID, cardID, GoalPrompt{
			Category:   "travel",
			Difficulty: "easy",
			Budget:     "free",
		})
		if !errors.Is(err, ErrAIProviderUnavailable) {
			t.Fatalf("expected ErrAIProviderUnavailable, got %v", err)
		}
		if tx.commits != 0 {
			t.Fatalf("expected no commit on changed-card failure, got %d", tx.commits)
		}
		if tx.rollbacks == 0 {
			t.Fatalf("expected rollback on changed-card failure")
		}
	})
}
