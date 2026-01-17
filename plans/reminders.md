# Email Reminders Plan

## Goal
Help users follow through on their goals with **positive, configurable email reminders**:

1. **Monthly (or periodic) “Card Check‑in” emails**: include current progress, a card image, and a short list of suggested goals to knock out next.
2. **Per‑goal reminders**: users can opt into reminders for individual goals (bingo items) with simple presets and optional custom scheduling.

The experience should be **opt‑in, low‑friction, non‑spammy**, and easy to adjust or turn off.

## Non‑Goals (initial scope)
- Push notifications (APNs/FCM) and native mobile integrations
- Complex segmentation/marketing automation
- Cross‑channel campaigns (SMS, in‑app banners) beyond minimal “settings saved” feedback
- Delivering reminders for anonymous (non‑account) cards

## Product + UX Spec

### Reminder surfaces
1. **Profile → Reminders** section (`#profile`):
   - Master toggle: “Email reminders” (default OFF)
   - “Card check‑ins” subsection
   - “Goal reminders” subsection (list + management)
   - “Send a test email” button (strongly recommended for confidence + E2E determinism)
2. **Goal detail modal** (on a finalized card):
   - “Remind me” action with friendly presets:
     - Tomorrow morning
     - Next week
     - Next month
     - Custom date/time
   - If reminders exist: “Edit reminder” + “Stop reminders for this goal”

### Gating / safety defaults
- Require `users.email_verified = true` to enable reminder emails (same gating as notification emails).
- Default state is OFF.
- Rate safety:
  - At most **1 card check‑in email per user per day**.
  - At most **N goal reminder emails per user per day** (start with N=3; configurable).

### “Current bingo card” definition
Users can have multiple cards; avoid surprising emails.

**Decision**: card check‑ins are **per card**, but the UI should also support “apply to all cards” for ease of use.

Recommendation:
- In the “Card check‑ins” section, offer:
  - “This card” (per-card schedule), and
  - “All my cards” (bulk apply the same schedule across eligible cards).
- “Eligible cards”: the user’s **finalized, non‑archived** cards (exclude drafts and archived cards).
- Keep a “default card” picker only for the “Send test email” action if needed.

### Card check‑in email content
Keep it short and upbeat:
- Subject examples:
  - “Your Year of Bingo check‑in”
  - “A quick bingo nudge for {Month}”
- Body:
  - Progress summary (e.g., “8/24 complete · 1 bingo”)
  - Card image (see “Card image delivery”)
  - “Suggested next goals” (up to 3) + “Open my card” button
  - Manage reminders link (`/profile`)
  - One‑click unsubscribe link (public, tokenized)

### Goal reminder email content
- Subject examples:
  - “Reminder: {goal}”
  - “A quick nudge: {goal}”
- Body:
  - Goal text (escaped)
  - Card title/year
  - “Open this goal” deep link that opens the item modal
  - Snooze links (optional): “Remind me tomorrow / next week”
  - Manage reminders link + unsubscribe link

### Recommendations (“which goals to knock out”)
Prioritize goals that help users get to a bingo sooner:
- Compute “closest to bingo” lines (rows/cols/diagonals) and pick incomplete items from lines with the fewest remaining.
- Rank candidate items by how many near‑complete lines they participate in.
- Exclude completed items and FREE space.
- Fallback if the algorithm yields none: pick 1–3 incomplete items (oldest first or random).

### Card image delivery
To reliably display an image in email clients without implementing multi‑provider attachments/MIME:

**Requirement**: the image must be visible in the email body (no attachments).

**Preferred**: tokenized, short‑lived public PNG URL embedded as `<img src="...">`, served through Cloudflare for caching.
- Create an expiring, random token tied to `{user_id, card_id, show_completions}`.
- Serve the PNG at `GET /r/img/{token}.png` (or similar), with `Cache-Control: public, max-age=3600`.
- Token expiry: 7–14 days is enough for email opens and forwards while limiting long‑term exposure.

#### PNG generation (V1: implementable without `plans/png.md`)
Implement a minimal server-side renderer dedicated to reminder emails (share-links can reuse it later, but reminders should not depend on that plan).

**Approach**: render the PNG on-demand in the `GET /r/img/{token}.png` handler.
- Token lookup loads `{user_id, card_id, show_completions, expires_at}` from `reminder_image_tokens`.
- Load the card + items (and optionally stats) from DB.
- Render a PNG (e.g., `1200x630`) with:
  - Header: card title (or “{year} Bingo Card”)
  - Grid: supports 2x2, 3x3, 4x4, 5x5 (based on `bingo_cards.grid_size`)
  - FREE space cell when enabled
  - Completion styling (filled cell background and/or checkmark) when `show_completions = true`
  - Simple stats line: `X/Y complete · Z bingos`
- Return `Content-Type: image/png` and caching headers. Cloudflare can cache per-token URL.

**Font/text rendering**: Go stdlib alone is not enough to render wrapped cell text. Choose one:
1. Add `golang.org/x/image` and use `font` + `opentype` with an embedded Go font (no font files needed):
   - `golang.org/x/image/font`
   - `golang.org/x/image/font/opentype`
   - `golang.org/x/image/font/gofont/goregular` (or similar)
2. Use `github.com/fogleman/gg` as in `plans/png.md` (higher-level API, faster to implement).

Either option is acceptable for reminders; pick the simplest for this repo and keep the renderer in a reusable package.

**Suggested code shape**:
- `internal/services/card_image.go` (or `internal/services/image.go`): `RenderReminderPNG(card models.BingoCard, items []models.BingoItem, opts RenderOptions) ([]byte, error)`
  - Word-wrap: measure words with the font drawer; clamp lines (e.g. 3–4) and add `…` when truncated.
  - Escaping is not relevant for PNG, but do not log raw goal text in debug logs.
- `internal/handlers/reminder_image.go`: validates token, loads card, calls renderer, writes response.
- `cmd/server/main.go`: register public route `GET /r/img/{token}.png` without auth (token is the gate).

**Performance**: rendering on-demand is fine initially; Cloudflare caching makes repeated opens cheap. If needed, add an in-process LRU keyed by token.

#### Optional: use Cloudflare R2 (only if needed)
If you want to offload image requests from the app server later:
- Generate the PNG **once** at email-send time.
- Upload to R2 with a random object key derived from the same token (e.g. `reminders/{token}.png`).
- Configure an R2 lifecycle rule to delete objects older than 14 days.
- Serve via a Cloudflare-backed public URL and embed that URL directly in the email.

This keeps the “no attachments” requirement and avoids implementing signed URLs, while remaining safe because the object key is unguessable and short-lived.

Later, `plans/png.md` (share links) can reuse the same renderer, but reminders should be fully implementable without it.

## Data Model (PostgreSQL)

### Table: `reminder_settings` (user‑level)
One row per user; stores global email reminder preferences.

Columns (suggested):
- `user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE`
- `email_enabled BOOLEAN NOT NULL DEFAULT false`
- `daily_email_cap INT NOT NULL DEFAULT 3`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Backfill in migration:
- `INSERT INTO reminder_settings (user_id) SELECT id FROM users ON CONFLICT DO NOTHING;`

### Table: `card_checkin_reminders` (per‑card schedule)
One row per configured card check‑in schedule.

Columns (suggested):
- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`
- `card_id UUID NOT NULL REFERENCES bingo_cards(id) ON DELETE CASCADE`
- `enabled BOOLEAN NOT NULL DEFAULT true`
- `frequency TEXT NOT NULL` (start with `monthly`; add `weekly`/`daily` later)
- `schedule JSONB NOT NULL DEFAULT '{}'::jsonb` (versioned schedule payload; extensible without schema churn)
- `include_image BOOLEAN NOT NULL DEFAULT true`
- `include_recommendations BOOLEAN NOT NULL DEFAULT true`
- `next_send_at TIMESTAMPTZ` (computed in UTC)
- `last_sent_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Constraints/indexes:
- `UNIQUE (user_id, card_id)` (one check‑in schedule per card)
- Index: `CREATE INDEX idx_card_checkin_due ON card_checkin_reminders(next_send_at) WHERE enabled = true;`

### Table: `goal_reminders` (per‑item)
Stores reminders for individual bingo items.

Columns (suggested):
- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`
- `card_id UUID NOT NULL REFERENCES bingo_cards(id) ON DELETE CASCADE`
- `item_id UUID NOT NULL REFERENCES bingo_items(id) ON DELETE CASCADE`
- `enabled BOOLEAN NOT NULL DEFAULT true`
- `kind TEXT NOT NULL` (`one_time`, `weekly`, `monthly`)
- `schedule JSONB NOT NULL DEFAULT '{}'::jsonb` (extensible; e.g. one-time datetime, or weekly day/time)
- `next_send_at TIMESTAMPTZ` (UTC)
- `last_sent_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Constraints/indexes:
- `UNIQUE (user_id, item_id)` (one active reminder per goal to keep UX simple)
- Index: `CREATE INDEX idx_goal_reminders_due ON goal_reminders(next_send_at) WHERE enabled = true;`

### Table: `reminder_image_tokens` (short‑lived public image access)
Used to render a card PNG inside emails.

Columns (suggested):
- `token VARCHAR(64) PRIMARY KEY` (URL‑safe random)
- `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`
- `card_id UUID NOT NULL REFERENCES bingo_cards(id) ON DELETE CASCADE`
- `show_completions BOOLEAN NOT NULL DEFAULT true`
- `expires_at TIMESTAMPTZ NOT NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `last_accessed_at TIMESTAMPTZ`
- `access_count INT NOT NULL DEFAULT 0`

Indexes:
- `CREATE INDEX idx_reminder_image_tokens_expires ON reminder_image_tokens(expires_at);`

### Table: `reminder_email_log` (optional but recommended)
Audits sends and prevents duplicates across multiple server instances.

Columns (suggested):
- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`
- `source_type TEXT NOT NULL` (`card_checkin`, `goal_reminder`)
- `source_id UUID NOT NULL`
- `sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `provider_message_id TEXT` (nullable)
- `status TEXT NOT NULL` (`sent`, `failed`)

Constraints:
- `UNIQUE (source_type, source_id, sent_at::date)` for check‑ins (or a more direct “one per day” policy)

## Backend Implementation (Go)

### Services
1. `ReminderService`
   - Settings:
     - `GetSettings(ctx, userID)`
     - `UpdateSettings(ctx, userID, patch)`
   - Card check‑ins:
     - `UpsertCardCheckin(ctx, userID, cardID, schedule)`
     - `DeleteCardCheckin(ctx, userID, cardID)`
   - Goal reminders:
     - `ListGoalReminders(ctx, userID, cardID?)`
     - `UpsertGoalReminder(ctx, userID, itemID, schedule)`
     - `DeleteGoalReminder(ctx, userID, reminderID)`
   - Runner:
     - `RunDue(ctx, now time.Time, limit int) (sent int, err error)` (used by ticker and tests)

2. Email composition helpers
   - Reuse `EmailService.SendNotificationEmail(to, subject, html, text)` for reminder emails (it already accepts pre‑rendered content).
   - Add a dedicated builder for reminder email HTML/text to keep copy consistent and escaped.

3. PNG/image generation
   - Reuse the generator proposed in `plans/png.md` (or implement a minimal `GenerateCardPNG(...)` used by both share links and reminder images).

### Runner / scheduling approach
Add a periodic loop in `cmd/server/main.go` (similar to notification cleanup):
- Poll interval: `REMINDERS_POLL_INTERVAL` (default 1m; in test containers, can be 1s).
- Each run:
  - Claim due rows using `FOR UPDATE SKIP LOCKED` in a transaction.
  - For each claimed reminder:
    - Re‑load necessary card/item state.
    - Skip if:
      - user email is unverified,
      - global reminders are disabled,
      - card/item is deleted,
      - the goal is already completed (for goal reminders),
      - rate caps would be exceeded.
    - Send email best‑effort; record in `reminder_email_log`.
    - Compute and update `next_send_at` (or disable a one‑time reminder after sending).

This approach prevents double‑send in multi‑instance deployments without needing Redis locks.

### Extensible scheduling design
Implement scheduling behind a small internal API so adding weekly/daily is localized:
- Parse `frequency` + `schedule` JSON into a typed struct per frequency.
- Validate inputs at write time (handlers) and re-validate at send time (defense-in-depth).
- Compute `next_send_at` using:
  - a frequency-specific `Next(after time.Time) (time.Time, error)` function.
Monthly V1 schedule payload example:
- `{"day_of_month": 1, "time": "09:00"}` interpreted in **server time** (no user timezones). Clamp day to 1–28 initially.

Default schedule choices (for ease):
- Day of month: `1`
- Time: `09:00` server time (or whichever is easiest to implement consistently)

### Cleanup jobs
- Daily cleanup (can piggyback the same ticker infrastructure):
  - Delete expired `reminder_image_tokens`
  - Delete old `reminder_email_log` rows (e.g., older than 90 days)

## HTTP Endpoints

### Session‑only API (used by SPA)
Add handlers under `/api/reminders/*` (registered in `cmd/server/main.go` and documented in `web/static/openapi.yaml`):
- `GET /api/reminders/settings`
- `PUT /api/reminders/settings`
- `GET /api/reminders/cards` (list available cards + existing card check‑in config)
- `PUT /api/reminders/cards/{cardId}` (upsert card check‑in schedule)
- `DELETE /api/reminders/cards/{cardId}`
- `GET /api/reminders/goals?card_id=...` (list goal reminders)
- `POST /api/reminders/goals` (create/upsert goal reminder by `item_id`)
- `DELETE /api/reminders/goals/{id}`
- `POST /api/reminders/test` (send a test check‑in email for the selected/default card)

### Public endpoints (no auth)
- `GET /r/img/{token}.png` (serves reminder card PNG)
- `GET /r/unsubscribe?token=...` (one‑click disable reminder emails)

Unsubscribe token design options:
1. Store tokens in DB (simpler revocation/rotation, slightly more schema).
2. HMAC‑signed stateless token `{user_id, purpose, exp}` using server secret.

## Frontend Implementation (Vanilla JS SPA)

### API client
Add `API.reminders.*` methods in `web/static/js/api.js`.

### UI: Profile → Reminders
In `App.renderProfile()`:
- Add a “Reminders” section:
  - Email verified gating (disabled toggle + helper text)
  - Card selector + schedule controls (per card)
  - “Apply this schedule to all my cards” action (bulk update)
  - Goal reminder list (next send time, card title, goal text, edit/delete)
  - “Send test email” action and toast confirmation

### UI: Goal modal
In `App.showItemDetailModal()`:
- Add “Remind me” controls using `data-action` delegation.
- Use DOM APIs + `textContent` for user strings.

### Deep link for “Open this goal”
Add a simple path query contract:
- `/card/{id}?item={itemId}`
On route load, open the modal for that item.

## Email Templates
- Keep templates inline in Go (like existing email templates) or move to dedicated template files; either is fine as long as content is escaped.
- Do not render untrusted strings as raw HTML:
  - goal content, card title, username must be escaped (use the same helper used in notification emails).

## Testing Plan

### Go unit + handler tests
- `internal/services/reminder_test.go`:
  - schedule computation (monthly day‑of‑month; timezone handling)
  - due selection uses `SKIP LOCKED` and doesn’t double‑send
  - completed goals skip/disable reminders
  - rate caps enforced
  - recommendation picker returns stable results and excludes completed/FREE
- `internal/handlers/reminder_test.go`:
  - auth required
  - email verified gating for enabling
  - validation errors (invalid time/day)

### Playwright E2E
Add scenarios and update `plans/playwright.md` coverage outline:
1. User verifies email → enables reminders in profile → clicks “Send test email” → email arrives in Mailpit.
2. Create an XSS goal like `"<img src=x onerror=alert(1)>"`, set a goal reminder, send test email, and assert the email body contains escaped text (`&lt;img`) and not an actual `<img`.
3. Email includes a PNG URL; fetch it via Playwright `request` and assert response is `image/png` (and starts with PNG magic bytes).

## Rollout Notes
- Start with monthly check‑ins + one‑time goal reminders; add repeating goal reminders after validating engagement.
- Keep defaults conservative (OFF; require verified email; caps).
- Instrument logs/metrics for send success/failure and “disabled by unsubscribe”.

## Open Questions
None (requirements clarified):
- Default send time: `09:00` server time.
- “Apply to all cards”: only finalized, non‑archived cards.
- No timezone support.
