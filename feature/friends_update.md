# Friends + Sharing Rework (Privacy-First) — Implementation Plan

## Summary

Rework Year of Bingo social/sharing to be friendlier while reducing stalking risk and accidental information disclosure.

Key shifts:
- Make friending primarily invitation-based (share a link/code out-of-band) rather than discoverability-based.
- Keep username search as an optional fallback, but harden it against enumeration and harassment.
- Add a separate “share link” system for cards (revocable, expiring, configurable visibility) that does not require friending.
- Remove unnecessary PII exposure (notably email addresses) from social surfaces.
- Add abuse controls (blocklist, request cooldowns, rate limiting) and safer defaults for what friends can see (no notes/proof URLs by default).

## Testing-First Delivery (TDD + Coverage Guardrails)

This work must be delivered in a testing-first way:
- **TDD where practical**: add unit/handler tests first (red), then implement (green), then refactor.
- **Playwright alongside UI changes**: new or changed user journeys must be covered by E2E specs as part of the same slice of work.
- **Coverage must not decrease**: run `./scripts/test.sh --coverage` and keep the reported percentage >= baseline for the branch.
- **Container parity**: use `./scripts/test.sh` for accurate results (runs Go + JS tests in a container).
- **E2E parity**: use `make e2e` for end-to-end validation (destructive; resets volumes) and update `plans/playwright.md` when adding scenarios.

Practical guardrail for coverage:
- Capture the current coverage output at the start of the branch as the baseline.
- Treat any drop as a release blocker unless new tests in the same PR bring it back up.

Non-goals:
- Public profiles, follower graphs, global feeds, “people you may know”, or any indexed directory of users.
- Real-time presence/location signals.

---

## Current State (as implemented)

- Friends discovery: `GET /api/friends/search?q=...` returns `id` + `username` for `users.searchable = true`.
  - Query behavior: substring match on `LOWER(username)` with minimum length 2, `LIMIT 20`.
- Friending flow: request/accept/reject/cancel/remove in `friendships` table.
- Friend card access: friend can view another user’s finalized cards if `visible_to_friends = true`.
- Social interaction: reactions are allowed only between friends.
- Privacy settings:
  - `users.searchable` (opt-in discoverability in search).
  - `bingo_cards.visible_to_friends` (per-card, default true).
- PII leak today: friend list + requests include email fields from `users.email` in backend and UI.
- Notes/proof URLs: `BingoItem` includes `notes` and `proof_url` and these are currently returned in friend card responses (implicit via the card model).

---

## Privacy & Abuse Threat Model

### Primary risks
- **User enumeration**: a malicious user can probe `friends/search` to discover many searchable accounts.
- **Harassment**: repeated friend requests; “reject” does not prevent re-request; “searchable” toggles don’t protect against someone who already knows your user ID.
- **PII disclosure**:
  - Emails shown in social UI are high-risk and unnecessary.
  - Card notes / proof URLs can contain sensitive info or external identifiers.
- **Share-link leakage**: a share URL can be forwarded beyond intended recipients.

### Success criteria
- A user can be social without becoming easily discoverable.
- A user can stop contact from a person (block) and remain unsearchable by them.
- Sharing defaults prevent accidental disclosure (especially notes/proof URLs).
- Public sharing is possible without “public profiles”.

---

## UX / Product Design

### 1) Friends page redesign

Replace “Search-first” with “Invite-first”.

Sections:
- **Invite** (primary): “Invite a friend” with copyable link and (optional) short code.
- **Requests**: pending received/sent requests, without emails.
- **Friends**: list + “View cards”, “Remove”, “Block”.
- **Blocked**: list blocked users with “Unblock”.

### 2) Invitation-based friending

Two acceptable invite models; pick one:

**Option A — One-time invite links (recommended)**
- User clicks “Create invite” → gets a unique, unguessable URL.
- Invites are **revocable** and **expirable** (default expiry e.g. 14 days).
- Recipient opens link:
  - If logged out: prompt to log in/register first.
  - After auth: show “Accept invite” CTA; acceptance creates friendship without exposing the inviter in any global directory.

**Option B — Rotating friend code**
- Each user has a friend code that can be rotated (invalidates previous).
- Recipient enters code; acceptance requires explicit confirmation.
- Simpler UX for “tell me your code”, but harder to support “multiple outstanding invites” and auditing.

Plan assumes **Option A**.

### 3) Hardened username search (secondary path)

Keep search, but reduce enumeration:
- Increase minimum query length (e.g. 4).
- Match mode: exact match (case-insensitive) or strict prefix-only (no substring).
- Do not return results when query is too broad.
- Add per-user and per-IP rate limiting for search.

### 4) Safer “what friends can see”

Split “friend visibility” into two layers:
- **Card visibility**: whether friends can see the card exists at all (current `visible_to_friends` behavior).
- **Card details visibility**: what fields friends can see if the card is visible.

Defaults:
- Friends can see item text and completion status (as today).
- Friends **cannot** see:
  - `notes`
  - `proof_url`
  - sensitive metadata (e.g. timestamps) unless explicitly needed.

Add optional toggles per card (owner-controlled):
- “Allow friends to view notes”
- “Allow friends to view proof links”

### 5) Sharing without friending (card share links)

Add per-card “Share” functionality that is independent of the friends system.

Features:
- Create share link(s) per card.
- Share link settings:
  - Expiration: default (e.g. 30–180 days) + “never” option.
  - Show completions: yes/no.
  - Show notes: no by default; ideally never for public links.
  - Show proof links: no by default; ideally never for public links.
  - Show username: yes/no (for privacy-conscious sharing).
- Revocation: delete share link(s) anytime.
- Public share route does **not** require authentication and does not expose a user profile.

Implementation can follow and extend `plans/png.md`:
- Serve an embeddable PNG (`/share/{token}.png`) and/or an HTML share page (`/share/{token}`) with OG tags.

---

## Backend Design

### Data model changes

#### A) Remove email from social responses
- Stop selecting and returning `users.email` in:
  - friend list
  - pending requests
  - sent requests
- Ensure models/JSON responses do not include email for social views.

#### B) Blocklist

New table: `user_blocks`

Fields (suggested):
- `blocker_id` (uuid, FK users.id)
- `blocked_id` (uuid, FK users.id)
- `created_at`
Constraints:
- unique `(blocker_id, blocked_id)`

Semantics:
- Blocking should:
  - remove any existing friendship (accepted) between the two users
  - delete any pending friend requests in either direction
  - prevent future requests/invites from either side (policy choice; at minimum block prevents blocked user from sending to blocker)
  - hide the blocker from search results for the blocked user (even if `searchable = true`)

#### C) Friend invites (Option A)

New table: `friend_invites`

Fields (suggested):
- `id` (uuid)
- `inviter_user_id` (uuid, FK users.id)
- `invite_token` (opaque string, unique, unguessable)
- `expires_at` (nullable; default non-null)
- `revoked_at` (nullable)
- `accepted_by_user_id` (nullable)
- `accepted_at` (nullable)
- `created_at`

Constraints:
- unique `invite_token`
- optional: prevent excessive outstanding invites per user (soft limit)

Token handling:
- Store token hashed (recommended) or store raw token with strict database access controls.
- Treat invite tokens like credentials (log redaction, never include in structured logs).

#### D) Share links (cards)

Prefer implementing `plans/png.md`’s `card_shares` table, with privacy-first defaults:
- `show_completions` boolean
- `show_username` boolean (new vs current plan)
- `show_notes` boolean (default false; consider disallowing for public shares)
- `show_proof_urls` boolean (default false; consider disallowing for public shares)
- `expires_at`, `revoked_at`, `last_accessed_at`, `access_count`

### Service layer

Add/extend services under `internal/services/`:

- `BlockService`
  - `Block(ctx, blockerID, blockedID)`
  - `Unblock(ctx, blockerID, blockedID)`
  - `IsBlocked(ctx, a, b)` (directional + symmetric helper)
  - `ListBlocked(ctx, blockerID)`

- `FriendInviteService`
  - `CreateInvite(ctx, inviterID, expiresInDays)`
  - `RevokeInvite(ctx, inviterID, inviteID)`
  - `GetInviteForAccept(ctx, inviteToken)` (validate not expired/revoked/accepted)
  - `AcceptInvite(ctx, inviteToken, recipientUserID)` (creates friendship, marks accepted)

- `FriendService` updates
  - `SearchUsers`: exclude blocked users, hardened match mode, longer min length, consistent ordering.
  - `SendRequest`: reject if blocked (either direction), add cooldown logic.

- `ShareService` (or reuse plan naming)
  - Create/list/revoke shares for a card and serve public views.

### Handlers / Routes

Session-only social endpoints remain under `requireSession` in `cmd/server/main.go`.

New/updated endpoints (names illustrative; finalize during implementation):

- Invites
  - `POST /api/friends/invites` → create invite (returns URL/token)
  - `GET /api/friends/invites` → list my active invites
  - `DELETE /api/friends/invites/{id}/revoke` → revoke
  - `POST /api/friends/invites/accept` → accept (session required, token in body)

- Blocking
  - `POST /api/blocks` with `{ user_id }`
  - `DELETE /api/blocks/{user_id}`
  - `GET /api/blocks`

- Search hardening
  - Keep `GET /api/friends/search` but enforce new rules and rate limits.

- Share links (public + authenticated management)
  - Authenticated management:
    - `GET /api/cards/{id}/shares`
    - `POST /api/cards/{id}/shares`
    - `DELETE /api/cards/{id}/shares/{shareId}`
  - Public:
    - `GET /share/{token}` (HTML preview, noindex)
    - `GET /share/{token}.png` (image)

OpenAPI:
- Decide whether to document session-only endpoints in `web/static/openapi.yaml` using `cookieAuth`.
- Any newly added token-based endpoints must be documented per `agent_docs/api.md`.

### Rate limiting & abuse prevention

Add app-level rate limiting for:
- `friends/search`
- `friends/requests`
- invite creation + acceptance
- share public endpoints (token guessing defense)

Implementation approach:
- Use Redis (already in stack) for token-bucket/counter with TTL.
- Key by `userID` where available, plus `IP` as backstop.
- Return 429 with a generic message.
- Cap active invites per user to limit invite spam.

### Response shaping / privacy-by-default

Introduce “sanitized card” responses for friend/share views:
- Friend view should return a card payload that omits:
  - `notes`, `proof_url` by default
  - unnecessary metadata fields (e.g. internal `user_id`, timestamps) unless UI requires them
- Public share should never return raw card JSON unless required; prefer rendering PNG/HTML from server-side data.

Logging:
- Ensure tokens (invite/share) are never logged.
- Consider reducing social endpoint logs to avoid sensitive query strings.

---

## Frontend Implementation (Vanilla JS SPA)

### Routing

Add new route(s) in `web/static/js/app.js`:
- `#friend-invite/{token}`: accept flow UI (login gate + confirm acceptance).
- `#share/{token}` (if SPA handles preview; alternatively served as server-rendered HTML for OG tags).

### Friends page (`renderFriends`)

Changes:
- Make “Invite” the primary entry point.
- Remove any display of emails in friends/requests lists.
- Add “Block” actions and a “Blocked” list.
- Keep “Search by username” as secondary; update helper text and enforce stricter query rules client-side.

### Card UI

Add a “Share” entry point:
- From dashboard card actions and/or finalized card page.
- Modal/page to:
  - create share link with settings
  - list existing links
  - revoke links
  - copy/share buttons

### API client (`web/static/js/api.js`)

Add `API.friends.invites.*`, `API.friends.block.*`, `API.cards.shares.*` namespaces per patterns in `agent_docs/architecture.md`.

---

## Policy & Content Updates

Update in-app copy to set expectations:
- Friends: “Only people you approve can see your cards. Notes and proof links are private by default.”
- Sharing: “Anyone with the link can view it. You can revoke links at any time.”

Update legal/help pages as needed (privacy/security/FAQ) to reflect:
- invitation-based friending
- block controls
- share links (public access) and recommended safe usage

---

## Testing Plan (must not reduce coverage)

Testing is part of the implementation, not a follow-up. Each phase below should land with its tests in the same PR.

Required commands for each PR that changes these features:
- `./scripts/test.sh` (Go + JS tests in container)
- `./scripts/test.sh --coverage` (coverage must not decrease)
- `make e2e` (whenever user-visible friend/share flows change)

### Go unit/handler tests

- Friend service:
  - search rules (min length, match mode, excludes self)
  - excludes blocked users
  - request blocked/cooldown behaviors
- Invite service:
  - create/revoke/accept, expiry, double-accept, revoked invites
  - acceptance creates friendship and is idempotent/consistent
- Block service:
  - block removes friendship/pending requests
  - blocked prevents new requests/search visibility
- Handler tests for new endpoints and for:
  - email fields no longer present in responses
  - friend card responses omit notes/proof URLs by default

TDD sequencing guideline (recommended):
1) Service unit tests (fast, deterministic).
2) Handler tests (status codes + response shaping + auth requirements).
3) UI implementation + Playwright tests (locks behavior end-to-end).

### Frontend tests
- Add/extend JS unit tests in `web/static/js/tests/runner.js` for any new pure logic helpers (parsers, validators, formatters, etc).
- Use Playwright for SPA route and interaction coverage (avoid calling internal globals directly; prefer click/keyboard interactions).

### Playwright E2E (`tests/e2e/*.spec.js`)

Add scenarios:
- Create invite link → accept invite (new user + existing user flows).
- Block user stops re-requesting and removes friendship.
- Friend view does not show notes/proof URLs by default.
- Create share link → open public share → revoke link → access denied.

Update coverage outline: `plans/playwright.md` (per AGENTS.md requirement).

Run full suite in container: `./scripts/test.sh`.

---

## Migration & Rollout

Each rollout step is executed in a testing-first way: tests land before or with the code that makes them pass, and coverage must not decrease.

1) Ship the “no-email-in-social” change first (low risk).
   - Add handler tests asserting friend list/requests payloads never include email fields.
   - Add/update Playwright assertion if the Friends UI currently renders any email text.
2) Add blocklist (immediate harassment mitigation).
   - Add unit tests for block semantics (removes friendships/requests; prevents new requests/search visibility).
   - Add handler tests for block endpoints and blocked behavior in existing friend endpoints.
   - Add Playwright spec: block user → blocked user cannot re-request; blocker no longer sees them in search.
3) Add invite links and redesign Friends page around invites.
   - Add unit tests for invite lifecycle (create/revoke/accept/expire/double-accept).
   - Add handler tests for invite endpoints (auth required, error cases).
   - Add Playwright specs for invite acceptance (logged-out → login → accept; logged-in accept).
4) Harden username search + add rate limiting.
   - Add unit/handler tests for stricter search rules (min length, match mode) and exclusion rules (blocked users, self).
   - Add handler tests for rate limiting response (429) without leaking whether a target exists.
5) Add share links (public) with privacy-first defaults.
   - Add unit/handler tests for share creation/revocation/expiry and response shaping (no notes/proof by default).
   - Add Playwright spec: create share link → open public share → revoke → access denied.
6) Review privacy/security pages and update FAQ.
   - Add/update Playwright navigation smoke tests if routes or copy are materially changed.

Backward compatibility:
- Existing friendships remain unchanged.
- Existing `visible_to_friends` behavior remains but friend views become more restrictive (notes/proof hidden unless enabled).

---

## Open Questions / Decisions

- Invite model: one-time links vs rotating codes (plan assumes one-time).
- Share implementation priority: PNG-only vs HTML+PNG; whether to reuse `plans/png.md` as-is or extend it with additional privacy flags.
- Whether to document session-only social endpoints in `web/static/openapi.yaml` using `cookieAuth` (recommended for completeness).
- Whether friends should ever be allowed to see notes/proof URLs (default no; allow toggles only if strongly desired).
- Exact rate limits and cooldown durations (should be adjustable via config).
