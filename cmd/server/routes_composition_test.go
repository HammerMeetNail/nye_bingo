package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
	"github.com/HammerMeetNail/yearofbingo/internal/middleware"
)

func TestRegisterAPIRoutes_KeyRoutesAndMethodBindings(t *testing.T) {
	mux := http.NewServeMux()
	identity := func(h http.Handler) http.Handler { return h }

	registerAPIRoutes(mux, &apiRouteHandlers{
		healthHandler:       (*handlers.HealthHandler)(nil),
		csrfMiddleware:      middleware.NewCSRFMiddleware(false),
		authHandler:         (*handlers.AuthHandler)(nil),
		providerAuthHandler: (*handlers.ProviderAuthHandler)(nil),
		accountHandler:      (*handlers.AccountHandler)(nil),
		apiTokenHandler:     (*handlers.ApiTokenHandler)(nil),
		cardHandler:         (*handlers.CardHandler)(nil),
		templatesHandler:    (*handlers.TemplateHandler)(nil),
		suggestionHandler:   (*handlers.SuggestionHandler)(nil),
		friendHandler:       (*handlers.FriendHandler)(nil),
		blockHandler:        (*handlers.BlockHandler)(nil),
		inviteHandler:       (*handlers.FriendInviteHandler)(nil),
		notificationHandler: (*handlers.NotificationHandler)(nil),
		reminderHandler:     (*handlers.ReminderHandler)(nil),
		reactionHandler:     (*handlers.ReactionHandler)(nil),
		supportHandler:      (*handlers.SupportHandler)(nil),
		aiHandler:           (*handlers.AIHandler)(nil),
		billingHandler:      (*handlers.BillingHandler)(nil),
	}, &apiRouteMiddleware{
		requireRead:                identity,
		requireWrite:               identity,
		requireSession:             identity,
		authLoginIPLimiter:         testRateLimiter(),
		authLoginEmailLimiter:      testRateLimiter(),
		authRegisterIPLimiter:      testRateLimiter(),
		authRegisterEmailLimiter:   testRateLimiter(),
		authEmailFlowIPLimiter:     testRateLimiter(),
		authEmailFlowEmailLimiter:  testRateLimiter(),
		authResetPasswordIPLimiter: testRateLimiter(),
		aiRateLimiter:              testRateLimiter(),
		aiPremiumRateLimiter:       testRateLimiter(),
		redeemLimiter:              testRateLimiter(),
	})

	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{method: http.MethodGet, path: "/health", want: true},
		{method: http.MethodGet, path: "/api/csrf", want: true},
		{method: http.MethodPost, path: "/api/auth/login", want: true},
		{method: http.MethodGet, path: "/api/auth/login", want: false},
		{method: http.MethodGet, path: "/api/account/export", want: true},
		{method: http.MethodGet, path: "/api/cards", want: true},
		{method: http.MethodDelete, path: "/api/cards/123", want: true},
		{method: http.MethodGet, path: "/api/share/token-1", want: true},
		{method: http.MethodGet, path: "/api/templates", want: true},
		{method: http.MethodGet, path: "/api/friends", want: true},
		{method: http.MethodGet, path: "/api/reminders/settings", want: true},
		{method: http.MethodGet, path: "/api/reactions/emojis", want: true},
		{method: http.MethodPost, path: "/api/support", want: true},
		{method: http.MethodPost, path: "/api/ai/generate", want: true},
		{method: http.MethodGet, path: "/api/billing/status", want: true},
	}

	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		_, pattern := mux.Handler(req)
		got := pattern != ""
		if got != c.want {
			t.Fatalf("route lookup mismatch for %s %s: got=%t pattern=%q want=%t", c.method, c.path, got, pattern, c.want)
		}
	}
}

func TestRegisterWebRoutes_KeyRoutesAndMethodBindings(t *testing.T) {
	mux := http.NewServeMux()
	identity := func(h http.Handler) http.Handler { return h }

	registerWebRoutes(mux, &webRouteHandlers{
		pageHandler:           (*handlers.PageHandler)(nil),
		reminderPublicHandler: (*handlers.ReminderPublicHandler)(nil),
		ogImageHandler:        (*handlers.OGImageHandler)(nil),
		shareOGImageHandler:   (*handlers.ShareOGImageHandler)(nil),
		sharePublicHandler:    (*handlers.SharePublicHandler)(nil),
	}, identity)

	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{method: http.MethodGet, path: "/static/app.js", want: true},
		{method: http.MethodGet, path: "/r/img/token-1", want: true},
		{method: http.MethodPost, path: "/r/unsubscribe", want: true},
		{method: http.MethodGet, path: "/og/default.png", want: true},
		{method: http.MethodGet, path: "/s/token-1", want: true},
		{method: http.MethodGet, path: "/api/docs", want: true},
		{method: http.MethodGet, path: "/dashboard", want: true},
		{method: http.MethodPost, path: "/api/docs", want: false},
	}

	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		_, pattern := mux.Handler(req)
		got := pattern != ""
		if got != c.want {
			t.Fatalf("route lookup mismatch for %s %s: got=%t pattern=%q want=%t", c.method, c.path, got, pattern, c.want)
		}
	}
}

func testRateLimiter() *middleware.RateLimiter {
	return middleware.NewRateLimiter(nil, 10, time.Minute, "test:", func(_ *http.Request) string {
		return ""
	}, false)
}
