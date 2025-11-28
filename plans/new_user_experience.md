# New User Experience: Anonymous Card Creation

## Overview

Allow anonymous users to create and edit a single bingo card without logging in. When they attempt to finalize the card, prompt for registration or login. The card is then saved to their account automatically.

## Current Behavior

1. User visits site (not logged in)
2. Clicks "Create New Card" → redirected to `#login`
3. Must register/login first
4. Then can create and finalize card

## New Behavior

1. User visits site (not logged in)
2. Clicks "Create New Card" → allowed to proceed
3. Card stored in localStorage (client-side only)
4. Can add items, shuffle, edit freely
5. Clicks "Finalize" → modal prompts for Register or Login
6. After successful auth:
   - **New user**: Card is created and finalized on server
   - **Existing user**: Card is transferred with conflict resolution if needed
7. User sees their finalized card

## Technical Design

### Data Structure

**LocalStorage Key**: `yearofbingo_anonymous_card`

```javascript
{
  year: 2025,
  title: "My 2025 Goals",
  category: "personal",
  items: [
    { position: 0, text: "Read 12 books", notes: "" },
    { position: 1, text: "Learn to cook", notes: "" },
    // ... up to 24 items (positions 0-11, 13-24, skipping 12)
  ],
  createdAt: "2025-01-15T10:30:00Z",
  updatedAt: "2025-01-15T14:22:00Z"
}
```

### Implementation Phases

---

## Phase 1: Frontend - Anonymous Card Creation

### 1.1 Update Routing (`app.js`)

Remove `requireAuth()` wrapper from `#create` route:

```javascript
// Before
case 'create':
  this.requireAuth(() => this.renderCreate());
  break;

// After
case 'create':
  this.renderCreate();  // Allow anonymous access
  break;
```

### 1.2 New Anonymous Card Module

Create `web/static/js/anonymous-card.js`:

```javascript
const AnonymousCard = {
  STORAGE_KEY: 'yearofbingo_anonymous_card',

  // Check if anonymous card exists
  exists() { ... },

  // Get the anonymous card from localStorage
  get() { ... },

  // Save anonymous card to localStorage
  save(card) { ... },

  // Clear the anonymous card
  clear() { ... },

  // Check if user already has an anonymous card (limit to one)
  hasCard() { ... },

  // Add item to anonymous card
  addItem(position, text) { ... },

  // Remove item from anonymous card
  removeItem(position) { ... },

  // Update item text
  updateItem(position, text) { ... },

  // Shuffle items
  shuffle() { ... },

  // Get item count
  getItemCount() { ... },

  // Check if card is ready to finalize (24 items)
  isReady() { ... }
};
```

### 1.3 Update Card Creation Flow

Modify `showCreateCardModal()` and `handleCreateCardModal()`:

- If user is authenticated: use existing API flow
- If user is anonymous: create card in localStorage, navigate to `#create` (editor view)

### 1.4 Update Card Editor (`renderCreate()`)

Two modes:
1. **Authenticated mode**: Existing behavior (API calls)
2. **Anonymous mode**: All operations go to localStorage

Key functions to update:
- `renderCreate()` - check auth, load from API or localStorage
- `handleAddItem()` - save to API or localStorage
- `handleRemoveItem()` - save to API or localStorage
- `handleShuffle()` - save to API or localStorage
- Suggestions still come from API (public endpoint)

### 1.5 Anonymous Card Indicator

Add visual indicator when editing anonymous card:
- Banner at top: "This card is saved locally. Create an account to save it permanently."
- Maybe a subtle "unsaved" icon in the header

---

## Phase 2: Frontend - Finalize with Auth Prompt

### 2.1 Auth Modal for Finalize

When anonymous user clicks "Finalize Card":

```javascript
finalizeCard() {
  if (!this.user) {
    this.showFinalizeAuthModal();
    return;
  }
  // ... existing finalize logic
}
```

### 2.2 New Finalize Auth Modal

`showFinalizeAuthModal()` displays:

```
┌────────────────────────────────────────────┐
│  Create an Account to Save Your Card       │
│                                            │
│  Your bingo card is ready! Create an       │
│  account to finalize and save it.          │
│                                            │
│  [Register]  [Login]  [Cancel]             │
│                                            │
└────────────────────────────────────────────┘
```

### 2.3 Inline Registration/Login

On "Register" click:
- Show inline registration form in modal
- On success: trigger card transfer flow

On "Login" click:
- Show inline login form in modal
- On success: trigger card transfer flow with conflict check

---

## Phase 3: Backend - Card Import Endpoint

### 3.1 New Endpoint: `POST /api/cards/import`

**Purpose**: Accept a fully-formed card from client and create it on server

**Request Body**:
```json
{
  "year": 2025,
  "title": "My 2025 Goals",
  "category": "personal",
  "items": [
    { "position": 0, "text": "Read 12 books" },
    { "position": 1, "text": "Learn to cook" }
  ],
  "finalize": true
}
```

**Response (Success)**:
```json
{
  "id": "uuid",
  "year": 2025,
  "title": "My 2025 Goals",
  "is_finalized": true,
  "items": [ ... ]
}
```

**Response (Conflict - existing card for year)**:
```json
{
  "error": "card_exists",
  "message": "You already have a card for 2025",
  "existing_card": {
    "id": "uuid",
    "title": "Existing Card Title",
    "is_finalized": true,
    "item_count": 24
  }
}
```

### 3.2 Card Service: `ImportCard()`

New method in `internal/services/card.go`:

```go
func (s *CardService) ImportCard(ctx context.Context, userID uuid.UUID, req ImportCardRequest) (*models.BingoCard, error) {
  // 1. Check for existing card for this year
  // 2. If conflict, return special error with existing card info
  // 3. Create card
  // 4. Add all items in a transaction
  // 5. If finalize=true, mark as finalized
  // 6. Return complete card
}
```

### 3.3 Conflict Detection

When importing, check for conflicts:

```go
existingCards, _ := s.GetByUserAndYear(ctx, userID, req.Year)
if len(existingCards) > 0 {
  // Return conflict error with existing card details
}
```

---

## Phase 4: Frontend - Conflict Resolution

### 4.1 Conflict Modal

When import returns `card_exists` error:

```
┌────────────────────────────────────────────────────┐
│  You Already Have a 2025 Card                      │
│                                                    │
│  "Existing Card Title" (24 items, finalized)       │
│                                                    │
│  What would you like to do?                        │
│                                                    │
│  [Keep Existing]    [Replace with New]             │
│        [Save as New Card]                          │
│                                                    │
└────────────────────────────────────────────────────┘
```

### 4.2 Conflict Options

1. **Keep Existing**: Discard anonymous card, navigate to existing card
2. **Replace with New**: Delete existing card, import anonymous card (dangerous - confirm)
3. **Save as New Card**: Prompt for new title, import with different title

### 4.3 Implementation

```javascript
async handleCardConflict(existingCard, anonymousCard) {
  const choice = await this.showConflictModal(existingCard);

  switch (choice) {
    case 'keep':
      AnonymousCard.clear();
      window.location.hash = `#card/${existingCard.id}`;
      break;

    case 'replace':
      if (await this.confirmReplace(existingCard)) {
        await API.cards.delete(existingCard.id);
        await this.importAnonymousCard();
      }
      break;

    case 'new':
      const newTitle = await this.promptForTitle();
      anonymousCard.title = newTitle;
      await this.importAnonymousCard(anonymousCard);
      break;
  }
}
```

---

## Phase 5: Polish and Edge Cases

### 5.1 Navigation Guards

- If user has anonymous card and navigates to `#login` or `#register`, show reminder
- After successful auth, automatically check for anonymous card to import

### 5.2 Card Expiration (Optional)

Consider adding expiration to anonymous cards:
- Cards created in previous years could be auto-cleared
- Or prompt user: "Your 2024 card draft was found. Import it or discard?"

### 5.3 "Get Started" Button Update

Update the landing page "Get Started" button:
- If logged in: go to dashboard
- If not logged in: go to `#create` (start anonymous card)

### 5.4 Multiple Card Prevention

Before creating anonymous card, check if one already exists:
```javascript
if (AnonymousCard.exists()) {
  // Navigate to existing anonymous card editor
  window.location.hash = '#create';
  return;
}
```

### 5.5 Clear Anonymous Card on Logout

When user logs out, optionally clear anonymous card or keep it for next session.

---

## File Changes Summary

### New Files
- `web/static/js/anonymous-card.js` - Anonymous card localStorage management

### Modified Files

**Backend**:
- `internal/handlers/card.go` - Add `ImportCard` handler
- `internal/services/card.go` - Add `ImportCard` method
- `cmd/server/main.go` - Register new route

**Frontend**:
- `web/static/js/app.js` - Update routing, card creation, finalize flow
- `web/static/js/api.js` - Add `API.cards.import()` method
- `web/templates/index.html` - Include new JS file
- `web/static/css/styles.css` - Styles for new modals and indicators

---

## Testing Plan

### Manual Testing

1. **Anonymous card creation**: Create card without login, add items, verify localStorage
2. **Registration flow**: Finalize → Register → Verify card saved
3. **Login flow (no conflict)**: Finalize → Login (new user) → Verify card saved
4. **Login flow (with conflict)**: Finalize → Login (existing user with card) → Test all conflict options
5. **Browser refresh**: Verify anonymous card persists
6. **Multiple cards**: Verify can only have one anonymous card

### Automated Tests

1. **Unit tests**: `AnonymousCard` module functions
2. **Go tests**: `ImportCard` service method and handler
3. **Integration tests**: Full flow from anonymous creation to import

---

## Migration Notes

No database migrations required. This feature is purely additive.

---

## Version

This feature should increment the MINOR version (e.g., 0.4.0 → 0.5.0) as it's a new feature that is backwards compatible.
