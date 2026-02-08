package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
	"github.com/HammerMeetNail/yearofbingo/internal/httpx"
	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/middleware"
)

type routeRateLimiters struct {
	authLoginIPLimiter         *middleware.RateLimiter
	authLoginEmailLimiter      *middleware.RateLimiter
	authRegisterIPLimiter      *middleware.RateLimiter
	authRegisterEmailLimiter   *middleware.RateLimiter
	authEmailFlowIPLimiter     *middleware.RateLimiter
	authEmailFlowEmailLimiter  *middleware.RateLimiter
	authResetPasswordIPLimiter *middleware.RateLimiter
	aiRateLimiter              *middleware.RateLimiter
	aiPremiumRateLimiter       *middleware.RateLimiter
	redeemLimiter              *middleware.RateLimiter
}

func buildRouteRateLimiters(
	cfg *config.Config,
	logger *logging.Logger,
	redisClient *redis.Client,
	lookupEnv func(string) (string, bool),
) *routeRateLimiters {
	aiRateLimit := resolveAIRateLimit(cfg, logger, lookupEnv)
	aiPremiumRateLimit := resolveAIPremiumRateLimit(cfg, logger, lookupEnv)
	authLimits := resolveAuthRateLimits(cfg)

	userKey := func(r *http.Request) string {
		user := handlers.GetUserFromContext(r.Context())
		if user != nil {
			return user.ID.String()
		}
		return ""
	}
	emailOrIPKey := func(r *http.Request) string {
		if email := middleware.RateLimitEmailKey(r); email != "" {
			return email
		}
		return "no_email:" + httpx.ClientIP(r)
	}

	return &routeRateLimiters{
		authLoginIPLimiter:       middleware.NewRateLimiter(redisClient, authLimits.loginIP, 15*time.Minute, "ratelimit:auth:login:ip:", func(r *http.Request) string { return "" }, false),
		authLoginEmailLimiter:    middleware.NewRateLimiter(redisClient, authLimits.loginEmail, 15*time.Minute, "ratelimit:auth:login:email:", emailOrIPKey, false),
		authRegisterIPLimiter:    middleware.NewRateLimiter(redisClient, authLimits.registerIP, 1*time.Hour, "ratelimit:auth:register:ip:", func(r *http.Request) string { return "" }, false),
		authRegisterEmailLimiter: middleware.NewRateLimiter(redisClient, authLimits.registerEmail, 1*time.Hour, "ratelimit:auth:register:email:", emailOrIPKey, false),

		authEmailFlowIPLimiter:     middleware.NewRateLimiter(redisClient, authLimits.emailFlowIP, 1*time.Hour, "ratelimit:auth:emailflow:ip:", func(r *http.Request) string { return "" }, false),
		authEmailFlowEmailLimiter:  middleware.NewRateLimiter(redisClient, authLimits.emailFlowEmail, 1*time.Hour, "ratelimit:auth:emailflow:email:", emailOrIPKey, false),
		authResetPasswordIPLimiter: middleware.NewRateLimiter(redisClient, authLimits.resetPasswordIP, 1*time.Hour, "ratelimit:auth:reset:ip:", func(r *http.Request) string { return "" }, false),

		aiRateLimiter:        middleware.NewRateLimiter(redisClient, aiRateLimit, 1*time.Hour, "ratelimit:ai:", userKey, false),
		aiPremiumRateLimiter: middleware.NewRateLimiter(redisClient, aiPremiumRateLimit, 1*time.Hour, "ratelimit:ai-premium:", userKey, false),
		redeemLimiter:        middleware.NewRateLimiter(redisClient, 10, 1*time.Hour, "ratelimit:redeem:", userKey, false),
	}
}

func resolveAIRateLimit(cfg *config.Config, logger *logging.Logger, lookupEnv func(string) (string, bool)) int64 {
	aiRateLimit := int64(10)
	if cfg.Server.Environment == "development" {
		aiRateLimit = 100
		logger.Info("Using development AI rate limit", map[string]interface{}{"limit": aiRateLimit})
	}
	if v, ok := lookupEnv("AI_RATE_LIMIT"); ok && v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			aiRateLimit = parsed
			logger.Info("Using AI rate limit from env", map[string]interface{}{"limit": aiRateLimit})
		} else {
			logger.Warn("Invalid AI_RATE_LIMIT; using default", map[string]interface{}{
				"value": v,
				"limit": aiRateLimit,
			})
		}
	}
	return aiRateLimit
}

func resolveAIPremiumRateLimit(cfg *config.Config, logger *logging.Logger, lookupEnv func(string) (string, bool)) int64 {
	limit := int64(cfg.AI.PremiumEndpointRateLimit)
	if limit <= 0 {
		limit = 60
	}
	if v, ok := lookupEnv("AI_PREMIUM_ENDPOINT_RATE_LIMIT"); ok && v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			limit = parsed
			logger.Info("Using premium AI endpoint rate limit from env", map[string]interface{}{"limit": limit})
		} else {
			logger.Warn("Invalid AI_PREMIUM_ENDPOINT_RATE_LIMIT; using default", map[string]interface{}{
				"value": v,
				"limit": limit,
			})
		}
	}
	return limit
}

// authRateLimits holds rate limit values for auth endpoints.
type authRateLimits struct {
	loginIP         int64
	loginEmail      int64
	registerIP      int64
	registerEmail   int64
	emailFlowIP     int64
	emailFlowEmail  int64
	resetPasswordIP int64
}

// resolveAuthRateLimits returns rate limit values for auth endpoints.
// In development mode, limits are significantly higher to avoid breaking e2e tests.
func resolveAuthRateLimits(cfg *config.Config) authRateLimits {
	if cfg.Server.Environment == "development" {
		// Development: high limits to allow e2e tests to run without hitting rate limits
		return authRateLimits{
			loginIP:         1000,
			loginEmail:      500,
			registerIP:      1000,
			registerEmail:   500,
			emailFlowIP:     1000,
			emailFlowEmail:  500,
			resetPasswordIP: 1000,
		}
	}
	// Production: strict limits to prevent abuse
	return authRateLimits{
		loginIP:         30,
		loginEmail:      10,
		registerIP:      10,
		registerEmail:   5,
		emailFlowIP:     10,
		emailFlowEmail:  5,
		resetPasswordIP: 10,
	}
}
