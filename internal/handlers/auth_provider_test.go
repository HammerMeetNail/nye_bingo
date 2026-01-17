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
