package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/database"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func main() {
	opts, err := parseRevokeFlags(os.Args[1:])
	if err != nil {
		log.Fatalf("parse flags: %v", err)
	}
	if err := validateRevokeOptions(opts); err != nil {
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
	if err := revokePremium(context.Background(), dbAdapter, normalizeRevokeEmail(opts.email)); err != nil {
		log.Fatal(err)
	}
}

type revokeOptions struct {
	email  string
	reason string
}

type revokeDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) services.Row
	Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
}

func parseRevokeFlags(args []string) (revokeOptions, error) {
	var opts revokeOptions
	fs := flag.NewFlagSet("revoke_premium", flag.ContinueOnError)
	fs.StringVar(&opts.email, "email", "", "user email")
	fs.StringVar(&opts.reason, "reason", "", "optional reason (not stored in v1)")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	return opts, nil
}

func validateRevokeOptions(opts revokeOptions) error {
	if strings.TrimSpace(opts.email) == "" {
		return fmt.Errorf("--email is required")
	}
	return nil
}

func normalizeRevokeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func revokePremium(ctx context.Context, db revokeDB, email string) error {
	var userID uuid.UUID
	if err := db.QueryRow(ctx,
		`SELECT id FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`,
		email,
	).Scan(&userID); err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	if _, err := db.Exec(ctx,
		`UPDATE users
		 SET billing_plan = 'free',
		     billing_source = 'none',
		     billing_status = 'inactive',
		     billing_current_period_end = NULL,
		     billing_cancel_at_period_end = false,
		     billing_updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	); err != nil {
		return fmt.Errorf("revoke premium: %w", err)
	}
	return nil
}
