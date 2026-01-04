package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
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
}

func NewCSRFMiddleware(secure bool) *CSRFMiddleware {
	return &CSRFMiddleware{secure: secure}
}

func (m *CSRFMiddleware) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public tokenized endpoints (no session) should not require CSRF headers/cookies.
		if r.URL.Path == "/r/unsubscribe" {
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"CSRF token missing"}`))
			return
		}

		headerToken := r.Header.Get(csrfHeaderName)
		if headerToken == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"CSRF token header missing"}`))
			return
		}

		// Constant-time comparison
		if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"CSRF token mismatch"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *CSRFMiddleware) ensureToken(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(csrfCookieName)
	if err == nil && cookie.Value != "" {
		// Token exists, expose it in response header for JS to read
		w.Header().Set(csrfHeaderName, cookie.Value)
		return
	}

	// Generate new token
	token, err := generateCSRFToken()
	if err != nil {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   csrfMaxAge,
		HttpOnly: false, // JS needs to read this
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set(csrfHeaderName, token)
}

func generateCSRFToken() (string, error) {
	bytes := make([]byte, csrfTokenLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GetToken endpoint for JS to fetch CSRF token
func (m *CSRFMiddleware) GetToken(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		token, err := generateCSRFToken()
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
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"token":"` + cookie.Value + `"}`))
}
