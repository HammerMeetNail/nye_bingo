# Stripe Billing + Premium (Additive) Plan

## Overview

**Goal:** Add Stripe-based billing and an entitlement system so we can ship **premium features that are 100% additive** (no existing functionality is removed, restricted, degraded, or paywalled).

**Non-negotiables:**
- **No regression / no paywalls**: all existing features continue working for free users exactly as they do today.
- **Security-first**: Stripe signature verification, least-privilege data flow, no client-trusted pricing, no secret leakage.
- **TDD-first**: new billing logic is covered by unit tests; webhook handling is fully testable and idempotent.
- **Positive UX**: clear value, easy upgrade/cancel, no dark patterns, no spammy nags.

## Current Status (Implemented Foundation)

As of **January 31, 2026**, the billing + entitlement **foundation is implemented** behind `BILLING_ENABLED` (default `false`).

**Implemented (code-as-built):**
- DB: `migrations/000021_billing_stripe.up.sql` / `.down.sql` (user billing fields + `stripe_webhook_events` + `premium_codes`).
- Config: `internal/config/config.go` (+ tests) and `.env.example` (Stripe env vars; production startup validation when `BILLING_ENABLED=true`).
- CSRF: explicit exemption for `POST /api/billing/webhook` in `internal/middleware/csrf.go` (+ tests).
- Billing service: `internal/services/billing/*` including:
  - entitlement model + premium checks:
    - tier signal: `billing.IsPremium(user, now)`
    - feature gates: `billing.HasFeature(user, now, feature)`
    - currently shipped feature flags include `templates` and `edit_after_finalize`
  - webhook signature verification (HMAC) + idempotency via `stripe_webhook_events`
  - premium code redemption (hashed codes, one-time, transactional)
- Stripe integration: implemented via a small Stripe **HTTP API client** (`internal/services/billing/stripe_client.go`) rather than `stripe-go` (no SDK dependency).
  - supports multiple line items in a single Checkout Session (e.g., subscription + tip)
- Handlers/routes: `internal/handlers/billing.go` + `cmd/server/main.go`:
  - `GET /api/billing/status`
    - now includes `source` (`none|stripe_subscription|stripe_lifetime|code|grant`) so the UI can display accurate renewal/expiry messaging
  - `POST /api/billing/checkout` (combined checkout; supports Premium purchase optionally combined with a tip)
  - `POST /api/billing/checkout/subscription` (`interval: month|year`)
  - `POST /api/billing/checkout/lifetime`
  - `POST /api/billing/checkout/tip` (`amount: 5|10|20`)
  - `POST /api/billing/portal`
  - `POST /api/billing/redeem` (rate-limited `10/hour` per-user; fail-closed)
  - `POST /api/billing/webhook` (CSRF-exempt; signature-verified)
  - `POST /api/cards/{id}/edit` (premium-gated finalized-card edit draft flow; server-side entitlement enforced)
- Frontend: `web/static/js/api.js`, `web/static/js/app.js`, `web/static/css/styles.css`:
  - Navbar shows a visible **Premium** entry point (star + label) that links to `/premium`
  - `/premium` page:
    - upgrade CTA + post-checkout polling (`?billing=success`)
    - "Have a code?" redeem flow lives in a modal; logged-out users can enter a code and are prompted to sign in/create an account before the code is redeemed
  - Upgrade modal supports selecting Premium option + optional tip, then a single Checkout redirect
  - Finalized cards include an **Edit** (Premium) action that opens a modal and creates an editable draft via `/api/cards/{id}/edit`
  - Billing UI messaging is source-aware (renew vs active-until vs expires vs no-expiration)
  - Premium badge shown for account holder and friends (friends list + friend card view)
- Auth responses include `is_premium` (for the current session user): `internal/handlers/auth.go`.
- Owner/admin scripts: `scripts/create_premium_codes/`, `scripts/grant_premium/`, `scripts/revoke_premium/`.
  - Convenience make targets added:
    - `make premium-code` (local)
    - `make premium-code-prod` (via SSH tunnel)
- Dev tooling: `make local-billing` starts the Stripe webhook listener before the app so `STRIPE_WEBHOOK_SECRET` is available at boot.
- OpenAPI: billing endpoints and schema updated in `web/static/openapi.yaml` (including the combined checkout endpoint and `BillingStatus.source`).

**Not implemented yet (planned for later / still TODO):**
- Additional webhook event handling for invoices (`invoice.payment_*`) and any richer Stripe state sync beyond subscription + checkout completion.
- Remaining premium roadmap features (for example, premium AI in `plans/premium_ai.md`).

## Decisions (Owner Inputs)

### Monetization & Tiering
- **Multiple monetization models**, but **one entitlement tier**: `Premium`.
- **No trials**.
- **Cancel at period end** (no surprise immediate cutoff).
- **Proration enabled** (primarily for monthly↔yearly switches via Stripe Customer Portal).
- **Refunds available**: handled case-by-case via support; align with Stripe best practices (no instant self-serve refunds in v1).
- **All monetization models ship on day 1**:
  - subscription (monthly + yearly)
  - lifetime purchase
  - tip jar
  - premium codes
  - manual grants
- **Stripe Tax required at launch**.
- **Premium badge visible** to the account holder and friends (not public search).

### Account & Entitlements
- Must be logged in or create an account before purchase; flow should be low-friction.
- Premium applies **only** to the purchasing account (no family/team sharing).
- No grandfathering rules.
- Must preserve the ability to grant Premium outside Stripe:
  - **Manual grant** (owner-controlled, tied to user account)
  - **Premium codes** (owner generates codes; users redeem)

### Premium Feature Principles
- Additive value only; no predatory monetization.
- Increased limits are allowed if reasonable and cost-bounded.
- Premium usage (especially AI) must not create unbounded cost exposure.

### Technical / Ops
- Stripe integration is implemented via a small server-side Stripe HTTP client (no `stripe-go` dependency). Migrating to `stripe-go` later is optional.
- Separate keys per environment (test vs live) via env vars (like Gemini).
- Prod secrets stored in env vars.
- Admin actions live in Stripe Dashboard (no in-app admin billing UI).

---

## Premium Design Principles (Additive Only)

### Hard Rules
- Never add watermarks/ads/limits to the free tier that do not already exist.
- Never move an existing feature from free → paid.
- Never degrade performance for free users to make Premium look better.
- Premium-only features must fail gracefully (“This is a Premium feature” with a clear CTA), without blocking core workflows.

### Entitlement Philosophy
- Treat Premium as **feature unlocks**, not feature restrictions.
- Compute entitlements server-side; frontend display is only for UX.

---

## Recommended Monetization Models (All Unlock `Premium`)

### 1) Subscription (Primary)
- Stripe Billing subscription via Stripe Checkout (monthly + yearly prices).

### 2) Lifetime Purchase (Secondary)
- One-time Stripe Checkout payment that grants Premium forever (no recurring billing).

### 3) Tip Jar / Donation (Day 1)
- One-time Stripe Checkout payment that **does not** unlock Premium (pure support).
- This keeps monetization flexible without adding a second entitlement tier.

### 4) Owner-Granted Premium (Non-Stripe)
- Manual grant tied to a specific user account (duration-limited or lifetime).

### 5) Premium Codes (Non-Stripe)
- Owner generates codes; users redeem to gain Premium (duration-limited or lifetime).
- Codes are stored hashed (no plaintext storage) and are one-time by default.

---

## Pricing + Refund Policy (Final Proposal for Owner Review)

These are intentionally low to match app simplicity and reduce churn risk.

| Offer | Price | Notes |
|---|---:|---|
| Premium monthly | **$2.99/mo** | Low friction |
| Premium yearly | **$19.00/yr** | ~47% discount vs monthly |
| Premium lifetime | **$39.00** | One-time |
| Tip jar | $5 / $10 / $20 presets | No entitlement |

All pricing lives in Stripe (Prices) and is referenced by **Price IDs** in env vars.

### Refund window (simple + non-predatory)
- Subscription: **7 days** from the **first** paid invoice on that subscription for a full refund (upon request) + immediate cancellation.
  - Renewals: no automatic refunds; handle genuine mistakes case-by-case.
- Lifetime: **14 days** from purchase for a full refund (upon request).
- Tip jar: **48 hours** from purchase for a full refund (upon request).

### How refunds are executed (v1)
- No self-serve refunds in-app.
- Owner processes refunds in the Stripe Dashboard.
- If a refund requires manually removing Premium entitlement (e.g., lifetime refund), use an owner script (add `scripts/revoke_premium.go` in Phase 5.1) to set the user back to free.

---

## Stripe Integration Approach (Security + Low Friction)

### Stripe Products to Use
- **Stripe Checkout** for starting subscriptions (Stripe-hosted UI; lowest PCI scope).
- **Stripe Billing** subscriptions as the system of record.
- **Stripe Customer Portal** for self-serve cancellation, payment method updates, invoices.
- **Webhooks** to keep our DB in sync and make entitlement decisions offline.

### Stripe Tax (Required)
- Use **Stripe Tax** with **automatic tax calculation** enabled in Checkout Sessions.
- Checkout Sessions should collect billing address as required to calculate tax.
- Stripe Dashboard must be configured with tax registrations and settings before production enablement.

### Environments
- Use Stripe **test mode** keys locally/staging.
- Use Stripe **live mode** keys in production.
- Webhook signing secret is distinct per endpoint.

### Redirect URLs (Hash-Routed SPA Safe)
Because the app is hash-routed, use root path with query params:
- `success_url`: `APP_BASE_URL/?billing=success&session_id={CHECKOUT_SESSION_ID}#profile`
- `cancel_url`: `APP_BASE_URL/?billing=cancel#profile`

On the frontend, show a “Processing your upgrade…” status and poll until Premium is active (webhook-driven eventual consistency).

---

## Data Model (Minimal, Practical, Auditable)

### Migration: `000021_billing_stripe.*.sql`

#### 1) Add billing + entitlement fields to `users`
Add columns:
- `stripe_customer_id VARCHAR(64) UNIQUE NULL`
- `stripe_subscription_id VARCHAR(64) UNIQUE NULL`
- `billing_plan VARCHAR(20) NOT NULL DEFAULT 'free'` (`free`, `premium`)
- `billing_source VARCHAR(20) NOT NULL DEFAULT 'none'` (`none`, `stripe_subscription`, `stripe_lifetime`, `code`, `grant`)
- `billing_status VARCHAR(20) NOT NULL DEFAULT 'inactive'` (`inactive`, `active`, `trialing`, `past_due`, `canceled`)
- `billing_current_period_end TIMESTAMPTZ NULL` (nullable; NULL means “no known end” e.g., lifetime/grant)
- `billing_cancel_at_period_end BOOLEAN NOT NULL DEFAULT false`
- `billing_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Rationale:
- Keep entitlement checks fast (single row).
- Allow webhook processing to update a single place of truth for the app.

**Enum semantics (concrete)**
- `billing_plan`:
  - `free`: no Premium entitlement right now.
  - `premium`: user currently has Premium entitlement (via Stripe, grant, or code).
- `billing_source` (why Premium is active):
  - `none`: no premium entitlement.
  - `stripe_subscription`: Premium is active from a Stripe subscription.
  - `stripe_lifetime`: Premium is active from a one-time lifetime payment.
  - `code`: Premium is active from a redeemed Premium code.
  - `grant`: Premium is active from an owner manual grant.
- `billing_status` (high-level, internal; do **not** store arbitrary Stripe strings here):
  - `inactive`: free tier / no entitlement.
  - `active`: Premium currently active (non-expired).
  - `trialing`: should not occur (we have no trial), but if Stripe ever reports it, treat as Premium.
  - `past_due`: subscription payment issue; treat as Premium **until** `billing_current_period_end` (grace).
  - `canceled`: subscription has ended (no Premium entitlement).

**Stripe subscription status → `billing_status` mapping**
- Stripe may send subscription statuses like: `incomplete`, `incomplete_expired`, `trialing`, `active`, `past_due`, `canceled`, `unpaid`, `paused`.
- Map them as:
  - `active` → `active`
  - `trialing` → `trialing`
  - `past_due` → `past_due`
  - `canceled` → `canceled`
  - `unpaid` / `incomplete` / `incomplete_expired` / `paused` / unknown → `inactive` (and set `billing_plan='free'`)

**Subscription end / expiry rules (single source of truth)**
- Always compute “is Premium?” server-side using `IsPremium(user, now)` (see Entitlements below).
- When a subscription fully ends (Stripe status `canceled` OR `billing_current_period_end <= NOW()`):
  - set `billing_plan='free'`, `billing_status='inactive'`, `billing_source='none'`
  - set `billing_current_period_end=NULL`, `billing_cancel_at_period_end=false`

#### 2) Stripe webhook idempotency table
Create table `stripe_webhook_events`:
- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `stripe_event_id VARCHAR(128) NOT NULL UNIQUE`
- `event_type VARCHAR(128) NOT NULL`
- `livemode BOOLEAN NOT NULL`
- `created_at TIMESTAMPTZ NOT NULL` (event created time from Stripe)
- `received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `processed_at TIMESTAMPTZ NULL`
- `processing_error TEXT NULL`

This makes webhook handling **idempotent** and debuggable.

#### 3) Premium codes table (hashed)
Create table `premium_codes` (one-time by default):
- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `code_hash VARCHAR(64) NOT NULL UNIQUE` (hex sha256)
- `duration_days INT NULL` (NULL = lifetime)
- `expires_at TIMESTAMPTZ NULL` (code validity)
- `redeemed_by_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL`
- `redeemed_at TIMESTAMPTZ NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Optional (recommended for audit): `premium_grants` table to record manual grants.

---

## Backend Implementation Plan (TDD + Service Layer)

### Phase 0: Stripe Company + Dashboard Setup (Manual)

This is a one-time Stripe setup checklist for Year of Bingo (do **test mode first**, then repeat in **live mode**).

#### 0.1 Create Stripe account + business profile
1. Create a Stripe account (use a shared owner email you control long-term).
2. In Stripe Dashboard → **Settings → Business**:
   - **Business name**: “Year of Bingo” (or legal entity name)
   - **Support email**: use the same support email you publish in-app
   - **Website**: `https://yearofbingo.com`
   - **Public business information**: add address/phone (whatever you’re comfortable exposing)
   - **Statement descriptor**: set something recognizable like `YEAROFBINGO` (keep it short)
3. Complete **identity / business verification** as Stripe requests.
4. Add a **bank account** for payouts (Settings → Bank accounts and scheduling).

#### 0.2 Branding (Checkout + Portal UX)
1. Stripe Dashboard → **Settings → Branding**:
   - Upload logo (square works best)
   - Set brand color that matches the site
2. Stripe Dashboard → **Settings → Branding → Customer Portal**:
   - Ensure portal branding matches checkout (logo/colors)

#### 0.3 Products + Prices (what the code needs)
Create these in **test mode first** (then repeat in live mode):
1. Create Stripe **Product**: “Year of Bingo Premium”.
2. Create **Prices**:
   - `premium_monthly` (USD)
   - `premium_yearly` (USD)
   - `premium_lifetime` (USD, one-time)
3. Create Stripe **Product**: “Year of Bingo Tip Jar”.
4. Create **Prices** (one-time):
   - `tip_5` ($5)
   - `tip_10` ($10)
   - `tip_20` ($20)

Important:
- Use **fixed** prices (no client-entered amounts in v1) to keep the UX simple and prevent abuse.
- Decide whether prices are **tax exclusive** (recommended). Stripe Tax will add tax at checkout when applicable.

#### 0.4 Customer Portal (cancel at period end + proration)
1. Stripe Dashboard → **Billing → Customer Portal** (or Settings → Billing → Customer portal):
   - Enable **cancellation** and configure it to cancel at **period end**.
   - Enable **plan switching** monthly↔yearly.
   - Enable **proration** for plan changes (so upgrades/downgrades are fair).
   - Enable **payment method update** and **invoice history**.
2. Save the portal configuration.

#### 0.5 Webhooks (test + live)
Create one webhook endpoint in **test mode** and one in **live mode**.
1. Stripe Dashboard → **Developers → Webhooks → Add endpoint**
2. Endpoint URL:
   - Prod: `https://yearofbingo.com/api/billing/webhook`
   - Local dev (optional): use Stripe CLI to forward to `http://localhost:8080/api/billing/webhook`
3. Select events:
   - `checkout.session.completed`
   - `customer.subscription.created`
   - `customer.subscription.updated`
   - `customer.subscription.deleted`
   - `invoice.payment_succeeded`
   - `invoice.payment_failed`
4. Copy the **Signing secret** (`whsec_...`) for:
   - test mode → `STRIPE_WEBHOOK_SECRET` in dev/staging
   - live mode → `STRIPE_WEBHOOK_SECRET` in production

#### 0.6 Stripe Tax (required at launch)
1. Stripe Dashboard → **Tax**:
   - Enable **Stripe Tax**
   - Add your **tax registrations** (where you’re required to collect)
2. For both Premium and Tip products, set an appropriate **tax code** for what you’re selling.
   - Use Stripe’s tax-code picker; choose the closest match for a small consumer web app / digital service.
3. Verify tax calculation:
   - In test mode, run a Checkout Session and confirm tax is calculated and shown.

#### 0.7 API keys (test + live)
1. Stripe Dashboard → **Developers → API keys**
2. Copy the **Secret key** for each mode:
   - test mode secret → `STRIPE_SECRET_KEY` in dev/staging
   - live mode secret → `STRIPE_SECRET_KEY` in production
3. Optional hardening (recommended later): use **restricted keys** with only required permissions.

#### 0.8 Record values for env vars (per environment)
For each environment, you need:
- `STRIPE_SECRET_KEY`
- `STRIPE_WEBHOOK_SECRET`
- `STRIPE_PREMIUM_PRICE_MONTHLY`
- `STRIPE_PREMIUM_PRICE_YEARLY`
- `STRIPE_PREMIUM_PRICE_LIFETIME`
- `STRIPE_TIP_PRICE_5`
- `STRIPE_TIP_PRICE_10`
- `STRIPE_TIP_PRICE_20`
- `BILLING_ENABLED` (`false` until ready)

#### 0.9 Test mode checkout checklist (before live launch)
1. With `BILLING_ENABLED=true` in staging, confirm:
   - subscription checkout success redirects back to the app and Premium becomes active via webhook
   - portal opens, cancellation is at period end
   - monthly↔yearly switch works and prorates
   - lifetime purchase grants Premium
   - tip jar purchase succeeds and does **not** grant Premium
   - webhook rejects bad signatures
2. Use Stripe test cards (example): `4242 4242 4242 4242` (any future expiry/CVC).

#### 0.10 Live launch checklist
Repeat product/price creation, portal config, webhooks, tax setup in **live mode**, then:
1. Set production env vars.
2. Set `BILLING_ENABLED=true`.
3. Make a real $5 tip purchase and a Premium purchase to confirm end-to-end.

### Phase 1: Config & Secrets
**Files:**
- `internal/config/config.go`
- `internal/config/config_test.go`
- `README.md` (env var table)

Add `BillingConfig` to `Config`:
- `Enabled bool` (env: `BILLING_ENABLED`, default false)
- `StripeSecretKey string` (env: `STRIPE_SECRET_KEY`)
- `StripeWebhookSecret string` (env: `STRIPE_WEBHOOK_SECRET`)
- `StripePremiumMonthlyPriceID string` (env: `STRIPE_PREMIUM_PRICE_MONTHLY`)
- `StripePremiumYearlyPriceID string` (env: `STRIPE_PREMIUM_PRICE_YEARLY`)
- `StripePremiumLifetimePriceID string` (env: `STRIPE_PREMIUM_PRICE_LIFETIME`)
- `StripeTip5PriceID string` (env: `STRIPE_TIP_PRICE_5`)
- `StripeTip10PriceID string` (env: `STRIPE_TIP_PRICE_10`)
- `StripeTip20PriceID string` (env: `STRIPE_TIP_PRICE_20`)

**Config validation rule:**
- In `APP_ENV=production`, if `BILLING_ENABLED=true`, fail startup if any required Stripe env var is missing.

### Phase 2: CSRF Exemption for Webhooks (Security-Critical)
Stripe webhooks cannot send your CSRF token, so we must explicitly exempt the webhook route.

**Files:**
- `internal/middleware/csrf.go`
- `internal/middleware/csrf_test.go`

Implementation:
- Add an allowlist of CSRF-exempt paths (exact match), e.g. `POST /api/billing/webhook`.
- Only exempt that single route (not a broad prefix).

Tests (write first):
- `POST /api/billing/webhook` without CSRF token **passes through** to handler.
- A different `POST /api/*` without CSRF token still returns 403.

---

## Implementation Details (Concrete Enough for a Less-Capable Implementation Agent)

### File Changes Summary (Backend)
- `internal/config/config.go` + `internal/config/config_test.go` (Stripe env + BillingConfig)
- `internal/middleware/csrf.go` + `internal/middleware/csrf_test.go` (webhook CSRF exemption)
- `internal/services/billing/*` (new: entitlements + stripe client + repo)
- `internal/handlers/billing.go` (new: checkout/portal/status/redeem/webhook)
- `cmd/server/main.go` (route registration)
- `migrations/000021_billing_stripe.up.sql` + `.down.sql`
- `web/static/openapi.yaml` (endpoint docs)

### CSRF Exemption Implementation (Exact Shape)

In `internal/middleware/csrf.go`, add a very small allowlist:
```go
var csrfExempt = map[string]bool{
  "/api/billing/webhook": true,
}

// inside Protect(...)
if r.Method == http.MethodPost && csrfExempt[r.URL.Path] {
  next.ServeHTTP(w, r)
  return
}
```

Keep all other unsafe requests protected.

### Stripe API Client (As Implemented)

Current implementation uses a small Stripe HTTP client wrapper (no `stripe-go`) in:
- `internal/services/billing/stripe_client.go`

Key rules are the same:
- Never accept a `price_id` from the client; map `interval/amount -> env-configured price id` server-side.
- Stripe Tax is enabled in Checkout Session creation (`automatic_tax` + `billing_address_collection`).

### Webhook Verification + Idempotency (Example Code)

Signature verification is implemented via HMAC verification of `Stripe-Signature`:
- `internal/services/billing/webhook.go` (`billing.VerifyStripeSignature`)

Idempotency table usage:
1. Insert event id (unique):
```sql
INSERT INTO stripe_webhook_events (stripe_event_id, event_type, livemode, created_at)
VALUES ($1, $2, $3, to_timestamp($4))
ON CONFLICT (stripe_event_id) DO NOTHING;
```
If rows affected is 0 → already processed; return 200.

2. Process event.
3. Update `processed_at` or `processing_error`.

### Premium Code Redemption (Server-Side Only)

**Normalization** (important for UX; users will paste with hyphens/spaces):
- uppercase
- remove spaces and hyphens
- enforce prefix `YOBP` (optional) and min length

**Format validation (concrete)**
- After normalization, accept only codes matching: `^YOBP[A-Z2-7]{24}$`
  - This corresponds to: prefix `YOBP` + 15 random bytes encoded as Base32 (no padding) = 24 chars.
- If the code fails validation, return `400 {"error":"Invalid code"}` (do not reveal whether a code exists).

**Hashing**:
```go
sum := sha256.Sum256([]byte(code))
codeHashHex := hex.EncodeToString(sum[:])
```

**Atomic redeem transaction** (sketch):
1. `SELECT ... FROM premium_codes WHERE code_hash=$1 AND redeemed_at IS NULL AND (expires_at IS NULL OR expires_at > NOW()) FOR UPDATE`
2. `UPDATE premium_codes SET redeemed_by_user_id=$2, redeemed_at=NOW() WHERE id=$3`
3. Apply entitlement:
   - if `duration_days IS NULL` → lifetime: `billing_current_period_end=NULL`
   - else → `billing_current_period_end = NOW() + (duration_days || ' days')::interval`
4. Update user:
   - `billing_plan='premium'`
   - `billing_source='code'`
   - `billing_status='active'`
   - `billing_cancel_at_period_end=false`

### Redeem Endpoint Rate Limiting

Add a Redis rate limiter middleware for `POST /api/billing/redeem`:
- prefix: `ratelimit:redeem:`
- key: userID (fallback to IP)
- limit: start with `10/hour` (adjust later)
- fail-closed

### Script: create premium codes (DB-backed)

In `scripts/create_premium_codes.go`:
- Load config (DB DSN) and connect.
- For each code:
  - generate 15 random bytes (`crypto/rand`)
  - base32 encode (no padding), group into 4s, prefix `YOBP-`
  - insert `sha256(code)` into `premium_codes` with duration/expires
- Print plaintext codes once to stdout.

Because redemption checks the DB allowlist, public repo visibility does not make codes guessable.

### Phase 3: Billing Service + Stripe Client
**Files (as implemented):**
- `internal/services/billing/service.go`
- `internal/services/billing/store.go` (DB access behind an interface for unit tests)
- `internal/services/billing/stripe_client.go`
- `internal/services/billing/entitlements.go`
- `internal/services/billing/types.go`
- `internal/services/billing/webhook.go`

Design:
- Create a small `StripeClient` interface so tests can run without network:
  - `CreateCustomer(ctx, email, userID string) (customerID string, err error)`
  - `CreateCheckoutSession(ctx, params CheckoutSessionParams) (url string, err error)`
  - `CreatePortalSession(ctx, customerID, returnURL string) (url string, err error)`

**Make It Unit-Testable (No DB Required)**
The repo’s Go tests run without a database (see `scripts/test.sh`), so billing logic must be testable with fakes.

This is implemented as `StoreInterface` in `internal/services/billing/store.go`, and unit tests provide fakes for the store and Stripe client.

Stripe Tax requirements in Checkout Session creation:
- `automatic_tax.enabled = true`
- `billing_address_collection = required`
- Provide `customer` (create/store Stripe customer once per user)

Entitlements (as implemented):
- `IsPremium(user, now) bool` returns true when:
  - `billing_plan == 'premium'`
  - and (`billing_current_period_end` is NULL OR `billing_current_period_end > now`)

### Phase 4: Billing Handlers & Routes
**New file:**
- `internal/handlers/billing.go`

**Register in:** `cmd/server/main.go`

Endpoints (session-auth only; must have logged-in user):
1. `GET /api/billing/status`
   - Returns current plan/status/period_end/cancel_at_period_end.
2. `POST /api/billing/checkout/subscription`
   - Body: `{ "interval": "month" | "year" }` (required)
   - Server maps interval → configured Stripe price ID (never accept a raw price ID from client).
   - Returns `{ "url": "https://checkout.stripe.com/..." }`.
3. `POST /api/billing/checkout/lifetime`
   - Returns Checkout URL for the lifetime price.
4. `POST /api/billing/checkout/tip`
   - Body: `{ "amount": 5 | 10 | 20 }` (required)
   - Server maps amount → env-configured tip price ID (never accept a raw price ID from client).
5. `POST /api/billing/portal`
   - Returns `{ "url": "https://billing.stripe.com/..." }`.
6. `POST /api/billing/redeem`
   - Body: `{ "code": "XXXX-XXXX-..." }`
   - Validates + redeems a premium code (one-time by default) and updates user entitlements.
7. `POST /api/billing/webhook` (no auth, CSRF-exempt)
   - Verifies Stripe signature.
   - Idempotently processes event (insert into `stripe_webhook_events` first; ignore duplicates).

**Route registration sketch (matches `cmd/server/main.go` patterns)**
```go
// init rate limiter for redeem codes (cost + abuse sensitive)
redeemLimiter := middleware.NewRateLimiter(redisDB.Client, 10, 1*time.Hour, "ratelimit:redeem:", func(r *http.Request) string {
  user := handlers.GetUserFromContext(r.Context())
  if user != nil {
    return user.ID.String()
  }
  return ""
}, false)

// billing endpoints (session-only)
mux.Handle("GET /api/billing/status", requireSession(http.HandlerFunc(billingHandler.Status)))
mux.Handle("POST /api/billing/checkout/subscription", requireSession(http.HandlerFunc(billingHandler.CheckoutSubscription)))
mux.Handle("POST /api/billing/checkout/lifetime", requireSession(http.HandlerFunc(billingHandler.CheckoutLifetime)))
mux.Handle("POST /api/billing/checkout/tip", requireSession(http.HandlerFunc(billingHandler.CheckoutTip)))
mux.Handle("POST /api/billing/portal", requireSession(http.HandlerFunc(billingHandler.Portal)))
mux.Handle("POST /api/billing/redeem", requireSession(redeemLimiter.Middleware(http.HandlerFunc(billingHandler.Redeem))))

// webhook is unauthenticated (signature-verified), must be CSRF-exempt
mux.Handle("POST /api/billing/webhook", http.HandlerFunc(billingHandler.Webhook))
```

Webhook processing rules:
- Always verify `Stripe-Signature` with `STRIPE_WEBHOOK_SECRET`.
- Use `http.MaxBytesReader` to cap request body size (e.g., 1MB).
- For subscription-related events, update `users` billing fields by **Stripe customer id** or metadata user id.
- Never trust client-provided user id for upgrades; link via Stripe objects.

Subscription flow specifics:
- Checkout session sets `customer = users.stripe_customer_id` (create if missing).
- On subscription events, set:
  - `stripe_subscription_id = subscription.id`
  - `billing_plan='premium'`
  - `billing_source='stripe_subscription'`
  - `billing_status` from Stripe subscription status
  - `billing_current_period_end` from Stripe subscription current_period_end
  - `billing_cancel_at_period_end` from Stripe subscription cancel_at_period_end

Lifetime flow specifics:
- Use Checkout in payment mode with lifetime price.
- On `checkout.session.completed` where metadata indicates lifetime purchase, set:
  - `billing_plan='premium'`
  - `billing_source='stripe_lifetime'`
  - `billing_status='active'`
  - `billing_current_period_end=NULL`
  - `billing_cancel_at_period_end=false`

Tip flow specifics:
- Record the payment for analytics only; no entitlement changes.

### Phase 5: Database Updates in Services
**Files (examples):**
- `internal/services/user.go` (add methods to update billing fields)
- or new `internal/services/billing/repo.go` with explicit SQL.

Required operations:
- Set `stripe_customer_id` for a user.
- Upsert subscription status fields on webhook updates.
- Clear subscription fields when canceled/deleted.
- Redeem premium codes atomically (single transaction: check valid + mark redeemed + update user).

### Phase 5.1: Owner Tooling (Manual Grant + Code Generation)
Because there is no in-app admin panel, implement owner-only tools as scripts.

**New files (recommended):**
- `scripts/grant_premium.go`
- `scripts/revoke_premium.go`
- `scripts/create_premium_codes.go`

**Grant script requirements:**
- Inputs: `--email`, `--duration_days` (optional), `--lifetime` (optional), `--reason` (optional)
- Behavior:
  - Find user by email (case-insensitive).
  - Update `users`:
    - `billing_plan='premium'`
    - `billing_source='grant'`
    - `billing_status='active'`
    - `billing_current_period_end = NOW()+duration` (or NULL for lifetime)
    - `billing_cancel_at_period_end=false`
  - Optionally insert audit row into `premium_grants`.

**Revoke script requirements:**
- File: `scripts/revoke_premium.go`
- Inputs: `--email`, `--reason` (optional)
- Behavior:
  - Find user by email (case-insensitive).
  - Update `users` to a clean free state:
    - `billing_plan='free'`
    - `billing_source='none'`
    - `billing_status='inactive'`
    - `billing_current_period_end=NULL`
    - `billing_cancel_at_period_end=false`
    - `billing_updated_at=NOW()`
  - Do **not** attempt to cancel Stripe subscriptions from this script (do that in Stripe Dashboard).

**Premium code generator requirements:**
- Inputs: `--count`, `--duration_days` (optional), `--lifetime` (optional), `--expires_days` (optional)
- Generate codes using `crypto/rand` with **>= 120 bits** of entropy per code (recommended: 15 random bytes).
- Encode as human-friendly **Base32 (RFC4648, no padding)** (A–Z and 2–7 only; avoids ambiguous 0/1), grouped for readability/typing.
  - Example format: `YOBP-ABCD-EFGH-IJKL-MNOP-QRST-UVWX`
- Store only `sha256(code)` (hex) in DB; never store plaintext.
- Output plaintext codes to stdout once for copying/sending.
  - Because the repo is public, code security comes from **random entropy + DB allowlist + redeem rate limiting**, not from hiding the algorithm.

### Phase 6: OpenAPI Documentation
**File:** `web/static/openapi.yaml`

Add:
- `cookieAuth` security for billing endpoints (similar to `/ai/generate`).
- Schemas for `BillingStatus`, `CheckoutSessionResponse`, `PortalSessionResponse`, `RedeemCodeRequest`.
- For privacy: expose to clients only what’s needed (e.g., `is_premium` and billing status fields, not Stripe IDs).

**Concrete OpenAPI sketch (copy/paste shape)**
```yaml
  /billing/status:
    get:
      summary: Get billing status for current session user
      description: Requires a browser session cookie (API tokens are not allowed).
      security:
        - cookieAuth: []
      responses:
        '200':
          description: Billing status
          content:
            application/json:
              schema:
                type: object
                properties:
                  is_premium: { type: boolean }
                  plan: { type: string, enum: [free, premium] }
                  status: { type: string, enum: [inactive, active, trialing, past_due, canceled] }
                  current_period_end: { type: string, format: date-time, nullable: true }
                  cancel_at_period_end: { type: boolean }

	  /billing/checkout/subscription:
	    post:
	      summary: Create Stripe Checkout Session (subscription)
	      description: Requires a browser session cookie (API tokens are not allowed).
      security:
        - cookieAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [interval]
              properties:
                interval: { type: string, enum: [month, year] }
      responses:
        '200':
          description: Checkout URL
          content:
            application/json:
              schema:
                type: object
                properties:
	                  url: { type: string }

	  /billing/checkout/lifetime:
	    post:
	      summary: Create Stripe Checkout Session (lifetime)
	      description: Requires a browser session cookie (API tokens are not allowed).
	      security:
	        - cookieAuth: []
	      responses:
	        '200':
	          description: Checkout URL
	          content:
	            application/json:
	              schema:
	                type: object
	                properties:
	                  url: { type: string }

	  /billing/checkout/tip:
	    post:
	      summary: Create Stripe Checkout Session (tip jar)
	      description: Requires a browser session cookie (API tokens are not allowed).
	      security:
	        - cookieAuth: []
	      requestBody:
	        required: true
	        content:
	          application/json:
	            schema:
	              type: object
	              required: [amount]
	              properties:
	                amount: { type: integer, enum: [5, 10, 20] }
	      responses:
	        '200':
	          description: Checkout URL
	          content:
	            application/json:
	              schema:
	                type: object
	                properties:
	                  url: { type: string }

  /billing/portal:
    post:
      summary: Create Stripe Customer Portal Session
      description: Requires a browser session cookie (API tokens are not allowed).
      security:
        - cookieAuth: []
      responses:
        '200':
          description: Portal URL
          content:
            application/json:
              schema:
                type: object
                properties:
                  url: { type: string }

  /billing/redeem:
    post:
      summary: Redeem a Premium code
      description: Requires a browser session cookie (API tokens are not allowed).
      security:
        - cookieAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [code]
              properties:
                code: { type: string }
      responses:
        '200':
          description: Redeemed
          content:
            application/json:
              schema:
                type: object
                properties:
                  is_premium: { type: boolean }
```

---

## Frontend Implementation Plan (Positive UX)

### Phase 7: UI Entry Points (No Nagging)
**Files:**
- `web/static/js/app.js`
- `web/static/js/api.js`
- `web/static/css/styles.css`

UX placements:
- Profile page: show “Plan: Free/Premium” with:
  - If Free: “Upgrade to Premium” button
  - If Premium: “Manage Subscription” button

Anonymous users:
- Show an “Upgrade” CTA in Profile/About, but clicking it routes to `#login` / `#register` and then returns to upgrade flow.
- Avoid interrupting core flows (no surprise modals).

### Phase 8: Upgrade Flow
1. Add API methods:
   - `API.billing.getStatus()`
   - `API.billing.createSubscriptionCheckoutSession(interval)`
   - `API.billing.createLifetimeCheckoutSession()`
   - `API.billing.createTipCheckoutSession(amount)` (where `amount` is `5|10|20`)
   - `API.billing.createPortalSession()`
   - `API.billing.redeemCode(code)`
2. “Upgrade” modal/page with:
   - Clear list of premium benefits (additive). Suggested copy:
     - “Templates + 1‑click New Year rollover”
     - “AI Enhancements: 100/month” (Goal Assistant, Regenerate, AI Fill Empty)
     - “AI requires a verified email after 5 total generations” (anti-abuse; same rule as free users)
     - “Premium badge (visible to friends)”
   - Optional “Tip Jar” section (separate from Premium purchase) with $5/$10/$20 buttons
   - Monthly/yearly toggle + lifetime option (if enabled)
   - Button redirects to Stripe Checkout URL
3. After return:
   - If `?billing=success&session_id=...`, show “Processing…” and poll `/api/billing/status` until active (max ~60s).
   - If still not active, show friendly message + link to Support.

### Phase 8.1: Premium Badge (Account + Friends)
- Add `is_premium` boolean in:
  - `/api/auth/me` (account holder sees their own status)
  - friend-only endpoints (friends see the badge), e.g.:
    - `/api/friends`
    - `/api/friends/{id}/card`
    - `/api/friends/{id}/cards`
- Do **not** include `is_premium` in public-ish discovery endpoints like `/api/friends/search`, and do not show it on public share pages/links.
- UI:
  - Profile: “Premium” badge near username
  - Friends card view: small badge next to friend username (no billing details)

### Phase 9: Gating UX for Premium Features
For premium-only UI actions:
- Show the button (discoverability) but gate with a modal:
  - “This is a Premium feature” + short value explanation + “Upgrade” CTA.
- Never block core card creation/edit/finalize flows.

---

## Premium Feature Gating Pattern (Server-Enforced)

### Required Backend Pattern
Any premium-only endpoint must:
1. Require auth (logged in user).
2. Check entitlement server-side (don’t trust frontend).
3. Return `403` with a clear error message, e.g.:
   - `{"error":"Premium required"}`.

---

## Premium Features People Will Pay For (Additive Roadmap)

These are intentionally chosen to feel “worth it” without harming free users.

### 1) Templates + 1‑Click “New Year” Rollover (Premium)
- Premium users can mark a card as a reusable template and generate next year’s card in one click.
- Includes safe options like “carry over incomplete items” and “reshuffle layout”.
- Requires a dedicated plan before implementation:
  - **Create** `plans/premium_templates.md` (same style: schema, endpoints, UX, tests, rollout).

### 2) Edit After Finalize (Premium)

**Problem:** Today, finalized cards are intentionally immutable. If a user notices a typo or wants to refine a goal after finalizing, they must either “live with it” or use **Clone** as a workaround (which changes layout and doesn’t preserve the idea of “this is the same card, just fixed”).

**Premium value:** A clean, explicit workflow to make changes **after finalize** without degrading free users.

#### Decision: implement as “Create editable copy” (recommended)
This is the safest interpretation of “edit after finalize”:
- The original finalized card remains immutable (preserves trust / “history”).
- Premium users get a 1‑click action that creates a new **draft** card prefilled from the finalized card.

This approach is also easiest for an implementation agent because it can reuse existing card creation + insert logic (see `internal/services/card.go`).

#### User experience (UX)
- On a finalized card, show a button: **“Edit”** (Premium).
  - Free users either don’t see it, or see it gated with a Premium modal (preferred for discoverability).
- Clicking “Edit” opens a modal with:
  - Title (default: `"<existing title> (Edit)"`)
  - Optional “Shuffle layout” toggle (default **OFF** for editing; preserves layout)
  - Optional “Reset completion data” toggle (default **ON**; matches templates/rollover rules)
    - If “reset” is enabled, the new draft’s items start with:
      - `is_completed=false`
      - `completed_at=NULL`
      - `notes=NULL`
      - `proof_url=NULL`
- Submitting creates a new draft and routes to it.

#### Backend: endpoint + behavior (copy/paste oriented)
Add endpoint:
- `POST /api/cards/{id}/edit` (Premium; requires write scope; server‑enforced entitlement)

Request body:
```json
{
  "title": "Optional override",
  "shuffle_layout": false,
  "reset_progress": true
}
```

Response on success (`201`):
```json
{ "card": { /* BingoCard */ } }
```

Conflict response (`409`):
```json
{
  "error":"Card conflict",
  "conflict":{"year":2026,"title":"My 2026 Goals"},
  "suggested_title":"My 2026 Goals (Edit)"
}
```

Rules:
1. Require auth (`user != nil`)
2. Enforce Premium entitlement server-side.
3. Authorize ownership (user owns source card).
4. Require the source card to be `is_finalized=true` (otherwise return `400`).
5. The new card is always created as a **draft** (`is_finalized=false`).
6. Default behavior should preserve:
   - `year` (same year as source)
   - `category` (copy)
   - `grid_size`, `header_text`, `has_free_space`, and `free_space_position` (copy)
   - `visible_to_friends` (copy)
7. Item copy rules:
   - Always copy the **item content**.
   - If `shuffle_layout=false` (default), keep the same `position` values.
   - If `shuffle_layout=true`, choose valid positions excluding FREE (same logic as `plans/premium_templates.md`).
   - If `reset_progress=true` (default), new items are inserted with defaults (no completion/notes/proof).
     - Note: the simplest implementation is to insert new `bingo_items` with only `content` + `position`, since defaults already reset progress.

Implementation sketch (reuse existing primitives):
- Add a new service method on `CardService` (or a small new `PremiumEditService`) that:
  1) loads the source card (`GetByID`) and checks ownership
  2) generates a non-conflicting title (suffix strategy)
  3) uses the existing “import/create in one transaction” pattern to create the new draft + insert items.
     - You can reuse `CardService.Import` (recommended) because it supports explicit `GridSize`, `HeaderText`, `HasFreeSpace`, `FreeSpacePos`, and item positions.

#### Frontend wiring
- Add a button on finalized cards: **Edit** (Premium), next to Clone.
- Add modal:
  - Title input
  - Shuffle checkbox (default unchecked)
  - Reset checkbox (default checked)
- On submit:
  - call `POST /api/cards/{id}/edit`
  - if `409`, show suggested title and prompt user to retry
  - on success, navigate to `/card/{newId}`

#### Tests (TDD-first, no DB required)
Add tests that keep the behavior correct and safe:
1. Premium enforcement:
   - non-premium user gets `403` from `POST /api/cards/{id}/edit`
2. Ownership enforcement:
   - user cannot edit someone else’s card (`403`)
3. Finalized requirement:
   - draft card returns `400` (“must be finalized”)
4. Layout behavior:
   - shuffle OFF preserves positions
   - shuffle ON never uses FREE position and stays within range
5. Progress reset:
   - verify newly created items have `is_completed=false` and `notes/proof` empty (if your test harness can inspect item fields)

**Note:** Free users still have **Clone** as an escape hatch. Premium “Edit after finalize” is a faster, safer, more explicit flow that preserves layout and provides better conflict handling.

### 3) Premium AI Boost (Cost‑Bounded) (Premium)
- Premium includes a **simple monthly allowance** of “AI Enhancements” and adds goal-focused tools:
  - Goal Assistant / ideator (strictly tied to a specific bingo goal)
  - Regenerate a goal in the wizard
  - AI fill empty squares on a draft card
- Recommended default allowance: **100 AI Enhancements/month** (configurable via env var, but keep the marketed promise simple).
- Constraint: do not reduce existing AI functionality for free users; no sneaky limits.
- Requires a dedicated plan before implementation:
  - **Use** `plans/premium_ai.md` (must preserve session-only auth, verified-email gating, and logging constraints from `plans/ai_goals.md`).

---

## Cost Controls (AI)

Keep Premium simple and bounded without confusing users:
- Premium AI value is sold as **`N` AI Enhancements/month** (clear meter, clear reset date).
- Each Premium AI action consumes **exactly 1** enhancement (see `plans/premium_ai.md`).
- Keep free AI behavior unchanged.

Optional future work (only if needed) should be planned separately:
- `plans/ai_cost_controls.md` (global monthly budget, emergency kill switch, dashboards).

---

## Testing Plan (TDD, No Network)

### Unit Tests (required)
1. CSRF exemption tests (middleware).
2. Webhook signature verification tests:
   - Valid signature accepted.
   - Invalid signature rejected.
   - Missing signature rejected.
3. Webhook idempotency tests:
   - Same Stripe event delivered twice → only first updates user, second is no-op.
4. Entitlements logic tests:
   - status combinations map correctly to Premium/non-Premium.
5. Billing handler tests using a mocked `StripeClient`:
   - checkout session returns URL
   - portal session returns URL
   - unauthenticated requests rejected
6. Premium code redemption tests:
   - invalid/expired code rejected
   - valid code applies Premium and cannot be redeemed twice

**Stripe webhook test helper (no network, deterministic)**
You can generate a `Stripe-Signature` header in unit tests without calling Stripe:
```go
func stripeSignatureHeader(t *testing.T, secret string, payload []byte, ts int64) string {
  t.Helper()
  signed := fmt.Sprintf("%d.%s", ts, string(payload))
  mac := hmac.New(sha256.New, []byte(secret))
  _, _ = mac.Write([]byte(signed))
  sig := hex.EncodeToString(mac.Sum(nil))
  return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}
```
Then:
- `req.Header.Set("Stripe-Signature", stripeSignatureHeader(t, secret, payload, 1700000000))`
- `req.Body = io.NopCloser(bytes.NewReader(payload))`

Example test skeleton:
```go
func TestBillingWebhook_InvalidSignatureRejected(t *testing.T) {
  h := &BillingHandler{webhookSecret: "whsec_test"} // plus any deps
  payload := []byte(`{"id":"evt_test","type":"checkout.session.completed","livemode":false,"created":1700000000,"data":{"object":{}}}`)

  req := httptest.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader(payload))
  req.Header.Set("Stripe-Signature", "t=1700000000,v1=not-a-real-sig")
  rr := httptest.NewRecorder()

  h.Webhook(rr, req)
  if rr.Code != http.StatusBadRequest {
    t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
  }
}
```

### Integration-ish Tests (optional, still offline)
- Webhook handler with a captured fixture payload + computed signature.

### E2E Tests (Playwright, mocked Stripe; no Stripe listener)
We want billing coverage in `make e2e` without depending on the Stripe CLI listener/webhook forwarding.

Approach:
- Add an env var to point the server-side Stripe HTTP client at a local mock:
  - `STRIPE_API_BASE_URL` (defaults to Stripe when unset).
- Run a lightweight mock Stripe API in the E2E stack:
  - `tests/stripe_mock/` + `Containerfile.stripe-mock`
  - Minimal endpoints the app uses (`/v1/customers`, `/v1/checkout/sessions`, `/v1/billing_portal/sessions`)
  - Test/debug endpoints used by Playwright (e.g. `GET /test/last-checkout-session`)
- In Playwright, simulate Stripe webhooks by POSTing signed events directly to:
  - `POST /api/billing/webhook` (same signature verification as production)

Coverage:
- `tests/e2e/billing-premium.spec.js`:
  - "subscription + tip" combined checkout (multiple line items) + webhook activation
  - "lifetime" purchase + webhook activation

How to run:
- Targeted: `./scripts/e2e.sh tests/e2e/billing-premium.spec.js`
- Full suite: `make e2e` (uses `./scripts/e2e.sh`)

### Manual Verification Checklist
1. Start checkout from Profile → redirect to Stripe Checkout.
2. Complete payment in test mode → return to app, Premium becomes active within ~1 minute.
3. Open Customer Portal → cancel subscription → Premium removed at period end (or immediately, per policy).
4. Webhook endpoint rejects invalid signatures.

---

## Recommended Implementation Order (Plans)

This ordering minimizes risk and keeps each step testable:

1. `plans/monetization.md`
   - Build the billing foundation first: config + migrations + entitlements + webhook idempotency + codes/grants + upgrade UI.
   - Use manual grants/codes to test Premium gating before Stripe is fully live.
2. `plans/premium_templates.md`
   - Pure app/DB logic; no external cost; easiest premium feature to validate and ship safely.
3. `plans/premium_ai.md`
   - Cost-sensitive and more complex; implement after entitlements are stable.

## Rollout Plan (Low Risk)

1. Ship code behind `BILLING_ENABLED=false` by default (no UI entry points).
2. Enable in staging with Stripe test keys.
3. Validate webhook processing + entitlement updates.
4. Enable in production with live keys.
5. Monitor:
   - webhook errors (`stripe_webhook_events.processing_error`)
   - support requests about billing
