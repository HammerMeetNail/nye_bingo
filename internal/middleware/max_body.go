package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/HammerMeetNail/yearofbingo/internal/httpx"
)

type MaxBodySize struct {
	maxBytes int64
}

func NewMaxBodySize(maxBytes int64) *MaxBodySize {
	return &MaxBodySize{maxBytes: maxBytes}
}

func (m *MaxBodySize) Apply(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldLimitBody(r) {
			next.ServeHTTP(w, r)
			return
		}

		if r.ContentLength > m.maxBytes && r.ContentLength != -1 {
			writeBodyTooLarge(w)
			return
		}

		limited := io.LimitReader(r.Body, m.maxBytes+1)
		body, err := io.ReadAll(limited)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		_ = r.Body.Close()

		if int64(len(body)) > m.maxBytes {
			writeBodyTooLarge(w)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, httpx.WithBodyBytes(r, body))
	})
}

func shouldLimitBody(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return false
	}
	return strings.HasPrefix(r.URL.Path, "/api/")
}

func writeBodyTooLarge(w http.ResponseWriter) {
	writeJSONError(w, http.StatusRequestEntityTooLarge, "Request body too large")
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
