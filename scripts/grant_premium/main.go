package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/database"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func main() {
	var email string
	var durationDays int
	var lifetime bool
	var reason string

	flag.StringVar(&email, "email", "", "user email")
	flag.IntVar(&durationDays, "duration_days", 0, "duration in days (0 means unset)")
	flag.BoolVar(&lifetime, "lifetime", false, "grant lifetime premium")
	flag.StringVar(&reason, "reason", "", "optional reason (not stored in v1)")
	flag.Parse()

	_ = reason

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		log.Fatal("--email is required")
	}
	if lifetime && durationDays > 0 {
		log.Fatal("choose either --lifetime or --duration_days, not both")
	}
	if !lifetime && durationDays == 0 {
		log.Fatal("must specify either --lifetime or --duration_days")
	}
	if durationDays < 0 {
		log.Fatal("--duration_days must be >= 0")
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
	ctx := context.Background()

	var userID uuid.UUID
	if err := dbAdapter.QueryRow(ctx,
		`SELECT id FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`,
		email,
	).Scan(&userID); err != nil {
		log.Fatalf("find user: %v", err)
	}

	var periodEnd *time.Time
	if !lifetime && durationDays > 0 {
		t := time.Now().UTC().Add(time.Duration(durationDays) * 24 * time.Hour)
		periodEnd = &t
	}

	if _, err := dbAdapter.Exec(ctx,
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
		log.Fatalf("grant premium: %v", err)
	}
}
