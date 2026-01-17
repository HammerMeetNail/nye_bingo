# Delete Account + Export Data

## Overview
Add a **Data Export** section and a separate **Danger Zone** section (delete-only) on `#profile` that lets an authenticated user:
1) **Export all of their data** (download), and
2) **Permanently delete their account** (irreversible).

This plan is phased and written for an AI agent to implement with **TDD** (unit tests first, then code, then E2E).

## Goals
- User can export a complete, self-contained dataset of “their data” from within the profile page.
- User can delete their account from within the profile page with **clear warnings** and **strong confirmation checks** (GitHub-style typed confirmation).
- Deletion is **immediate** and **irreversible** from the product’s point of view.
- Account deletion is a **soft delete**: scrub user PII and disable access; do not hard-delete the user’s content rows.
- Implementation follows existing patterns:
  - Go stdlib handlers + service layer (`internal/services/*`).
  - SPA `App` object + `data-action` event delegation (no inline handlers).
  - New API routes registered in `cmd/server/main.go` and documented in `web/static/openapi.yaml`.
  - Tests run in containers via `./scripts/test.sh` and E2E via `make e2e`.

## Non-goals (explicitly out of scope)
- “Grace period” recovery / undo.
- Admin tooling for account deletion.
- Background “delete later” jobs.
- Export formats beyond ZIP/JSON (CSV is optional as an extra convenience, not required for “all data”).

## UX / Product Spec

### Placement
- In `App.renderProfile()`, add **two** cards at the end of the profile page, with **Delete Account last**:
  1) **Export Your Data** (recommended before deleting)
  2) **Danger Zone: Delete Account** (final option at bottom)
- The export feature is independent of deletion:
  - Export does not require opening the delete modal or typing confirmations.
  - Export does not change or delete any data.
  - Export remains available even if the user never intends to delete the account.

### Export UX
- Export card copy:
  - Title: “Export Your Data”
  - Description: “Download a ZIP of CSV files containing your account data (cards, items, friends, reminders, notifications, etc.).”
  - Button: “Download Export”
- On click:
  - Show a loading state on the button.
  - Download a ZIP file from the server (preferred: server generates ZIP/CSVs).
  - Show toast: “Export downloaded” (or error toast).

### Delete UX (GitHub-style typed confirmation)
- Danger Zone card styling:
  - Red accent/border and warning copy in-page (not just in a modal).
  - Primary destructive CTA: “Delete Account”
- Clicking “Delete Account” opens a modal with:
  - A prominent warning banner: “This will permanently delete your account and all associated data. This cannot be undone.”
  - A checklist of what will be deleted (cards, items, friends, reminders, notifications, API tokens, shares).
  - A typed confirmation input that must match the user’s username (GitHub-style):
    - Label: `Type "<username>" to confirm`
    - Requirement: exact match after trimming whitespace (**case-sensitive**).
  - A password input:
    - Label: “Enter your password”
  - A checkbox:
    - Label: “I understand this action is permanent and cannot be undone.”
  - Delete button remains disabled until all checks pass.
- On successful deletion:
  - Clear session cookie (server).
  - UI transitions to logged-out state (App.user = null), route to `#home`.
  - Toast: “Account deleted”.

### Accessibility
- Modal focus management uses existing modal system.
- Inputs have `<label for>` and `autocomplete="current-password"` for the password field.
- Delete button has clear disabled/enabled states.

## API Spec (new endpoints)

### `GET /api/account/export` (session-only)
Purpose: return a ZIP file containing CSV exports of all data associated to the user.

- Auth: **session only** (`requireSession`), plus existing auth middleware must have a user in context.
- Method: `GET`
- Response `200`:
  - `Content-Type: application/zip`
  - `Content-Disposition: attachment; filename="yearofbingo_account_export_<YYYY-MM-DD>.zip"`

ZIP contents (minimum set; add new files if new user-associated tables are added later):
- `README.txt` (export version, generated_at, notes about excluded secret material)
- `user.csv`
- `cards.csv`
- `items.csv`
- `friendships.csv`
- `blocks.csv`
- `api_tokens.csv`
- `notification_settings.csv`
- `notifications.csv`
- `reminder_settings.csv`
- `card_checkin_reminders.csv`
- `goal_reminders.csv`
- `reminder_email_log.csv`
- `reminder_image_tokens.csv` (no token secret; metadata only)
- `reminder_unsubscribe_tokens.csv` (no token secret; metadata only)
- `email_verification_tokens.csv` (no token secret; metadata only)
- `password_reset_tokens.csv` (no token secret; metadata only)
- `ai_generation_logs.csv`
- `card_shares.csv` (no share token secret; metadata only)
- `friend_invites.csv` (no invite token secret; metadata only)
- `sessions.csv` (no session token/hash; metadata only, or omit entirely)

CSV conventions:
- UTF-8, comma separated, with header row.
- RFC3339 timestamps in UTC where applicable.
- Null values become empty cells.
- IDs exported as UUID strings.

Export must **exclude**:
- `password_hash`
- raw session tokens
- token values/hashes (`api_tokens.token_hash`, invite token hashes, password reset token hashes, share tokens, reminder image tokens, etc.)

If certain secret values are not stored in plaintext (e.g. token hashes), document them as “not exportable”.

### `DELETE /api/account` (session-only)
Purpose: permanently disable the authenticated account via **soft delete** and scrub PII.

- Auth: **session only** (`requireSession`), user must be in context.
- CSRF: required (existing middleware already enforces for mutating verbs).
- Request body:
```json
{
  "confirm_username": "user123",
  "password": "Password1",
  "confirm": true
}
```
- Response `200` JSON:
```json
{ "message": "Account deleted" }
```
- Idempotency:
  - If the request is authenticated and the account is already deleted (`users.deleted_at IS NOT NULL`) (e.g., concurrent retry/double-submit), return `200` with the same response body and clear the session cookie.
  - If the request is not authenticated (e.g., after cookie/session is cleared), return `401` as usual.
  - This is primarily to handle retries/double-submits safely; it does not imply any “undo” or additional side effects.
- Error cases:
  - `400` invalid request (missing fields, `confirm != true`, username mismatch)
  - `401` wrong password or not authenticated
  - `403` token-auth attempts (RequireSession)
  - `500` unexpected server errors

## Backend Implementation Details

## Soft Delete Specification (important)
Account deletion MUST be a soft delete:
- Add `users.deleted_at TIMESTAMPTZ` (nullable).
- When deleting:
  - Set `deleted_at = NOW()`.
  - Scrub PII and unique identifiers so they can be reused:
    - `email` → `deleted+<user_id>@deleted.invalid`
    - `username` → `deleted-<user_id>` (or `deleted-<shortid>-<user_id>` if length constraints exist)
    - `password_hash` → random non-matching string (disables password login)
    - `email_verified` → false, `email_verified_at` → NULL
    - `searchable` → false
  - Ensure scrubbing values are deterministic and guaranteed unique to satisfy existing uniqueness constraints (email UNIQUE, username unique index on `LOWER(username)`).
- The user must not be able to authenticate after deletion:
  - Update user lookups used by session/token auth to require `deleted_at IS NULL`.
  - Specifically update queries in:
    - `internal/services/user.go` (`GetByID`, `GetByEmail`, uniqueness checks)
    - `internal/services/auth.go` (`getUserByID` used by session validation)
    - Friend search / friend list / blocks / notifications code paths that join against `users` (must not display deleted users as discoverable/searchable).
- “Irreversible” means: there is no UI or API to restore; the scrubbed identifiers cannot be recovered.

Additional “disappear” requirements:
- Deleted users must not appear in:
  - friend search results
  - existing friends lists / pending request lists
  - invite flows that show inviter usernames
- Recommended implementation: add `users.deleted_at IS NULL` filters to all queries that surface user identities to other users, and treat deleted users as “not found”.

Share-link hard stop:
- Any existing card share links must stop working after account deletion.
- Implement by revoking share rows during delete (`DELETE FROM bingo_card_shares ...`) and by adding a defense-in-depth check in share resolution code: if the owning user has `deleted_at IS NOT NULL`, return 404 even if a token row somehow still exists.

Friends/visibility hard stop:
- Deleted users’ content must not be viewable by other users (friends or otherwise).
- Implement by:
  - Filtering deleted users out of friend search/lists, and
  - Enforcing `users.deleted_at IS NULL` on friend-card reads and any other “view another user’s card” code paths.

### New service: `internal/services/account.go`
Create an `AccountService` responsible for:
- `BuildExportZip(ctx, userID) ([]byte, error)` (CSV-in-ZIP, secrets excluded)
- `Delete(ctx, userID) error`

Implementation notes:
- Prefer single-purpose SQL queries with explicit column selection (no `SELECT *`).
- Export should be built from the DB as the source of truth; do not call handlers from services.
- Delete should:
  - Run in a DB transaction.
  - Apply the soft delete update to `users`.
  - Best-effort cleanup of **access paths** that could still expose user content:
    - Revoke API tokens (`DELETE FROM api_tokens WHERE user_id = $1`) so API access can’t continue.
    - Revoke share links (`DELETE FROM bingo_card_shares WHERE card_id IN (SELECT id FROM bingo_cards WHERE user_id = $1)`).
    - Revoke reminder image tokens and unsubscribe tokens (delete rows; they’re access tokens).
    - Delete active sessions in Postgres (`DELETE FROM sessions WHERE user_id = $1`).
  - Redis sessions on other devices are acceptable because user lookup will fail after `deleted_at` gating; they expire naturally.

### Export code structure (recommended)
Avoid building a huge in-memory “export struct” unless it’s genuinely helpful. Prefer:
- Small internal row structs per CSV (or write rows directly from scans).
- Shared helpers:
  - `writeCSV(zipFile io.Writer, header []string, rows [][]string) error`
  - `formatTime(*time.Time) string` (RFC3339 UTC)
  - `boolString(bool) string` (`true`/`false` or `yes`/`no`, pick one and keep consistent)

### New handler: `internal/handlers/account.go`
Add:
- `AccountHandler.Export(w, r)` → `GET /api/account/export`
- `AccountHandler.Delete(w, r)` → `DELETE /api/account`

Handler responsibilities:
- Fetch `user := handlers.GetUserFromContext(r.Context())` and reject if nil.
- For delete:
  - Decode JSON request.
  - Validate typed confirmation and required checkbox.
  - Verify password via `AuthService.VerifyPassword(user.PasswordHash, req.Password)`.
  - Best-effort delete the current session cookie token (`r.Cookie("session_token")` + `AuthService.DeleteSession`).
  - Call `AccountService.Delete(...)`.
  - Clear session cookie (same pattern as `AuthHandler.Logout`).
  - Return JSON `{message: "Account deleted"}`.
  - Idempotency note: `AccountService.Delete` should treat “already soft-deleted” as success.

### Routing + OpenAPI
- Register routes in `cmd/server/main.go`:
  - `GET /api/account/export` → `requireSession(accountHandler.Export)`
  - `DELETE /api/account` → `requireSession(accountHandler.Delete)`
- Document both endpoints in `web/static/openapi.yaml` (new schemas, request/response, security notes).

## Frontend Implementation Details

### API client additions (`web/static/js/api.js`)
Add:
- `API.account.export()` → `GET /api/account/export` and return a `Blob` (ZIP)
- `API.account.delete(confirmUsername, password, confirm)` → `DELETE /api/account` with JSON body

### Profile UI + actions (`web/static/js/app.js`)
In `renderProfile()`:
- Add the two final cards (Export, then Danger Zone).
- Use `data-action` hooks only:
  - Export button: `data-action="export-account"`
  - Delete button: `data-action="open-delete-account-modal"`

In `handleActionClick()` / `handleActionSubmit()`:
- Implement:
  - `export-account` click → `App.exportAccountData()`
  - `open-delete-account-modal` click → open modal containing a `<form data-action="delete-account">…</form>`
  - `delete-account` submit → validate inputs, call `API.account.delete(...)`, then clear local state and route to home.

ZIP download format:
- Filename: `yearofbingo_account_export_<YYYY-MM-DD>.zip`
- The server provides the ZIP; the client just triggers a download and shows a toast.

### Styles (`web/static/css/styles.css`)
Add minimal styles for a Danger Zone card:
- `.profile-danger` / `.danger-zone` (red border/background tint)
- `.danger-zone__warning` (prominent text)

## Phased Implementation (TDD-first)

### Phase 1 — Add soft delete schema + failing tests (backend)
**Goal:** lock in soft delete semantics and block auth for deleted users.
- Add migration: `ALTER TABLE users ADD COLUMN deleted_at TIMESTAMPTZ;`
- Add Go unit tests for auth gating:
  - After setting `deleted_at`, `AuthService.ValidateSession` must fail with “not authenticated” behavior (via `getUserByID` returning not found).
  - `UserService.GetByEmail/GetByID` must not return deleted users.

Files:
- Add new migration in `migrations/` (next numeric prefix)
- Update `internal/services/user.go`
- Update `internal/services/auth.go`
- Add/extend unit tests in `internal/services/auth_test.go` and `internal/services/user_service_test.go`

Acceptance:
- `./scripts/test.sh --go` passes and deleted users cannot authenticate.

### Phase 2 — Define routes + failing handler tests (backend)
**Goal:** lock in export + delete contracts before implementing full logic.
- Add Go unit tests for:
  - `AccountHandler.Delete` validation (missing fields, username mismatch, missing confirm checkbox).
  - `AccountHandler.Export` auth requirement (401 when unauthenticated).
- Add OpenAPI stubs (schemas + paths), documenting ZIP download for export.

Files:
- Add `internal/handlers/account_test.go` (new)
- Update `cmd/server/main.go` (routes)
- Update `web/static/openapi.yaml`

Acceptance:
- `./scripts/test.sh --go` passes with minimal placeholder implementations (export can return 501 until Phase 3).

### Phase 3 — Implement export ZIP endpoint (backend + unit tests)
**Goal:** export ZIP contains CSVs for all user-associated tables.
- Implement ZIP streaming in Go:
  - Use `archive/zip` to build ZIP in-memory (unit-testable) or stream directly to `http.ResponseWriter`.
  - Use `encoding/csv` for CSV generation (header row + records).
- Write service-level unit tests for `AccountService.ExportZip` (or similar) using `fakeDB`:
  - Ensures each CSV file is created with expected headers.
  - Ensures secrets are excluded (no hashes/tokens).
  - Ensures rows are scoped to the authenticated user.
- Handler unit tests:
  - Content-Type is `application/zip`.
  - Body starts with ZIP signature `PK\x03\x04`.

Files:
- Add `internal/services/account.go`
- Add `internal/services/account_service_test.go`
- Update/add `internal/handlers/account.go`

Acceptance:
- Container Go test suite passes; export endpoint returns real data in local dev.

### Phase 4 — Export UI (frontend + E2E)
**Goal:** user can download a ZIP containing CSVs.
- Implement `App.exportAccountData()`:
  - Calls `API.account.export()` and downloads the returned ZIP blob via existing `App.downloadBlob(...)`.
- Add Playwright spec `tests/e2e/account-export.spec.js`:
  - Register → go to `/profile` → click “Download Export”
  - Assert a download occurred and file name matches prefix
  - Sanity-check ZIP header bytes (`PK\x03\x04`) like existing bulk export coverage
  - Optional: assert the ZIP bytes contain ASCII file names like `user.csv` and `cards.csv` (string search on the binary buffer).

Also update coverage outline in `plans/playwright.md`.

Acceptance:
- `make e2e` passes with the new export spec.

### Phase 5 — Delete endpoint (backend + unit tests)
**Goal:** deleting account soft-deletes user and disables access.
- Add service tests for `AccountService.Delete`:
  - Uses a transaction (`DB.Begin`) and applies the `users` scrub + `deleted_at` update.
  - Revokes token-based access paths (api tokens, share tokens, reminder image tokens, sessions table).
  - Repeated deletes are safe (user already deleted → no-op update, cleanup queries remain safe).
- Add handler tests for delete:
  - Wrong password → `401`
  - Success → `200`, cookie cleared header present (match existing logout patterns)
  - Two sequential deletes (same authenticated session) → both return `200` with the same message
- Implement deletion:
  - Verify password in handler using `AuthService.VerifyPassword`.
  - Soft delete user in DB (scrub + `deleted_at`).

Acceptance:
- Go unit tests pass; manual smoke test: after delete, `GET /api/auth/me` returns 401.

### Phase 6 — Delete UI (frontend + JS unit tests)
**Goal:** strong confirmations, safe UX.
- Add modal UI + form with strict enable/disable logic:
  - Delete button disabled until username match + password non-empty + checkbox checked.
  - Display inline validation hints.
- Add JS unit tests for the confirm logic helper (if extracted), especially trimming rules.
- Implement `API.account.delete(...)` call and success flow:
  - Clear `App.user`, reset state, route to `/`, show toast.

Acceptance:
- Manual: cannot delete without typing username correctly; wrong password shows error.

### Phase 7 — E2E delete flow + irreversible assertions
**Goal:** ensure end-to-end deletion works and is irreversible.
- Add Playwright spec `tests/e2e/account-delete.spec.js`:
  - Register user, create a card, add at least one item, finalize (optional).
  - Visit `/profile`, open delete modal.
  - Attempt submit with wrong username → button disabled.
  - Enter correct username + password + check checkbox → delete.
  - Assert redirected/logged out (`Year of Bingo` home heading).
  - Attempt login with same credentials → fails (expect toast or error).
  - (Optional) `page.request` call to `/api/auth/me` to assert 401.
  - Add coverage that an **unverified** user can still export and delete.
  - Add coverage that existing share links stop working (create share before deletion, delete account, then visit share URL → not found).
- Update coverage outline in `plans/playwright.md`.

Acceptance:
- `make e2e` passes.

## Notes
- Delete endpoint is session-only and does not accept an email parameter; it is not intended to be used for “delete by email”, which would create email enumeration risks.
- Export includes “everything associated to the user” as CSV, but never exports secret/token material (hashes, raw tokens).
