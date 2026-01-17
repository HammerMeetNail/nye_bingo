# Notifications System Plan

## Goal
Add a first-class, in-app notification system (stored server-side) with per-scenario toggles under Profile settings.

### Scenarios to notify
1. Friend request received (someone sends you a request)
2. Friend request accepted (your request is accepted OR your invite link is accepted)
3. Friend completes a bingo (first bingo per card; no re-notify if “re-earned”)
4. Friend creates a new card visible to friends (finalize+visible, or later private→visible; no re-notify)

## Non-goals (initial scope)
- Push notifications (APNs/FCM) and native mobile integrations
- Real-time delivery via WebSockets/SSE (polling is acceptable)
- Digest emails / complex scheduling
- Admin/system broadcasts

## UX / Product Spec

### Notification surfaces
- **Navbar**: Add “Notifications” link (or bell icon) with unread badge count.
- **Notifications page** (`#notifications`): List notifications newest-first with:
  - Unread/read styling
  - “Mark as read” per item
  - “Mark all as read”
  - Deep links where possible (otherwise link to `#friends`)
- **Profile settings** (`#profile`): “Notifications” section with:
  - In-app: master toggle (default ON) + per-scenario toggles (default ON)
  - Email: master toggle (default OFF) + per-scenario toggles (default OFF)
  - Each master toggle disables that channel’s per-scenario controls in the UI (values still stored)

### Notification content (in-app)
Use **structured data** + render-time escaping (no HTML in DB). Example copy:
- Request received: “`{friend}` sent you a friend request.” → link `#friends`
- Request accepted: “`{friend}` accepted your friend request.” → link `#friends` (or friend cards if we have `friendship_id`)
- Bingo: “`{friend}` got a bingo on `{card_name}` (`{bingos}` total).” → link to friend’s cards
- New card: “`{friend}` created a new card: `{card_name}`.” → link to friend’s cards

All user-controlled strings must be rendered with `App.escapeHtml` / `textContent`.

## Data Model

### Table: `notification_settings`
One row per user.

Columns:
- `user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE`
- `in_app_enabled BOOLEAN NOT NULL DEFAULT true`
- `in_app_friend_request_received BOOLEAN NOT NULL DEFAULT true`
- `in_app_friend_request_accepted BOOLEAN NOT NULL DEFAULT true`
- `in_app_friend_bingo BOOLEAN NOT NULL DEFAULT true`
- `in_app_friend_new_card BOOLEAN NOT NULL DEFAULT true`
- `email_enabled BOOLEAN NOT NULL DEFAULT false`
- `email_friend_request_received BOOLEAN NOT NULL DEFAULT false`
- `email_friend_request_accepted BOOLEAN NOT NULL DEFAULT false`
- `email_friend_bingo BOOLEAN NOT NULL DEFAULT false`
- `email_friend_new_card BOOLEAN NOT NULL DEFAULT false`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Backfill existing users in the migration:
- `INSERT INTO notification_settings (user_id) SELECT id FROM users ON CONFLICT DO NOTHING;`

### Table: `notifications`
One row per delivered notification per recipient.

Columns:
- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE` (recipient)
- `type TEXT NOT NULL` (enum-like string; see below)
- `actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL` (friend who triggered it)
- `friendship_id UUID REFERENCES friendships(id) ON DELETE SET NULL` (optional but useful for deep links)
- `card_id UUID REFERENCES bingo_cards(id) ON DELETE SET NULL` (for bingo/new-card)
- `bingo_count INT` (for bingo notifications)
- `in_app_delivered BOOLEAN NOT NULL DEFAULT true` (whether this event was enabled for in-app at creation time)
- `email_delivered BOOLEAN NOT NULL DEFAULT false` (whether this event was enabled for email at creation time)
- `email_sent_at TIMESTAMPTZ` (set only when email send succeeds)
- `read_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Indexes:
- `CREATE INDEX idx_notifications_user_created ON notifications(user_id, created_at DESC);`
- `CREATE INDEX idx_notifications_user_unread ON notifications(user_id) WHERE read_at IS NULL;`
- `CREATE INDEX idx_notifications_type ON notifications(type);`

Deduplication (avoid spamming):
- New card: `UNIQUE (user_id, type, card_id)` for `friend_new_card`
- Bingo (first bingo only): `UNIQUE (user_id, type, card_id)` for `friend_bingo`
- Requests: `UNIQUE (user_id, type, friendship_id)` for request received/accepted

Type values (string constants in code):
- `friend_request_received`
- `friend_request_accepted`
- `friend_bingo`
- `friend_new_card`

Optional check constraint:
- `CHECK (type IN (...))`

## Backend Implementation

### 1. Migrations
Add `migrations/000016_notifications.up.sql` and `.down.sql`:
- Create `notification_settings`
- Create `notifications`
- Add indexes + unique constraints
- Backfill `notification_settings` for existing users

### 2. Models
Add:
- `internal/models/notification.go`:
  - `Notification` struct
  - `NotificationType` string constants
  - `NotificationSettings` struct

### 3. Services
Add:
- `internal/services/notification.go` with a `NotificationService` using the existing `DB` interface.
- `internal/services/interfaces.go`: add `NotificationServiceInterface` and possibly `NotificationSettingsServiceInterface` (or keep both in one service).

Responsibilities:
- `GetSettings(ctx, userID)` (create defaults if missing)
- `UpdateSettings(ctx, userID, patch)` (partial update; update `updated_at`)
- `List(ctx, userID, limit, beforeID/beforeTime, unreadOnly)` (simple pagination; list should return only `in_app_delivered = true`)
- `MarkRead(ctx, userID, notificationID)`
- `MarkAllRead(ctx, userID)`
- `UnreadCount(ctx, userID)`

Event helpers (used by other services):
- `NotifyFriendRequestReceived(ctx, recipientID, actorID, friendshipID)`
- `NotifyFriendRequestAccepted(ctx, recipientID, actorID, friendshipID)`
- `NotifyFriendsNewCard(ctx, actorID, cardID)` (bulk insert to all friends)
- `NotifyFriendsBingo(ctx, actorID, cardID, bingoCount)` (bulk insert to all friends)

Settings gating:
- Settings belong to the **recipient**.
- Bulk-notify queries should join `notification_settings` and compute two booleans per recipient:
  - `in_app_enabled && in_app_<scenario>`
  - `email_enabled && email_<scenario>`
- Insert a row when **either** channel is enabled; set `in_app_delivered` / `email_delivered` accordingly.
- Only attempt to send email when `email_delivered = true`, and only if the user has a deliverable address:
  - Recommended: require `users.email_verified = true` before allowing/sending email notifications.

Blocking gating:
- When selecting recipients, exclude any relationship where either side has blocked the other via `user_blocks`.

Failure policy:
- Notifications should be **best-effort**: if an insert fails, log and do not fail the user’s primary action (send request, complete item, etc.).

### 3b. Email delivery
Send notification emails immediately (no batching) when `email_delivered = true`.

Recommended constraints:
- Only allow enabling email notifications when `users.email_verified = true` (disable the email toggle UI until verified).

Implementation approach:
- Extend the existing email service:
  - Update `internal/services/interfaces.go` `EmailServiceInterface` to add a method like:
    - `SendNotificationEmail(ctx context.Context, toEmail, subject, html, text string) error`
  - Implement it in `internal/services/email.go` using the existing provider.
- In `NotificationService`, after inserting notifications with `email_delivered = true`:
  - Send emails **only for rows inserted** (use `INSERT ... RETURNING id, user_id` so retries/duplicate events don’t re-email).
  - On success, set `email_sent_at = NOW()`.
  - Use a goroutine to avoid delaying the primary request/response; log failures.

Email content:
- Subject lines per type (short + human):
  - “New friend request”
  - “Friend request accepted”
  - “Your friend got a bingo!”
  - “Your friend created a new bingo card”
- Body should include:
  - Who/what happened
  - A link to view it: `BASE_URL/notifications` (and/or `/friends`)
  - A “Manage notification settings” link to `BASE_URL/profile`

### 4. Hook points (existing services)

#### Friend request received
In `internal/services/friend.go` `SendRequest(...)`:
- After friendship row is created (and transaction committed), create a notification for `friendID`:
  - type: `friend_request_received`
  - actor: `userID`
  - friendship_id: new friendship row ID
  - link target: `/friends` (frontend decides)

#### Friend request accepted
In `internal/services/friend.go` `AcceptRequest(...)`:
- After updating friendship status to accepted, notify the sender (`friendship.UserID`) that `userID` accepted.

#### Invite accepted (token invite)
In `internal/services/friend_invite.go` `AcceptInvite(...)`:
- After creating the friendship and committing:
  - Notify the inviter that `recipientID` accepted.
  - If we want deep links, update `AcceptInvite` to also return the created `friendship_id` (and/or the recipient username) so the UI can link directly.

#### Friend creates a new visible card
Trigger when a card becomes visible to friends *and* finalized:
- In `internal/services/card.go` `Finalize(...)`:
  - If `visible_to_friends` is true on finalize, notify friends with type `friend_new_card` and `card_id`.
- Also handle `Import(...)` when `Finalize=true` and `VisibleToFriends=true`.
- Also handle `UpdateVisibility(...)` (and `BulkUpdateVisibility(...)` if applicable):
  - If a finalized card transitions `visible_to_friends: false → true`, notify friends (dedup per recipient+card via unique constraint).

Recommendation: do **not** notify on draft creation because friends cannot view non-finalized cards.

#### Friend completes a bingo
Trigger on the **first time** a card ever has at least one bingo (and do not re-notify if the bingo is “re-earned” by uncompleting/re-completing):
- In `internal/services/card.go` `CompleteItem(...)`:
  - After updating the item, compute `bingos := countBingos(updatedItems, gridSize, freePos)`
  - If `bingos > 0`, call `NotifyFriendsBingo(actorID=userID, cardID, bingoCount=bingos)`
  - The unique constraint `(recipient, type, card_id)` ensures we only notify once per card per recipient, preventing spam from toggling completion state.

Gating:
- Only notify if card is finalized and `visible_to_friends = true` (already available on the loaded card).

### 5. Handlers + Routes
Add `internal/handlers/notification.go` with session-only endpoints:
- `GET /api/notifications` (list; supports `?unread=1&limit=50`)
- `POST /api/notifications/{id}/read` (mark one read)
- `POST /api/notifications/read-all` (mark all read)
- `GET /api/notifications/unread-count`
- `GET /api/notifications/settings`
- `PUT /api/notifications/settings` (update toggles)

Register in `cmd/server/main.go` using `requireSession`.

### 6. OpenAPI
Update `web/static/openapi.yaml`:
- Add schemas: `Notification`, `NotificationSettings`
- Add paths under `/notifications/*` using `cookieAuth`
- Document query params, response shapes, and error responses

## Frontend Implementation

### 1. API client
Update `web/static/js/api.js` with `API.notifications.*`:
- `list({ unread, limit })`
- `markRead(id)`
- `markAllRead()`
- `unreadCount()`
- `getSettings()`
- `updateSettings(patch)`

### 2. Routing + UI
Update `web/static/js/app.js`:
- `setupNavigation()` add Notifications link + badge element.
- Add `renderNotifications(container)` for `#notifications` route.
- Add event handlers for mark read / mark all read using the existing `data-action` pattern.
- Update `renderProfile()` to render notification settings section and wire change events.

Rendering rules:
- Prefer DOM APIs (`createElement`, `textContent`) for notification rows.
- If building HTML strings, escape all interpolated values with `App.escapeHtml`.

Badge refresh strategy:
- On login / `checkAuth()`
- After visiting `#notifications` (and after marking read)
- Optional: periodic polling (e.g. every 60s) when logged in

### 3. Styling
Update `web/static/css/styles.css`:
- Notification list layout, unread highlight
- Small badge pill style in navbar

## Testing Plan (must not reduce coverage)

### Go unit/integration tests
Add handler/service tests similar to existing patterns:
- `internal/handlers/notification_test.go`:
  - List requires auth (401)
  - Settings GET/PUT validates payload and persists
  - Mark read respects ownership
- `internal/services/notification_test.go`:
  - Bulk notify inserts for friends only
  - Settings gating (disabled users don’t get inserts)
  - Dedup constraints respected (second insert is no-op)

### Frontend unit tests
If there are pure helpers added (formatting/building notification copy), add tests in:
- `web/static/js/tests/runner.js`

### Playwright E2E
Add/extend specs in `tests/e2e/`:
1. Friend request notification:
   - User A sends request to User B
   - User B sees unread badge increment and notification on `#notifications`
2. Friend accepted notification:
   - User B accepts
   - User A sees notification
3. New card visible notification:
   - User A finalizes a card with visibility on
   - User B sees notification; link navigates to friend cards and card is visible
4. Bingo notification:
   - User A completes enough items to trigger a bingo
   - User B sees a bingo notification
   - Assert spam prevention: A uncompletes a cell and recompletes; B does **not** get a second “first bingo” notification
5. XSS regression:
   - Use a username like `"<img src=x onerror=alert(1)>"` and assert notification renders as text (no `img` node in DOM).

If new E2E scenarios are added, update the coverage outline in `plans/playwright.md`.

## Implementation Order (Agent Checklist)
1. Create migrations (`000016_notifications.*`) + run in test container
2. Add models (`Notification`, `NotificationSettings`)
3. Implement `NotificationService` + interfaces + service tests
4. Wire service into `cmd/server/main.go` dependency graph
5. Add handlers + routes + handler tests
6. Update `web/static/openapi.yaml`
7. Update frontend API (`web/static/js/api.js`)
8. Add UI (`/notifications`, navbar badge, profile toggles) + CSS
9. Add/extend Playwright specs + XSS regression + update `plans/playwright.md`
10. Run `./scripts/test.sh --coverage` and ensure coverage does not drop

## Retention / Cleanup
- Store notifications indefinitely for UX, but delete rows older than 1 year:
  - `DELETE FROM notifications WHERE created_at < NOW() - INTERVAL '1 year'`
- Run cleanup best-effort:
  - On server startup, and then on a 24h ticker in a goroutine (stops on server shutdown).
6. Email notification opt-in:
   - User B opts into email notifications for (at least) friend request received
   - Trigger the event from A → B
   - Assert an email arrives in Mailpit with a link to `/notifications` or `/friends`
   - Assert default behavior: with email toggles off, no notification emails are sent
