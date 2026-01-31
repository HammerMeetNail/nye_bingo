package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/database"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func main() {
	var count int
	var durationDays int
	var lifetime bool
	var expiresDays int

	flag.IntVar(&count, "count", 1, "number of codes to generate")
	flag.IntVar(&durationDays, "duration_days", 0, "duration in days (0 means unset)")
	flag.BoolVar(&lifetime, "lifetime", false, "generate lifetime codes")
	flag.IntVar(&expiresDays, "expires_days", 0, "codes expire this many days from now (0 means no expiry)")
	flag.Parse()

	if count <= 0 || count > 10000 {
		log.Fatalf("invalid --count: %d", count)
	}
	if lifetime && durationDays > 0 {
		log.Fatal("choose either --lifetime or --duration_days, not both")
	}
	if durationDays < 0 {
		log.Fatal("--duration_days must be >= 0")
	}
	if expiresDays < 0 {
		log.Fatal("--expires_days must be >= 0")
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

	var expiresAt *time.Time
	if expiresDays > 0 {
		t := time.Now().UTC().Add(time.Duration(expiresDays) * 24 * time.Hour)
		expiresAt = &t
	}

	var durationPtr *int
	if !lifetime && durationDays > 0 {
		durationPtr = &durationDays
	}

	enc := base32.StdEncoding.WithPadding(base32.NoPadding)

	for i := 0; i < count; i++ {
		raw := make([]byte, 15)
		if _, err := rand.Read(raw); err != nil {
			log.Fatalf("rand: %v", err)
		}
		suffix := enc.EncodeToString(raw) // 24 chars
		normalized := "YOBP" + suffix
		display := "YOBP-" + group4(suffix)

		sum := sha256.Sum256([]byte(normalized))
		hashHex := hex.EncodeToString(sum[:])

		if _, err := dbAdapter.Exec(ctx,
			`INSERT INTO premium_codes (code_hash, duration_days, expires_at)
			 VALUES ($1, $2, $3)`,
			hashHex, durationPtr, expiresAt,
		); err != nil {
			log.Fatalf("insert code: %v", err)
		}

		fmt.Println(display)
	}
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
