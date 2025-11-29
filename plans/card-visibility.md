# Card Visibility Feature Plan

**Status: IMPLEMENTED (v0.8.0)**

## Overview

Enable users to control which bingo cards are visible to friends and allow friends to view all non-private cards from their connections.

**Key Requirements:**
- Users can set individual cards as private or visible to friends
- Default: cards are visible to friends (existing behavior preserved)
- Friends can view all non-private finalized cards
- Private cards are completely hidden from friend views (no indication they exist)
- Visibility can be set during finalization
- Bulk visibility controls for managing multiple cards

## UI Placement

**Dashboard (unfinalized card):**
- Visibility toggle positioned on the left side, directly under the "0/24 items added" progress section

**Card Page (finalized card):**
- Visibility toggle in the top right corner
- Parallel to the back button (which is top left)
- Positioned to the right of the "edit card name" button

**Finalization Flow:**
- Checkbox option during finalization: "Visible to friends" (checked by default)

**Archive Page:**
- Bulk visibility controls to set multiple cards at once
- Individual card visibility toggles

## Database Changes

### Migration: `000008_card_visibility.up.sql`

```sql
-- Add visibility column to bingo_cards
-- Default to true (visible to friends) to maintain backward compatibility
ALTER TABLE bingo_cards ADD COLUMN visible_to_friends BOOLEAN NOT NULL DEFAULT true;

-- Index for efficient friend card queries
CREATE INDEX idx_bingo_cards_visibility ON bingo_cards(user_id, is_finalized, visible_to_friends);
```

### Migration: `000008_card_visibility.down.sql`

```sql
DROP INDEX IF EXISTS idx_bingo_cards_visibility;
ALTER TABLE bingo_cards DROP COLUMN visible_to_friends;
```

## Backend Changes

### 1. Model Update (`internal/models/card.go`)

Add `VisibleToFriends` field to `BingoCard` struct:

```go
type BingoCard struct {
    ID               uuid.UUID
    UserID           uuid.UUID
    Year             int
    Category         *string
    Title            *string
    IsActive         bool
    IsFinalized      bool
    VisibleToFriends bool      // NEW: default true
    CreatedAt        time.Time
    UpdatedAt        time.Time
    Items            []BingoItem
}
```

### 2. Card Service (`internal/services/card.go`)

**Update existing methods to include visibility column:**

- `CreateCard()` - Include `visible_to_friends` in INSERT (default true)
- `GetCard()` - Add `visible_to_friends` to SELECT
- `GetCards()` - Add `visible_to_friends` to SELECT
- `GetArchivedCards()` - Add `visible_to_friends` to SELECT

**Add new method:**

```go
// UpdateVisibility sets the visibility of a card to friends
func (s *CardService) UpdateVisibility(ctx context.Context, cardID, userID uuid.UUID, visibleToFriends bool) error {
    // Verify card belongs to user
    // Update visible_to_friends column
    // Return error if card not found or unauthorized
}
```

### 3. Friend Service (`internal/services/friend.go`)

**Update `GetFriendCards()` to filter by visibility:**

Current query filters by `is_finalized = true`. Add filter:
```sql
AND visible_to_friends = true
```

**Update `GetFriendCard()` (single card) to respect visibility:**

Add visibility check to prevent accessing private cards even by direct ID.

### 4. Card Handler (`internal/handlers/card.go`)

**Add new endpoint handler:**

```go
// UpdateCardVisibility handles PUT /api/cards/{id}/visibility
func (h *CardHandler) UpdateCardVisibility(w http.ResponseWriter, r *http.Request) {
    // Parse card ID from path
    // Get user from context
    // Parse request body: { "visible_to_friends": true/false }
    // Call service.UpdateVisibility()
    // Return updated card or error
}
```

**Update card response serialization** to include `visible_to_friends` field.

### 5. Route Registration (`cmd/server/main.go`)

Add new routes:
```go
mux.HandleFunc("PUT /api/cards/{id}/visibility", cardHandler.UpdateCardVisibility)
mux.HandleFunc("PUT /api/cards/visibility/bulk", cardHandler.BulkUpdateVisibility)
```

### 5b. Bulk Update Handler (`internal/handlers/card.go`)

```go
// BulkUpdateVisibility handles PUT /api/cards/visibility/bulk
func (h *CardHandler) BulkUpdateVisibility(w http.ResponseWriter, r *http.Request) {
    // Parse request body: { "card_ids": [...], "visible_to_friends": true/false }
    // Get user from context
    // Call service.BulkUpdateVisibility() for each card owned by user
    // Return success count or error
}
```

### 5c. Finalize Endpoint Update (`internal/handlers/card.go`)

Update `FinalizeCard` handler to accept optional `visible_to_friends` in request body:
```go
type FinalizeRequest struct {
    VisibleToFriends *bool `json:"visible_to_friends"` // Optional, defaults to true
}
```

### 6. Friend Handler (`internal/handlers/friend.go`)

**Update `GetFriendCard()` to return 404** if the specific card is private (not just if no cards exist).

## Frontend Changes

### 1. API Client (`web/static/js/api.js`)

Add new methods to cards namespace:

```javascript
cards: {
    // ... existing methods ...

    async updateVisibility(cardId, visibleToFriends) {
        return API.request('PUT', `/api/cards/${cardId}/visibility`, {
            visible_to_friends: visibleToFriends
        });
    },

    async bulkUpdateVisibility(cardIds, visibleToFriends) {
        return API.request('PUT', `/api/cards/visibility/bulk`, {
            card_ids: cardIds,
            visible_to_friends: visibleToFriends
        });
    },
}
```

### 2. App.js Updates

**A. Dashboard View (`renderDashboard`)**

Add visibility toggle for unfinalized cards:
- Position: Left side, directly under "X/24 items added" progress section
- Eye icon = visible to friends (default)
- Eye-slash icon = private
- Label text: "Visible to friends" or "Private"

```javascript
// Under progress section in dashboard
<div class="visibility-control">
    <button class="visibility-toggle" aria-label="Toggle visibility">
        <i class="fas fa-eye"></i>
    </button>
    <span class="visibility-label">Visible to friends</span>
</div>
```

**B. Finalized Card View (`renderFinalizedCard`)**

Add visibility toggle for card owner:
- Position: Top right corner, parallel to back button
- Right of the "edit card name" button
- Eye = visible to friends, Eye-slash = private
- Click to toggle visibility
- Show toast notification on change

```javascript
// In card header, top right section:
<div class="card-header-actions">
    <button class="edit-title-btn">...</button>
    <button class="visibility-toggle" aria-label="Card visible to friends">
        <i class="fas fa-eye"></i>
    </button>
</div>
```

**C. New Method: `toggleCardVisibility()`**

```javascript
async toggleCardVisibility(cardId, visibleToFriends) {
    try {
        await API.cards.updateVisibility(cardId, visibleToFriends);
        App.showToast(visibleToFriends
            ? 'Card is now visible to friends'
            : 'Card is now private', 'success');
        // Re-render current view to update UI
        App.route();
    } catch (error) {
        App.showToast('Failed to update visibility', 'error');
    }
}
```

**D. Finalization Modal**

Add visibility checkbox to finalization confirmation:
- Checkbox: "Visible to friends" (checked by default)
- Pass visibility setting when calling finalize endpoint

```javascript
// In finalization modal
<label class="checkbox-label">
    <input type="checkbox" id="finalize-visibility" checked>
    <span>Visible to friends</span>
</label>
```

**E. Archive View (`renderArchive`)**

Add bulk visibility controls:
- "Select All" / "Deselect All" buttons
- Checkboxes on each archived card
- "Make Selected Visible" / "Make Selected Private" buttons
- Individual visibility toggles on each card

```javascript
// Bulk controls section
<div class="archive-bulk-controls">
    <button onclick="App.selectAllArchiveCards()">Select All</button>
    <button onclick="App.deselectAllArchiveCards()">Deselect All</button>
    <button onclick="App.bulkSetVisibility(true)">Make Visible</button>
    <button onclick="App.bulkSetVisibility(false)">Make Private</button>
</div>
```

**F. New Method: `bulkSetVisibility()`**

```javascript
async bulkSetVisibility(visibleToFriends) {
    const selectedCards = this.getSelectedArchiveCards();
    if (selectedCards.length === 0) {
        App.showToast('No cards selected', 'warning');
        return;
    }
    try {
        await API.cards.bulkUpdateVisibility(selectedCards, visibleToFriends);
        App.showToast(`${selectedCards.length} cards updated`, 'success');
        App.route();
    } catch (error) {
        App.showToast('Failed to update visibility', 'error');
    }
}
```

### 3. Styles (`web/static/css/styles.css`)

```css
/* Visibility toggle button */
.visibility-toggle {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 0.5rem;
    color: var(--text-secondary);
    font-size: 1.2rem;
    transition: color 0.2s ease;
}

.visibility-toggle:hover {
    color: var(--primary);
}

.visibility-toggle[aria-pressed="false"] {
    color: var(--text-muted);
}

/* Visibility badge for archive cards */
.visibility-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.25rem 0.5rem;
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
    font-weight: 500;
}

.visibility-badge--private {
    background: var(--surface-secondary);
    color: var(--text-muted);
}

.visibility-badge--visible {
    background: var(--success-light);
    color: var(--success);
}
```

## API Contract

### PUT /api/cards/{id}/visibility

**Request:**
```json
{
    "visible_to_friends": false
}
```

**Response (200 OK):**
```json
{
    "id": "uuid",
    "year": 2025,
    "title": "My Goals",
    "category": "personal",
    "is_finalized": true,
    "visible_to_friends": false,
    "created_at": "...",
    "updated_at": "..."
}
```

**Error Responses:**
- `401 Unauthorized` - Not logged in
- `403 Forbidden` - Card belongs to another user
- `404 Not Found` - Card doesn't exist

### PUT /api/cards/visibility/bulk

**Request:**
```json
{
    "card_ids": ["uuid1", "uuid2", "uuid3"],
    "visible_to_friends": true
}
```

**Response (200 OK):**
```json
{
    "updated_count": 3
}
```

**Error Responses:**
- `401 Unauthorized` - Not logged in
- `400 Bad Request` - Invalid request body or empty card_ids

**Note:** Cards not owned by the user are silently skipped (no error).

### POST /api/cards/{id}/finalize (Updated)

**Request (optional body):**
```json
{
    "visible_to_friends": true
}
```

If no body or `visible_to_friends` not specified, defaults to `true`.

### Updated GET /api/cards Response

All card endpoints now include `visible_to_friends`:
```json
{
    "id": "uuid",
    "year": 2025,
    "visible_to_friends": true,
    // ... other fields
}
```

### GET /api/friends/{id}/cards

Only returns cards where `visible_to_friends = true`. Private cards are completely excluded from the response (no indication they exist).

## Implementation Order

1. **Database Migration** - Add `visible_to_friends` column with default true
2. **Model Update** - Add `VisibleToFriends` field to BingoCard struct
3. **Service Layer** - Update all card queries to include visibility field
4. **Service Layer** - Add `UpdateVisibility()` and `BulkUpdateVisibility()` methods
5. **Card Handler** - Add `UpdateCardVisibility` and `BulkUpdateVisibility` handlers
6. **Card Handler** - Update `FinalizeCard` to accept visibility parameter
7. **Friend Service** - Filter `GetFriendCards()` by visibility
8. **Routes** - Register new endpoints
9. **Frontend API** - Add `updateVisibility` and `bulkUpdateVisibility` methods
10. **Frontend UI - Dashboard** - Add visibility toggle under progress section
11. **Frontend UI - Finalized Card** - Add visibility toggle in header (top right)
12. **Frontend UI - Finalization Modal** - Add visibility checkbox
13. **Frontend UI - Archive** - Add bulk controls and individual toggles
14. **Styles** - Add visibility toggle, badge, and bulk control styles
15. **Testing** - Unit tests for visibility logic
16. **Version Bump** - Update to v0.8.0 (new feature)

## Testing Plan

### Backend Tests

1. **Card Service Tests:**
   - Test `UpdateVisibility` with valid card and owner
   - Test `UpdateVisibility` with wrong owner (should fail)
   - Test `UpdateVisibility` with non-existent card
   - Test `BulkUpdateVisibility` updates multiple cards
   - Test `BulkUpdateVisibility` skips cards not owned by user
   - Test `GetCards` includes visibility field

2. **Friend Handler Tests:**
   - Test `GetFriendCards` excludes private cards
   - Test `GetFriendCards` includes visible cards
   - Test `GetFriendCard` returns 404 for private card

3. **Card Handler Tests:**
   - Test PUT `/api/cards/{id}/visibility` success
   - Test PUT `/api/cards/{id}/visibility` unauthorized
   - Test PUT `/api/cards/visibility/bulk` success
   - Test PUT `/api/cards/visibility/bulk` with empty array
   - Test `FinalizeCard` with `visible_to_friends: false`
   - Test `FinalizeCard` defaults to visible when no body
   - Test card responses include `visible_to_friends`

### Frontend Tests

1. Test visibility toggle updates API
2. Test UI reflects visibility state
3. Test bulk selection in archive view
4. Test bulk visibility update
5. Test finalization modal visibility checkbox

### Manual Testing Script

Add to `scripts/`:
```bash
# Test visibility toggle
# 1. Create and finalize a card with visibility unchecked
# 2. Verify card is private (friend cannot see)
# 3. Toggle visibility to public via card page
# 4. Verify friend can now see card
# 5. Create multiple archived cards
# 6. Use bulk controls to make all private
# 7. Verify friend sees no cards
```

## Security Considerations

1. **Authorization Check** - Only card owner can change visibility
2. **Friend Query Filter** - Private cards filtered at database level (not just UI)
3. **Direct Access Prevention** - Cannot access private card even with direct card ID via friend endpoint
4. **Default Safe** - Default visibility is "visible" to match current behavior (no breaking change)

## Migration Safety

- Existing cards default to `visible_to_friends = true`
- No breaking changes to existing functionality
- Friends continue to see all currently visible cards
- Users must explicitly set cards to private

## Implementation Notes

**Implemented November 2025**

### UI Placement (Final)

- **Dashboard**: Eye icon toggle on right side of each card row, between View/Continue and delete buttons
- **Card Editor (unfinalized)**: Button with icon + label in header row, upper right (parallel to Back button)
- **Finalized Card View**: Button with icon + label in header actions area
- **Archive**: Bulk controls and individual toggles as planned

### Dependencies Added

- **FontAwesome 6.5.1**: Added from cdnjs.cloudflare.com for eye/eye-slash icons
- **CSP Updates**: Added cdnjs.cloudflare.com to `script-src`, `style-src`, and `font-src` directives

### Files Modified

- `migrations/000008_card_visibility.up.sql` / `down.sql`
- `internal/models/card.go` - Added `VisibleToFriends` field
- `internal/services/card.go` - Updated queries, added `UpdateVisibility`, `BulkUpdateVisibility`
- `internal/handlers/card.go` - Added visibility handlers
- `internal/handlers/friend.go` - Filter by visibility
- `internal/middleware/security.go` - CSP updates for FontAwesome
- `cmd/server/main.go` - New routes
- `web/templates/index.html` - FontAwesome CSS link
- `web/static/js/api.js` - New API methods
- `web/static/js/app.js` - UI components and toggle logic
- `web/static/css/styles.css` - Visibility toggle styles
