package middleware

import (
	"net/http"

	"github.com/HammerMeetNail/yearofbingo/internal/httpx"
)

// TrustedProxyHeaders marks whether X-Forwarded-* headers should be trusted based on the connecting peer.
//
// This does not modify headers; it only annotates the request context for downstream helpers.
type TrustedProxyHeaders struct {
	checker *httpx.TrustedProxyChecker
}

func NewTrustedProxyHeaders(checker *httpx.TrustedProxyChecker) *TrustedProxyHeaders {
	return &TrustedProxyHeaders{checker: checker}
}

func (m *TrustedProxyHeaders) Apply(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trusted := false
		if m.checker != nil {
			trusted = m.checker.IsTrustedRequest(r)
		}
		next.ServeHTTP(w, httpx.WithTrustedForwardedHeaders(r, trusted))
	})
}
