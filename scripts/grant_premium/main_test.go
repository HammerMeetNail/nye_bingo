package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type grantRow struct {
	scanFunc func(dest ...any) error
}

func (r grantRow) Scan(dest ...any) error {
	if r.scanFunc == nil {
		return fmt.Errorf("scanFunc not set")
	}
	return r.scanFunc(dest...)
}

type grantDBStub struct {
	QueryRowFunc func(ctx context.Context, sql string, args ...any) services.Row
	ExecFunc     func(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
}

func (g *grantDBStub) QueryRow(ctx context.Context, sql string, args ...any) services.Row {
	return g.QueryRowFunc(ctx, sql, args...)
}

func (g *grantDBStub) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	return g.ExecFunc(ctx, sql, args...)
}

type grantTag struct{}

func (grantTag) RowsAffected() int64 { return 1 }

func assign(dest []any, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("scan dest mismatch")
	}
	for i, value := range values {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr || dv.IsNil() {
			return fmt.Errorf("dest %d not pointer", i)
		}
		if value == nil {
			dv.Elem().Set(reflect.Zero(dv.Elem().Type()))
			continue
		}
		vv := reflect.ValueOf(value)
		if vv.Type().AssignableTo(dv.Elem().Type()) {
			dv.Elem().Set(vv)
			continue
		}
		if vv.Type().ConvertibleTo(dv.Elem().Type()) {
			dv.Elem().Set(vv.Convert(dv.Elem().Type()))
			continue
		}
		return fmt.Errorf("cannot assign %T", value)
	}
	return nil
}

func rowFromValues(values ...any) services.Row {
	return grantRow{scanFunc: func(dest ...any) error {
		return assign(dest, values)
	}}
}

func TestParseGrantFlags(t *testing.T) {
	opts, err := parseGrantFlags([]string{"--email=test@example.com", "--duration_days=7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.email != "test@example.com" || opts.durationDays != 7 {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestValidateGrantOptions(t *testing.T) {
	if err := validateGrantOptions(grantOptions{}); err == nil {
		t.Fatal("expected error for missing email")
	}
	if err := validateGrantOptions(grantOptions{email: "a@b.com", lifetime: true, durationDays: 1}); err == nil {
		t.Fatal("expected error for lifetime + duration")
	}
	if err := validateGrantOptions(grantOptions{email: "a@b.com"}); err == nil {
		t.Fatal("expected error for missing duration")
	}
	if err := validateGrantOptions(grantOptions{email: "a@b.com", durationDays: -1}); err == nil {
		t.Fatal("expected error for negative duration")
	}
}

func TestNormalizeEmail(t *testing.T) {
	got := normalizeEmail(" Test@Example.com ")
	if got != "test@example.com" {
		t.Fatalf("unexpected normalized email: %s", got)
	}
}

func TestBuildGrantPeriodEnd(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if buildGrantPeriodEnd(now, 0, true) != nil {
		t.Fatal("expected nil period end for lifetime")
	}
	period := buildGrantPeriodEnd(now, 7, false)
	if period == nil || !period.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("unexpected period end: %v", period)
	}
}

func TestGrantPremium_Success(t *testing.T) {
	userID := uuid.New()
	var execArgs []any
	db := &grantDBStub{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues(userID)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			execArgs = args
			return grantTag{}, nil
		},
	}
	period := time.Now().UTC()
	err := grantPremium(context.Background(), db, "test@example.com", &period)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(execArgs) < 2 {
		t.Fatalf("expected exec args, got %v", execArgs)
	}
}

func TestGrantPremium_QueryError(t *testing.T) {
	db := &grantDBStub{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return grantRow{scanFunc: func(dest ...any) error { return fmt.Errorf("nope") }}
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return grantTag{}, nil
		},
	}
	err := grantPremium(context.Background(), db, "test@example.com", nil)
	if err == nil || !strings.Contains(err.Error(), "find user") {
		t.Fatalf("expected find user error, got %v", err)
	}
}

func TestGrantPremium_ExecError(t *testing.T) {
	db := &grantDBStub{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues(uuid.New())
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return grantTag{}, fmt.Errorf("fail")
		},
	}
	err := grantPremium(context.Background(), db, "test@example.com", nil)
	if err == nil || !strings.Contains(err.Error(), "grant premium") {
		t.Fatalf("expected grant premium error, got %v", err)
	}
}
