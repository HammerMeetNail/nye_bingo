package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
)

type RateLimiter struct {
	redis  *redis.Client
	limit  int64
	window time.Duration
	prefix string
	keyFn  func(r *http.Request) string
	// failOpen controls behavior when Redis errors: when true, requests are allowed through.
	// For cost-sensitive endpoints, set to false to fail closed.
	failOpen bool
}

func NewRateLimiter(redis *redis.Client, limit int64, window time.Duration, prefix string, keyFn func(r *http.Request) string, failOpen bool) *RateLimiter {
	return &RateLimiter{
		redis:    redis,
		limit:    limit,
		window:   window,
		prefix:   prefix,
		keyFn:    keyFn,
		failOpen: failOpen,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rl.redis == nil {
			next.ServeHTTP(w, r)
			return
		}

		keySuffix := rl.keyFn(r)
		if keySuffix == "" {
			// Fallback to IP if key function returns empty string
			keySuffix = GetClientIP(r)
		}

		key := fmt.Sprintf("%s%s", rl.prefix, keySuffix)
		ctx := r.Context()

		// Use a Lua script to atomically INCR and set EXPIRE if new
		luaScript := `
			local current
			current = redis.call("INCR", KEYS[1])
			if current == 1 then
				redis.call("EXPIRE", KEYS[1], ARGV[1])
			end
			return current
		`
		ttlSeconds := int64(rl.window.Seconds())
		result, err := rl.redis.Eval(ctx, luaScript, []string{key}, ttlSeconds).Result()
		if err != nil {
			logging.Error("Rate limit Redis error", map[string]interface{}{"error": err.Error()})
			if rl.failOpen {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusServiceUnavailable, "Rate limiting temporarily unavailable")
			return
		}

		var count int64
		// Redis may return int64 or float64 depending on the client/driver details for Lua, handle both
		switch v := result.(type) {
		case int64:
			count = v
		case float64:
			count = int64(v)
		default:
			logging.Error("Rate limit Redis script returned unexpected type", map[string]interface{}{"type": fmt.Sprintf("%T", result)})
			if rl.failOpen {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusServiceUnavailable, "Rate limiting temporarily unavailable")
			return
		}

		if count > rl.limit {
			writeError(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// GetClientIP extracts the client IP from the request, respecting X-Forwarded-For
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (set by Cloudflare/proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs; the first one is the client
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
