package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
	"github.com/HammerMeetNail/yearofbingo/internal/httpx"
	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

type notificationRunnerStub struct {
	mu             sync.Mutex
	cleanupCalls   int
	setContextCall int
	asyncCtx       context.Context
	cleanupErr     error
}

func (s *notificationRunnerStub) CleanupOld(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupCalls++
	return s.cleanupErr
}

func (s *notificationRunnerStub) SetAsyncContext(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setContextCall++
	s.asyncCtx = ctx
}

func (s *notificationRunnerStub) stats() (cleanupCalls int, setContextCalls int, ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanupCalls, s.setContextCall, s.asyncCtx
}

type reminderRunnerStub struct {
	mu           sync.Mutex
	cleanupCalls int
	runDueCalls  int
	cleanupErr   error
	runDueErr    error
}

func (s *reminderRunnerStub) CleanupOld(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupCalls++
	return s.cleanupErr
}

func (s *reminderRunnerStub) RunDue(context.Context, time.Time, int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runDueCalls++
	if s.runDueErr != nil {
		return 0, s.runDueErr
	}
	return 0, nil
}

func (s *reminderRunnerStub) stats() (cleanupCalls int, runDueCalls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanupCalls, s.runDueCalls
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal(message)
}

func TestStartNotificationBackgroundJobsWithInterval_RunsInitialAndTickerCleanup(t *testing.T) {
	runner := &notificationRunnerStub{}
	cancel := startNotificationBackgroundJobsWithInterval(runner, logging.New(), 5*time.Millisecond)
	t.Cleanup(cancel)

	waitForCondition(t, 250*time.Millisecond, func() bool {
		cleanupCalls, _, _ := runner.stats()
		return cleanupCalls >= 2
	}, "expected notification cleanup to run at least twice")

	cleanupCalls, setContextCalls, asyncCtx := runner.stats()
	if cleanupCalls < 2 {
		t.Fatalf("expected cleanup calls >= 2, got %d", cleanupCalls)
	}
	if setContextCalls != 1 {
		t.Fatalf("expected SetAsyncContext called once, got %d", setContextCalls)
	}
	if asyncCtx == nil {
		t.Fatal("expected async context to be set")
	}
}

func TestStartNotificationBackgroundJobs_UsesDefaultIntervalPath(t *testing.T) {
	runner := &notificationRunnerStub{}
	cancel := startNotificationBackgroundJobs(runner, logging.New())
	cancel()
	cleanupCalls, setContextCalls, _ := runner.stats()
	if cleanupCalls < 1 {
		t.Fatalf("expected initial cleanup call, got %d", cleanupCalls)
	}
	if setContextCalls != 1 {
		t.Fatalf("expected SetAsyncContext called once, got %d", setContextCalls)
	}
}

func TestStartReminderBackgroundJobsWithCleanupInterval_RunsDueAndCleanupTickers(t *testing.T) {
	runner := &reminderRunnerStub{}
	cancel := startReminderBackgroundJobsWithCleanupInterval(
		runner,
		logging.New(),
		func(key string) (string, bool) {
			if key == "REMINDERS_POLL_INTERVAL" {
				return "5ms", true
			}
			return "", false
		},
		5*time.Millisecond,
	)
	t.Cleanup(cancel)

	waitForCondition(t, 250*time.Millisecond, func() bool {
		cleanupCalls, runDueCalls := runner.stats()
		return cleanupCalls >= 2 && runDueCalls >= 1
	}, "expected reminder cleanup ticker and runDue ticker to execute")

	cleanupCalls, runDueCalls := runner.stats()
	if cleanupCalls < 2 {
		t.Fatalf("expected cleanup calls >= 2, got %d", cleanupCalls)
	}
	if runDueCalls < 1 {
		t.Fatalf("expected runDue calls >= 1, got %d", runDueCalls)
	}
}

func TestStartReminderBackgroundJobs_UsesDefaultCleanupIntervalPath(t *testing.T) {
	runner := &reminderRunnerStub{}
	cancel := startReminderBackgroundJobs(
		runner,
		logging.New(),
		func(string) (string, bool) { return "", false },
	)
	cancel()
	cleanupCalls, _ := runner.stats()
	if cleanupCalls < 1 {
		t.Fatalf("expected initial cleanup call, got %d", cleanupCalls)
	}
}

func TestBuildRouteRateLimiters_KeysAndLimits(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	cfg := &config.Config{
		Server: config.ServerConfig{Environment: "production"},
		AI:     config.AIConfig{PremiumEndpointRateLimit: 3},
	}

	limiters := buildRouteRateLimiters(cfg, logging.New(), redisClient, func(key string) (string, bool) {
		if key == "AI_RATE_LIMIT" {
			return "1", true
		}
		if key == "AI_PREMIUM_ENDPOINT_RATE_LIMIT" {
			return "2", true
		}
		return "", false
	})

	if limiters == nil {
		t.Fatal("expected non-nil rate limiters")
	}
	if limiters.authLoginEmailLimiter == nil || limiters.authRegisterEmailLimiter == nil || limiters.aiRateLimiter == nil {
		t.Fatal("expected auth email and ai limiters to be initialized")
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	reqEmail := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	reqEmail.RemoteAddr = "198.51.100.11:1234"
	reqEmail = httpx.WithBodyBytes(reqEmail, []byte(`{"email":"User@Example.com"}`))
	resp := httptest.NewRecorder()
	limiters.authLoginEmailLimiter.Middleware(next).ServeHTTP(resp, reqEmail)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for auth email limiter, got %d", resp.Code)
	}
	if !mr.Exists("ratelimit:auth:login:email:user@example.com") {
		t.Fatal("expected auth email limiter to key by normalized email")
	}

	reqNoEmail := httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)
	reqNoEmail.RemoteAddr = "198.51.100.12:4321"
	reqNoEmail = httpx.WithBodyBytes(reqNoEmail, []byte(`{"not_email":"x"}`))
	resp = httptest.NewRecorder()
	limiters.authRegisterEmailLimiter.Middleware(next).ServeHTTP(resp, reqNoEmail)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for auth register email limiter, got %d", resp.Code)
	}
	if !mr.Exists("ratelimit:auth:register:email:no_email:198.51.100.12") {
		t.Fatal("expected auth register email limiter to fall back to client ip")
	}

	userID := uuid.New()
	reqAI := httptest.NewRequest(http.MethodPost, "/api/ai/generate", nil)
	reqAI = reqAI.WithContext(handlers.SetUserInContext(reqAI.Context(), &models.User{ID: userID}))

	resp = httptest.NewRecorder()
	limiters.aiRateLimiter.Middleware(next).ServeHTTP(resp, reqAI)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for first ai request, got %d", resp.Code)
	}

	resp = httptest.NewRecorder()
	limiters.aiRateLimiter.Middleware(next).ServeHTTP(resp, reqAI)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 when AI limit exceeded, got %d", resp.Code)
	}
	if !mr.Exists("ratelimit:ai:" + userID.String()) {
		t.Fatal("expected ai limiter to key by user id from context")
	}
}
