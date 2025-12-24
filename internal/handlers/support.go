package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

const (
	supportRateLimitMax    = 5             // max requests per window
	supportRateLimitWindow = 1 * time.Hour // rate limit window
	supportRateLimitPrefix = "ratelimit:support:"
)

type SupportHandler struct {
	emailService services.EmailServiceInterface
	redis        *redis.Client
}

func NewSupportHandler(emailService services.EmailServiceInterface, redisClient *redis.Client) *SupportHandler {
	return &SupportHandler{
		emailService: emailService,
		redis:        redisClient,
	}
}

type SupportRequest struct {
	Email    string `json:"email"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

var validCategories = map[string]bool{
	"Bug Report":       true,
	"Feature Request":  true,
	"Account Issue":    true,
	"General Question": true,
}

func (h *SupportHandler) Submit(w http.ResponseWriter, r *http.Request) {
	// Check rate limit
	clientIP := getClientIP(r)
	if !h.checkRateLimit(r, clientIP) {
		writeError(w, http.StatusTooManyRequests, "Too many requests. Please try again later.")
		return
	}

	var req SupportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate email
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid email address")
		return
	}

	// Validate category
	if !validCategories[req.Category] {
		writeError(w, http.StatusBadRequest, "Invalid category")
		return
	}

	// Validate message
	req.Message = strings.TrimSpace(req.Message)
	if len(req.Message) < 10 {
		writeError(w, http.StatusBadRequest, "Message must be at least 10 characters")
		return
	}
	if len(req.Message) > 5000 {
		writeError(w, http.StatusBadRequest, "Message must be less than 5000 characters")
		return
	}

	// Get user ID if logged in
	userID := ""
	if user := GetUserFromContext(r.Context()); user != nil {
		userID = user.ID.String()
	}

	// Send the support email
	if err := h.emailService.SendSupportEmail(r.Context(), req.Email, req.Category, req.Message, userID); err != nil {
		logging.Error("Failed to send support email", map[string]interface{}{
			"error":    err.Error(),
			"email":    req.Email,
			"category": req.Category,
		})
		writeError(w, http.StatusInternalServerError, "Failed to send message. Please try again later.")
		return
	}

	logging.Info("Support request submitted", map[string]interface{}{
		"email":    req.Email,
		"category": req.Category,
		"user_id":  userID,
		"ip":       clientIP,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Your message has been sent. We'll get back to you soon!",
	})
}

// checkRateLimit checks if the client has exceeded the rate limit
func (h *SupportHandler) checkRateLimit(r *http.Request, clientIP string) bool {
	if h.redis == nil {
		return true // no rate limiting if Redis not configured
	}

	ctx := r.Context()
	key := fmt.Sprintf("%s%s", supportRateLimitPrefix, clientIP)

	// Increment counter
	count, err := h.redis.Incr(ctx, key).Result()
	if err != nil {
		logging.Error("Rate limit Redis error", map[string]interface{}{"error": err.Error()})
		return true // allow request on Redis error
	}

	// Set expiry on first request
	if count == 1 {
		h.redis.Expire(ctx, key, supportRateLimitWindow)
	}

	return count <= supportRateLimitMax
}

// getClientIP extracts the client IP from the request, respecting X-Forwarded-For
func getClientIP(r *http.Request) string {
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
