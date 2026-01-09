# Playwright Local E2E Plan

## Goals
- Provide a single `make e2e` command that starts from a clean DB, seeds data, and runs Playwright UI tests in a container.
- Validate core front-end flows while asserting back-end behavior via the UI (and targeted API checks when needed).
- Provide separate commands for backend unit tests, frontend unit tests, and e2e tests.

## Decisions (from user)
- `make e2e` always starts fresh by dropping volumes (`podman compose down -v`).
- Playwright runs in a container; no npm install on the host.
- Default browser is Firefox; Chromium and WebKit are optional via flags.
- E2E tests use both seeded data and UI-created data.
- E2E runs with `AI_STUB=1` so AI wizard flows are deterministic without external APIs.

## Proposed Workflow (`make e2e`)
1. `podman compose down -v` (destructive reset).
2. Build web assets (`./scripts/build-assets.sh`).
3. `podman compose up -d --build`.
4. Wait for `http://localhost:8080/health` to return 200.
5. Seed via `./scripts/seed.sh http://localhost:8080`.
6. Run Playwright in a container with `PLAYWRIGHT_BASE_URL` pointed at the app.
7. Leave the stack running for inspection (no cleanup by default).

## Playwright Container Strategy
- Add `Containerfile.playwright` based on `mcr.microsoft.com/playwright:<version>` (includes Firefox/Chromium/WebKit).
- Add a minimal `e2e/package.json` with `@playwright/test` installed into the image.
- Use `NODE_PATH` so tests can resolve `@playwright/test` without a host `node_modules`.
- Run Playwright on the compose network and set:
  - Seed script base URL: `http://localhost:8080` (host).
  - Playwright base URL: `http://app:8080` (container network).
- Add a compose profile/service `playwright` to simplify networking (`podman compose --profile e2e run --rm playwright`).

## Browser Selection
- `playwright.config.js` defines projects: `firefox` (default), `chromium`, `webkit`.
- `make e2e` runs `firefox` only.
- Allow overrides via `BROWSERS` env:
  - `make e2e BROWSERS=chromium`
  - `make e2e BROWSERS=firefox,webkit`
- Map `BROWSERS` to `--project` flags in `scripts/e2e.sh`.

## Test Coverage Outline
## Current E2E Scenarios
- `tests/e2e/seeded-data.spec.js`
  - Seeded dashboard cards render with progress and FREE space (invariant checks).
  - Friend reactions can be added on seeded friend cards.
  - Cards can be archived via bulk actions and show archived view.
- `tests/e2e/anonymous-flow.spec.js`
  - Anonymous draft creation, shuffle (FREE stays), reload persistence.
  - Anonymous finalize triggers auth flow and imports card.
- `tests/e2e/anonymous-alternatives.spec.js`
  - Anonymous finalize supports login instead of register.
  - Anonymous card conflicts allow keeping the existing card (discarding anon card).
  - Anonymous card conflicts allow replacing the existing card with anon content.
- `tests/e2e/authenticated-flow.spec.js`
  - Logged-in user configures header/FREE, finalizes, completes/uncompletes.
  - Clone flow supports grid size/header changes on new draft.
- `tests/e2e/bulk-actions.spec.js`
  - Bulk visibility updates, export ZIP download (ZIP header sanity), and bulk delete.
- `tests/e2e/editor-actions.spec.js`
  - Draft goal edit/remove updates grid and progress.
  - Finalized visibility toggle switches between visible/private.
- `tests/e2e/visibility-enforcement.spec.js`
  - Friends can view visible cards, but private cards are hidden.
- `tests/e2e/auth-flows.spec.js`
  - Magic link login via email token.
  - Password reset via emailed token.
  - Email verification banner + resend + verify flow.
- `tests/e2e/friend-management.spec.js`
  - Cancel sent friend requests, reject incoming requests.
  - Remove an existing friend.
- `tests/e2e/friend-invites.spec.js`
  - Invite link acceptance connects friends.
  - Blocking removes friendships and hides search results.
- `tests/e2e/notifications.spec.js`
  - Viewing notifications marks them read and clears the badge.
  - Notifications can be deleted individually or cleared all at once.
  - Friend acceptance notification delivered to sender.
  - New card notification links to friend card view.
  - Bingo notification fires once per card (dedupe).
  - XSS-style usernames render as text in notifications.
  - Email notifications respect opt-in and include links.
- `tests/e2e/reminders-test-email.spec.js`
  - Email-verified users can enable reminders and receive a test email.
- `tests/e2e/reminders-scheduled-goal.spec.js`
  - Scheduled goal reminders are delivered by the background reminder runner (Mailpit).
- `tests/e2e/reminders-xss.spec.js`
  - Reminder emails escape XSS-style goal content in suggested goals.
- `tests/e2e/reminders-image.spec.js`
  - Reminder test emails include a PNG URL that returns valid image data.
- `tests/e2e/profile-settings.spec.js`
  - Searchable toggle gates friend search visibility.
  - Password change validation and re-login with new password.
- `tests/e2e/profile-tokens.spec.js`
  - Create/revoke API tokens, copy-to-clipboard toast, and token auth validation.
  - Revoke all tokens with confirmation.
- `tests/e2e/completion-notes.spec.js`
  - Completion notes are saved and persist across reloads.
- `tests/e2e/bingo-celebration.spec.js`
  - Completing a row triggers a bingo celebration toast.
- `tests/e2e/drag-drop.spec.js`
  - Drag-and-drop reorders items without moving FREE.
- `tests/e2e/draft-safety.spec.js`
  - Clear-all removes draft items.
  - Full drafts warn before leaving without finalizing.
- `tests/e2e/social-flow.spec.js`
  - Friend request, accept flow, view friend card, add reaction.
- `tests/e2e/support-form.spec.js`
  - Support form validation, success toast, and email delivery via Mailpit.
- `tests/e2e/ai-wizard-create.spec.js`
  - AI wizard generates goals and creates a card (stubbed).
- `tests/e2e/ai-wizard-append.spec.js`
  - AI wizard appends goals to an existing card (stubbed).
- `tests/e2e/ai-wizard-gating.spec.js`
  - Unverified users are prompted to verify after free generations are used.
- `tests/e2e/ai-wizard-mix.spec.js`
  - AI wizard handles mixed card/create flows (stubbed).
- `tests/e2e/ai-wizard-anonymous-gate.spec.js`
  - Anonymous AI wizard flows enforce gating (stubbed).
- `tests/e2e/ai-guide-editor.spec.js`
  - AI guide refine flow updates a draft goal.
  - Empty cell click opens add-goal modal and saves a new goal.
- `tests/e2e/xss-hardening.spec.js`
  - XSS-style usernames render as literal text in friends UI.
  - Friend actions (remove/block/unblock) and invite copy/revoke still work.
  - XSS-style card titles/goals render as literal text (no injected DOM nodes).
- `tests/e2e/card-share.spec.js`
  - Share links render a read-only card view with XSS payloads escaped.
  - Revoked share links display a not-found message.
- `tests/e2e/csp-header.spec.js`
  - Document responses include strict CSP (no `unsafe-inline` / `unsafe-hashes` for scripts).

## Unit Test Separation
- Add make targets:
  - `make test-backend`: Go unit tests only.
  - `make test-frontend`: JS unit tests only.
  - `make test`: existing combined run.
- Implementation options:
  - Extend `./scripts/test.sh` with `--go` and `--js` flags, or
  - Add `./scripts/test-go.sh` and `./scripts/test-js.sh`.

## Files to Add/Modify
- `scripts/e2e.sh` (or `scripts/playwright.sh`)
- `Containerfile.playwright`
- `package.json` + `package-lock.json` (Playwright only)
- `playwright.config.js`
- `tests/e2e/*.spec.js`
- `Makefile` targets (`e2e`, `e2e-headed`, `test-backend`, `test-frontend`)
- Update docs in `agent_docs/testing.md`

## Acceptance Criteria
- `make e2e` always starts from a clean DB, seeds data, runs Playwright in Firefox, and returns non-zero on failure.
- `make e2e BROWSERS=chromium` and `make e2e BROWSERS=webkit` work.
- Backend-only and frontend-only unit test targets run independently.
- E2E suite covers both seeded and UI-created data flows.

## Scenario Backlog
- Support form rate-limit handling (only if deterministic in CI).
- AI wizard append-to-card flow and verified-user unlimited access.
