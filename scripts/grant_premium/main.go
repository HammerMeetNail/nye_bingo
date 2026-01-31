package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/database"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func main() {
	opts, err := parseGrantFlags(os.Args[1:])
	if err != nil {
		log.Fatalf("parse flags: %v", err)
	}
	if err := validateGrantOptions(opts); err != nil {
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
	periodEnd := buildGrantPeriodEnd(time.Now().UTC(), opts.durationDays, opts.lifetime)
	if err := grantPremium(context.Background(), dbAdapter, normalizeEmail(opts.email), periodEnd); err != nil {
		log.Fatal(err)
	}
}

type grantOptions struct {
	email        string
	durationDays int
	lifetime     bool
	reason       string
}

type grantDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) services.Row
	Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
}

func parseGrantFlags(args []string) (grantOptions, error) {
	var opts grantOptions
	fs := flag.NewFlagSet("grant_premium", flag.ContinueOnError)
	fs.StringVar(&opts.email, "email", "", "user email")
	fs.IntVar(&opts.durationDays, "duration_days", 0, "duration in days (0 means unset)")
	fs.BoolVar(&opts.lifetime, "lifetime", false, "grant lifetime premium")
	fs.StringVar(&opts.reason, "reason", "", "optional reason (not stored in v1)")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	return opts, nil
}

func validateGrantOptions(opts grantOptions) error {
	if strings.TrimSpace(opts.email) == "" {
		return fmt.Errorf("--email is required")
	}
	if opts.lifetime && opts.durationDays > 0 {
		return fmt.Errorf("choose either --lifetime or --duration_days, not both")
	}
	if !opts.lifetime && opts.durationDays == 0 {
		return fmt.Errorf("must specify either --lifetime or --duration_days")
	}
	if opts.durationDays < 0 {
		return fmt.Errorf("--duration_days must be >= 0")
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func buildGrantPeriodEnd(now time.Time, durationDays int, lifetime bool) *time.Time {
	if lifetime || durationDays <= 0 {
		return nil
	}
	t := now.Add(time.Duration(durationDays) * 24 * time.Hour)
	return &t
}

func grantPremium(ctx context.Context, db grantDB, email string, periodEnd *time.Time) error {
	var userID uuid.UUID
	if err := db.QueryRow(ctx,
		`SELECT id FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`,
		email,
	).Scan(&userID); err != nil {
		return fmt.Errorf("find user: %w", err)
	}
	if _, err := db.Exec(ctx,
		`UPDATE users
		 SET billing_plan = 'premium',
		     billing_source = 'grant',
		     billing_status = 'active',
		     billing_current_period_end = $2,
		     billing_cancel_at_period_end = false,
		     billing_updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID, periodEnd,
	); err != nil {
		return fmt.Errorf("grant premium: %w", err)
	}
	return nil
}
