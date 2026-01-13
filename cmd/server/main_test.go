package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/logging"
)

func TestResolveAIRateLimit_Defaults(t *testing.T) {
	logger := logging.New().SetOutput(&bytes.Buffer{})
	cfg := &config.Config{Server: config.ServerConfig{Environment: "production"}}

	limit := resolveAIRateLimit(cfg, logger, func(key string) (string, bool) {
		return "", false
	})
	if limit != 10 {
		t.Fatalf("expected default limit 10, got %d", limit)
	}
}

func TestResolveAIRateLimit_DevelopmentDefault(t *testing.T) {
	logger := logging.New().SetOutput(&bytes.Buffer{})
	cfg := &config.Config{Server: config.ServerConfig{Environment: "development"}}

	limit := resolveAIRateLimit(cfg, logger, func(key string) (string, bool) {
		return "", false
	})
	if limit != 100 {
		t.Fatalf("expected dev limit 100, got %d", limit)
	}
}

func TestResolveAIRateLimit_FromEnv(t *testing.T) {
	logger := logging.New().SetOutput(&bytes.Buffer{})
	cfg := &config.Config{Server: config.ServerConfig{Environment: "production"}}

	limit := resolveAIRateLimit(cfg, logger, func(key string) (string, bool) {
		return "25", true
	})
	if limit != 25 {
		t.Fatalf("expected env limit 25, got %d", limit)
	}
}

func TestResolveAIRateLimit_InvalidEnv(t *testing.T) {
	logger := logging.New().SetOutput(&bytes.Buffer{})
	cfg := &config.Config{Server: config.ServerConfig{Environment: "production"}}

	limit := resolveAIRateLimit(cfg, logger, func(key string) (string, bool) {
		return "nope", true
	})
	if limit != 10 {
		t.Fatalf("expected fallback limit 10, got %d", limit)
	}
}

func TestResolveRemindersPollInterval_Defaults(t *testing.T) {
	logger := logging.New().SetOutput(&bytes.Buffer{})
	interval := resolveRemindersPollInterval(logger, func(key string) (string, bool) {
		return "", false
	})
	if interval != time.Minute {
		t.Fatalf("expected default interval 1m, got %v", interval)
	}
}

func TestResolveRemindersPollInterval_FromEnv(t *testing.T) {
	logger := logging.New().SetOutput(&bytes.Buffer{})
	interval := resolveRemindersPollInterval(logger, func(key string) (string, bool) {
		return "30s", true
	})
	if interval != 30*time.Second {
		t.Fatalf("expected interval 30s, got %v", interval)
	}
}

func TestResolveRemindersPollInterval_InvalidEnv(t *testing.T) {
	logger := logging.New().SetOutput(&bytes.Buffer{})
	interval := resolveRemindersPollInterval(logger, func(key string) (string, bool) {
		return "nope", true
	})
	if interval != time.Minute {
		t.Fatalf("expected fallback interval 1m, got %v", interval)
	}
}
