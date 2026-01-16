package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type mockOAuthProvider struct {
	provider services.Provider
	authURL  string
	state    string
	nonce    string
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
	return services.IdentityClaims{}, nil
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

	assertErrorResponse(t, rr, http.StatusBadRequest, "Signup session expired. Please restart Google login.")
}
