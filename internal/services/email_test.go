package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
)

func TestGenerateToken(t *testing.T) {
	token, hash, err := GenerateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Token should be 64 hex characters (32 bytes)
	if len(token) != 64 {
		t.Errorf("expected token length 64, got %d", len(token))
	}

	// Hash should be 64 hex characters (SHA256 = 32 bytes)
	if len(hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash))
	}

	// Token and hash should be different
	if token == hash {
		t.Error("token and hash should not be equal")
	}

	// Hash should be deterministic
	computedHash := HashToken(token)
	if computedHash != hash {
		t.Error("HashToken should produce same hash as GenerateToken")
	}
}

func TestGenerateToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	hashes := make(map[string]bool)

	for i := 0; i < 100; i++ {
		token, hash, err := GenerateToken()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}

		if tokens[token] {
			t.Errorf("duplicate token generated on iteration %d", i)
		}
		tokens[token] = true

		if hashes[hash] {
			t.Errorf("duplicate hash generated on iteration %d", i)
		}
		hashes[hash] = true
	}
}

func TestHashToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"simple token", "abc123"},
		{"empty token", ""},
		{"long token", strings.Repeat("a", 1000)},
		{"special characters", "!@#$%^&*()"},
		{"unicode", "日本語"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := HashToken(tt.token)

			// Hash should always be 64 characters (SHA256)
			if len(hash) != 64 {
				t.Errorf("expected hash length 64, got %d", len(hash))
			}

			// Hash should be deterministic
			hash2 := HashToken(tt.token)
			if hash != hash2 {
				t.Error("hash should be deterministic")
			}
		})
	}
}

func TestEmailService_VerifyEmail_InvalidToken(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return context.Canceled
			}}
		},
	}

	service := NewEmailService(&config.EmailConfig{}, db)
	err := service.VerifyEmail(context.Background(), "token")
	if err == nil || !strings.Contains(err.Error(), "invalid verification token") {
		t.Fatalf("expected invalid verification token error, got %v", err)
	}
}

func TestEmailService_VerifyEmail_Expired(t *testing.T) {
	expired := time.Now().Add(-1 * time.Hour)
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(uuid.New(), expired)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			t.Fatal("unexpected exec for expired token")
			return fakeCommandTag{}, nil
		},
	}

	service := NewEmailService(&config.EmailConfig{}, db)
	err := service.VerifyEmail(context.Background(), "token")
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired token error, got %v", err)
	}
}

func TestEmailService_VerifyMagicLink_Used(t *testing.T) {
	usedAt := time.Now().Add(-1 * time.Minute)
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(uuid.New(), "user@example.com", time.Now().Add(1*time.Hour), &usedAt)
		},
	}

	service := NewEmailService(&config.EmailConfig{}, db)
	_, err := service.VerifyMagicLink(context.Background(), "token")
	if err == nil || !strings.Contains(err.Error(), "already been used") {
		t.Fatalf("expected used token error, got %v", err)
	}
}

func TestEmailService_VerifyPasswordResetToken_Expired(t *testing.T) {
	expired := time.Now().Add(-1 * time.Hour)
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(uuid.New(), uuid.New(), expired, (*time.Time)(nil))
		},
	}

	service := NewEmailService(&config.EmailConfig{}, db)
	_, err := service.VerifyPasswordResetToken(context.Background(), "token")
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired token error, got %v", err)
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	token := "test_token_12345"

	// Hash the same token multiple times
	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		hashes[i] = HashToken(token)
	}

	// All hashes should be identical
	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("hash at index %d differs from hash at index 0", i)
		}
	}
}

func TestHashToken_DifferentInputs(t *testing.T) {
	tokens := []string{"token1", "token2", "token3", "Token1"}
	hashes := make(map[string]string)

	for _, token := range tokens {
		hash := HashToken(token)
		if existing, ok := hashes[hash]; ok {
			t.Errorf("collision: %q and %q produce same hash", token, existing)
		}
		hashes[hash] = token
	}
}

func TestConsoleProvider_Send(t *testing.T) {
	provider := NewConsoleProvider()

	email := &Email{
		To:      "test@example.com",
		Subject: "Test Subject",
		HTML:    "<h1>Test</h1>",
		Text:    "Test",
	}

	err := provider.Send(context.Background(), email)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewResendProvider(t *testing.T) {
	provider := NewResendProvider("test_api_key")
	if provider == nil {
		t.Fatal("expected provider to be created")
	}
	if provider.client == nil {
		t.Error("expected client to be initialized")
	}
}

func TestNewSMTPProvider(t *testing.T) {
	provider := NewSMTPProvider("localhost", 1025)
	if provider == nil {
		t.Fatal("expected provider to be created")
	}
	if provider.host != "localhost" {
		t.Errorf("expected host 'localhost', got %q", provider.host)
	}
	if provider.port != 1025 {
		t.Errorf("expected port 1025, got %d", provider.port)
	}
}

func TestTokenExpiryConstants(t *testing.T) {
	// Verify token expiry constants are reasonable

	if VerificationTokenExpiry.Hours() != 24 {
		t.Errorf("expected verification token expiry to be 24 hours, got %v", VerificationTokenExpiry)
	}

	if MagicLinkTokenExpiry.Minutes() != 15 {
		t.Errorf("expected magic link token expiry to be 15 minutes, got %v", MagicLinkTokenExpiry)
	}

	if PasswordResetTokenExpiry.Hours() != 1 {
		t.Errorf("expected password reset token expiry to be 1 hour, got %v", PasswordResetTokenExpiry)
	}
}

func TestEmailStruct(t *testing.T) {
	email := Email{
		To:      "test@example.com",
		Subject: "Test Subject",
		HTML:    "<p>HTML content</p>",
		Text:    "Text content",
	}

	if email.To != "test@example.com" {
		t.Errorf("expected To 'test@example.com', got %q", email.To)
	}
	if email.Subject != "Test Subject" {
		t.Errorf("expected Subject 'Test Subject', got %q", email.Subject)
	}
}

func TestEmailRenderingFunctions(t *testing.T) {
	// Create a minimal service just to test rendering
	svc := &EmailService{
		baseURL: "https://yearofbingo.com",
	}

	t.Run("verification email", func(t *testing.T) {
		html, text := svc.renderVerificationEmail("https://example.com/verify?token=abc")

		if !strings.Contains(html, "Verify Email Address") {
			t.Error("HTML should contain verify button text")
		}
		if !strings.Contains(html, "https://example.com/verify?token=abc") {
			t.Error("HTML should contain verify URL")
		}
		if !strings.Contains(text, "verify your email") {
			t.Error("text should mention verification")
		}
	})

	t.Run("magic link email", func(t *testing.T) {
		html, text := svc.renderMagicLinkEmail("https://example.com/magic?token=def")

		if !strings.Contains(html, "Sign In") {
			t.Error("HTML should contain sign in button")
		}
		if !strings.Contains(html, "15 minutes") {
			t.Error("HTML should mention 15 minute expiry")
		}
		if !strings.Contains(text, "sign in") {
			t.Error("text should mention sign in")
		}
	})

	t.Run("password reset email", func(t *testing.T) {
		html, text := svc.renderPasswordResetEmail("https://example.com/reset?token=ghi")

		if !strings.Contains(html, "Reset Password") {
			t.Error("HTML should contain reset button")
		}
		if !strings.Contains(html, "1 hour") {
			t.Error("HTML should mention 1 hour expiry")
		}
		if !strings.Contains(text, "reset your password") {
			t.Error("text should mention password reset")
		}
	})

	t.Run("support email", func(t *testing.T) {
		html, text := svc.renderSupportEmail("user@test.com", "Bug Report", "Something is broken", "user-123")

		if !strings.Contains(html, "Support Request") {
			t.Error("HTML should contain support request header")
		}
		if !strings.Contains(html, "user@test.com") {
			t.Error("HTML should contain from email")
		}
		if !strings.Contains(html, "Bug Report") {
			t.Error("HTML should contain category")
		}
		if !strings.Contains(html, "Something is broken") {
			t.Error("HTML should contain message")
		}
		if !strings.Contains(html, "user-123") {
			t.Error("HTML should contain user ID")
		}
		if !strings.Contains(text, "Bug Report") {
			t.Error("text should contain category")
		}
	})

	t.Run("support email without user ID", func(t *testing.T) {
		html, _ := svc.renderSupportEmail("anon@test.com", "Question", "How does this work?", "")

		if !strings.Contains(html, "Not logged in") {
			t.Error("HTML should show 'Not logged in' when user ID is empty")
		}
	})

	t.Run("support email XSS prevention", func(t *testing.T) {
		html, _ := svc.renderSupportEmail("test@test.com", "Test", "<script>alert('xss')</script>", "")

		if strings.Contains(html, "<script>") {
			t.Error("HTML should escape script tags to prevent XSS")
		}
		if !strings.Contains(html, "&lt;script&gt;") {
			t.Error("HTML should contain escaped script tag")
		}
	})
}
