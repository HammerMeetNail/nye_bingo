package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfTokenLen   = 32
	csrfMaxAge     = 12 * 60 * 60 // 12 hours
)

type CSRFMiddleware struct {
	secure bool
	// secret signs CSRF tokens (signed double-submit). An attacker who can plant a
	// cookie on the victim still cannot forge a token that passes validation
	// because they cannot produce a valid signature.
	secret []byte
}

// NewCSRFMiddleware creates the middleware with a random per-process signing
// secret. Tokens are reissued automatically by the client after a restart, so a
// process-scoped secret is sufficient.
func NewCSRFMiddleware(secure bool) *CSRFMiddleware {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// crypto/rand failure is fatal-grade; panic during startup is acceptable.
		panic("csrf: failed to generate signing secret: " + err.Error())
	}
	return &CSRFMiddleware{secure: secure, secret: secret}
}

var csrfExemptPostPaths = map[string]bool{
	"/api/billing/webhook": true,
}

func (m *CSRFMiddleware) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public tokenized endpoints (no session) should not require CSRF headers/cookies.
		if r.URL.Path == "/r/unsubscribe" {
			next.ServeHTTP(w, r)
			return
		}

		// Stripe webhooks cannot send CSRF tokens.
		if r.Method == http.MethodPost && csrfExemptPostPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Safe methods don't need CSRF protection
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			m.ensureToken(w, r)
			next.ServeHTTP(w, r)
			return
		}

		// Validate CSRF token for state-changing methods
		cookie, err := r.Cookie(csrfCookieName)
		if err != nil {
			m.reject(w, "CSRF token missing")
			return
		}

		headerToken := r.Header.Get(csrfHeaderName)
		if headerToken == "" {
			m.reject(w, "CSRF token header missing")
			return
		}

		// Double-submit: cookie and header must match (constant-time)...
		if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) != 1 {
			m.reject(w, "CSRF token mismatch")
			return
		}

		// ...and the token must carry a valid signature so it cannot be forged by
		// an attacker who merely plants a matching cookie+header pair.
		if !m.validToken(cookie.Value) {
			m.reject(w, "CSRF token invalid")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *CSRFMiddleware) reject(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}

func (m *CSRFMiddleware) ensureToken(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(csrfCookieName)
	if err == nil && m.validToken(cookie.Value) {
		// A valid signed token already exists; expose it for JS to read.
		w.Header().Set(csrfHeaderName, cookie.Value)
		return
	}

	// No cookie, or a legacy/invalid one: (re)issue a fresh signed token. This
	// upgrades legacy unsigned cookies on the next safe request.
	token, err := m.generateToken()
	if err != nil {
		return
	}

	m.setCookie(w, token)
	w.Header().Set(csrfHeaderName, token)
}

func (m *CSRFMiddleware) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   csrfMaxAge,
		HttpOnly: false, // JS needs to read this
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// generateToken returns a signed token of the form "<base64(random)>.<base64(hmac)>".
func (m *CSRFMiddleware) generateToken() (string, error) {
	raw := make([]byte, csrfTokenLen)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	payload := base64.URLEncoding.EncodeToString(raw)
	return payload + "." + m.sign(payload), nil
}

func (m *CSRFMiddleware) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// validToken reports whether token is a well-formed token signed by this process.
func (m *CSRFMiddleware) validToken(token string) bool {
	payload, sig, found := strings.Cut(token, ".")
	if !found || payload == "" || sig == "" {
		return false
	}
	expected := m.sign(payload)
	return subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) == 1
}

// GetToken endpoint for JS to fetch CSRF token
func (m *CSRFMiddleware) GetToken(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(csrfCookieName)
	if err == nil && m.validToken(cookie.Value) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"` + cookie.Value + `"}`))
		return
	}

	token, err := m.generateToken()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Failed to generate CSRF token"}`))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   csrfMaxAge,
		HttpOnly: false,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(csrfMaxAge * time.Second),
	})

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"token":"` + token + `"}`))
}
