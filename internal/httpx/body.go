package httpx

import (
	"context"
	"net/http"
)

type bodyBytesKey struct{}

// WithBodyBytes stores a copy of the request body bytes in the request context.
//
// Intended for middleware that needs to read the body (e.g., size limits) while still allowing
// downstream handlers/middlewares to read it again.
func WithBodyBytes(r *http.Request, body []byte) *http.Request {
	ctx := context.WithValue(r.Context(), bodyBytesKey{}, body)
	return r.WithContext(ctx)
}

// BodyBytes returns the stored request body bytes, if present.
func BodyBytes(r *http.Request) ([]byte, bool) {
	v := r.Context().Value(bodyBytesKey{})
	b, ok := v.([]byte)
	return b, ok
}
