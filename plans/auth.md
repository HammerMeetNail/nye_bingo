# Email Authentication Enhancement Plan

This plan enhances the existing authentication system with email verification and magic link authentication while maintaining a smooth user experience.

## Overview

**Goals:**
1. Email verification for email/password registration
2. Magic link authentication (passwordless option)
3. Password reset via email
4. Minimal disruption to existing user flow
5. High email deliverability (avoid spam folders)

**Current State:** ✓ FULLY IMPLEMENTED
- Email/password registration with email verification
- Magic link (passwordless) authentication
- Password reset via email
- Profile page with account management
- Sessions managed via Redis + PostgreSQL fallback
- Secure password hashing with bcrypt (cost 12)
- CSRF protection in place
- Resend email provider integration (production)
- Mailpit for local development email testing

---

## Email Service

### Selected: Resend

**Status:** Domain `yearofbingo.com` verified, API key obtained.

**Why Resend:**
- Modern, developer-focused API
- Excellent Go SDK: `github.com/resend/resend-go/v2`
- Free tier: 3,000 emails/month (generous for small apps)
- Good deliverability with proper DNS setup
- Simple dashboard and logging
- React Email templates (not needed for this project)

**Pricing:**
| Tier | Emails/Month | Cost |
|------|--------------|------|
| Free | 3,000 | $0 |
| Pro | 50,000 | $20/mo |
| Enterprise | 100,000+ | Custom |

**Alternative Options (for reference):**

| Service | Free Tier | Notes |
|---------|-----------|-------|
| **Postmark** | 100/mo | Best deliverability, expensive |
| **SendGrid** | 100/day | Good API, owned by Twilio |
| **Amazon SES** | 62K/mo (from EC2) | Cheapest at scale |
| **Mailgun** | 100/day (trial) | Good deliverability |

---

## DNS Configuration for Email Deliverability

All DNS changes made in Cloudflare dashboard for yearofbingo.com.

### Required DNS Records

#### 1. SPF Record (Sender Policy Framework)
Tells receiving servers which IPs can send email for your domain.

```
Type: TXT
Name: @
Value: v=spf1 include:_spf.resend.com ~all
```

**Note:** If you have existing SPF records, merge them:
```
v=spf1 include:_spf.resend.com include:other-service.com ~all
```

#### 2. DKIM Record (DomainKeys Identified Mail)
Cryptographic signature proving email authenticity. Resend provides these in your domain settings.

```
Type: TXT
Name: resend._domainkey
Value: [provided-by-resend-dashboard]
```

Resend typically provides 1-3 DKIM records - add all of them.

#### 3. DMARC Record (Domain-based Message Authentication)
Policy for handling authentication failures.

```
Type: TXT
Name: _dmarc
Value: v=DMARC1; p=none; rua=mailto:dmarc@yearofbingo.com; pct=100
```

**DMARC Policy Options:**
- `p=none` - Monitor only (start here, recommended)
- `p=quarantine` - Send to spam (after monitoring shows good results)
- `p=reject` - Reject outright (strictest, use after confidence built)

**Recommendation:** Start with `p=none` for a few weeks to monitor, then upgrade to `p=quarantine`.

### DNS Setup Checklist
- [x] Verify domain in Resend dashboard
- [ ] Add SPF record (`include:_spf.resend.com`)
- [ ] Add DKIM records (from Resend dashboard)
- [ ] Add DMARC record (start with `p=none`)
- [ ] Test with mail-tester.com (aim for 9+/10 score)

---

## Database Schema Changes

### New Migration: `000007_email_verification.up.sql`

```sql
-- Add email verification fields to users
ALTER TABLE users
ADD COLUMN email_verified BOOLEAN DEFAULT false,
ADD COLUMN email_verified_at TIMESTAMPTZ;

-- Email verification tokens
CREATE TABLE email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_email_verification_token_hash ON email_verification_tokens(token_hash);
CREATE INDEX idx_email_verification_user_id ON email_verification_tokens(user_id);

-- Magic link tokens (passwordless login)
CREATE TABLE magic_link_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_magic_link_token_hash ON magic_link_tokens(token_hash);
CREATE INDEX idx_magic_link_email ON magic_link_tokens(email);

-- Password reset tokens
CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_password_reset_token_hash ON password_reset_tokens(token_hash);
CREATE INDEX idx_password_reset_user_id ON password_reset_tokens(user_id);
```

### Down Migration: `000007_email_verification.down.sql`

```sql
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS magic_link_tokens;
DROP TABLE IF EXISTS email_verification_tokens;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
```

---

## Architecture

### New Components

```
internal/
├── services/
│   └── email.go          # Email sending service
├── models/
│   ├── verification.go   # VerificationToken model
│   ├── magic_link.go     # MagicLinkToken model
│   └── password_reset.go # PasswordResetToken model
└── handlers/
    └── auth.go           # Extended with new endpoints
```

### Token Security Model

All tokens follow the same security pattern as existing session tokens:
1. Generate 32 random bytes using `crypto/rand`
2. Store SHA256 hash in database (not raw token)
3. Send unhashed token to user via email
4. On verification, hash submitted token and compare

**Token Expiration:**
- Email verification: 24 hours
- Magic link: 15 minutes
- Password reset: 1 hour

---

## API Endpoints

### New Endpoints

```
# Email Verification
POST /api/auth/verify-email          - Verify email with token
POST /api/auth/resend-verification   - Resend verification email

# Magic Link Authentication
POST /api/auth/magic-link            - Request magic link email
GET  /api/auth/magic-link/verify     - Verify magic link token (redirects)

# Password Reset
POST /api/auth/forgot-password       - Request password reset email
POST /api/auth/reset-password        - Reset password with token
```

### Endpoint Specifications

#### POST /api/auth/magic-link
Request a magic link to be sent to email.

```json
// Request
{ "email": "user@example.com" }

// Response (always 200 to prevent email enumeration)
{ "message": "If an account exists, a login link has been sent" }
```

**Behavior:**
- If user exists: Send magic link email
- If user doesn't exist: Do nothing (but return same response)
- Rate limit: 3 requests per email per 15 minutes

#### GET /api/auth/magic-link/verify?token=xxx
Verify magic link and create session.

**Behavior:**
- Valid token → Create session, redirect to `/`
- Invalid/expired → Redirect to `/login?error=invalid_link`
- Already used → Redirect to `/login?error=link_used`

#### POST /api/auth/verify-email

```json
// Request
{ "token": "abc123..." }

// Response
{ "message": "Email verified successfully" }
```

#### POST /api/auth/forgot-password

```json
// Request
{ "email": "user@example.com" }

// Response (always 200)
{ "message": "If an account exists, reset instructions have been sent" }
```

#### POST /api/auth/reset-password

```json
// Request
{
  "token": "abc123...",
  "password": "newSecurePassword123"
}

// Response
{ "message": "Password reset successfully" }
```

---

## User Flows

### Flow 1: Email/Password Registration (Enhanced)

```
1. User enters email, password, display name
2. Account created with email_verified = false
3. Verification email sent automatically
4. User redirected to "Check your email" page
5. User clicks link in email
6. Email verified, user redirected to home
```

**Unverified User Restrictions:**
- Can log in and use the app normally
- Gentle reminder banner shown on homepage
- Cannot be found by friends (search excludes unverified)
- Reminder emails sent at 1 day, 3 days, 7 days

**Rationale:** Don't block users from using the app. Verification protects friend features and ensures email works for password reset.

### Flow 2: Magic Link Login

```
1. User clicks "Sign in with email link"
2. User enters email address
3. Server sends magic link email
4. User shown "Check your email" message
5. User clicks link in email
6. Session created, user redirected to home
```

**New User via Magic Link:**
- If email doesn't exist, prompt to create account
- Pre-fill email, ask for display name
- Mark email as verified (they clicked the link)

### Flow 3: Password Reset

```
1. User clicks "Forgot password?" on login
2. User enters email address
3. Server sends reset email (if account exists)
4. User clicks link in email
5. User enters new password
6. Password updated, all sessions invalidated
7. New session created, user logged in
```

---

## Email Templates

### Email Design Principles
- Plain text fallback for all emails
- Minimal HTML (avoid spam triggers)
- Clear, single call-to-action
- Mobile-friendly
- No tracking pixels initially

### Template: Email Verification

**Subject:** Verify your Year of Bingo account

```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h1 style="color: #333; font-size: 24px;">Welcome to Year of Bingo!</h1>

  <p>Please verify your email address by clicking the button below:</p>

  <a href="{{.VerifyURL}}"
     style="display: inline-block; background: #4F46E5; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; margin: 20px 0;">
    Verify Email Address
  </a>

  <p style="color: #666; font-size: 14px;">
    This link expires in 24 hours. If you didn't create an account, you can ignore this email.
  </p>

  <p style="color: #666; font-size: 14px;">
    Or copy this link: {{.VerifyURL}}
  </p>

  <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
  <p style="color: #999; font-size: 12px;">Year of Bingo - yearofbingo.com</p>
</body>
</html>
```

**Plain Text:**
```
Welcome to Year of Bingo!

Please verify your email address by visiting:
{{.VerifyURL}}

This link expires in 24 hours.

If you didn't create an account, you can ignore this email.

--
Year of Bingo
yearofbingo.com
```

### Template: Magic Link

**Subject:** Your Year of Bingo login link

```html
<!-- Similar structure, different content -->
<h1>Sign in to Year of Bingo</h1>
<p>Click the button below to sign in:</p>
<a href="{{.LoginURL}}">Sign In</a>
<p>This link expires in 15 minutes and can only be used once.</p>
```

### Template: Password Reset

**Subject:** Reset your Year of Bingo password

```html
<h1>Reset Your Password</h1>
<p>We received a request to reset your password. Click below to choose a new password:</p>
<a href="{{.ResetURL}}">Reset Password</a>
<p>This link expires in 1 hour. If you didn't request this, you can ignore this email.</p>
```

---

## Frontend Changes

### New Routes (Hash-based)

```
#login                    - Login form (email/password OR magic link)
#login?error=xxx          - Login with error message
#register                 - Registration form
#verify-email?token=xxx   - Email verification landing
#forgot-password          - Request password reset
#reset-password?token=xxx - Enter new password
#check-email?type=xxx     - "Check your email" interstitial
```

### UI Components

#### Enhanced Login Page
```
┌─────────────────────────────────────┐
│         Sign in to Year of Bingo    │
├─────────────────────────────────────┤
│  [Email                          ]  │
│  [Password                       ]  │
│                                     │
│  [        Sign In              ]    │
│                                     │
│  Forgot password?                   │
│                                     │
├────────── OR ───────────────────────┤
│                                     │
│  [  Sign in with email link    ]    │
│                                     │
│  Don't have an account? Register    │
└─────────────────────────────────────┘
```

#### Magic Link Request
```
┌─────────────────────────────────────┐
│       Sign in with email link       │
├─────────────────────────────────────┤
│  [Email                          ]  │
│                                     │
│  [     Send login link         ]    │
│                                     │
│  Back to sign in                    │
└─────────────────────────────────────┘
```

#### Check Email Interstitial
```
┌─────────────────────────────────────┐
│              ✉️                      │
│         Check your email            │
├─────────────────────────────────────┤
│  We sent a login link to            │
│  user@example.com                   │
│                                     │
│  Click the link to sign in.         │
│  The link expires in 15 minutes.    │
│                                     │
│  [Resend email]   [Back to login]   │
└─────────────────────────────────────┘
```

#### Unverified Email Banner
Shown on home page when logged in but email not verified:
```
┌─────────────────────────────────────────────────────────────┐
│ ⚠️ Please verify your email to enable all features.         │
│ [Resend verification email]                          [×]    │
└─────────────────────────────────────────────────────────────┘
```

---

## Implementation Phases

### Phase 9.5: Email Service Setup ✓ COMPLETE

**Deliverables:**
- [x] Choose email provider (Resend selected)
- [x] Set up account and verify domain
- [x] Configure DNS records (SPF, DKIM, DMARC)
- [x] Test email deliverability with mail-tester.com
- [x] Add environment variables for email config
- [x] Add RESEND_API_KEY to production secrets

**Environment Variables:**
```bash
# Email Service (Resend)
EMAIL_PROVIDER=resend
RESEND_API_KEY=re_xxxxxxxxxxxx
EMAIL_FROM_ADDRESS=noreply@yearofbingo.com
EMAIL_FROM_NAME="Year of Bingo"

# Application URLs
APP_BASE_URL=https://yearofbingo.com
```

### Phase 9.6: Backend Email Infrastructure ✓ COMPLETE

**Deliverables:**
- [x] Create `internal/services/email.go` with Resend integration
- [x] Email template system (HTML + plain text)
- [x] Rate limiting for email sends (via Redis)
- [x] Database migrations for token tables
- [x] Token generation and validation utilities

**Email Service Interface:**
```go
type EmailService interface {
    SendVerificationEmail(ctx context.Context, user *models.User, token string) error
    SendMagicLinkEmail(ctx context.Context, email string, token string) error
    SendPasswordResetEmail(ctx context.Context, user *models.User, token string) error
}
```

### Phase 9.7: Email Verification ✓ COMPLETE

**Deliverables:**
- [x] Modify registration to set `email_verified = false`
- [x] Send verification email on registration
- [x] `POST /api/auth/verify-email` endpoint
- [x] `POST /api/auth/resend-verification` endpoint
- [x] Update User model with verification fields
- [x] Exclude unverified users from friend search
- [x] Frontend verification flow and banner

### Phase 9.8: Magic Link Authentication ✓ COMPLETE

**Deliverables:**
- [x] `POST /api/auth/magic-link` endpoint
- [x] `GET /api/auth/magic-link/verify` endpoint
- [x] Handle new user registration via magic link
- [x] Frontend magic link request form
- [x] "Check your email" interstitial page
- [x] Token cleanup job (expired tokens)

### Phase 9.9: Password Reset ✓ COMPLETE

**Deliverables:**
- [x] `POST /api/auth/forgot-password` endpoint
- [x] `POST /api/auth/reset-password` endpoint
- [x] Frontend forgot password flow
- [x] Frontend reset password form
- [x] Invalidate all sessions on password reset

### Phase 9.10: Existing User Migration ✓ COMPLETE

**Strategy:** Gradual verification, no forced migration.

**Deliverables:**
- [x] Migration marks all existing users as `email_verified = true`
- [x] (Trust existing users who successfully registered)
- [x] Show banner for unverified users until they verify
- [x] Monitoring for unverified user count via dashboard/profile

**Implemented Approach:** Existing users are marked as verified. New registrations start unverified with gentle banner reminders.

### Phase 9.11: Profile Page ✓ COMPLETE

**Deliverables:**
- [x] Profile route (`#profile`) accessible from username in nav
- [x] Email verification status display with badge
- [x] Alert banner for unverified emails with resend option
- [x] Profile information display (display name, email, member since)
- [x] Change password form with validation
- [x] Sign out button

---

## Local Development

### Email Testing Strategy

Local development needs a way to test email flows without sending real emails. Three approaches supported:

#### Option 1: Mailpit (Recommended for Local Dev)

[Mailpit](https://github.com/axllent/mailpit) is a lightweight email testing tool with a web UI. All emails are captured locally and viewable in browser.

**Add to `compose.yaml`:**
```yaml
services:
  mailpit:
    image: axllent/mailpit:latest
    container_name: yearofbingo-mailpit
    ports:
      - "8025:8025"  # Web UI
      - "1025:1025"  # SMTP
    environment:
      MP_SMTP_AUTH_ACCEPT_ANY: 1
      MP_SMTP_AUTH_ALLOW_INSECURE: 1
```

**Environment variables for local dev:**
```bash
EMAIL_PROVIDER=smtp
SMTP_HOST=mailpit
SMTP_PORT=1025
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_TLS=false
APP_BASE_URL=http://localhost:8080
```

**Access:** Open http://localhost:8025 to view captured emails.

#### Option 2: Console Logger (Simplest)

Log email content to stdout instead of sending. Useful for quick testing or CI.

```bash
EMAIL_PROVIDER=console
APP_BASE_URL=http://localhost:8080
```

**Output example:**
```
=== EMAIL ===
To: user@example.com
Subject: Verify your Year of Bingo account
---
Click to verify: http://localhost:8080/verify-email?token=abc123...
=============
```

The token/link is printed directly to container logs for easy copy-paste.

#### Option 3: File Logger

Write emails to files for inspection.

```bash
EMAIL_PROVIDER=file
EMAIL_FILE_PATH=/tmp/emails
APP_BASE_URL=http://localhost:8080
```

### Email Service Implementation

The email service should support multiple providers via interface:

```go
// internal/services/email.go

type EmailProvider interface {
    Send(ctx context.Context, email *Email) error
}

type EmailService struct {
    provider EmailProvider
    baseURL  string
}

func NewEmailService(cfg *config.Config) *EmailService {
    var provider EmailProvider

    switch cfg.EmailProvider {
    case "resend":
        provider = NewResendProvider(cfg.ResendAPIKey)
    case "smtp":
        provider = NewSMTPProvider(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass)
    case "console":
        provider = NewConsoleProvider()
    case "file":
        provider = NewFileProvider(cfg.EmailFilePath)
    default:
        provider = NewConsoleProvider() // Safe default for dev
    }

    return &EmailService{provider: provider, baseURL: cfg.AppBaseURL}
}
```

### Development Convenience Features

#### Auto-Verify in Development

For faster iteration, optionally auto-verify emails in development:

```bash
# .env.development
AUTO_VERIFY_EMAIL=true
```

When enabled:
- Registration still creates unverified user
- Verification email still sent (to Mailpit)
- User is immediately marked as verified
- Allows testing both verified and unverified flows

#### Extended Token Expiration

Longer token expiration for local dev (easier debugging):

```bash
# .env.development
MAGIC_LINK_EXPIRY=60m      # 60 minutes instead of 15
VERIFICATION_EXPIRY=7d      # 7 days instead of 24h
PASSWORD_RESET_EXPIRY=24h   # 24 hours instead of 1h
```

#### Skip Rate Limiting

Disable rate limiting in development:

```bash
# .env.development
RATE_LIMIT_ENABLED=false
```

### Local Development Workflow

#### Testing Email Verification

```bash
# 1. Start services with Mailpit
podman compose up

# 2. Register new user via UI or API
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test123!","display_name":"Test"}'

# 3. Open Mailpit UI
open http://localhost:8025

# 4. Click verification link from email (or copy token)

# 5. Alternatively, get token from console logs if using console provider
podman logs yearofbingo-app | grep "verify-email"
```

#### Testing Magic Links

```bash
# 1. Request magic link
curl -X POST http://localhost:8080/api/auth/magic-link \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com"}'

# 2. Check Mailpit or console logs for link

# 3. Visit link or call verify endpoint
open "http://localhost:8080/magic-link?token=<token>"
```

#### Testing Password Reset

```bash
# 1. Request reset
curl -X POST http://localhost:8080/api/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com"}'

# 2. Get reset link from Mailpit

# 3. Complete reset via UI or API
curl -X POST http://localhost:8080/api/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{"token":"<token>","password":"NewPass123!"}'
```

### Updated compose.yaml for Development

```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      # ... existing env vars ...
      EMAIL_PROVIDER: smtp
      SMTP_HOST: mailpit
      SMTP_PORT: 1025
      APP_BASE_URL: http://localhost:8080
      AUTO_VERIFY_EMAIL: false
      RATE_LIMIT_ENABLED: false
    depends_on:
      - postgres
      - redis
      - mailpit

  postgres:
    # ... existing config ...

  redis:
    # ... existing config ...

  mailpit:
    image: axllent/mailpit:latest
    container_name: yearofbingo-mailpit
    ports:
      - "8025:8025"
      - "1025:1025"
    environment:
      MP_SMTP_AUTH_ACCEPT_ANY: 1
      MP_SMTP_AUTH_ALLOW_INSECURE: 1
```

### Environment Configuration Summary

| Variable | Development | Production |
|----------|-------------|------------|
| `EMAIL_PROVIDER` | `smtp` or `console` | `resend` |
| `SMTP_HOST` | `mailpit` | - |
| `SMTP_PORT` | `1025` | - |
| `RESEND_API_KEY` | - | `re_xxxx` (from GitHub Secret) |
| `APP_BASE_URL` | `http://localhost:8080` | `https://yearofbingo.com` |
| `AUTO_VERIFY_EMAIL` | `true` (optional) | `false` |
| `RATE_LIMIT_ENABLED` | `false` | `true` |
| `MAGIC_LINK_EXPIRY` | `60m` | `15m` |
| `VERIFICATION_EXPIRY` | `7d` | `24h` |
| `PASSWORD_RESET_EXPIRY` | `24h` | `1h` |

### CI/CD Considerations

For automated tests in CI:

```yaml
# .github/workflows/ci.yaml
env:
  EMAIL_PROVIDER: console
  APP_BASE_URL: http://localhost:8080
  AUTO_VERIFY_EMAIL: true
  RATE_LIMIT_ENABLED: false
```

Integration tests can:
1. Parse console output to extract tokens
2. Or query the database directly for token hashes in test mode
3. Or use a test helper that returns unhashed tokens

```go
// internal/testutil/email.go
type TestEmailCapture struct {
    Emails []CapturedEmail
    mu     sync.Mutex
}

func (c *TestEmailCapture) Send(ctx context.Context, email *Email) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.Emails = append(c.Emails, CapturedEmail{
        To:      email.To,
        Subject: email.Subject,
        Body:    email.Body,
        // Extract token from body for easy test access
        Token:   extractToken(email.Body),
    })
    return nil
}
```

---

## Security Considerations

### Rate Limiting

| Endpoint | Limit | Window |
|----------|-------|--------|
| `/api/auth/magic-link` | 3 requests | 15 minutes per email |
| `/api/auth/forgot-password` | 3 requests | 15 minutes per email |
| `/api/auth/resend-verification` | 3 requests | 15 minutes per user |
| `/api/auth/verify-email` | 10 attempts | 15 minutes per IP |
| `/api/auth/reset-password` | 5 attempts | 15 minutes per IP |

### Token Security

- All tokens: 32 bytes from `crypto/rand`
- SHA256 hash stored in database
- Single-use for magic links and password resets
- Verification tokens can be reused until verified

### Enumeration Prevention

- Magic link and forgot password always return success
- Same response time for existing vs non-existing emails
- No user existence leakage in error messages

### Email Security

- Links use HTTPS only
- Tokens include user-specific context where applicable
- Short expiration times minimize attack window
- Single-use tokens for sensitive operations

---

## Testing Plan

### Unit Tests

- [ ] Token generation and hashing
- [ ] Token validation (valid, expired, invalid hash)
- [ ] Email template rendering
- [ ] Rate limiting logic

### Integration Tests

- [ ] Full registration → verification flow
- [ ] Magic link login flow (existing user)
- [ ] Magic link registration flow (new user)
- [ ] Password reset flow
- [ ] Expired token handling
- [ ] Rate limit enforcement

### Manual Testing

- [ ] Email deliverability to Gmail, Outlook, Yahoo
- [ ] Check spam scores with mail-tester.com
- [ ] Mobile email client rendering
- [ ] Link expiration behavior
- [ ] Error message clarity

---

## Rollout Plan

### Stage 1: DNS & Email Setup (Before code deployment)
1. Configure DNS records
2. Wait 24-48 hours for propagation
3. Verify with mail-tester.com

### Stage 2: Deploy Backend Changes
1. Run database migrations
2. Deploy with email sending enabled
3. Monitor email delivery rates

### Stage 3: Enable Verification for New Users
1. New registrations require verification
2. Existing users unaffected (marked verified)
3. Monitor for issues

### Stage 4: Enable Magic Link Option
1. Add magic link UI option
2. A/B test adoption if desired
3. Monitor usage patterns

### Stage 5: Monitor & Iterate
1. Track verification completion rates
2. Monitor email bounce rates
3. Adjust reminder email timing
4. Improve templates based on engagement

---

## Monitoring & Alerts

### Key Metrics

- Email send success rate
- Email bounce rate
- Verification completion rate
- Magic link usage rate
- Time to verify (registration → verification)
- Password reset completion rate

### Alerts

- Email service API errors > 1% of requests
- Bounce rate > 5%
- Verification completion rate < 50%
- Unusual spike in password reset requests

---

## Cost Estimate

### Resend Pricing

| Monthly Users | Emails/Month | Resend Cost |
|---------------|--------------|-------------|
| 0-500 | ~1,000 | Free (3,000/mo included) |
| 500-1,500 | ~3,000 | Free |
| 1,500-5,000 | ~10,000 | $20/month (Pro tier) |
| 5,000-20,000 | ~40,000 | $20/month |

**Email types per user:**
- Verification: 1-2 emails
- Password reset: 0-1 emails/year
- Magic link: 0-5 emails/month (if used)

**Note:** The free tier (3,000 emails/month) should cover Year of Bingo well into the thousands of users.

---

## Open Questions

1. **Existing User Strategy:** Mark all as verified (recommended) or require re-verification?
2. **Verification Requirement Level:** Block features until verified, or just show banner?
3. **Magic Link Default:** Make magic link the primary option, or keep password as default?

---

## Appendix A: Resend Setup (Completed)

1. ~~Create Resend account at resend.com~~ ✓
2. ~~Add sending domain: yearofbingo.com~~ ✓
3. ~~Verify domain ownership~~ ✓
4. Add DNS records in Cloudflare (SPF, DKIM, DMARC)
5. ~~Get API key from dashboard~~ ✓
6. Add API key to GitHub Secrets
7. Send test email via API
8. Check deliverability with mail-tester.com

### Go Client Setup

```go
import "github.com/resend/resend-go/v2"

client := resend.NewClient(apiKey)

params := &resend.SendEmailRequest{
    From:    "Year of Bingo <noreply@yearofbingo.com>",
    To:      []string{user.Email},
    Subject: "Verify your Year of Bingo account",
    Html:    htmlContent,
    Text:    textContent,
    Tags: []resend.Tag{
        {Name: "category", Value: "verification"},
    },
}

sent, err := client.Emails.Send(params)
```

---

## Appendix B: Production Secrets Management

### Recommended: GitHub Secrets + SSH Deployment

This approach keeps secrets in GitHub and injects them during deployment.

#### Step 1: Add Secrets to GitHub

In your repository: **Settings → Secrets and variables → Actions**

**Already configured:**
- `RESEND_API_KEY` - Your Resend API key ✓
- `SSH_PRIVATE_KEY` - SSH private key for Hetzner server ✓
- `SERVER_HOST` - Hetzner server IP or hostname ✓

**Optional (can hardcode in workflow instead):**
- `DEPLOY_USER` - SSH username, or just hardcode `root` or `deploy` in the workflow

#### Step 2: Deployment Workflow

Add a deploy job to `.github/workflows/ci.yaml`:

```yaml
deploy:
  name: Deploy to Production
  runs-on: ubuntu-latest
  needs: [scan-and-push]  # Run after image is pushed
  if: github.ref == 'refs/heads/main'

  steps:
    - name: Deploy to Hetzner
      uses: appleboy/ssh-action@v1.0.3
      with:
        host: ${{ secrets.SERVER_HOST }}
        username: root  # or use a secret if you prefer
        key: ${{ secrets.SSH_PRIVATE_KEY }}
        script: |
          cd /opt/yearofbingo

          # Update environment file with secrets
          cat > .env.production << 'EOF'
          EMAIL_PROVIDER=resend
          RESEND_API_KEY=${{ secrets.RESEND_API_KEY }}
          EMAIL_FROM_ADDRESS=noreply@yearofbingo.com
          APP_BASE_URL=https://yearofbingo.com
          SERVER_SECURE=true
          # ... other production env vars
          EOF

          # Pull latest image and restart
          podman pull quay.io/yearofbingo/yearofbingo:latest
          podman compose -f compose.prod.yaml down
          podman compose -f compose.prod.yaml up -d
```

#### Step 3: Server Setup (One-time)

On your Hetzner server:

```bash
# Create deployment directory
mkdir -p /opt/yearofbingo
cd /opt/yearofbingo

# Create compose.prod.yaml (or copy from repo)
# This file references .env.production for secrets

# Create initial .env.production (will be overwritten by CI)
touch .env.production
chmod 600 .env.production
```

#### compose.prod.yaml Example

```yaml
services:
  app:
    image: quay.io/yearofbingo/yearofbingo:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    env_file:
      - .env.production
    environment:
      # Non-secret config can go here
      SERVER_HOST: "0.0.0.0"
      SERVER_PORT: "8080"
      DB_HOST: postgres
      REDIS_HOST: redis
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    volumes:
      - postgres_data:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: yearofbingo
      POSTGRES_USER: yearofbingo
      POSTGRES_PASSWORD: ${DB_PASSWORD}

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

### Alternative: Environment File on Server

If you prefer not to pass secrets through GitHub Actions:

1. **Manually create `.env.production` on server** with all secrets
2. **CI only pulls image and restarts** without touching env file
3. **Pros:** Secrets never transit through GitHub
4. **Cons:** Manual secret rotation, harder to audit

```yaml
# Simpler deploy job (secrets already on server)
deploy:
  steps:
    - name: Deploy
      uses: appleboy/ssh-action@v1.0.3
      with:
        host: ${{ secrets.SERVER_HOST }}
        username: root
        key: ${{ secrets.SSH_PRIVATE_KEY }}
        script: |
          cd /opt/yearofbingo
          podman pull quay.io/yearofbingo/yearofbingo:latest
          podman compose -f compose.prod.yaml down
          podman compose -f compose.prod.yaml up -d
```

### Security Best Practices

1. **Least privilege:** Create a dedicated `deploy` user on Hetzner with limited sudo
2. **Key rotation:** Rotate SSH keys and API keys periodically
3. **Audit logging:** GitHub Actions logs show who triggered deployments
4. **Branch protection:** Require PR reviews before merging to main
5. **Secret scanning:** Enable GitHub secret scanning to detect leaked keys
6. **Env file permissions:** `chmod 600 .env.production` on server

### Quick Setup Checklist

- [x] Add `RESEND_API_KEY` to GitHub Secrets
- [x] `SSH_PRIVATE_KEY` already configured
- [x] `SERVER_HOST` already configured
- [ ] Create `/opt/yearofbingo` on Hetzner server (if not exists)
- [ ] Create `compose.prod.yaml` on server
- [ ] Add deploy job to CI workflow
- [ ] Test deployment with a PR merge to main
