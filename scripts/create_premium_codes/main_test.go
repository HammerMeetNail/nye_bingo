package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type stubCommandTag struct{}

func (stubCommandTag) RowsAffected() int64 { return 1 }

type stubExecer struct {
	calls int
	args  []any
	err   error
}

func (s *stubExecer) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	s.calls++
	s.args = args
	return stubCommandTag{}, s.err
}

func TestParseCreatePremiumFlags(t *testing.T) {
	opts, err := parseCreatePremiumFlags([]string{"--count=2", "--duration_days=30", "--expires_days=7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.count != 2 || opts.durationDays != 30 || opts.expiresDays != 7 {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestValidateCreatePremiumOptions(t *testing.T) {
	if err := validateCreatePremiumOptions(createPremiumOptions{count: 0}); err == nil {
		t.Fatal("expected error for count")
	}
	if err := validateCreatePremiumOptions(createPremiumOptions{count: 1, lifetime: true, durationDays: 5}); err == nil {
		t.Fatal("expected error for lifetime with duration")
	}
	if err := validateCreatePremiumOptions(createPremiumOptions{count: 1, durationDays: -1}); err == nil {
		t.Fatal("expected error for duration")
	}
	if err := validateCreatePremiumOptions(createPremiumOptions{count: 1, expiresDays: -1}); err == nil {
		t.Fatal("expected error for expires")
	}
}

func TestBuildExpiresAt(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if buildExpiresAt(now, 0) != nil {
		t.Fatal("expected nil expiresAt")
	}
	got := buildExpiresAt(now, 2)
	if got == nil || !got.Equal(now.Add(48*time.Hour)) {
		t.Fatalf("unexpected expiresAt: %v", got)
	}
}

func TestBuildDurationPtr(t *testing.T) {
	if buildDurationPtr(0, false) != nil {
		t.Fatal("expected nil duration")
	}
	if buildDurationPtr(10, true) != nil {
		t.Fatal("expected nil duration for lifetime")
	}
	got := buildDurationPtr(10, false)
	if got == nil || *got != 10 {
		t.Fatalf("unexpected duration: %v", got)
	}
}

func TestGeneratePremiumCode(t *testing.T) {
	raw := bytes.Repeat([]byte{0}, 15)
	normalized, display, hashHex, err := generatePremiumCode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(normalized, "YOBP") {
		t.Fatalf("unexpected normalized: %s", normalized)
	}
	if !strings.HasPrefix(display, "YOBP-") {
		t.Fatalf("unexpected display: %s", display)
	}
	if len(hashHex) != 64 {
		t.Fatalf("unexpected hash length: %d", len(hashHex))
	}
}

func TestRunCreatePremium(t *testing.T) {
	raw := bytes.Repeat([]byte{1}, 30)
	out := &bytes.Buffer{}
	execer := &stubExecer{}
	opts := createPremiumOptions{count: 2, lifetime: true}
	deps := createPremiumDeps{
		randReader: bytes.NewReader(raw),
		now:        func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
		execer:     execer,
		out:        out,
	}

	if err := runCreatePremium(context.Background(), opts, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if execer.calls != 2 {
		t.Fatalf("expected 2 inserts, got %d", execer.calls)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestInsertPremiumCode_Error(t *testing.T) {
	execer := &stubExecer{err: context.Canceled}
	err := insertPremiumCode(context.Background(), execer, "hash", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
