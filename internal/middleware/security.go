package middleware

import (
	"net/http"
)

// SecurityHeaders adds security-related HTTP headers to responses.
type SecurityHeaders struct {
	secure bool
}

// NewSecurityHeaders creates a new security headers middleware.
func NewSecurityHeaders(secure bool) *SecurityHeaders {
	return &SecurityHeaders{secure: secure}
}

// Apply adds security headers to all responses.
func (s *SecurityHeaders) Apply(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Enable XSS filter in older browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer policy - don't leak full URL to other origins
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy - disable unnecessary browser features
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Content Security Policy
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-hashes' https://static.cloudflareinsights.com https://cdnjs.cloudflare.com; " +
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://fonts.cdnfonts.com https://cdnjs.cloudflare.com; " +
			"font-src 'self' https://fonts.gstatic.com https://fonts.cdnfonts.com https://cdnjs.cloudflare.com data:; " +
			"img-src 'self' data:; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		w.Header().Set("Content-Security-Policy", csp)

		// HSTS - only in secure mode (production)
		if s.secure {
			// max-age of 1 year, include subdomains
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}
