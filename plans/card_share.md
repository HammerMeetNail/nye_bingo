# Card Sharing (Public, View-Only Links)

## Overview

Allow a signed-in user to generate a **shareable link** for a specific bingo card. Anyone with the link can **view** the card (read-only) without being a registered user or signed in.

This is an **unlisted** sharing model: possession of the link grants access.

## Goals

- Card owner can **enable sharing** for a card and copy a share link.
- Card owner can **disable sharing** (revoke link) and then re-enable to rotate the link.
- Anyone with the link can view:
  - Card title/year/category (as appropriate)
  - Grid header + size + FREE-space behavior
  - Items and completion state (view-only)
- Shared view never allows edits (no item add/remove/shuffle/swap/complete/notes).
- Sharing does not change existing friend visibility behavior; it is an explicit opt-in “anyone with link”.
- Strong focus on security (unguessable tokens) and tests (TDD-first).

## Non-Goals

- “Discoverable” public profiles or listing shared cards.
- Search engines indexing cards (aim for `noindex` on shared view).
- Fine-grained sharing permissions (edit access, per-item permissions).
- Sharing via user-to-user invitations (friends already covered separately).

## Key Decisions (Recommended Defaults)

- **Only finalized cards are shareable** (simpler; avoids leaking draft iteration and prevents ambiguity about “live editing”).
- Shared view includes **completion state** but not **private notes**.
- Tokens are **long-lived by default** (no expiration), but the owner can optionally set an expiration.
- One active share link per card (disable + re-enable rotates it).

## User Stories

1. As a card owner, I can enable sharing and copy a link.
2. As a card owner, I can revoke the link so it stops working immediately.
3. As a card owner, I can disable sharing and create a new link (old one stops working, new one works).
4. As an anonymous visitor, I can open a shared link and view the card.
5. As an anonymous visitor, I cannot modify the card and I do not see editing UI.
6. As a security measure, shared pages do not execute user-provided HTML/JS (XSS regression coverage required).

## Data Model

### New table: `bingo_card_shares`

Store one active share record per card.

Recommended schema:

```sql
CREATE TABLE bingo_card_shares (
  card_id UUID PRIMARY KEY REFERENCES bingo_cards(id) ON DELETE CASCADE,
  token VARCHAR(64) NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ,
  last_accessed_at TIMESTAMPTZ,
  access_count INT NOT NULL DEFAULT 0
);
```

Notes:
- `token` should be generated using `crypto/rand` and encoded as lowercase hex (similar to reminder tokens).
- `VARCHAR(64)` supports 32 random bytes encoded as hex (`randomToken(32)` → 64 chars).
- `expires_at` is nullable; `NULL` means “never expires”.
- `last_accessed_at/access_count` are optional; if implemented, update them in a throttled way (e.g., only once per hour) to reduce DB writes on popular links.

Migration files:
- `migrations/000018_card_shares.up.sql`
- `migrations/000018_card_shares.down.sql`

## API Design

### Owner (session-only) endpoints

These are UI-driven and should use `requireSession` middleware (consistent with other “account-only UI ops”).

1) **Enable sharing / regenerate link**
- `POST /api/cards/{id}/share`
- Auth: session (cookie)
- Behavior:
  - Verify card belongs to user.
  - Verify card `is_finalized == true` (recommended default).
  - Generate new token and upsert into `bingo_card_shares`.
  - If request includes an expiration, persist it; otherwise default to no expiration.
  - Return share token + metadata; client builds the URL.

Request (example):
```json
{ "expires_in_days": 30 }
```

Response (example):
```json
{
  "enabled": true,
  "url": "abc123...",
  "created_at": "2026-01-01T00:00:00Z",
  "expires_at": "2026-01-31T00:00:00Z"
}
```

2) **Get sharing status**
- `GET /api/cards/{id}/share`
- Auth: session
- Behavior:
  - If no share exists: `{ "enabled": false }`
  - If exists: return `{ enabled, url, created_at, expires_at, last_accessed_at, access_count }` (where `url` is the share token)

3) **Disable sharing**
- `DELETE /api/cards/{id}/share`
- Auth: session
- Behavior:
  - Delete row from `bingo_card_shares` for `card_id`.

### Public (no-auth) endpoint

4) **Fetch a shared card**
- `GET /api/share/{token}`
- Auth: none
- Behavior:
  - Look up `bingo_card_shares.token == token`, load referenced card + items.
  - Enforce share constraints:
    - Card must still exist.
    - Card must be finalized (if adopting the “finalized only” rule).
    - Share must not be expired (`expires_at IS NULL OR expires_at > NOW()`).
  - Return a **public-safe** card payload:
    - Include only data needed for rendering the card.
    - Exclude user identifiers and internal-only fields.
    - Exclude private notes (recommended).

Response shape should be intentionally separate from the authenticated “full card” response to reduce accidental data leaks, e.g.:
```json
{
  "card": {
    "id": "uuid",
    "year": 2026,
    "title": "My 2026 Bingo",
    "category": "Personal",
    "grid_size": 5,
    "header_text": "BINGO",
    "has_free_space": true,
    "free_space_position": 12,
    "is_finalized": true
  },
  "items": [
    { "position": 0, "text": "…", "is_completed": false },
    { "position": 1, "text": "…", "is_completed": true }
  ]
}
```

Security headers:
- Return `Cache-Control: no-store` (tokenized resources; safer by default).
- Consider adding a `X-Robots-Tag: noindex` header (or `<meta name="robots" ...>` in index template if easier).

### OpenAPI

- Document owner endpoints in `web/static/openapi.yaml` (session cookie auth model is currently implicit in Swagger; still document request/response).
- Document `GET /api/share/{token}` as **no security**.

Also add a short note in the spec that “Shared links are unlisted; possession of token grants read-only access.”

## Backend Implementation (Go)

### Service layer

Add methods to `internal/services/card.go` (or a focused `internal/services/card_share.go` if preferred):

- `CreateOrRotateShare(ctx, userID, cardID, expiresAt *time.Time) (token string, createdAt time.Time, expiresAt *time.Time, err error)`
- `GetShareStatus(ctx, userID, cardID) (*models.CardShare, err error)`
- `RevokeShare(ctx, userID, cardID) error`
- `GetSharedCardByToken(ctx, token) (*models.PublicSharedCard, error)`

Implementation notes:
- Use `crypto/rand` + hex encoding for token generation.
- Owner checks should reuse existing card ownership helpers already in `CardService`.
- Public load should use a dedicated query that:
  - Joins share → card → items
  - Orders items by `position`
  - Omits notes

### Handler layer

Add handler methods in `internal/handlers/card.go` or a new `internal/handlers/card_share.go`:

- `POST /api/cards/{id}/share`
- `GET /api/cards/{id}/share`
- `DELETE /api/cards/{id}/share`
- `GET /api/share/{token}`

Wire routes in `cmd/server/main.go`.

## Frontend Implementation (Vanilla JS SPA)

### Routing

Add a public route, e.g. `#share/{token}`:
- Extend existing hash parsing/routing logic to recognize `share` as a public route (works without `App.user`).
- Add `App.renderSharedCard(token)` which:
  - Calls `API.share.get(token)` (new API method).
  - Renders using the finalized-card renderer in **read-only** mode.
  - Displays a badge like “Shared view” and hides edit actions.

### UI for owners

Add a “Share” action for finalized card view:
- Button: “Share” (opens modal).
- Modal states:
  - Sharing disabled → “Enable sharing” button.
  - Sharing enabled → show link + “Copy” + “Disable”.
  - Expiration controls only appear when sharing is disabled.
  - When sharing is enabled, show the remaining days until expiration as read-only text (not editable).
  - To change expiration, the owner must disable sharing and create a new link.
  - Expiration controls (when disabled):
    - Default selection: “Never expires”
    - Optional presets: 7/30/90 days
    - (If easy) a custom number-of-days input with validation
  - Confirmation prompt before “Disable” (disabling invalidates the old link).

Implementation constraints:
- No inline scripts/handlers; use the existing `data-action` event delegation pattern.
- When rendering user content, always use `textContent` (or `App.escapeHtml` if building strings).

### API client additions

In `web/static/js/api.js` add:
- `API.cards.shareStatus(cardID)`
- `API.cards.shareEnable(cardID)` (or `shareRotate`)
- `API.cards.shareDisable(cardID)`
- `API.share.get(token)` (public fetch)

## Testing Plan (TDD-First)

All steps below assume running tests via `./scripts/test.sh` (containerized).

### 1) Backend unit tests (services)

Add tests before implementation for:
- Token generation:
  - Length/charset (hex, expected length).
  - Uniqueness (basic probabilistic test: generate N tokens, ensure no duplicates in-memory).
- `CreateOrRotateShare`:
  - Rejects non-owner.
  - Rejects non-finalized (if adopting finalized-only).
  - Sets `expires_at` when provided, and clears it when omitted (default no-expire).
  - Upserts and returns a URL-safe token.
- `RevokeShare`:
  - Idempotent behavior (revoke when none exists should succeed or return a typed not-found—choose one and test it).
- `GetSharedCardByToken`:
  - Not found returns typed error → handler maps to 404.
  - Expired token returns typed error → handler maps to 404 (avoid leaking “expired vs revoked”).
  - Returns only public-safe fields; does not include notes.

Pattern: use the existing `fakeDB` approach in `internal/services/*_test.go` (see `internal/services/api_token_test.go`).

### 2) Backend handler tests (httptest)

Add handler tests before implementation for:
- `POST /api/cards/{id}/share`:
  - 401 when not authenticated.
  - 403/404 for non-owner.
  - 400 when card not finalized (if enforced).
  - 200 returns `{url}` when successful.
- `GET /api/share/{token}`:
  - 200 for valid token.
  - 404 for invalid/revoked token.
  - 404 for expired token.
  - Response contains no user PII fields.
  - Response uses correct content-type and cache headers (`no-store`).

### 3) Frontend unit tests (JS runner)

Add/extend tests in `web/static/js/tests/runner.js` for:
- Hash parsing for `#share/{token}` route.
- Any new utility used for generating/copying share URLs (if implemented as pure functions).

### 4) Playwright E2E (required)

Add a new spec, e.g. `tests/e2e/card-share.spec.js`:
- Flow:
  1. Register/login.
  2. Create a card, add a known XSS payload item: `"<img src=x onerror=alert(1)>"`.
  3. Finalize card.
  4. Enable sharing, copy link.
  5. Open a new browser context (logged out) and navigate to the share link.
  6. Assert:
     - Card renders and contains the payload as text (no `<img>` nodes created in grid).
     - No edit controls are visible (no “Add item”, no shuffle, no finalize, no completion toggles).
- Add a second test for revocation:
  - After revoking, visiting the old link shows a “Not found / link revoked” message.
- Add a third test for expiration (optional but recommended):
  - Enable sharing with a short expiry, then simulate time passing (or set `expires_at` in DB via test helper if available), and assert link no longer works.

Update `plans/playwright.md` to include the new coverage outline entry.

## Step-by-Step Implementation Order (TDD)

1. Add failing service tests for share token creation/revocation/public load.
2. Add migration `000018_card_shares` (up/down) and run container tests to apply.
3. Implement `CardService` share methods until unit tests pass.
4. Add failing handler tests (owner + public endpoints), then implement handlers/routes.
5. Update `web/static/openapi.yaml` for new endpoints/schemas.
6. Add JS route + API methods + minimal UI (Share modal) with unit tests.
7. Add Playwright spec(s) + update `plans/playwright.md`, then iterate until E2E passes.
8. Final validation: `./scripts/test.sh --coverage` to ensure coverage does not decrease.

## Open Questions (Need Product Decision)

1. For expiration configuration, is “presets + optional custom days” sufficient, or do you want an explicit “expires on date” picker?
2. Should expired links show a distinct “Expired” message, or always the same generic 404-style messaging?
