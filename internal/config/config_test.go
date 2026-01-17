package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing env vars that might interfere
	envVars := []string{
		"SERVER_HOST", "SERVER_PORT", "SERVER_SECURE", "DEBUG", "DEBUG_LOG_MAX_CHARS",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DB",
		"AI_STUB", "GEMINI_API_KEY", "GEMINI_MODEL", "GEMINI_THINKING_LEVEL", "GEMINI_THINKING_BUDGET", "GEMINI_TEMPERATURE", "GEMINI_MAX_OUTPUT_TOKENS",
		"OAUTH_ALLOWED_PROVIDERS", "GOOGLE_OAUTH_ENABLED", "GOOGLE_OAUTH_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_SECRET", "GOOGLE_OAUTH_REDIRECT_URL", "GOOGLE_OIDC_ISSUER_URL", "GOOGLE_OIDC_SCOPES",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Server defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected Server.Host to be 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected Server.Port to be 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Secure != false {
		t.Error("expected Server.Secure to be false")
	}
	if cfg.Server.Debug != false {
		t.Error("expected Server.Debug to be false")
	}
	if cfg.Server.DebugMaxChars != 8000 {
		t.Errorf("expected Server.DebugMaxChars to be 8000, got %d", cfg.Server.DebugMaxChars)
	}

	// Database defaults
	if cfg.Database.Host != "localhost" {
		t.Errorf("expected Database.Host to be localhost, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("expected Database.Port to be 5432, got %d", cfg.Database.Port)
	}
	if cfg.Database.User != "bingo" {
		t.Errorf("expected Database.User to be bingo, got %s", cfg.Database.User)
	}
	if cfg.Database.Password != "bingo" {
		t.Errorf("expected Database.Password to be bingo, got %s", cfg.Database.Password)
	}
	if cfg.Database.DBName != "nye_bingo" {
		t.Errorf("expected Database.DBName to be nye_bingo, got %s", cfg.Database.DBName)
	}
	if cfg.Database.SSLMode != "disable" {
		t.Errorf("expected Database.SSLMode to be disable, got %s", cfg.Database.SSLMode)
	}

	// Redis defaults
	if cfg.Redis.Host != "localhost" {
		t.Errorf("expected Redis.Host to be localhost, got %s", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6379 {
		t.Errorf("expected Redis.Port to be 6379, got %d", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "" {
		t.Errorf("expected Redis.Password to be empty, got %s", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("expected Redis.DB to be 0, got %d", cfg.Redis.DB)
	}

	// AI defaults
	if cfg.AI.GeminiAPIKey != "" {
		t.Errorf("expected AI.GeminiAPIKey to be empty, got %q", cfg.AI.GeminiAPIKey)
	}
	if cfg.AI.GeminiModel != "gemini-3-flash-preview" {
		t.Errorf("expected AI.GeminiModel to be gemini-3-flash-preview, got %q", cfg.AI.GeminiModel)
	}
	if cfg.AI.GeminiThinkingLevel != "minimal" {
		t.Errorf("expected AI.GeminiThinkingLevel to be minimal, got %q", cfg.AI.GeminiThinkingLevel)
	}
	if cfg.AI.GeminiThinkingBudget != 0 {
		t.Errorf("expected AI.GeminiThinkingBudget to be 0, got %d", cfg.AI.GeminiThinkingBudget)
	}
	if cfg.AI.GeminiTemperature != 0.8 {
		t.Errorf("expected AI.GeminiTemperature to be 0.8, got %v", cfg.AI.GeminiTemperature)
	}
	if cfg.AI.GeminiMaxOutputTokens != 4096 {
		t.Errorf("expected AI.GeminiMaxOutputTokens to be 4096, got %d", cfg.AI.GeminiMaxOutputTokens)
	}
	if cfg.AI.Stub != false {
		t.Error("expected AI.Stub to be false")
	}

	if cfg.OAuth.Google.Enabled != false {
		t.Error("expected OAuth.Google.Enabled to be false")
	}
	if cfg.OAuth.Google.IssuerURL != "https://accounts.google.com" {
		t.Errorf("expected OAuth.Google.IssuerURL to be https://accounts.google.com, got %q", cfg.OAuth.Google.IssuerURL)
	}
	if len(cfg.OAuth.Google.Scopes) != 3 {
		t.Fatalf("expected default OAuth.Google.Scopes length 3, got %d", len(cfg.OAuth.Google.Scopes))
	}
	if cfg.OAuth.Google.Scopes[0] != "openid" {
		t.Errorf("expected OAuth.Google.Scopes[0] to be openid, got %q", cfg.OAuth.Google.Scopes[0])
	}
}

func TestLoad_CustomValues(t *testing.T) {
	// Set custom values
	os.Setenv("SERVER_HOST", "127.0.0.1")
	os.Setenv("SERVER_PORT", "3000")
	os.Setenv("SERVER_SECURE", "true")
	os.Setenv("DEBUG", "true")
	os.Setenv("DEBUG_LOG_MAX_CHARS", "1234")
	os.Setenv("DB_HOST", "db.example.com")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_USER", "admin")
	os.Setenv("DB_PASSWORD", "secret123")
	os.Setenv("DB_NAME", "mydb")
	os.Setenv("DB_SSLMODE", "require")
	os.Setenv("REDIS_HOST", "redis.example.com")
	os.Setenv("REDIS_PORT", "6380")
	os.Setenv("REDIS_PASSWORD", "redispass")
	os.Setenv("REDIS_DB", "1")
	os.Setenv("OAUTH_ALLOWED_PROVIDERS", "google,apple")
	os.Setenv("GOOGLE_OAUTH_ENABLED", "true")
	os.Setenv("GOOGLE_OAUTH_CLIENT_ID", "client-id")
	os.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "client-secret")
	os.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "https://example.com/callback")
	os.Setenv("GOOGLE_OIDC_ISSUER_URL", "https://issuer.example.com")
	os.Setenv("GOOGLE_OIDC_SCOPES", "openid,email")

	defer func() {
		// Clean up
		os.Unsetenv("SERVER_HOST")
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("SERVER_SECURE")
		os.Unsetenv("DEBUG")
		os.Unsetenv("DEBUG_LOG_MAX_CHARS")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("DB_SSLMODE")
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("REDIS_DB")
		os.Unsetenv("OAUTH_ALLOWED_PROVIDERS")
		os.Unsetenv("GOOGLE_OAUTH_ENABLED")
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_ID")
		os.Unsetenv("GOOGLE_OAUTH_CLIENT_SECRET")
		os.Unsetenv("GOOGLE_OAUTH_REDIRECT_URL")
		os.Unsetenv("GOOGLE_OIDC_ISSUER_URL")
		os.Unsetenv("GOOGLE_OIDC_SCOPES")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Server values
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected Server.Host to be 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("expected Server.Port to be 3000, got %d", cfg.Server.Port)
	}
	if cfg.Server.Secure != true {
		t.Error("expected Server.Secure to be true")
	}
	if cfg.Server.Debug != true {
		t.Error("expected Server.Debug to be true")
	}
	if cfg.Server.DebugMaxChars != 1234 {
		t.Errorf("expected Server.DebugMaxChars to be 1234, got %d", cfg.Server.DebugMaxChars)
	}

	// Database values
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("expected Database.Host to be db.example.com, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("expected Database.Port to be 5433, got %d", cfg.Database.Port)
	}
	if cfg.Database.User != "admin" {
		t.Errorf("expected Database.User to be admin, got %s", cfg.Database.User)
	}
	if cfg.Database.Password != "secret123" {
		t.Errorf("expected Database.Password to be secret123, got %s", cfg.Database.Password)
	}
	if cfg.Database.DBName != "mydb" {
		t.Errorf("expected Database.DBName to be mydb, got %s", cfg.Database.DBName)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("expected Database.SSLMode to be require, got %s", cfg.Database.SSLMode)
	}

	// Redis values
	if cfg.Redis.Host != "redis.example.com" {
		t.Errorf("expected Redis.Host to be redis.example.com, got %s", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6380 {
		t.Errorf("expected Redis.Port to be 6380, got %d", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "redispass" {
		t.Errorf("expected Redis.Password to be redispass, got %s", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 1 {
		t.Errorf("expected Redis.DB to be 1, got %d", cfg.Redis.DB)
	}

	if len(cfg.OAuth.AllowedProviders) != 2 {
		t.Fatalf("expected OAuth.AllowedProviders length 2, got %d", len(cfg.OAuth.AllowedProviders))
	}
	if cfg.OAuth.AllowedProviders[0] != "google" {
		t.Errorf("expected OAuth.AllowedProviders[0] to be google, got %q", cfg.OAuth.AllowedProviders[0])
	}
	if cfg.OAuth.Google.Enabled != true {
		t.Error("expected OAuth.Google.Enabled to be true")
	}
	if cfg.OAuth.Google.ClientID != "client-id" {
		t.Errorf("expected OAuth.Google.ClientID to be client-id, got %q", cfg.OAuth.Google.ClientID)
	}
	if cfg.OAuth.Google.ClientSecret != "client-secret" {
		t.Errorf("expected OAuth.Google.ClientSecret to be client-secret, got %q", cfg.OAuth.Google.ClientSecret)
	}
	if cfg.OAuth.Google.RedirectURL != "https://example.com/callback" {
		t.Errorf("expected OAuth.Google.RedirectURL to be https://example.com/callback, got %q", cfg.OAuth.Google.RedirectURL)
	}
	if cfg.OAuth.Google.IssuerURL != "https://issuer.example.com" {
		t.Errorf("expected OAuth.Google.IssuerURL to be https://issuer.example.com, got %q", cfg.OAuth.Google.IssuerURL)
	}
	if len(cfg.OAuth.Google.Scopes) != 2 || cfg.OAuth.Google.Scopes[1] != "email" {
		t.Errorf("unexpected OAuth.Google.Scopes: %#v", cfg.OAuth.Google.Scopes)
	}
}

func TestLoad_InvalidIntFallsBackToDefault(t *testing.T) {
	os.Setenv("SERVER_PORT", "notanumber")
	defer os.Unsetenv("SERVER_PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected Server.Port to fall back to 8080, got %d", cfg.Server.Port)
	}
}

func TestLoad_InvalidBoolFallsBackToDefault(t *testing.T) {
	os.Setenv("SERVER_SECURE", "notabool")
	defer os.Unsetenv("SERVER_SECURE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Secure != false {
		t.Error("expected Server.Secure to fall back to false")
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	expected := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"
	if got := cfg.DSN(); got != expected {
		t.Errorf("expected DSN %q, got %q", expected, got)
	}
}

func TestRedisConfig_Addr(t *testing.T) {
	cfg := RedisConfig{
		Host: "redis.example.com",
		Port: 6380,
	}

	expected := "redis.example.com:6380"
	if got := cfg.Addr(); got != expected {
		t.Errorf("expected Addr %q, got %q", expected, got)
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "returns default when not set",
			key:          "TEST_GET_ENV_1",
			envValue:     "",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "returns env value when set",
			key:          "TEST_GET_ENV_2",
			envValue:     "custom",
			defaultValue: "default",
			expected:     "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "returns default when not set",
			key:          "TEST_GET_ENV_INT_1",
			envValue:     "",
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "returns parsed int when set",
			key:          "TEST_GET_ENV_INT_2",
			envValue:     "42",
			defaultValue: 100,
			expected:     42,
		},
		{
			name:         "returns default when invalid",
			key:          "TEST_GET_ENV_INT_3",
			envValue:     "notanumber",
			defaultValue: 100,
			expected:     100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			got := getEnvInt(tt.key, tt.defaultValue)
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue bool
		expected     bool
	}{
		{
			name:         "returns default when not set",
			key:          "TEST_GET_ENV_BOOL_1",
			envValue:     "",
			defaultValue: false,
			expected:     false,
		},
		{
			name:         "returns true when set to true",
			key:          "TEST_GET_ENV_BOOL_2",
			envValue:     "true",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "returns false when set to false",
			key:          "TEST_GET_ENV_BOOL_3",
			envValue:     "false",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "returns true when set to 1",
			key:          "TEST_GET_ENV_BOOL_4",
			envValue:     "1",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "returns default when invalid",
			key:          "TEST_GET_ENV_BOOL_5",
			envValue:     "notabool",
			defaultValue: true,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			got := getEnvBool(tt.key, tt.defaultValue)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
