# AI Goal Assist in Card Editor (Post-Create) — Implementation Plan

## Status
- Implemented (backend, frontend, tests, and E2E coverage)
- Empty-cell clicks open the shared add/edit modal; AI assist lives in that modal
- Removed the extra “AI (1)” button; AI assist remains in-editor only

## Problem
The AI Goal Wizard solves the “blank canvas” problem when creating a card, but users often get stuck on *1–2 remaining goals* during draft editing. This feature adds lightweight, in-context AI help inside the editor (without forcing a full wizard flow).

## Goals
- Generate 1–5 *candidate* goals that the user can pick from.
- Refine an existing goal in-place while preserving its core meaning.
- Add a new goal quickly (generate → pick → insert into the existing add/edit flows).
- Keep AI access session-only, preserve unverified gating, rate limiting, and usage logging.

## Non-Goals
- Editing card metadata (title/category/grid/header/FREE) via AI.
- Auto-writing multiple cells automatically (that’s the wizard/Fill workflow).
- Persisting AI drafts/sessions to the database.

## Constraints & Requirements
- **Session-only** access (no API tokens): same rule as `/api/ai/generate`.
- Preserve **verified-email gating** (5 free generations for unverified) and return `free_remaining` like the wizard.
- Preserve Redis-backed **AI rate limiting** (same middleware as `/api/ai/generate`).
- Preserve **AI usage logging** (`ai_generation_logs`) and do not store user prompt content.
- Reuse existing **prompt-injection safeguards**: `sanitizeInput` + `escapeXMLTags` boundaries.
- Deterministic behavior under `AI_STUB=1` for E2E.
- TDD: new behavior ships with tests; coverage must not decrease.

## UX (Editor)
This plan keeps AI in existing edit flows and adds empty-cell editing for draft cards.

### 1) Refine in “Edit Goal” modal (MVP)
Entry: click a filled cell → existing “Edit Goal” modal.
- Add a compact “AI Assist” section below the textarea:
  - Hint input (optional): “What should change?” (shorter, cheaper, more local, etc.)
  - Button: “🧙 Refine with AI”
  - Results: 3 clickable suggestions (radio/list buttons)
  - Clicking a suggestion replaces the textarea content (user can still edit).
  - Existing “Save” continues to call the current update-item flow.

Why this helps: users already use this modal when they’re stuck; AI becomes a one-click rewrite tool.

### 2) Add goal from an empty cell (MVP)
Entry: click an empty draft cell.
- Open the same modal used for editing goals (blank textarea).
- “Save/Add” inserts the goal into the clicked position using `POST /api/cards/{id}/items` with `position`.

## API Contract
Add a new endpoint:
- `POST /api/ai/guide` (OpenAPI path: `/ai/guide`)

Request:
- `mode`: `"refine"` | `"new"`
- `current_goal`: string (required when `mode=refine`, otherwise omitted/empty)
- `hint`: string (optional)
- `count`: integer (optional; default: 3 for refine, 5 for new; max 5)
- `avoid`: array of strings (optional; existing goal texts/titles to discourage duplicates; truncated server-side)

Response:
- `goals`: array of strings
- `free_remaining`: integer (optional, only meaningful for unverified users)

Example:
```json
{
  "mode": "refine",
  "current_goal": "Visit a local farmer's market",
  "hint": "make it more budget-friendly",
  "count": 3,
  "avoid": ["Market Mission: ...", "Coffee Crawl: ..."]
}
```

## Backend Plan (Go)

### 1) Service: generate guide goals
Implement in `internal/services/ai/gemini.go` following the existing `GenerateGoals` request/parse pattern (no new “generic request runner” required for MVP):

- New types:
  - `type GuidePrompt struct { Mode string; CurrentGoal string; Hint string; Count int; Avoid []string }`
- New method:
  - `GenerateGuideGoals(ctx, userID, prompt) ([]string, UsageStats, error)`

Behavior:
- Validate:
  - `mode` must be `refine|new`
  - `count` defaults by mode, clamp `1..5`
  - `refine` requires non-empty `current_goal`
- Sanitize user inputs:
  - `current_goal`, `hint`, and each `avoid[]` item via `sanitizeInput` + `escapeXMLTags`
  - Keep all goal text within the existing item limits (<= 500 chars).
- Prompt rules (match the wizard’s tone and constraints):
  - Impersonal imperative phrasing, no “you/your/you’re”
  - Output JSON array of strings (exactly `count` items)
  - For refine: “preserve meaning” and keep format similar to current goal (“Title: short description” when possible)
  - For new: produce distinct options; discourage duplicates from `avoid`
- Determinism under `AI_STUB=1`:
  - Return stable strings based on `mode/current_goal/hint` and requested `count`.
- Logging + safety:
  - Use existing safety settings.
  - Log usage via the same `logUsageWithTimeout` pattern used by `GenerateGoals`.

### 2) Handler: `POST /api/ai/guide`
Add `internal/handlers/ai_guide.go` (or extend `internal/handlers/ai.go`) with:
- Strict JSON decoding (`MaxBytesReader`, `DisallowUnknownFields`)
- Input limits aligned to item constraints:
  - `current_goal` and `hint`: max 500 chars each (rune-aware if convenient, but consistent with existing patterns)
  - `avoid`: max 24 items; each max ~100 chars after trimming (server-side truncate)
- Authentication required (session-only enforced by middleware)
- Unverified gating mirrors `AIHandler.Generate`:
  - call `ConsumeUnverifiedFreeGeneration`
  - return `403` with `free_remaining: 0` when exhausted
  - refund on provider unavailable/not configured/rate limit errors (same refund logic)
- Error mapping mirrors `AIHandler.Generate` (`ErrSafetyViolation`, `ErrRateLimitExceeded`, `ErrAINotConfigured`, `ErrAIProviderUnavailable`)

### 3) Routing + middleware
Register in `cmd/server/main.go` using the same chain as `/api/ai/generate`:
- `requireSession(aiRateLimiter.Middleware(http.HandlerFunc(aiHandler.Guide)))`

### 4) OpenAPI
Update `web/static/openapi.yaml`:
- Add `/ai/guide` with `cookieAuth` security.
- Document request/response, including `free_remaining`.

## Frontend Plan (Vanilla JS)

### 1) API client
Extend `web/static/js/api.js`:
- Add `API.ai.guide(mode, currentGoal, hint, count, avoid)` which calls `API.request('POST', '/api/ai/guide', ...)` with a long timeout (match wizard’s `100000`).

### 2) Shared gating UI
Reuse the existing gating UX from `web/static/js/ai-wizard.js`:
- If `AIWizard.isVerificationRequiredForAI()` is true, show `AIWizard.showVerificationRequiredModal()`.
- On success/failure, if response includes `free_remaining`, update `App.user.ai_free_generations_used` the same way the wizard does.

### 3) Refine UI in edit modal (MVP)
Modify `App.showItemOptions` in `web/static/js/app.js`:
- Add hint input + “Refine with AI” button + a results list region.
- `Generate` calls `API.ai.guide('refine', currentText, hint, 3, avoidList)` where `avoidList` can be derived from `App.currentCard.items` (trimmed).
- Clicking a suggestion updates the existing textarea value.

### 4) Empty cell add modal (MVP)
Modify `App.showItemOptions` in `web/static/js/app.js` to allow empty-cell clicks.
- Empty cell opens a blank goal modal.
- Save adds the goal at the clicked position via `POST /api/cards/{id}/items` with `position`.

### 5) Styles
Prefer existing modal + button styles; add only small CSS if needed for the suggestions list layout.

## Testing Plan

### Backend unit tests (required)
- `internal/services/ai/guide_test.go`
  - validation: bad mode → `ErrInvalidInput`
  - refine requires current goal
  - stub: deterministic outputs with `count`
  - request shaping: response parsing count enforcement + trimming
- `internal/handlers/ai_guide_test.go` (mirror `internal/handlers/ai_test.go` structure)
  - 401 when no user in context
  - 400 for invalid body / invalid mode / missing current_goal / count out of bounds
  - unverified gating returns `free_remaining`
  - error mapping + refund behavior on retryable failures

### Playwright E2E (required)
Add `tests/e2e/ai-guide-editor.spec.js`:
- Refine flow: open draft editor → click filled cell → “Refine with AI” → pick suggestion → Save → cell updates.
- Empty-cell add flow: click empty cell → modal opens → add goal → cell updates.

Update coverage outline in `plans/playwright.md`.

## Acceptance Criteria
- Authenticated draft editor shows:
  - “Refine with AI” controls in the “Edit Goal” modal and can insert a picked suggestion into the textarea.
- Empty draft cell click opens a modal and can add a goal into that specific cell.
- Anonymous draft editor does not call AI endpoints; clicking AI assist shows `App.showAIAuthModal()`.
- `/api/ai/guide`:
  - Rejects API-token auth (403) via `RequireSession` middleware, like `/api/ai/generate`.
  - Enforces unverified gating and returns `free_remaining` on success and gating-related errors.
  - Produces deterministic output when `AI_STUB=1` so E2E is stable.
- New E2E spec passes under `AI_STUB=1` and exercises refine + empty-cell add flows.

## Implementation Notes (To Avoid Rework)
- Avoid list construction (frontend): use `(App.currentCard.items || []).map(i => i.content)` excluding the currently-edited goal; trim each entry and cap to 24 items; cap each string to ~100 chars before sending.
- UI state: keep AI-assist transient state in the modal DOM (IDs + event listeners) rather than in `App` global state, since the editor re-renders frequently.
- Add stable selectors for E2E:
  - Edit modal: `id="ai-refine-hint"`, `id="ai-refine-generate"`, `id="ai-refine-results"`
  - Each suggestion button: `data-ai-suggestion="0|1|2|..."`.
- Handler wiring: `internal/handlers/ai.go`’s `AIService` interface will need a new method; update `internal/handlers/ai_test.go` mock accordingly.
- Error strings: mirror existing AI handler wording (“Invalid request body”, “Invalid mode”, “We couldn't generate safe goals for that topic. Please try rephrasing.”, etc.) to keep UX consistent and tests predictable.

## Implementation Sequence
1) [x] Add service tests for guide generation.
2) [x] Implement `GenerateGuideGoals` + stub output.
3) [x] Add handler tests.
4) [x] Implement handler + route registration + OpenAPI update.
5) [x] Add Playwright spec.
6) [x] Implement editor UI changes in `app.js` + API client in `api.js`.
7) [x] Run `./scripts/test.sh`, then `make e2e` (AI stub enabled).

## Open Questions
- Resolved: include `avoid` to reduce duplicates in refine/new results.
