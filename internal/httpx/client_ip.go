package httpx

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the best-effort client IP for the request.
//
// It only honors X-Forwarded-For / X-Real-IP when the request has been marked as coming from a trusted
// proxy (see WithTrustedForwardedHeaders). Otherwise it falls back to RemoteAddr.
func ClientIP(r *http.Request) string {
	if TrustedForwardedHeaders(r) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// X-Forwarded-For can contain multiple IPs; the first one is the client
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
