package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/database"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func main() {
	opts, err := parseCreatePremiumFlags(os.Args[1:])
	if err != nil {
		log.Fatalf("parse flags: %v", err)
	}
	if err := validateCreatePremiumOptions(opts); err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.NewPostgresDB(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	dbAdapter := services.NewPoolAdapter(db.Pool)
	deps := createPremiumDeps{
		randReader: rand.Reader,
		now:        time.Now,
		execer:     dbAdapter,
		out:        os.Stdout,
	}
	if err := runCreatePremium(context.Background(), opts, deps); err != nil {
		log.Fatal(err)
	}
}

type createPremiumOptions struct {
	count        int
	durationDays int
	lifetime     bool
	expiresDays  int
}

type createPremiumDeps struct {
	randReader io.Reader
	now        func() time.Time
	execer     codeExecer
	out        io.Writer
}

type codeExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
}

func parseCreatePremiumFlags(args []string) (createPremiumOptions, error) {
	var opts createPremiumOptions
	fs := flag.NewFlagSet("create_premium_codes", flag.ContinueOnError)
	fs.IntVar(&opts.count, "count", 1, "number of codes to generate")
	fs.IntVar(&opts.durationDays, "duration_days", 0, "duration in days (0 means unset)")
	fs.BoolVar(&opts.lifetime, "lifetime", false, "generate lifetime codes")
	fs.IntVar(&opts.expiresDays, "expires_days", 0, "codes expire this many days from now (0 means no expiry)")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	return opts, nil
}

func validateCreatePremiumOptions(opts createPremiumOptions) error {
	if opts.count <= 0 || opts.count > 10000 {
		return fmt.Errorf("invalid --count: %d", opts.count)
	}
	if opts.lifetime && opts.durationDays > 0 {
		return fmt.Errorf("choose either --lifetime or --duration_days, not both")
	}
	if opts.durationDays < 0 {
		return fmt.Errorf("--duration_days must be >= 0")
	}
	if opts.expiresDays < 0 {
		return fmt.Errorf("--expires_days must be >= 0")
	}
	return nil
}

func runCreatePremium(ctx context.Context, opts createPremiumOptions, deps createPremiumDeps) error {
	expiresAt := buildExpiresAt(deps.now().UTC(), opts.expiresDays)
	durationPtr := buildDurationPtr(opts.durationDays, opts.lifetime)

	for i := 0; i < opts.count; i++ {
		_, display, hashHex, err := generatePremiumCode(deps.randReader)
		if err != nil {
			return fmt.Errorf("rand: %w", err)
		}
		if err := insertPremiumCode(ctx, deps.execer, hashHex, durationPtr, expiresAt); err != nil {
			return fmt.Errorf("insert code: %w", err)
		}
		if _, err := fmt.Fprintln(deps.out, display); err != nil {
			return err
		}
	}
	return nil
}

func buildExpiresAt(now time.Time, expiresDays int) *time.Time {
	if expiresDays <= 0 {
		return nil
	}
	t := now.Add(time.Duration(expiresDays) * 24 * time.Hour)
	return &t
}

func buildDurationPtr(durationDays int, lifetime bool) *int {
	if lifetime || durationDays <= 0 {
		return nil
	}
	return &durationDays
}

func generatePremiumCode(randReader io.Reader) (string, string, string, error) {
	raw := make([]byte, 15)
	if _, err := io.ReadFull(randReader, raw); err != nil {
		return "", "", "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	suffix := enc.EncodeToString(raw)
	normalized := "YOBP" + suffix
	display := "YOBP-" + group4(suffix)
	sum := sha256.Sum256([]byte(normalized))
	hashHex := hex.EncodeToString(sum[:])
	return normalized, display, hashHex, nil
}

func insertPremiumCode(ctx context.Context, execer codeExecer, hashHex string, durationPtr *int, expiresAt *time.Time) error {
	_, err := execer.Exec(ctx,
		`INSERT INTO premium_codes (code_hash, duration_days, expires_at)
		 VALUES ($1, $2, $3)`,
		hashHex, durationPtr, expiresAt,
	)
	return err
}

func group4(s string) string {
	s = strings.TrimSpace(s)
	var out strings.Builder
	for i, r := range s {
		if i > 0 && i%4 == 0 {
			out.WriteByte('-')
		}
		out.WriteRune(r)
	}
	return out.String()
}
