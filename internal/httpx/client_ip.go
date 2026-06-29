package httpx

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the best-effort client IP for the request.
//
// It only honors forwarding headers when the request has been marked as coming
// from a trusted proxy (see WithTrustedForwardedHeaders); otherwise it falls
// back to RemoteAddr.
//
// Header preference, in order:
//  1. CF-Connecting-IP — a single value set by Cloudflare and not appendable by
//     the client (ingress is firewalled to Cloudflare ranges in production).
//  2. X-Forwarded-For — the RIGHT-MOST entry. Cloudflare appends the real client
//     IP to any client-supplied XFF, so the right-most hop (the one added by the
//     nearest trusted proxy) is the trustworthy value. Taking the left-most entry
//     would let a client spoof the rate-limit key by prepending a fake address.
//  3. X-Real-IP — a single value set by some proxies.
func ClientIP(r *http.Request) string {
	if TrustedForwardedHeaders(r) {
		if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
			return cf
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if ip := rightmostForwarded(xff); ip != "" {
				return ip
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			return xri
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// rightmostForwarded returns the last non-empty entry of a comma-separated
// X-Forwarded-For header value.
func rightmostForwarded(xff string) string {
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		if v := strings.TrimSpace(parts[i]); v != "" {
			return v
		}
	}
	return ""
}
