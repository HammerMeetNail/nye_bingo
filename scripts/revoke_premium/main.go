package main

import (
	"context"
	"flag"
	"log"
	"strings"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/database"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func main() {
	var email string
	var reason string

	flag.StringVar(&email, "email", "", "user email")
	flag.StringVar(&reason, "reason", "", "optional reason (not stored in v1)")
	flag.Parse()

	_ = reason

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		log.Fatal("--email is required")
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

	if _, err := dbAdapter.Exec(ctx,
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
		log.Fatalf("revoke premium: %v", err)
	}
}
