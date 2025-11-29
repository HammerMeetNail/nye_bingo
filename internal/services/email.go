package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/smtp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/resend/resend-go/v2"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/logging"
)

// Token expiration durations
const (
	VerificationTokenExpiry  = 24 * time.Hour
	MagicLinkTokenExpiry     = 15 * time.Minute
	PasswordResetTokenExpiry = 1 * time.Hour
)

// Email represents an email to be sent
type Email struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// EmailProvider is the interface for sending emails
type EmailProvider interface {
	Send(ctx context.Context, email *Email) error
}

// EmailService handles all email-related operations
type EmailService struct {
	provider    EmailProvider
	db          *pgxpool.Pool
	fromAddress string
	fromName    string
	baseURL     string
}

// NewEmailService creates a new email service based on configuration
func NewEmailService(cfg *config.EmailConfig, db *pgxpool.Pool) *EmailService {
	var provider EmailProvider

	switch cfg.Provider {
	case "resend":
		provider = NewResendProvider(cfg.ResendAPIKey)
	case "smtp":
		provider = NewSMTPProvider(cfg.SMTPHost, cfg.SMTPPort)
	default:
		provider = NewConsoleProvider()
	}

	return &EmailService{
		provider:    provider,
		db:          db,
		fromAddress: cfg.FromAddress,
		fromName:    cfg.FromName,
		baseURL:     cfg.BaseURL,
	}
}

// GenerateToken creates a secure random token and returns both the token and its hash
func GenerateToken() (token string, hash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generating random bytes: %w", err)
	}
	token = hex.EncodeToString(bytes)
	hash = HashToken(token)
	return token, hash, nil
}

// HashToken creates a SHA256 hash of a token
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// SendVerificationEmail sends an email verification link
func (s *EmailService) SendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error {
	token, tokenHash, err := GenerateToken()
	if err != nil {
		return err
	}

	// Store token in database
	expiresAt := time.Now().Add(VerificationTokenExpiry)
	_, err = s.db.Exec(ctx,
		`INSERT INTO email_verification_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("storing verification token: %w", err)
	}

	verifyURL := fmt.Sprintf("%s/#verify-email?token=%s", s.baseURL, token)

	html, text := s.renderVerificationEmail(verifyURL)

	return s.provider.Send(ctx, &Email{
		To:      email,
		Subject: "Verify your Year of Bingo account",
		HTML:    html,
		Text:    text,
	})
}

// VerifyEmail verifies an email using a token
func (s *EmailService) VerifyEmail(ctx context.Context, token string) error {
	tokenHash := HashToken(token)

	// Find and validate token
	var userID uuid.UUID
	var expiresAt time.Time
	err := s.db.QueryRow(ctx,
		`SELECT user_id, expires_at FROM email_verification_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&userID, &expiresAt)
	if err != nil {
		return fmt.Errorf("invalid verification token")
	}

	if time.Now().After(expiresAt) {
		return fmt.Errorf("verification token has expired")
	}

	// Mark user as verified
	_, err = s.db.Exec(ctx,
		`UPDATE users SET email_verified = true, email_verified_at = NOW() WHERE id = $1`,
		userID)
	if err != nil {
		return fmt.Errorf("updating user verification status: %w", err)
	}

	// Delete all verification tokens for this user
	_, err = s.db.Exec(ctx,
		`DELETE FROM email_verification_tokens WHERE user_id = $1`,
		userID)
	if err != nil {
		logging.Error("Failed to delete verification tokens", map[string]interface{}{"error": err.Error(), "user_id": userID.String()})
	}

	return nil
}

// SendMagicLinkEmail sends a magic link for passwordless login
func (s *EmailService) SendMagicLinkEmail(ctx context.Context, email string) error {
	token, tokenHash, err := GenerateToken()
	if err != nil {
		return err
	}

	// Store token in database
	expiresAt := time.Now().Add(MagicLinkTokenExpiry)
	_, err = s.db.Exec(ctx,
		`INSERT INTO magic_link_tokens (email, token_hash, expires_at) VALUES ($1, $2, $3)`,
		email, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("storing magic link token: %w", err)
	}

	loginURL := fmt.Sprintf("%s/#magic-link?token=%s", s.baseURL, token)

	html, text := s.renderMagicLinkEmail(loginURL)

	return s.provider.Send(ctx, &Email{
		To:      email,
		Subject: "Your Year of Bingo login link",
		HTML:    html,
		Text:    text,
	})
}

// VerifyMagicLink verifies a magic link token and returns the email
func (s *EmailService) VerifyMagicLink(ctx context.Context, token string) (string, error) {
	tokenHash := HashToken(token)

	// Find and validate token
	var id uuid.UUID
	var email string
	var expiresAt time.Time
	var usedAt *time.Time
	err := s.db.QueryRow(ctx,
		`SELECT id, email, expires_at, used_at FROM magic_link_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&id, &email, &expiresAt, &usedAt)
	if err != nil {
		return "", fmt.Errorf("invalid magic link")
	}

	if usedAt != nil {
		return "", fmt.Errorf("magic link has already been used")
	}

	if time.Now().After(expiresAt) {
		return "", fmt.Errorf("magic link has expired")
	}

	// Mark token as used
	_, err = s.db.Exec(ctx,
		`UPDATE magic_link_tokens SET used_at = NOW() WHERE id = $1`,
		id)
	if err != nil {
		logging.Error("Failed to mark magic link as used", map[string]interface{}{"error": err.Error(), "id": id.String()})
	}

	return email, nil
}

// SendPasswordResetEmail sends a password reset link
func (s *EmailService) SendPasswordResetEmail(ctx context.Context, userID uuid.UUID, email string) error {
	token, tokenHash, err := GenerateToken()
	if err != nil {
		return err
	}

	// Store token in database
	expiresAt := time.Now().Add(PasswordResetTokenExpiry)
	_, err = s.db.Exec(ctx,
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("storing password reset token: %w", err)
	}

	resetURL := fmt.Sprintf("%s/#reset-password?token=%s", s.baseURL, token)

	html, text := s.renderPasswordResetEmail(resetURL)

	return s.provider.Send(ctx, &Email{
		To:      email,
		Subject: "Reset your Year of Bingo password",
		HTML:    html,
		Text:    text,
	})
}

// VerifyPasswordResetToken verifies a password reset token and returns the user ID
func (s *EmailService) VerifyPasswordResetToken(ctx context.Context, token string) (uuid.UUID, error) {
	tokenHash := HashToken(token)

	// Find and validate token
	var id uuid.UUID
	var userID uuid.UUID
	var expiresAt time.Time
	var usedAt *time.Time
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, expires_at, used_at FROM password_reset_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&id, &userID, &expiresAt, &usedAt)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid reset token")
	}

	if usedAt != nil {
		return uuid.Nil, fmt.Errorf("reset token has already been used")
	}

	if time.Now().After(expiresAt) {
		return uuid.Nil, fmt.Errorf("reset token has expired")
	}

	return userID, nil
}

// MarkPasswordResetUsed marks a password reset token as used
func (s *EmailService) MarkPasswordResetUsed(ctx context.Context, token string) error {
	tokenHash := HashToken(token)
	_, err := s.db.Exec(ctx,
		`UPDATE password_reset_tokens SET used_at = NOW() WHERE token_hash = $1`,
		tokenHash)
	return err
}

// Email templates

func (s *EmailService) renderVerificationEmail(verifyURL string) (html, text string) {
	html = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h1 style="color: #333; font-size: 24px;">Welcome to Year of Bingo!</h1>

  <p>Please verify your email address by clicking the button below:</p>

  <a href="%s"
     style="display: inline-block; background: #4F46E5; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; margin: 20px 0;">
    Verify Email Address
  </a>

  <p style="color: #666; font-size: 14px;">
    This link expires in 24 hours. If you didn't create an account, you can ignore this email.
  </p>

  <p style="color: #666; font-size: 14px;">
    Or copy this link: %s
  </p>

  <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
  <p style="color: #999; font-size: 12px;">Year of Bingo - yearofbingo.com</p>
</body>
</html>`, verifyURL, verifyURL)

	text = fmt.Sprintf(`Welcome to Year of Bingo!

Please verify your email address by visiting:
%s

This link expires in 24 hours.

If you didn't create an account, you can ignore this email.

--
Year of Bingo
yearofbingo.com`, verifyURL)

	return html, text
}

func (s *EmailService) renderMagicLinkEmail(loginURL string) (html, text string) {
	html = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h1 style="color: #333; font-size: 24px;">Sign in to Year of Bingo</h1>

  <p>Click the button below to sign in to your account:</p>

  <a href="%s"
     style="display: inline-block; background: #4F46E5; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; margin: 20px 0;">
    Sign In
  </a>

  <p style="color: #666; font-size: 14px;">
    This link expires in 15 minutes and can only be used once.
  </p>

  <p style="color: #666; font-size: 14px;">
    Or copy this link: %s
  </p>

  <p style="color: #666; font-size: 14px;">
    If you didn't request this link, you can safely ignore this email.
  </p>

  <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
  <p style="color: #999; font-size: 12px;">Year of Bingo - yearofbingo.com</p>
</body>
</html>`, loginURL, loginURL)

	text = fmt.Sprintf(`Sign in to Year of Bingo

Click the link below to sign in:
%s

This link expires in 15 minutes and can only be used once.

If you didn't request this link, you can safely ignore this email.

--
Year of Bingo
yearofbingo.com`, loginURL)

	return html, text
}

func (s *EmailService) renderPasswordResetEmail(resetURL string) (html, text string) {
	html = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h1 style="color: #333; font-size: 24px;">Reset Your Password</h1>

  <p>We received a request to reset your password. Click the button below to choose a new password:</p>

  <a href="%s"
     style="display: inline-block; background: #4F46E5; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; margin: 20px 0;">
    Reset Password
  </a>

  <p style="color: #666; font-size: 14px;">
    This link expires in 1 hour and can only be used once.
  </p>

  <p style="color: #666; font-size: 14px;">
    Or copy this link: %s
  </p>

  <p style="color: #666; font-size: 14px;">
    If you didn't request a password reset, you can safely ignore this email.
  </p>

  <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
  <p style="color: #999; font-size: 12px;">Year of Bingo - yearofbingo.com</p>
</body>
</html>`, resetURL, resetURL)

	text = fmt.Sprintf(`Reset Your Password

We received a request to reset your password.

Click the link below to choose a new password:
%s

This link expires in 1 hour and can only be used once.

If you didn't request a password reset, you can safely ignore this email.

--
Year of Bingo
yearofbingo.com`, resetURL)

	return html, text
}

// ResendProvider sends emails using the Resend API
type ResendProvider struct {
	client *resend.Client
}

func NewResendProvider(apiKey string) *ResendProvider {
	return &ResendProvider{
		client: resend.NewClient(apiKey),
	}
}

func (p *ResendProvider) Send(ctx context.Context, email *Email) error {
	params := &resend.SendEmailRequest{
		From:    "Year of Bingo <noreply@yearofbingo.com>",
		To:      []string{email.To},
		Subject: email.Subject,
		Html:    email.HTML,
		Text:    email.Text,
	}

	_, err := p.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("sending email via Resend: %w", err)
	}

	logging.Info("Email sent via Resend", map[string]interface{}{"to": email.To, "subject": email.Subject})
	return nil
}

// SMTPProvider sends emails via SMTP (for Mailpit in local dev)
type SMTPProvider struct {
	host string
	port int
}

func NewSMTPProvider(host string, port int) *SMTPProvider {
	return &SMTPProvider{host: host, port: port}
}

func (p *SMTPProvider) Send(ctx context.Context, email *Email) error {
	addr := fmt.Sprintf("%s:%d", p.host, p.port)

	// Build email message
	var buf bytes.Buffer
	buf.WriteString("From: Year of Bingo <noreply@yearofbingo.com>\r\n")
	buf.WriteString(fmt.Sprintf("To: %s\r\n", email.To))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(email.HTML)

	err := smtp.SendMail(addr, nil, "noreply@yearofbingo.com", []string{email.To}, buf.Bytes())
	if err != nil {
		return fmt.Errorf("sending email via SMTP: %w", err)
	}

	logging.Info("Email sent via SMTP", map[string]interface{}{"to": email.To, "subject": email.Subject})
	return nil
}

// ConsoleProvider logs emails to console (for development)
type ConsoleProvider struct{}

func NewConsoleProvider() *ConsoleProvider {
	return &ConsoleProvider{}
}

func (p *ConsoleProvider) Send(ctx context.Context, email *Email) error {
	logging.Info("=== EMAIL (Console Provider) ===", map[string]interface{}{"to": email.To, "subject": email.Subject})
	fmt.Printf("\n=== EMAIL ===\n")
	fmt.Printf("To: %s\n", email.To)
	fmt.Printf("Subject: %s\n", email.Subject)
	fmt.Printf("---\n")
	fmt.Printf("%s\n", email.Text)
	fmt.Printf("=============\n\n")
	return nil
}

// Suppress unused import warning for template package
var _ = template.New
