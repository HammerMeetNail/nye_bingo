package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type revokeRow struct {
	scanFunc func(dest ...any) error
}

func (r revokeRow) Scan(dest ...any) error {
	if r.scanFunc == nil {
		return fmt.Errorf("scanFunc not set")
	}
	return r.scanFunc(dest...)
}

type revokeDBStub struct {
	QueryRowFunc func(ctx context.Context, sql string, args ...any) services.Row
	ExecFunc     func(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
}

func (r *revokeDBStub) QueryRow(ctx context.Context, sql string, args ...any) services.Row {
	return r.QueryRowFunc(ctx, sql, args...)
}

func (r *revokeDBStub) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	return r.ExecFunc(ctx, sql, args...)
}

type revokeTag struct{}

func (revokeTag) RowsAffected() int64 { return 1 }

func assignRevoke(dest []any, values []any) error {
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

func revokeRowFromValues(values ...any) services.Row {
	return revokeRow{scanFunc: func(dest ...any) error {
		return assignRevoke(dest, values)
	}}
}

func TestParseRevokeFlags(t *testing.T) {
	opts, err := parseRevokeFlags([]string{"--email=test@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.email != "test@example.com" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestValidateRevokeOptions(t *testing.T) {
	if err := validateRevokeOptions(revokeOptions{}); err == nil {
		t.Fatal("expected error for missing email")
	}
}

func TestNormalizeRevokeEmail(t *testing.T) {
	got := normalizeRevokeEmail(" Test@Example.com ")
	if got != "test@example.com" {
		t.Fatalf("unexpected normalized email: %s", got)
	}
}

func TestRevokePremium_Success(t *testing.T) {
	userID := uuid.New()
	db := &revokeDBStub{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return revokeRowFromValues(userID)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return revokeTag{}, nil
		},
	}
	err := revokePremium(context.Background(), db, "test@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRevokePremium_QueryError(t *testing.T) {
	db := &revokeDBStub{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return revokeRow{scanFunc: func(dest ...any) error { return fmt.Errorf("nope") }}
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return revokeTag{}, nil
		},
	}
	err := revokePremium(context.Background(), db, "test@example.com")
	if err == nil || !strings.Contains(err.Error(), "find user") {
		t.Fatalf("expected find user error, got %v", err)
	}
}

func TestRevokePremium_ExecError(t *testing.T) {
	db := &revokeDBStub{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return revokeRowFromValues(uuid.New())
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return revokeTag{}, fmt.Errorf("fail")
		},
	}
	err := revokePremium(context.Background(), db, "test@example.com")
	if err == nil || !strings.Contains(err.Error(), "revoke premium") {
		t.Fatalf("expected revoke premium error, got %v", err)
	}
}
