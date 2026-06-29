# Year of Bingo — Security Review (2026-06-28)

Comprehensive security assessment of the live production application at
`https://yearofbingo.com`. Scope: backend (`cmd/`, `internal/`), frontend
templates/CSP, deployment (`compose.server.yaml`, `cloud-init.yaml`,
`Containerfile`), authentication/session/billing flows, and supporting
middleware.

> A prior audit (`temp/sec_audit.md`, 2026-01-29) was reviewed; its items
> (query-string token logging, `sanitizeNext` open-redirect, XFF trust model,
> CSP, CI pinning) appear remediated. This document focuses on **newly
> identified** issues plus a re-confirmation of the threat model. Findings are
> ordered by severity.

## Architecture summary (as built)
- Go `net/http` server behind a **Cloudflare Tunnel** (`cloudflared`) →
  `http://localhost:80` on a Hetzner box. TLS terminates at Cloudflare.
- Sessions: opaque 32-byte token, SHA-256 hashed, stored **Redis-first**
  (Postgres only used as a fallback when Redis writes fail).
- CSRF: double-submit cookie (`SameSite=Strict`) + `X-CSRF-Token` header.
- Auth: email/password (bcrypt cost 12), magic-link, Google OIDC, API bearer
  tokens (SHA-256 hashed, scoped).
- Billing: Stripe Checkout + webhook (HMAC-SHA256 signature verification).
- Rate limiting: Redis token-bucket via Lua, fail-closed on auth/AI.

Overall the codebase is **security-conscious**: parameterized SQL everywhere,
strict CSP without `unsafe-inline`, SRI on third-party scripts, constant-time
comparisons for tokens/CSRF/Stripe sigs, email-enumeration-safe auth responses,
bcrypt with sane cost, scoped/hashed API tokens, soft-delete with `deleted_at`
filtering, and a trusted-proxy abstraction. The issues below are mostly
logic/operational gaps rather than systemic flaws.

---

## HIGH

### H-1. Password change / reset does **not** revoke active sessions
**Location:** `internal/services/auth.go` (`CreateSession`, `DeleteAllUserSessions`),
called from `internal/handlers/auth.go:273` (ChangePassword) and `:487` (ResetPassword).

**What happens**
- `CreateSession` writes the session **only to Redis** (`session:<hash> → userID`);
  Postgres is written *only if the Redis write errors*. In normal operation no
  row exists in the `sessions` table.
- `DeleteAllUserSessions` enumerates sessions by querying **Postgres**
  (`SELECT token_hash FROM sessions WHERE user_id = $1`) and then deletes those
  hashes from Redis. Because the active sessions live only in Redis and there is
  **no per-user index in Redis** (the `RedisClient` interface only exposes
  `Set/Get/Expire/Del` — no `SCAN`/`SMEMBERS`), this query returns nothing and
  **no Redis sessions are deleted**.

**Impact**
- "Change password" and "Reset password" silently fail to invalidate other
  sessions. After a credential-reset performed precisely because an account is
  suspected compromised, the **attacker's existing session remains valid** for
  up to 30 days. Worse, `ValidateSession` refreshes the Redis TTL on every
  request (sliding expiration), so an active attacker session never expires.
- This defeats the primary account-recovery control. Account *deletion* is not
  affected (soft-delete makes `getUserByID` fail, killing the session
  indirectly), but password reset/change are.

**Fix**
- Maintain a per-user session index in Redis: on `CreateSession`,
  `SADD user_sessions:<userID> <hash>` (with matching TTL/cleanup). On
  `DeleteAllUserSessions`, read the set and `DEL` each `session:<hash>` plus the
  set itself. Add `SAdd`/`SMembers`/`SRem` to the `RedisClient` interface.
- Alternatively (simpler, robust): store a `sessions_version` / `session_epoch`
  integer per user; embed it in the session value and compare on every
  `ValidateSession`. Bump it on password change/reset to invalidate all prior
  sessions in O(1).
- Add a regression test that creates a Redis-backed session, calls
  `DeleteAllUserSessions`, and asserts the session no longer validates.

---

## MEDIUM

### M-1. IP-based rate limits are bypassable via `X-Forwarded-For` spoofing
**Location:** `internal/httpx/client_ip.go`, `internal/httpx/trusted_proxy.go`,
`cmd/server/ratelimits.go` (IP limiters), deployment `compose.server.yaml`.

**What happens**
- Production sets no `TRUSTED_PROXY_CIDRS`, so `TrustedProxyChecker` defaults to
  trusting **loopback/RFC1918**. `cloudflared` connects from `127.0.0.1`, so
  every request is "trusted" and `ClientIP` honors `X-Forwarded-For`.
- `ClientIP` returns the **left-most** XFF entry. Cloudflare **appends** the real
  client IP to any client-supplied `X-Forwarded-For`, producing
  `XFF: <attacker-supplied>, <realIP>`. Taking the first element means the
  rate-limit key is **fully attacker-controlled**.
- Affected limiters use the client IP: `authLoginIPLimiter`,
  `authRegisterIPLimiter`, `authEmailFlowIPLimiter`, `authResetPasswordIPLimiter`,
  and the support form. An attacker rotates a fake first XFF value per request to
  get unlimited buckets.

**Impact**
- Bypass of IP throttling for credential stuffing across many accounts, mass
  account registration, and password-reset / verification **email bombing**
  (reputation + cost). Per-*email* limiters still cap single-account brute force,
  so this is Medium rather than High.

**Fix**
- Since the app is committed to Cloudflare, derive the client IP from
  **`CF-Connecting-IP`** (single, set by Cloudflare, not client-spoofable once the
  firewall restricts ingress to Cloudflare ranges — which `cloud-init.yaml`
  already does). Fall back to the right-most XFF entry, never the left-most.
- Set an explicit `TRUSTED_PROXY_CIDRS` for the cloudflared peer rather than
  relying on "trust all private/loopback."

### M-2. `/health` leaks internal error details to anonymous users
**Location:** `internal/handlers/health.go` (`Health`), route `GET /health` (public).

**What happens** — On dependency failure the JSON body includes the raw error
string: `"postgres": "unhealthy: " + err.Error()` and same for Redis. These can
contain internal hostnames, ports, DSN fragments, or driver internals.

**Impact** — Information disclosure aiding reconnaissance during outages. The
endpoint is reachable by anyone who can reach the origin (and is used by the
container `HEALTHCHECK`).

**Fix** — Return a generic `"unhealthy"` status per dependency in the public
response; log the detailed error server-side only. Or gate the detailed body
behind `APP_ENV=development`. (`/ready`/`/live` already return opaque text — good.)

### M-3. CSRF token is not bound to the session (defense-in-depth)
**Location:** `internal/middleware/csrf.go`.

**What happens** — The double-submit token is a standalone random value; it is
not tied to the authenticated session. The cookie is `SameSite=Strict` + `Secure`
(in prod), which is the real protection, so this is not directly exploitable
today. However, in any scenario where an attacker can set a cookie on the victim
(sibling subdomain, future CDN/edge misconfig, MITM on a non-HSTS-preloaded
first visit), a fixated CSRF cookie+value pair would pass validation.

**Fix** — Bind the CSRF token to the session (e.g., HMAC(sessionID, secret) or a
signed token), or adopt the standard signed double-submit. Low effort, removes a
class of edge cases. Consider HSTS preload registration to harden first-visit.

---

## LOW / Informational

### L-1. Host-header influence on canonical/OG URLs
`internal/handlers/pages.go:resolveBaseURL` trusts `X-Forwarded-Host` /
`X-Forwarded-Proto` (because all requests are "trusted" per M-1). This only
affects OG/canonical URLs on `noindex` share pages — not email links (those use
the configured `cfg.Email.BaseURL`). Impact is limited cache-poisoning of OG
preview URLs. Pin the public host or validate `Host` against an allowlist.

### L-2. Unescaped sender/category in operator support email
`internal/services/email.go:renderSupportEmail` HTML-escapes the user `message`
(good) but interpolates `fromEmail` and `category` unescaped. Both are
constrained (`mail.ParseAddress` validation + category whitelist), so injection
is impractical, but escape them for consistency to avoid future regressions.

### L-3. `DB_SSLMODE=disable` in production
`compose.server.yaml` connects app→Postgres without TLS. Traffic stays on the
container/host network, so risk is low, but enabling `sslmode=require` (or
`verify-full` with a CA) is cheap defense-in-depth against host compromise /
sidecar sniffing.

### L-4. Long-lived sliding sessions
30-day sessions whose TTL refreshes on every request mean a token, once stolen,
is effectively permanent until used-then-idle for 30 days. Amplifies H-1.
Consider an absolute max lifetime (e.g., 30–90 days regardless of activity) and
periodic re-auth for sensitive actions.

### L-5. Login user-enumeration timing side-channel
`AuthHandler.Login` skips bcrypt when the email is unknown, returning
measurably faster than for a known email. Magic-link/forgot-password already use
constant responses. Optionally run a dummy bcrypt compare on the not-found path
to flatten timing.

### L-6. Third-party script/style CDN dependency (`cdnjs.cloudflare.com`)
CSP allows scripts/styles from `cdnjs` and `static.cloudflareinsights.com`. JS
(`jszip`) and Font Awesome CSS carry **SRI** (good). Residual risk is
availability and the (SRI-mitigated) supply-chain surface. Consider self-hosting
these assets to drop two external `script-src`/`style-src` origins and remove the
runtime dependency.

### L-7. Reminder unsubscribe POST is CSRF-exempt
`/r/unsubscribe` (POST) is intentionally CSRF-exempt and authorized by an
unguessable token in the form. Acceptable, but note a known token allows a
cross-site forced unsubscribe. Low impact; consider same-site form posting only.

### L-8. Operational hygiene
- `.env` exists in the working tree but is correctly git-ignored and **not**
  committed (verified). Ensure the server `/opt/yearofbingo/.env` is `0600` and
  owned by `deploy`.
- `cloud-init.yaml` embeds a cloudflared tunnel UUID and an R2 key *placeholder*;
  the actual tunnel/R2 credentials are provisioned out-of-band — confirm they are
  never committed and that `BACKUP_ENCRYPTION_KEY` is set before backups run.
- `WriteTimeout: 95s` is needed for AI calls but lengthens slow-loris exposure;
  Cloudflare in front mitigates. Keep `ReadTimeout` at 15s (good).

---

## Things verified as SOUND (no action needed)
- **SQL injection:** all queries parameterized; the only dynamic SQL
  (`notification.go`, notification-settings updates) builds **column names from a
  hard whitelist** (`isNotificationSettingsColumnAllowed`) with values bound as
  parameters.
- **Stripe webhook:** `VerifyStripeSignatureWithTolerance` parses `t`/`v1`,
  enforces a 300s replay tolerance, HMAC-SHA256, **constant-time** compare;
  events are idempotent via `WithWebhookEvent`. Webhook is CSRF-exempt correctly.
- **Authorization / IDOR:** card/template/friend/reaction services take the
  caller `userID` and enforce ownership (`ErrNotCardOwner`), friendship status,
  and block lists in SQL. No object is fetched by ID alone for mutation.
- **OAuth:** state + nonce cookies (`HttpOnly`, constant-time compared), nonce
  passed to OIDC verify, `sanitizeNext` blocks absolute/scheme-relative/encoded
  `//` redirects.
- **Tokens:** session and API tokens are 32 bytes from `crypto/rand`, stored only
  as SHA-256 hashes; API tokens are scoped and expiry-checked.
- **Static files:** `http.FileServer` (path-traversal safe); share/OG image
  tokens validated as 64-hex before use.
- **Prompt injection / AI cost:** user inputs are length-capped, sanitized, and
  XML-escaped before the Gemini prompt; system instruction rejects embedded
  commands; unverified users capped at 5 free generations; per-user Redis rate
  limits (fail-closed) on AI and redeem endpoints. Request bodies capped (1MiB
  global, 8KiB on AI generate).
- **Headers/CSP:** strict CSP (no `unsafe-inline`), `X-Frame-Options: DENY`,
  `frame-ancestors 'none'`, nosniff, Referrer-Policy, HSTS in secure mode.

---

## Suggested remediation order
1. **H-1** — fix session revocation (per-user index or session epoch) + test.
2. **M-1** — switch to `CF-Connecting-IP` / right-most XFF; set `TRUSTED_PROXY_CIDRS`.
3. **M-2** — stop leaking dependency errors on `/health`.
4. **M-3 / L-3 / L-4** — bind CSRF to session, enable DB TLS, add absolute session lifetime.
5. **L-1/L-2/L-5/L-6** — host pinning, escape support-email fields, flatten login timing, self-host CDN assets.

## Remediation status (2026-06-29)

Implemented in code (with tests):

- **H-1 (fixed):** `DeleteAllUserSessions` now stamps a per-user
  `users.sessions_invalidated_at` cutoff (migration `000024`). Session values in
  Redis embed their creation time (`<userID>|<unixCreatedAt>`); `ValidateSession`
  rejects any session created at/before the cutoff — including Redis-only sessions
  with no Postgres row. Legacy bare-userID values are treated as predating the
  cutoff, so they are revoked on the first password change/reset and otherwise keep
  working (no forced logout on deploy). Regression test:
  `TestAuthService_DeleteAllUserSessions_RevokesRedisSession`.
- **M-1 (fixed):** `httpx.ClientIP` now prefers `CF-Connecting-IP`, then the
  **right-most** `X-Forwarded-For` entry (never the left-most), then `X-Real-IP`.
  Spoofing the rate-limit key by prepending a fake XFF entry no longer works.
- **M-2 (fixed):** `/health` returns a generic `"unhealthy"` per dependency; the
  detailed error is logged server-side only.
- **M-3 (fixed):** CSRF tokens are now signed (HMAC-SHA256, per-process secret) —
  a signed double-submit. A planted cookie+header pair with no valid signature is
  rejected. Legacy unsigned cookies are transparently upgraded on the next safe
  request, and the frontend already refetches+retries on a CSRF 403, so there is
  no breakage on deploy.
- **L-1 (fixed):** In production, canonical/OG URLs are pinned to the configured
  `APP_BASE_URL` via `handlers.SetCanonicalBaseURL`, ignoring `Host` /
  `X-Forwarded-Host`.
- **L-2 (fixed):** `renderSupportEmail` now HTML-escapes `fromEmail` and
  `category` in addition to the message.
- **L-4 (fixed):** An absolute 90-day session lifetime is enforced from the
  embedded creation time, independent of the sliding TTL.
- **L-5 (fixed):** `Login` runs a dummy bcrypt comparison on the
  unknown-email / no-password paths to flatten the enumeration timing channel.

Intentionally deferred (operational / deploy-risk — *not* changed here):

- **L-3 (DB TLS):** Flipping `DB_SSLMODE=require` would break prod because the
  `postgres:16-alpine` container is not configured with TLS certs. Enabling it
  requires provisioning server certs first; tracked as ops follow-up.
- **M-1 (TRUSTED_PROXY_CIDRS):** Left at the default (trust loopback/RFC1918).
  Pinning the wrong cloudflared peer CIDR would collapse all rate-limit buckets to
  one key; `CF-Connecting-IP` is the real spoofing fix and ingress is already
  Cloudflare-only.
- **L-6 (self-host cdnjs/Font Awesome):** Vendoring these (incl. all webfont
  files) risks breaking icons/styles in prod and is better done as a dedicated,
  separately-verified change. SRI already mitigates the supply-chain surface.
- **L-7 / L-8:** Informational; no code change. L-8 items are server/file-permission
  hygiene handled at deploy time.

## How findings were validated
- Read auth/session/CSRF/rate-limit/billing/OAuth/AI/image/email handlers and
  services; traced `CreateSession` vs `DeleteAllUserSessions` storage paths and
  the `RedisClient` interface surface (no enumeration primitive).
- Traced `ClientIP` → `TrustedProxyChecker` defaults → `compose.server.yaml`
  (no `TRUSTED_PROXY_CIDRS`) → cloudflared loopback peer.
- Grepped for `fmt.Sprintf`/string-built SQL, inline scripts/handlers, CSP
  weakening, and external resource references (SRI confirmed).
- Reviewed `cloud-init.yaml` firewall (Cloudflare-only ingress) and container
  (non-root `appuser`, scratch-ish alpine runtime).
- Static review only; no exploitation performed against the live host.
