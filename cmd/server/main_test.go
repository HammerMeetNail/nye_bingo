package main

import (
	"testing"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/logging"
)

func TestResolveAuthRateLimits_Development(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Environment: "development"},
	}
	limits := resolveAuthRateLimits(cfg)
	if limits.loginIP != 1000 || limits.loginEmail != 500 {
		t.Fatalf("expected dev limits, got %+v", limits)
	}
}

func TestResolveAuthRateLimits_Production(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Environment: "production"},
	}
	limits := resolveAuthRateLimits(cfg)
	if limits.loginIP != 30 || limits.registerEmail != 5 {
		t.Fatalf("expected prod limits, got %+v", limits)
	}
}

func TestResolveAIRateLimit_EnvOverride(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Environment: "production"},
	}
	logger := logging.New()
	limit := resolveAIRateLimit(cfg, logger, func(key string) (string, bool) {
		if key == "AI_RATE_LIMIT" {
			return "25", true
		}
		return "", false
	})
	if limit != 25 {
		t.Fatalf("expected override limit 25, got %d", limit)
	}
}

func TestResolveAIRateLimit_InvalidEnv(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Environment: "production"},
	}
	logger := logging.New()
	limit := resolveAIRateLimit(cfg, logger, func(key string) (string, bool) {
		if key == "AI_RATE_LIMIT" {
			return "bad", true
		}
		return "", false
	})
	if limit != 10 {
		t.Fatalf("expected default limit 10, got %d", limit)
	}
}

func TestResolveAIRateLimit_DevelopmentDefault(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Environment: "development"},
	}
	logger := logging.New()
	limit := resolveAIRateLimit(cfg, logger, func(key string) (string, bool) {
		return "", false
	})
	if limit != 100 {
		t.Fatalf("expected dev limit 100, got %d", limit)
	}
}

func TestResolveRemindersPollInterval_EnvOverride(t *testing.T) {
	logger := logging.New()
	interval := resolveRemindersPollInterval(logger, func(key string) (string, bool) {
		if key == "REMINDERS_POLL_INTERVAL" {
			return "2m", true
		}
		return "", false
	})
	if interval != 2*time.Minute {
		t.Fatalf("expected 2m, got %s", interval)
	}
}

func TestResolveRemindersPollInterval_Invalid(t *testing.T) {
	logger := logging.New()
	interval := resolveRemindersPollInterval(logger, func(key string) (string, bool) {
		if key == "REMINDERS_POLL_INTERVAL" {
			return "bad", true
		}
		return "", false
	})
	if interval != time.Minute {
		t.Fatalf("expected 1m, got %s", interval)
	}
}
