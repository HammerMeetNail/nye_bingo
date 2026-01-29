package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type mockOAuthProvider struct {
	provider services.Provider
	authURL  string
	state    string
	nonce    string
	claims   services.IdentityClaims
	err      error
}

func (m *mockOAuthProvider) Provider() services.Provider {
	return m.provider
}

func (m *mockOAuthProvider) AuthCodeURL(state, nonce string) string {
	m.state = state
	m.nonce = nonce
	return m.authURL
}

func (m *mockOAuthProvider) ExchangeAndVerify(ctx context.Context, code, nonce string) (services.IdentityClaims, error) {
	if m.err != nil {
		return services.IdentityClaims{}, m.err
	}
	return m.claims, nil
}

type mockProviderAuthService struct {
	LinkFunc   func(ctx context.Context, claims services.IdentityClaims) (*services.ProviderLinkResult, error)
	CreateFunc func(ctx context.Context, pending services.PendingProviderUser, username string, searchable bool) (*models.User, error)
}

func (m *mockProviderAuthService) LinkOrFindUserFromProvider(ctx context.Context, claims services.IdentityClaims) (*services.ProviderLinkResult, error) {
	if m.LinkFunc != nil {
		return m.LinkFunc(ctx, claims)
	}
	return nil, nil
}

func (m *mockProviderAuthService) CreateUserFromProviderPending(ctx context.Context, pending services.PendingProviderUser, username string, searchable bool) (*models.User, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, pending, username, searchable)
	}
	return nil, nil
}

type fakeRedisClient struct {
	values   map[string]string
	setCalls int
	delCalls int
	setErr   error
	getErr   error
}

func (f *fakeRedisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[key] = value.(string)
	f.setCalls++
	return nil
}

func (f *fakeRedisClient) Get(ctx context.Context, key string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.values[key], nil
}

func (f *fakeRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return nil
}

func (f *fakeRedisClient) Del(ctx context.Context, keys ...string) error {
	f.delCalls += len(keys)
	for _, key := range keys {
		delete(f.values, key)
	}
	return nil
}

func TestProviderAuthHandler_Start_SetsCookies(t *testing.T) {
	mockProvider := &mockOAuthProvider{
		provider: services.ProviderGoogle,
		authURL:  "https://example.com/auth",
	}
	handler := NewProviderAuthHandler(nil, &mockAuthService{}, nil, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: mockProvider,
	}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/start", nil)
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderStart(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rr.Code)
	}
	if location := rr.Result().Header.Get("Location"); location != mockProvider.authURL {
		t.Fatalf("expected redirect to %q, got %q", mockProvider.authURL, location)
	}

	cookies := rr.Result().Cookies()
	var stateCookie, nonceCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == oauthStateCookieName {
			stateCookie = c
		}
		if c.Name == oauthNonceCookieName {
			nonceCookie = c
		}
	}
	if stateCookie == nil || nonceCookie == nil {
		t.Fatalf("expected state and nonce cookies to be set")
	}
	if stateCookie.Value != mockProvider.state {
		t.Fatalf("expected state cookie %q, got %q", mockProvider.state, stateCookie.Value)
	}
	if nonceCookie.Value != mockProvider.nonce {
		t.Fatalf("expected nonce cookie %q, got %q", mockProvider.nonce, nonceCookie.Value)
	}
}

func TestProviderAuthHandler_Callback_ErrorParam(t *testing.T) {
	mockProvider := &mockOAuthProvider{provider: services.ProviderGoogle}
	handler := NewProviderAuthHandler(&mockProviderAuthService{}, &mockAuthService{}, &fakeRedisClient{}, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: mockProvider,
	}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?error=access_denied", nil)
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rr.Code)
	}
	location := rr.Result().Header.Get("Location")
	if !strings.Contains(location, "/login?error=access_denied") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestProviderAuthHandler_Callback_InvalidState(t *testing.T) {
	mockProvider := &mockOAuthProvider{provider: services.ProviderGoogle}
	handler := NewProviderAuthHandler(&mockProviderAuthService{}, &mockAuthService{}, &fakeRedisClient{}, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: mockProvider,
	}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=abc&state=wrong", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "right"})
	req.AddCookie(&http.Cookie{Name: oauthNonceCookieName, Value: "nonce"})
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rr.Code)
	}
	location := rr.Result().Header.Get("Location")
	if !strings.Contains(location, "/login?error=oauth_invalid") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestProviderAuthHandler_Callback_ExistingUser(t *testing.T) {
	user := &models.User{ID: uuid.New(), Email: "user@example.com"}
	mockProvider := &mockOAuthProvider{
		provider: services.ProviderGoogle,
		claims: services.IdentityClaims{
			Provider:      services.ProviderGoogle,
			Subject:       "sub",
			Email:         "user@example.com",
			EmailVerified: true,
		},
	}
	mockAuth := &mockAuthService{
		CreateSessionFunc: func(ctx context.Context, userID uuid.UUID) (string, error) {
			return "session-token", nil
		},
	}
	mockProviderAuth := &mockProviderAuthService{
		LinkFunc: func(ctx context.Context, claims services.IdentityClaims) (*services.ProviderLinkResult, error) {
			return &services.ProviderLinkResult{User: user}, nil
		},
	}
	handler := NewProviderAuthHandler(mockProviderAuth, mockAuth, &fakeRedisClient{}, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: mockProvider,
	}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=abc&state=state123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state123"})
	req.AddCookie(&http.Cookie{Name: oauthNonceCookieName, Value: "nonce123"})
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rr.Code)
	}
	location := rr.Result().Header.Get("Location")
	if !strings.Contains(location, "/dashboard") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestSanitizeNext(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected string
	}{
		// Valid paths
		{name: "Allow rooted path", in: "/dashboard", expected: "/dashboard"},
		{name: "Allow rooted path with query", in: "/friend-invite/abc?x=y", expected: "/friend-invite/abc?x=y"},
		{name: "Allow legacy hash at start", in: "#share/xyz", expected: "/share/xyz"},
		{name: "Allow legacy hash in path", in: "/#share/xyz", expected: "/#share/xyz"},
		{name: "Allow root path", in: "/", expected: "/"},
		{name: "Allow nested path", in: "/card/abc-123/edit", expected: "/card/abc-123/edit"},
		{name: "Allow path with fragment", in: "/page#section", expected: "/page#section"},
		{name: "Allow path with query and fragment", in: "/page?foo=bar#section", expected: "/page?foo=bar#section"},
		{name: "Trim whitespace", in: "  /dashboard  ", expected: "/dashboard"},

		// Scheme-relative / double-slash attacks
		{name: "Reject scheme-relative", in: "//evil.example", expected: ""},
		{name: "Reject backslash variant", in: "/\\evil.example", expected: ""},
		{name: "Reject encoded scheme-relative (slashes)", in: "/%2f%2fevil.example", expected: ""},
		{name: "Reject encoded backslash scheme-relative", in: "/%5c%5cevil.example", expected: ""},
		{name: "Reject mixed encoded slash backslash", in: "/%2f%5cevil.example", expected: ""},
		{name: "Reject uppercase encoded slashes", in: "/%2F%2Fevil.example", expected: ""},
		{name: "Reject uppercase encoded backslashes", in: "/%5C%5Cevil.example", expected: ""},
		{name: "Reject triple slash", in: "///evil.example", expected: ""},

		// Absolute URLs
		{name: "Reject absolute URL", in: "https://evil.example/", expected: ""},
		{name: "Reject http URL", in: "http://evil.example/", expected: ""},
		{name: "Reject javascript scheme", in: "javascript:alert(1)", expected: ""},
		{name: "Reject data scheme", in: "data:text/html,<script>alert(1)</script>", expected: ""},
		{name: "Reject vbscript scheme", in: "vbscript:msgbox(1)", expected: ""},
		{name: "Reject file scheme", in: "file:///etc/passwd", expected: ""},

		// URLs with credentials
		{name: "Reject URL with user info", in: "//user@evil.example", expected: ""},
		{name: "Reject URL with user:pass", in: "//user:pass@evil.example", expected: ""},

		// Path without leading slash
		{name: "Reject relative path", in: "dashboard", expected: ""},
		{name: "Reject relative path with dots", in: "../admin", expected: ""},

		// Backslash attacks
		{name: "Reject backslash in path", in: "/foo\\bar", expected: ""},
		{name: "Reject encoded backslash in path", in: "/foo%5cbar", expected: ""},

		// Newline / carriage return injection
		{name: "Reject newline", in: "/foo\nbar", expected: ""},
		{name: "Reject carriage return", in: "/foo\rbar", expected: ""},
		{name: "Reject CRLF", in: "/foo\r\nbar", expected: ""},

		// Length limits
		{name: "Reject too long value", in: "/" + string(make([]byte, 600)), expected: ""},

		// Empty / whitespace
		{name: "Reject empty string", in: "", expected: ""},
		{name: "Reject whitespace only", in: "   ", expected: ""},

		// Path traversal
		{name: "Normalize path traversal", in: "/foo/../bar", expected: "/bar"},
		{name: "Normalize double dots", in: "/foo/bar/../baz", expected: "/foo/baz"},
		{name: "Normalize current dir", in: "/foo/./bar", expected: "/foo/bar"},

		// Opaque URLs
		{name: "Reject mailto", in: "mailto:foo@bar.com", expected: ""},
		{name: "Reject tel", in: "tel:+1234567890", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeNext(tt.in); got != tt.expected {
				t.Fatalf("sanitizeNext(%q)=%q, want %q", tt.in, got, tt.expected)
			}
		})
	}
}

func TestProviderAuthHandler_Callback_NewUser(t *testing.T) {
	mockProvider := &mockOAuthProvider{
		provider: services.ProviderGoogle,
		claims: services.IdentityClaims{
			Provider:      services.ProviderGoogle,
			Subject:       "sub",
			Email:         "user@example.com",
			EmailVerified: true,
		},
	}
	mockProviderAuth := &mockProviderAuthService{
		LinkFunc: func(ctx context.Context, claims services.IdentityClaims) (*services.ProviderLinkResult, error) {
			return &services.ProviderLinkResult{
				Pending: &services.PendingProviderUser{
					Provider: services.ProviderGoogle,
					Subject:  "sub",
					Email:    "user@example.com",
				},
			}, nil
		},
	}
	redis := &fakeRedisClient{values: map[string]string{}}
	handler := NewProviderAuthHandler(mockProviderAuth, &mockAuthService{}, redis, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: mockProvider,
	}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=abc&state=state123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state123"})
	req.AddCookie(&http.Cookie{Name: oauthNonceCookieName, Value: "nonce123"})
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rr.Code)
	}
	location := rr.Result().Header.Get("Location")
	if !strings.Contains(location, "/google-complete") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
	if redis.setCalls != 1 {
		t.Fatalf("expected pending record to be stored")
	}
}

func TestProviderAuthHandler_Callback_UnverifiedEmail(t *testing.T) {
	mockProvider := &mockOAuthProvider{
		provider: services.ProviderGoogle,
		claims: services.IdentityClaims{
			Provider:      services.ProviderGoogle,
			Subject:       "sub",
			Email:         "user@example.com",
			EmailVerified: false,
		},
	}
	mockProviderAuth := &mockProviderAuthService{
		LinkFunc: func(ctx context.Context, claims services.IdentityClaims) (*services.ProviderLinkResult, error) {
			return nil, services.ErrProviderEmailUnverified
		},
	}
	handler := NewProviderAuthHandler(mockProviderAuth, &mockAuthService{}, &fakeRedisClient{}, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: mockProvider,
	}, false)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=abc&state=state123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state123"})
	req.AddCookie(&http.Cookie{Name: oauthNonceCookieName, Value: "nonce123"})
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rr.Code)
	}
	location := rr.Result().Header.Get("Location")
	if !strings.Contains(location, "/login?error=oauth_unverified") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestProviderAuthHandler_Complete_MissingPending(t *testing.T) {
	mockProvider := &mockOAuthProvider{
		provider: services.ProviderGoogle,
		authURL:  "https://example.com/auth",
	}
	handler := NewProviderAuthHandler(nil, &mockAuthService{}, nil, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: mockProvider,
	}, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/google/complete", nil)
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderComplete(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "Signup session expired. Please restart OAuth login.")
}

func TestProviderAuthHandler_Complete_AlreadyAuthenticated(t *testing.T) {
	mockProvider := &mockOAuthProvider{provider: services.ProviderGoogle}
	handler := NewProviderAuthHandler(nil, &mockAuthService{}, &fakeRedisClient{}, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: mockProvider,
	}, false)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/google/complete", nil)
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New()}))
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderComplete(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "Already authenticated")
}

func TestProviderAuthHandler_Complete_Success(t *testing.T) {
	user := &models.User{ID: uuid.New(), Email: "user@example.com", Username: "tester"}
	pending := providerPendingRecord{
		Provider: "google",
		Subject:  "sub",
		Email:    "user@example.com",
	}
	pendingBytes, _ := json.Marshal(pending)
	redis := &fakeRedisClient{values: map[string]string{
		providerPendingRedisKey("token123"): string(pendingBytes),
	}}
	mockProviderAuth := &mockProviderAuthService{
		CreateFunc: func(ctx context.Context, pending services.PendingProviderUser, username string, searchable bool) (*models.User, error) {
			if pending.Subject != "sub" {
				return nil, errors.New("unexpected subject")
			}
			return user, nil
		},
	}
	mockAuth := &mockAuthService{
		CreateSessionFunc: func(ctx context.Context, userID uuid.UUID) (string, error) {
			return "session-token", nil
		},
	}
	handler := NewProviderAuthHandler(mockProviderAuth, mockAuth, redis, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: &mockOAuthProvider{provider: services.ProviderGoogle},
	}, false)

	body := bytes.NewBufferString(`{"username":"tester","searchable":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/google/complete", body)
	req.AddCookie(&http.Cookie{Name: providerPendingCookieName("google"), Value: "token123"})
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderComplete(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if redis.delCalls == 0 {
		t.Fatalf("expected pending record to be cleared")
	}
}

func TestProviderAuthHandler_Complete_UsernameConflict(t *testing.T) {
	pending := providerPendingRecord{
		Provider: "google",
		Subject:  "sub",
		Email:    "user@example.com",
	}
	pendingBytes, _ := json.Marshal(pending)
	redis := &fakeRedisClient{values: map[string]string{
		providerPendingRedisKey("token123"): string(pendingBytes),
	}}
	mockProviderAuth := &mockProviderAuthService{
		CreateFunc: func(ctx context.Context, pending services.PendingProviderUser, username string, searchable bool) (*models.User, error) {
			return nil, services.ErrUsernameAlreadyExists
		},
	}
	handler := NewProviderAuthHandler(mockProviderAuth, &mockAuthService{}, redis, map[services.Provider]services.OAuthProvider{
		services.ProviderGoogle: &mockOAuthProvider{provider: services.ProviderGoogle},
	}, false)

	body := bytes.NewBufferString(`{"username":"taken","searchable":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/google/complete", body)
	req.AddCookie(&http.Cookie{Name: providerPendingCookieName("google"), Value: "token123"})
	req.SetPathValue("provider", "google")
	rr := httptest.NewRecorder()

	handler.ProviderComplete(rr, req)

	assertErrorResponse(t, rr, http.StatusConflict, "Username already taken")
}

func TestSanitizeNext_RejectsUnsafePrefix(t *testing.T) {
	for _, input := range []string{"//evil.com", "/\\evil.com"} {
		if got := sanitizeNext(input); got != "" {
			t.Fatalf("expected %q to be rejected, got %q", input, got)
		}
	}
}

func TestSanitizeNext_AllowsLegacyHash(t *testing.T) {
	if got := sanitizeNext("#dashboard"); got != "/dashboard" {
		t.Fatalf("expected legacy hash to normalize to /dashboard, got %q", got)
	}
}

func TestRedirectTarget_FallsBackOnUnsafeNext(t *testing.T) {
	handler := &ProviderAuthHandler{}
	target := handler.redirectTarget("https://evil.com", "dashboard")
	if target != "/dashboard" {
		t.Fatalf("expected fallback path, got %q", target)
	}
}

func TestRedirectTarget_UsesSafeNext(t *testing.T) {
	handler := &ProviderAuthHandler{}
	target := handler.redirectTarget("/card/abc-123", "/dashboard")
	if target != "/card/abc-123" {
		t.Fatalf("expected safe next path, got %q", target)
	}
}

func TestRedirectTarget_SanitizesFallback(t *testing.T) {
	handler := &ProviderAuthHandler{}
	target := handler.redirectTarget("", "//evil.com")
	if target != "/" {
		t.Fatalf("expected unsafe fallback to become /, got %q", target)
	}
}

func TestRedirectTarget_NormalizesFallback(t *testing.T) {
	handler := &ProviderAuthHandler{}
	target := handler.redirectTarget("", "dashboard")
	if target != "/dashboard" {
		t.Fatalf("expected fallback to normalize, got %q", target)
	}
}

func TestSanitizeProviderErrorParam(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected string
	}{
		// Valid OAuth error codes (should pass through)
		{name: "access_denied", in: "access_denied", expected: "access_denied"},
		{name: "invalid_request", in: "invalid_request", expected: "invalid_request"},
		{name: "invalid_scope", in: "invalid_scope", expected: "invalid_scope"},
		{name: "server_error", in: "server_error", expected: "server_error"},
		{name: "temporarily_unavailable", in: "temporarily_unavailable", expected: "temporarily_unavailable"},
		{name: "unauthorized_client", in: "unauthorized_client", expected: "unauthorized_client"},
		{name: "unsupported_response_type", in: "unsupported_response_type", expected: "unsupported_response_type"},

		// Case insensitivity
		{name: "uppercase ACCESS_DENIED", in: "ACCESS_DENIED", expected: "access_denied"},
		{name: "mixed case Access_Denied", in: "Access_Denied", expected: "access_denied"},

		// Whitespace handling
		{name: "with leading space", in: "  access_denied", expected: "access_denied"},
		{name: "with trailing space", in: "access_denied  ", expected: "access_denied"},
		{name: "with both spaces", in: "  access_denied  ", expected: "access_denied"},

		// Invalid/unknown errors (should return oauth_error)
		{name: "empty string", in: "", expected: "oauth_error"},
		{name: "whitespace only", in: "   ", expected: "oauth_error"},
		{name: "unknown error", in: "some_unknown_error", expected: "oauth_error"},
		{name: "injection attempt", in: "<script>alert(1)</script>", expected: "oauth_error"},
		{name: "sql injection", in: "'; DROP TABLE users; --", expected: "oauth_error"},
		{name: "newline injection", in: "access_denied\nevil", expected: "oauth_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeProviderErrorParam(tt.in); got != tt.expected {
				t.Fatalf("sanitizeProviderErrorParam(%q)=%q, want %q", tt.in, got, tt.expected)
			}
		})
	}
}

func TestSanitizeErrorParam(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected string
	}{
		// Valid error strings
		{name: "simple error", in: "invalid_token", expected: "invalid_token"},
		{name: "with hyphen", in: "auth-failed", expected: "auth-failed"},
		{name: "with numbers", in: "error123", expected: "error123"},
		{name: "mixed case", in: "InvalidToken", expected: "InvalidToken"},
		{name: "underscore and hyphen", in: "auth_error-code", expected: "auth_error-code"},

		// Empty/whitespace
		{name: "empty string", in: "", expected: "oauth_error"},
		{name: "whitespace only", in: "   ", expected: "oauth_error"},

		// Whitespace trimming
		{name: "leading space", in: "  error", expected: "error"},
		{name: "trailing space", in: "error  ", expected: "error"},

		// Length limits
		{name: "exactly 60 chars", in: "abcdefghij1234567890abcdefghij1234567890abcdefghij1234567890", expected: "abcdefghij1234567890abcdefghij1234567890abcdefghij1234567890"},
		{name: "over 60 chars truncated", in: "abcdefghij1234567890abcdefghij1234567890abcdefghij1234567890extra", expected: "abcdefghij1234567890abcdefghij1234567890abcdefghij1234567890"},

		// Invalid characters (should return oauth_error)
		{name: "with space in middle", in: "invalid token", expected: "oauth_error"},
		{name: "with special chars", in: "error<script>", expected: "oauth_error"},
		{name: "with colon", in: "error:code", expected: "oauth_error"},
		{name: "with slash", in: "error/code", expected: "oauth_error"},
		{name: "with dot", in: "error.code", expected: "oauth_error"},
		{name: "with newline", in: "error\ncode", expected: "oauth_error"},
		{name: "with unicode", in: "error\u200b", expected: "oauth_error"},
		{name: "with emoji", in: "error😀", expected: "oauth_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeErrorParam(tt.in); got != tt.expected {
				t.Fatalf("sanitizeErrorParam(%q)=%q, want %q", tt.in, got, tt.expected)
			}
		})
	}
}
