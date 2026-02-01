package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Email    EmailConfig
	AI       AIConfig
	OAuth    OAuthConfig
	Billing  BillingConfig
}

type ServerConfig struct {
	Host              string
	Port              int
	Secure            bool   // Use HTTPS-only cookies
	Environment       string // "development", "production", "test"
	Debug             bool
	DebugMaxChars     int
	TrustedProxyCIDRs []string // Optional: CIDRs allowed to set X-Forwarded-* headers
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type AIConfig struct {
	GeminiAPIKey          string
	Stub                  bool
	GeminiModel           string
	GeminiThinkingLevel   string
	GeminiThinkingBudget  int
	GeminiTemperature     float64
	GeminiMaxOutputTokens int
}

type EmailConfig struct {
	Provider     string // "resend", "smtp", "console"
	FromAddress  string
	FromName     string
	BaseURL      string // Application base URL for links
	ResendAPIKey string
	// SMTP settings (for Mailpit in local dev)
	SMTPHost string
	SMTPPort int
}

type OAuthConfig struct {
	AllowedProviders []string
	Google           OAuthProviderConfig
}

type OAuthProviderConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	RedirectURL  string
	IssuerURL    string
	Scopes       []string
}

type BillingConfig struct {
	Enabled                      bool
	StripeSecretKey              string
	StripeWebhookSecret          string
	StripeAPIBaseURL             string
	StripePremiumMonthlyPriceID  string
	StripePremiumYearlyPriceID   string
	StripePremiumLifetimePriceID string
	StripeTip5PriceID            string
	StripeTip10PriceID           string
	StripeTip20PriceID           string
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode,
	)
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:              getEnv("SERVER_HOST", "0.0.0.0"),
			Port:              getEnvInt("SERVER_PORT", 8080),
			Secure:            getEnvBool("SERVER_SECURE", false),
			Environment:       getEnv("APP_ENV", "development"),
			Debug:             getEnvBool("DEBUG", false),
			DebugMaxChars:     getEnvInt("DEBUG_LOG_MAX_CHARS", 8000),
			TrustedProxyCIDRs: getEnvList("TRUSTED_PROXY_CIDRS", nil),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "bingo"),
			Password: getEnv("DB_PASSWORD", "bingo"),
			DBName:   getEnv("DB_NAME", "nye_bingo"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Email: EmailConfig{
			Provider:     getEnv("EMAIL_PROVIDER", "console"),
			FromAddress:  getEnv("EMAIL_FROM_ADDRESS", "noreply@yearofbingo.com"),
			FromName:     getEnv("EMAIL_FROM_NAME", "Year of Bingo"),
			BaseURL:      getEnv("APP_BASE_URL", "http://localhost:8080"),
			ResendAPIKey: getEnv("RESEND_API_KEY", ""),
			SMTPHost:     getEnv("SMTP_HOST", "localhost"),
			SMTPPort:     getEnvInt("SMTP_PORT", 1025),
		},
		AI: AIConfig{
			GeminiAPIKey:          getEnv("GEMINI_API_KEY", ""),
			GeminiModel:           getEnvNonEmpty("GEMINI_MODEL", "gemini-3-flash-preview"),
			GeminiThinkingLevel:   getEnvNonEmpty("GEMINI_THINKING_LEVEL", "minimal"),
			GeminiThinkingBudget:  getEnvInt("GEMINI_THINKING_BUDGET", 0),
			GeminiTemperature:     getEnvFloat64("GEMINI_TEMPERATURE", 0.8),
			GeminiMaxOutputTokens: getEnvInt("GEMINI_MAX_OUTPUT_TOKENS", 4096),
			Stub:                  getEnvBool("AI_STUB", false),
		},
		OAuth: OAuthConfig{
			AllowedProviders: getEnvList("OAUTH_ALLOWED_PROVIDERS", nil),
			Google: OAuthProviderConfig{
				Enabled:      getEnvBool("GOOGLE_OAUTH_ENABLED", false),
				ClientID:     getEnv("GOOGLE_OAUTH_CLIENT_ID", ""),
				ClientSecret: getEnv("GOOGLE_OAUTH_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("GOOGLE_OAUTH_REDIRECT_URL", ""),
				IssuerURL:    getEnvNonEmpty("GOOGLE_OIDC_ISSUER_URL", "https://accounts.google.com"),
				Scopes:       getEnvList("GOOGLE_OIDC_SCOPES", []string{"openid", "email", "profile"}),
			},
		},
		Billing: BillingConfig{
			Enabled:                      getEnvBool("BILLING_ENABLED", false),
			StripeSecretKey:              getEnv("STRIPE_SECRET_KEY", ""),
			StripeWebhookSecret:          getEnv("STRIPE_WEBHOOK_SECRET", ""),
			StripeAPIBaseURL:             getEnv("STRIPE_API_BASE_URL", ""),
			StripePremiumMonthlyPriceID:  getEnv("STRIPE_PREMIUM_PRICE_MONTHLY", ""),
			StripePremiumYearlyPriceID:   getEnv("STRIPE_PREMIUM_PRICE_YEARLY", ""),
			StripePremiumLifetimePriceID: getEnv("STRIPE_PREMIUM_PRICE_LIFETIME", ""),
			StripeTip5PriceID:            getEnv("STRIPE_TIP_PRICE_5", ""),
			StripeTip10PriceID:           getEnv("STRIPE_TIP_PRICE_10", ""),
			StripeTip20PriceID:           getEnv("STRIPE_TIP_PRICE_20", ""),
		},
	}

	if cfg.Server.Environment == "production" && cfg.Billing.Enabled {
		if err := validateBillingConfig(cfg.Billing); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func validateBillingConfig(b BillingConfig) error {
	required := map[string]string{
		"STRIPE_SECRET_KEY":             b.StripeSecretKey,
		"STRIPE_WEBHOOK_SECRET":         b.StripeWebhookSecret,
		"STRIPE_PREMIUM_PRICE_MONTHLY":  b.StripePremiumMonthlyPriceID,
		"STRIPE_PREMIUM_PRICE_YEARLY":   b.StripePremiumYearlyPriceID,
		"STRIPE_PREMIUM_PRICE_LIFETIME": b.StripePremiumLifetimePriceID,
		"STRIPE_TIP_PRICE_5":            b.StripeTip5PriceID,
		"STRIPE_TIP_PRICE_10":           b.StripeTip10PriceID,
		"STRIPE_TIP_PRICE_20":           b.StripeTip20PriceID,
	}

	var missing []string
	for k, v := range required {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("billing enabled but missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvNonEmpty(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		if strings.TrimSpace(value) != "" {
			return value
		}
		return defaultValue
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvFloat64(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getEnvList(key string, defaultValues []string) []string {
	if value, exists := os.LookupEnv(key); exists {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return defaultValues
		}
		parts := strings.Split(trimmed, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	}
	return defaultValues
}
