package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/HammerMeetNail/yearofbingo/internal/httpx"
)

func RateLimitEmailKey(r *http.Request) string {
	body, ok := httpx.BodyBytes(r)
	if !ok || len(body) == 0 {
		return ""
	}
	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	email := strings.TrimSpace(strings.ToLower(payload.Email))
	if email == "" {
		return ""
	}
	return email
}
