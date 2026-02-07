# Premium AI Enhancements (Simple Monthly Allowance + Goal Assistant) Plan

## Overview

Add Premium-only AI features that help users **plan and accomplish their bingo goals**, while keeping:
- **Session-only auth** for all AI endpoints (cookie; no API tokens).
- The existing **verified-email gating** (unverified users get 5 total AI generations tracked by `users.ai_free_generations_used`).
- **Privacy constraints**: do not store user prompt fields (focus/context/notes/conversations) in the DB.
- **Simple, explicit UX**: users can understand exactly what they’re buying.

This plan is **additive**: existing functionality stays free and unchanged. Premium adds extra AI capabilities behind a clear monthly allowance.

**Prerequisites**
- Billing + entitlements foundation exists (see `plans/monetization.md`).
- Existing AI endpoint `/api/ai/generate` remains session-only and works as implemented.

## Current Status (Implemented)

As of **February 7, 2026**, this feature set is implemented in code:
- Migration shipped as `migrations/000023_ai_premium_enhancements.*.sql`.
- Premium AI endpoints shipped:
  - `GET /api/ai/premium/status`
  - `POST /api/ai/assist`
  - `POST /api/ai/regenerate`
  - `POST /api/ai/fill-empty`
- Feature gating + flagging are active:
  - entitlement gate: `billing.FeatureAIEnhancements` / `billing.HasFeature(...)`
  - global kill switch: `FEATURE_AI_ENHANCEMENTS_ENABLED`
- Dedicated premium AI rate limiter is active via `AI_PREMIUM_ENDPOINT_RATE_LIMIT`.
- Frontend UI shipped for Goal Assistant, per-goal Regenerate, AI Fill Empty, and usage meter.
- Playwright E2E coverage shipped in `tests/e2e/ai-premium.spec.js`.

Post-implementation handler correctness fixes were also applied:
- required-field presence checks for `assist.position` and `regenerate.replace_index`
- rollover-safe premium refund accounting by reusing reservation timestamp/month on refund

---

## User-Facing Product Promise (Keep It Simple)

Premium includes **`N` “AI Enhancements” per month**.
- Display this explicitly in the UI: “AI Enhancements remaining: 73 / 100 (resets Feb 1)”.
- **Each** Premium AI action consumes **exactly 1** enhancement:
  - Goal Assistant request
  - Regenerate a goal
  - AI Fill Empty Squares

**Important:** The existing AI Goal Wizard `/api/ai/generate` is not changed by this plan (still uses the existing limits and email-verification gating). Premium is selling additional AI capabilities, not taking anything away.

Recommended starting value:
- `N = 100` AI Enhancements / month

---

## Premium AI Features

### Feature 1: Goal Assistant / Ideator (Strictly On-Goal)

Premium users can select a specific bingo item and ask for help completing it.

Strictness requirement:
- The assistant must only discuss the **selected goal** and how to accomplish it.
- If the user asks about unrelated topics, the assistant refuses and redirects to goal-relevant help.

UX:
- Add “Goal Assistant” to the item detail modal (where users see/complete an item).
- The assistant uses a guided “mode” selector (reduces off-topic prompts):
  - `breakdown` (turn goal into steps)
  - `next_step` (pick the next action)
  - `obstacles` (identify blockers + mitigations)
  - `schedule` (suggest a realistic schedule)
  - `ideas` (brainstorm variations that still satisfy the goal)
  - `motivation` (accountability + momentum)
- Optional text field: “Constraints / notes” (max 500 chars).

**Mode enum (canonical; must match frontend + backend + OpenAPI)**
Use these exact strings:
- `breakdown`
- `next_step`
- `obstacles`
- `schedule`
- `ideas`
- `motivation`

Backend validation sketch:
```go
var validAssistModes = map[string]bool{
  "breakdown": true,
  "next_step": true,
  "obstacles": true,
  "schedule": true,
  "ideas": true,
  "motivation": true,
}
```

Each request consumes 1 AI Enhancement.

### Feature 2: Regenerate This Goal (Wizard Swap)

In the AI goal wizard results list, Premium users can click “Regenerate” on a specific goal.
- Returns exactly one replacement goal.
- Must not duplicate any goal already on the list.
- Must preserve style constraints (short, bingo-friendly).

Each regenerate consumes 1 AI Enhancement.

### Feature 3: AI Fill Empty Squares (Draft Card)

Premium users can fill remaining empty squares on a **draft** card using AI.
- Generates exactly `N` goals where `N == number_of_empty_positions` (excluding FREE).
- Inserts them into empty positions atomically.

Each fill operation consumes 1 AI Enhancement.

---

## Configuration (Env Vars)

Add:
- `AI_PREMIUM_ENHANCEMENTS_PER_MONTH` (default: `100`)
- `AI_PREMIUM_ENDPOINT_RATE_LIMIT` (optional; default: `60` per hour)  
  Protective anti-abuse limiter for premium endpoints only (not marketed as a product limit).
- `FEATURE_AI_ENHANCEMENTS_ENABLED` (default: `true`)  
  Global premium AI feature switch (kill switch without disabling billing globally).

Keep existing:
- `AI_RATE_LIMIT` (current `/api/ai/generate` limiter)

---

## Database Plan

### Migration (implemented): `000023_ai_premium_enhancements.*.sql`
This plan originally proposed `000016_*`; the implemented migration number is `000023_*`.

#### 1) Track premium AI enhancements usage per month (UTC)
```sql
CREATE TABLE ai_premium_usage_monthly (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    month_start DATE NOT NULL, -- UTC month start, e.g. 2026-01-01
    enhancements_used INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, month_start)
);
```

#### 2) Extend `ai_generation_logs` to track feature
We already log token usage and status. Add a simple `feature` tag so we can audit premium usage patterns without storing prompts:
```sql
ALTER TABLE ai_generation_logs
  ADD COLUMN feature VARCHAR(30) NOT NULL DEFAULT 'generate';

CREATE INDEX idx_ai_logs_user_feature_date ON ai_generation_logs(user_id, feature, created_at);
```

Feature values:
- `generate` (existing wizard)
- `regenerate`
- `fill_empty`
- `assist`

---

## Backend Implementation Plan (TDD + Security)

## Implementation Details (Concrete Enough for Agentic Implementation)

This section is intentionally copy/paste oriented for a less-capable implementation agent.

### File Changes Summary

**Backend**
- Modify `internal/config/config.go` to add env vars:
  - `AI_PREMIUM_ENHANCEMENTS_PER_MONTH`
  - `AI_PREMIUM_ENDPOINT_RATE_LIMIT`
- Add migration `migrations/000023_ai_premium_enhancements.up.sql` + down migration.
- Modify `internal/services/ai/errors.go` to add Premium allowance errors.
- Modify `internal/services/ai/gemini.go` to add new AI methods + logging feature field.
- Modify `internal/handlers/ai.go` to add new endpoints.
- Modify `cmd/server/main.go` to register routes and attach a dedicated rate limiter for premium AI endpoints.
- Modify `web/static/openapi.yaml` to document new endpoints + schemas.

**Frontend**
- Modify `web/static/js/api.js` to add API calls.
- Modify `web/static/js/app.js` to add UI:
  - Premium AI meter display
  - Goal Assistant modal
  - Regenerate button per goal
  - AI Fill Empty button

### New/Updated Errors (Go)

Add to `internal/services/ai/errors.go`:
```go
var (
  ErrPremiumEnhancementsExhausted = errors.New("premium AI enhancements exhausted") // 429
  ErrPremiumRequired              = errors.New("premium required")                 // 403 (or use generic handler error)
)
```

### DB Helpers (UTC Month Start)

Use UTC month boundaries so behavior is predictable and easy to communicate:
```go
func monthStartUTC(t time.Time) time.Time {
  utc := t.UTC()
  return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func nextMonthStartUTC(t time.Time) time.Time {
  ms := monthStartUTC(t)
  return ms.AddDate(0, 1, 0)
}
```

### Premium Allowance SQL (Atomic Reserve + Best-Effort Refund)

Reserve (increment) 1 enhancement if under limit:
```sql
-- args: $1 user_id, $2 month_start (date), $3 limit (int)
WITH upsert AS (
  INSERT INTO ai_premium_usage_monthly (user_id, month_start, enhancements_used)
  VALUES ($1, $2, 1)
  ON CONFLICT (user_id, month_start)
  DO UPDATE SET
    enhancements_used = ai_premium_usage_monthly.enhancements_used + 1,
    updated_at = NOW()
  WHERE ai_premium_usage_monthly.enhancements_used < $3
  RETURNING enhancements_used
)
SELECT enhancements_used FROM upsert;
```

If no row is returned, the user is at the monthly limit.

Refund (decrement) 1 enhancement after a failed provider call or failed DB write:
```sql
-- args: $1 user_id, $2 month_start
UPDATE ai_premium_usage_monthly
SET enhancements_used = GREATEST(enhancements_used - 1, 0),
    updated_at = NOW()
WHERE user_id = $1 AND month_start = $2;
```

Status query:
```sql
SELECT enhancements_used
FROM ai_premium_usage_monthly
WHERE user_id = $1 AND month_start = $2;
```

### Make It Unit-Testable (No DB Required)
The repo’s Go tests run without a database (see `scripts/test.sh`), so premium allowance logic must be testable with fakes.

Add a small storage interface and provide:
- a Postgres implementation (uses the SQL above), and
- an in-memory implementation for tests.

Recommended interface:
```go
type PremiumUsageStore interface {
  GetUsed(ctx context.Context, userID uuid.UUID, monthStart time.Time) (used int, ok bool, err error)
  ReserveOne(ctx context.Context, userID uuid.UUID, monthStart time.Time, limit int) (used int, ok bool, err error) // ok=false when at limit
  RefundOne(ctx context.Context, userID uuid.UUID, monthStart time.Time) error
}
```

In-memory test store sketch (thread-safe):
```go
type memoryUsageStore struct {
  mu   sync.Mutex
  used map[string]int // key: userID + ":" + monthStart(YYYY-MM-01)
}

func (m *memoryUsageStore) key(userID uuid.UUID, monthStart time.Time) string {
  return userID.String() + ":" + monthStart.Format("2006-01-02")
}

func (m *memoryUsageStore) ReserveOne(_ context.Context, userID uuid.UUID, monthStart time.Time, limit int) (int, bool, error) {
  m.mu.Lock()
  defer m.mu.Unlock()
  k := m.key(userID, monthStart)
  if m.used == nil {
    m.used = map[string]int{}
  }
  if m.used[k] >= limit {
    return m.used[k], false, nil
  }
  m.used[k]++
  return m.used[k], true, nil
}
```

### AI Service Method Signatures (Recommended)

Extend `internal/services/ai/Service` with:
```go
// Premium allowance
func (s *Service) GetPremiumEnhancementsStatus(ctx context.Context, userID uuid.UUID, now time.Time, limit int) (used int, resetsAt time.Time, err error)
func (s *Service) ReservePremiumEnhancement(ctx context.Context, userID uuid.UUID, now time.Time, limit int) (remaining int, resetsAt time.Time, err error)
func (s *Service) RefundPremiumEnhancement(ctx context.Context, userID uuid.UUID, now time.Time) error

// Premium AI features
func (s *Service) RegenerateGoal(ctx context.Context, userID uuid.UUID, prompt GoalPrompt, existingGoals []string, replaceIndex int) (string, UsageStats, error)
func (s *Service) AssistGoal(ctx context.Context, userID uuid.UUID, goalText string, mode string, notes string) (string, UsageStats, error)
func (s *Service) GenerateFillGoals(ctx context.Context, userID uuid.UUID, prompt GoalPrompt, existingGoals []string) ([]string, UsageStats, error)
```

Notes:
- `RegenerateGoal` should request exactly 1 goal (string schema) and include an “avoid duplicates” list in the prompt.
- `GenerateFillGoals` requests exactly `N` goals where `N == empties`.
- `AssistGoal` does not accept a free-form user question; only `mode` and `notes`. This is the primary strictness guardrail.

### Gemini Schema Support (If Needed)

Today `geminiSchema` only supports arrays. For the assistant strict response, the easiest approach is to **still request a JSON string** (no object schema) and treat it as the assistant reply.

If you want the `{is_on_goal, reply}` object schema, extend `geminiSchema`:
```go
type geminiSchema struct {
  Type       string                  `json:"type"`
  Items      *geminiSchema           `json:"items,omitempty"`
  Properties map[string]*geminiSchema `json:"properties,omitempty"`
  Required   []string                `json:"required,omitempty"`
}
```

Then you can enforce:
```json
{
  "type":"object",
  "required":["is_on_goal","reply"],
  "properties":{
    "is_on_goal":{"type":"boolean"},
    "reply":{"type":"string"}
  }
}
```

### Handler Order of Operations (Important)

For each premium AI endpoint:
1. Parse + validate request (MaxBytesReader, DisallowUnknownFields).
2. `user := GetUserFromContext(ctx)`; if nil → 401.
3. Check Premium entitlement (server-side); if not premium → 403.
4. If `!user.EmailVerified` → call `ConsumeUnverifiedFreeGeneration` (enforces the 5-free-unverified cap).
5. Reserve 1 Premium Enhancement (enforces monthly allowance).
6. Call AI provider.
7. If provider fails → refund enhancement and return provider error.
8. On success → log to `ai_generation_logs` with the right `feature` and return success with `enhancements_remaining`.

This ensures we never spend AI provider calls when the user is already over limit, and we don’t “charge” an enhancement for failed calls.

### Phase 1: Premium AI Allowance Service (Atomic Consumption)

Files:
- `internal/services/ai/gemini.go` (add methods) OR extract usage into `internal/services/ai/usage.go`

Add:
- `GetPremiumEnhancementsStatus(ctx, userID) (limit int, used int, resetsAt time.Time, remaining int, err error)`
- `ConsumePremiumEnhancement(ctx, userID, feature) (remaining int, resetsAt time.Time, err error)`

Rules:
- Determine `month_start` in **UTC** (first day of current month).
- Use a DB transaction to enforce the monthly limit:
  1) `SELECT ... FOR UPDATE` the `(user_id, month_start)` row (create if missing)
  2) if `enhancements_used >= limit`, return `ErrPremiumEnhancementsExhausted`
  3) increment `enhancements_used` by 1 and commit
- Return remaining + reset timestamp (first day of next UTC month).

Premium gating:
- Handlers must enforce Premium entitlement before calling `ConsumePremiumEnhancement`.
- (Optional) Add a defense-in-depth check in `ConsumePremiumEnhancement` that confirms user is Premium based on the billing fields in `users`.

### Phase 2: Premium AI Endpoint Rate Limiter (Anti-Abuse)

Files:
- `cmd/server/main.go`
- `internal/middleware/ratelimit.go` (reuse existing RateLimiter)

Add a dedicated Redis rate limiter for premium AI endpoints:
- prefix: `ratelimit:ai-premium:`
- limit: `AI_PREMIUM_ENDPOINT_RATE_LIMIT` per hour
- key: user ID (fallback to IP if needed)
- fail-closed (cost-sensitive)

**Route registration sketch (matches `cmd/server/main.go` patterns)**
```go
aiPremiumRateLimit := int64(60)
if v, ok := os.LookupEnv("AI_PREMIUM_ENDPOINT_RATE_LIMIT"); ok && v != "" {
  if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
    aiPremiumRateLimit = parsed
  }
}

aiPremiumLimiter := middleware.NewRateLimiter(redisDB.Client, aiPremiumRateLimit, 1*time.Hour, "ratelimit:ai-premium:", func(r *http.Request) string {
  user := handlers.GetUserFromContext(r.Context())
  if user != nil {
    return user.ID.String()
  }
  return ""
}, false)

// Premium AI endpoints are session-only, rate-limited, and still CSRF-protected.
mux.Handle("GET /api/ai/premium/status", requireSession(http.HandlerFunc(aiHandler.PremiumStatus)))
mux.Handle("POST /api/ai/assist", requireSession(aiPremiumLimiter.Middleware(http.HandlerFunc(aiHandler.Assist))))
mux.Handle("POST /api/ai/regenerate", requireSession(aiPremiumLimiter.Middleware(http.HandlerFunc(aiHandler.Regenerate))))
mux.Handle("POST /api/ai/fill-empty", requireSession(aiPremiumLimiter.Middleware(http.HandlerFunc(aiHandler.FillEmpty))))
```

### Phase 3: New Endpoint — Premium AI Status (For UI Meter)

Files:
- `internal/handlers/ai.go`
- `web/static/openapi.yaml`
- `cmd/server/main.go`

Endpoint:
- `GET /api/ai/premium/status`
- Auth: session-only
- Response:
```json
{
  "limit": 100,
  "used": 27,
  "remaining": 73,
  "resets_at": "2026-02-01T00:00:00Z"
}
```

### Phase 4: New Endpoint — Goal Assistant (Premium)

Files:
- `internal/handlers/ai.go` (or `internal/handlers/ai_assist.go`)
- `internal/services/ai/gemini.go` (new `AssistGoal(...)`)
- `internal/services/card.go` (read card/item ownership; reuse existing patterns)
- `web/static/openapi.yaml`
- `cmd/server/main.go`

Endpoint:
- `POST /api/ai/assist`
- Auth: session-only
- Premium: required
- Consumes 1 AI Enhancement on success.
- Email verification gating: preserve existing rule (unverified users still limited to 5 total AI generations).

Request:
```json
{
  "card_id": "uuid",
  "position": 7,
  "mode": "breakdown",
  "notes": "Time: weekends. Budget: low."
}
```

Response:
```json
{
  "reply": "1) ... 2) ... 3) ...",
  "enhancements_remaining": 72,
  "resets_at": "2026-02-01T00:00:00Z"
}
```

Validation:
- `mode` must be one of the known enum values.
- `notes` max 500 chars; trim and normalize whitespace; escape `<` `>` (same as existing AI sanitization).
- `position` must be valid for the card’s `grid_size` and must not be the FREE position.
- Card must be owned by the user; item must exist at that position.

Prompting (strict):
- The server loads the authoritative goal text from the card item content.
- System instruction explicitly forbids off-topic conversation.
- Use structured output to reduce prompt injection:
  - response schema as JSON object: `{ "is_on_goal": boolean, "reply": string }`
  - If `is_on_goal` is false, return a standard refusal message and do not include unrelated content.
- Keep replies short and actionable (e.g., max ~1200 chars).

### Phase 5: Update Endpoint — Regenerate Single Goal (Premium)

Endpoint:
- `POST /api/ai/regenerate`
- Premium required + consumes 1 AI Enhancement on success.
- Keeps existing validation and “avoid duplicates” rules.

### Phase 6: New Endpoint — AI Fill Empty Squares (Premium)

Endpoint:
- `POST /api/ai/fill-empty`
- Premium required + consumes 1 AI Enhancement on success.

Request:
```json
{
  "card_id": "uuid",
  "category": "mix",
  "difficulty": "medium",
  "budget": "free",
  "focus": "",
  "context": ""
}
```

Behavior:
- Load card + ensure owned by user and `is_finalized=false`.
- Compute empty positions excluding FREE.
- Generate exactly `N` goals where `N == empties`.
- Insert items into those positions in one transaction.
- Return updated card + remaining enhancements.

### Error Handling (Consistent + Clear)

Return explicit, user-friendly errors:
- Not premium: `403 {"error":"Premium required"}`
- Monthly enhancements exhausted: `429 {"error":"Premium AI enhancements limit reached for this month","resets_at":"..." }`
- Unverified and out of free AI: reuse existing AI error message about email verification requirement.

---

## Frontend Implementation (Positive UX, No Surprises)

### API Client
File: `web/static/js/api.js`
- Add:
  - `API.ai.getPremiumStatus()`
  - `API.ai.assistGoal(cardId, position, mode, notes)`
  - `API.ai.regenerate(...)`
  - `API.ai.fillEmpty(...)`

### UI: Premium AI Meter
Where to show:
- Profile: “AI Enhancements remaining”
- Wizard + Goal Assistant modals: show remaining inline

### UI: Goal Assistant Modal
- Invoked from item detail modal.
- Mode selector + notes textbox.
- Output shown as a short checklist.
- If user isn’t Premium: show upgrade modal.
- If monthly allowance is exhausted: show a clear message + suggest using non-AI features (curated suggestions, manual notes).

---

## OpenAPI Updates (Required)

File: `web/static/openapi.yaml`
- Add:
  - `GET /ai/premium/status`
  - `POST /ai/assist`
  - update `/ai/regenerate` and `/ai/fill-empty`
- All must use `cookieAuth` and explicitly state “API tokens are not allowed”.

**Concrete OpenAPI sketch (copy/paste shape)**
```yaml
  /ai/premium/status:
    get:
      summary: Get Premium AI enhancement allowance status
      description: Requires a browser session cookie (API tokens are not allowed).
      security:
        - cookieAuth: []
      responses:
        '200':
          description: Status
          content:
            application/json:
              schema:
                type: object
                properties:
                  limit: { type: integer }
                  used: { type: integer }
                  remaining: { type: integer }
                  resets_at: { type: string, format: date-time }

  /ai/assist:
    post:
      summary: Goal Assistant (Premium)
      description: Requires a browser session cookie (API tokens are not allowed).
      security:
        - cookieAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [card_id, position, mode]
              properties:
                card_id: { type: string, format: uuid }
                position: { type: integer }
                mode:
                  type: string
                  enum: [breakdown, next_step, obstacles, schedule, ideas, motivation]
                notes: { type: string, maxLength: 500 }
      responses:
        '200':
          description: Assistant reply
          content:
            application/json:
              schema:
                type: object
                properties:
                  reply: { type: string }
                  enhancements_remaining: { type: integer }
                  resets_at: { type: string, format: date-time }
```

---

## Testing Plan (TDD)

### Go Tests
Create a table-driven test matrix. Suggested minimum coverage:

**Allowance**
- `ReservePremiumEnhancement`:
  - first use creates row and returns remaining
  - increments used up to limit
  - returns `ErrPremiumEnhancementsExhausted` at limit
- `RefundPremiumEnhancement`:
  - decrements after a simulated provider failure
  - never decrements below 0
- Month boundary:
  - a reservation on `YYYY-MM-31T23:59:59Z` counts toward that month
  - a reservation on `YYYY-MM-01T00:00:00Z` counts toward the new month

**Premium gating**
- non-premium user:
  - `/api/ai/premium/status` returns 403 (or returns limit=0 if you choose that design; be consistent)
  - `/api/ai/assist` returns 403

**Email verification gating**
- unverified user:
  - can call premium endpoint 5 times total (across `/generate` and premium endpoints)
  - 6th call returns the same “verify email” error as `/api/ai/generate`

**Goal Assistant strictness**
- invalid `mode` returns 400
- `notes` > 500 chars returns 400
- FREE position returns 400
- missing item at position returns 404

**Handler/provider failure fairness**
- if provider returns 503/429, the endpoint returns an error and the enhancement reservation is refunded (remaining increases back).

**No-network tests**
- Use a mocked HTTP transport (like existing `internal/services/ai/gemini_test.go`) so no network calls occur.

**Example test skeletons (copy/paste shape)**
```go
func TestPremiumEnhancements_ReserveAndRefund(t *testing.T) {
  userID := uuid.New()
  now := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
  store := &memoryUsageStore{} // from the plan above

  limit := 2
  monthStart := monthStartUTC(now)

  // reserve #1
  used, ok, err := store.ReserveOne(context.Background(), userID, monthStart, limit)
  if err != nil { t.Fatalf("reserve1 err: %v", err) }
  if !ok { t.Fatalf("reserve1 expected ok") }
  if used != 1 { t.Fatalf("reserve1 used=%d", used) }

  // reserve #2
  used, ok, err = store.ReserveOne(context.Background(), userID, monthStart, limit)
  if err != nil { t.Fatalf("reserve2 err: %v", err) }
  if !ok { t.Fatalf("reserve2 expected ok") }
  if used != 2 { t.Fatalf("reserve2 used=%d", used) }

  // reserve #3 should fail (at limit)
  _, ok, err = store.ReserveOne(context.Background(), userID, monthStart, limit)
  if err != nil { t.Fatalf("reserve3 err: %v", err) }
  if ok { t.Fatalf("reserve3 expected !ok at limit") }

  // refund one (simulate provider failure)
  if err := store.RefundOne(context.Background(), userID, monthStart); err != nil {
    t.Fatalf("refund err: %v", err)
  }
}
```

### Run
- `./scripts/test.sh`

---

## Rollout

1. Enable Premium for a few accounts via codes/manual grant.
2. Validate:
   - meter correctness
   - refusal behavior for off-topic assistant prompts
   - monthly exhaustion UX
3. Launch once Stripe billing is live.
