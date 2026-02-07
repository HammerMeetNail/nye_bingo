package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func TestMonthStartUTC(t *testing.T) {
	now := time.Date(2026, 2, 7, 15, 4, 5, 0, time.FixedZone("UTC-5", -5*3600))
	got := monthStartUTC(now)
	want := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestGetPremiumEnhancementsStatus_NoUsageRow(t *testing.T) {
	svc := &Service{
		db: &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			},
		},
		premiumEnhancementsPerMonth: 100,
	}

	limit, used, remaining, resetsAt, err := svc.GetPremiumEnhancementsStatus(context.Background(), uuid.New(), time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 100 || used != 0 || remaining != 100 {
		t.Fatalf("unexpected status values limit=%d used=%d remaining=%d", limit, used, remaining)
	}
	if !resetsAt.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected resets_at: %s", resetsAt)
	}
}

func TestReservePremiumEnhancement_AtLimit(t *testing.T) {
	svc := &Service{
		db: &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			},
		},
		premiumEnhancementsPerMonth: 2,
	}

	_, _, err := svc.ReservePremiumEnhancement(context.Background(), uuid.New(), time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrPremiumEnhancementsExhausted) {
		t.Fatalf("expected ErrPremiumEnhancementsExhausted, got %v", err)
	}
}

func TestReservePremiumEnhancement_Success(t *testing.T) {
	svc := &Service{
		db: &fakeDB{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					*(dest[0].(*int)) = 3
					return nil
				}}
			},
		},
		premiumEnhancementsPerMonth: 5,
	}

	remaining, resetsAt, err := svc.ReservePremiumEnhancement(context.Background(), uuid.New(), time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remaining != 2 {
		t.Fatalf("expected remaining 2, got %d", remaining)
	}
	if !resetsAt.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected resets_at: %s", resetsAt)
	}
}

func TestRefundPremiumEnhancement_DBError(t *testing.T) {
	svc := &Service{
		db: &fakeDB{
			execFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
				return nil, errors.New("boom")
			},
		},
	}

	err := svc.RefundPremiumEnhancement(context.Background(), uuid.New(), time.Now())
	if !errors.Is(err, ErrAIUsageTrackingUnavailable) {
		t.Fatalf("expected ErrAIUsageTrackingUnavailable, got %v", err)
	}
}

func TestAssistGoal_InvalidMode(t *testing.T) {
	svc := &Service{}
	_, _, err := svc.AssistGoal(context.Background(), uuid.New(), "Read a book", "unknown", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGenerateFillGoals_InvalidCount(t *testing.T) {
	svc := &Service{}
	_, _, err := svc.GenerateFillGoals(context.Background(), uuid.New(), GoalPrompt{Count: 25}, nil)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
