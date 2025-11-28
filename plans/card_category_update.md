# Multiple Cards Per Year Feature Plan

## Problem Statement

Currently, users are limited to exactly ONE bingo card per year. A user requested the ability to have multiple cards with different themes, for example:
- "Life Goals" - serious personal development goals
- "Foods to Try" - fun culinary adventures
- "Travel Bucket List" - places to visit

## Current Implementation

The one-card-per-year limit is enforced at three levels:

1. **Database**: `UNIQUE(user_id, year)` constraint on `bingo_cards` table
2. **Service**: `CardService.Create()` checks for existing card before creation
3. **Frontend**: Only shows create buttons for years without cards

## Design Decisions

After discussion, the following decisions were made:

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Naming approach** | Hybrid (category + title) | Maximum flexibility with optional structure |
| **Cards per year** | Unlimited | No artificial restrictions |
| **Category requirement** | Optional (defaults to showing year) | Backwards compatible, simple for quick cards |
| **Rename cards** | Yes, anytime | Full flexibility even after finalization |

---

## Proposed Solution: Hybrid Category + Title

### Database Changes

**Migration: Add category and title columns, update constraint**

```sql
-- Add category column (nullable, for optional categorization)
ALTER TABLE bingo_cards ADD COLUMN category VARCHAR(50);

-- Add title column (nullable, defaults to NULL for year-only display)
ALTER TABLE bingo_cards ADD COLUMN title VARCHAR(100);

-- Drop old unique constraint
ALTER TABLE bingo_cards DROP CONSTRAINT bingo_cards_user_id_year_key;

-- Add new unique constraint (prevent exact duplicates)
-- Using COALESCE to handle NULLs - cards are unique by user/year/title combo
ALTER TABLE bingo_cards ADD CONSTRAINT bingo_cards_user_year_title_unique
    UNIQUE(user_id, year, COALESCE(title, ''));
```

**Predefined Categories** (stored as strings, displayed in UI):
- `personal` - Personal Growth
- `health` - Health & Fitness
- `food` - Food & Dining
- `travel` - Travel & Adventure
- `hobbies` - Hobbies & Creativity
- `social` - Social & Relationships
- `professional` - Professional & Career
- `fun` - Fun & Silly

### Model Changes

**`internal/models/card.go`**
```go
type BingoCard struct {
    ID          uuid.UUID  `json:"id"`
    UserID      uuid.UUID  `json:"user_id"`
    Year        int        `json:"year"`
    Category    *string    `json:"category,omitempty"`  // NEW: optional
    Title       *string    `json:"title,omitempty"`     // NEW: optional
    IsActive    bool       `json:"is_active"`
    IsFinalized bool       `json:"is_finalized"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
    Items       []BingoItem `json:"items,omitempty"`
}

// DisplayName returns a human-readable name for the card
func (c *BingoCard) DisplayName() string {
    if c.Title != nil && *c.Title != "" {
        return *c.Title
    }
    return fmt.Sprintf("%d Bingo Card", c.Year)
}

type CreateCardParams struct {
    UserID   uuid.UUID
    Year     int
    Category *string  // NEW: optional
    Title    *string  // NEW: optional
}

type UpdateCardParams struct {
    Category *string
    Title    *string
}

// Valid categories
var ValidCategories = []string{
    "personal", "health", "food", "travel",
    "hobbies", "social", "professional", "fun",
}
```

### Service Changes

**`internal/services/card.go`**

1. Update `Create()` to:
   - Accept optional category and title parameters
   - Remove single-card-per-year check entirely (unlimited cards allowed)
   - Validate category is from allowed list (if provided)
   - Check for duplicate title in same year (if title provided)

2. Add `UpdateCardMeta()` for renaming cards

3. Update all queries to include `category` and `title` columns

```go
func (s *CardService) Create(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error) {
    // Validate category if provided
    if params.Category != nil && *params.Category != "" {
        if !isValidCategory(*params.Category) {
            return nil, ErrInvalidCategory
        }
    }

    // Validate title length if provided
    if params.Title != nil && len(*params.Title) > 100 {
        return nil, ErrTitleTooLong
    }

    // Check for duplicate title in same year (only if title provided)
    if params.Title != nil && *params.Title != "" {
        var exists bool
        err := s.db.QueryRow(ctx,
            "SELECT EXISTS(SELECT 1 FROM bingo_cards WHERE user_id = $1 AND year = $2 AND title = $3)",
            params.UserID, params.Year, *params.Title,
        ).Scan(&exists)
        if exists {
            return nil, ErrCardTitleExists
        }
    }

    // Create card (no max limit check - unlimited cards allowed)
    ...
}

func (s *CardService) UpdateMeta(ctx context.Context, cardID uuid.UUID, userID uuid.UUID, params models.UpdateCardParams) error {
    // Validate and update category/title
    // Allow updates anytime (before or after finalization)
    ...
}
```

### Handler Changes

**`internal/handlers/card.go`**

Update create request:
```go
type createCardRequest struct {
    Year     int     `json:"year"`
    Category *string `json:"category,omitempty"`  // NEW: optional
    Title    *string `json:"title,omitempty"`     // NEW: optional
}
```

Add new endpoint for updating card metadata:
```go
// PUT /api/cards/{id}/meta
type updateCardMetaRequest struct {
    Category *string `json:"category,omitempty"`
    Title    *string `json:"title,omitempty"`
}
```

### API Changes

**Create Card Request**
```json
POST /api/cards
{
    "year": 2024,
    "category": "food",           // optional
    "title": "Foods to Try"       // optional
}
```

**Update Card Metadata**
```json
PUT /api/cards/{id}/meta
{
    "category": "food",
    "title": "New Title"
}
```

**Card Response (all endpoints)**
```json
{
    "id": "uuid",
    "year": 2024,
    "category": "food",           // null if not set
    "title": "Foods to Try",      // null if not set
    "is_finalized": false,
    ...
}
```

**Categories List Endpoint** (for UI dropdown)
```json
GET /api/cards/categories

{
    "categories": [
        {"id": "personal", "name": "Personal Growth"},
        {"id": "health", "name": "Health & Fitness"},
        {"id": "food", "name": "Food & Dining"},
        {"id": "travel", "name": "Travel & Adventure"},
        {"id": "hobbies", "name": "Hobbies & Creativity"},
        {"id": "social", "name": "Social & Relationships"},
        {"id": "professional", "name": "Professional & Career"},
        {"id": "fun", "name": "Fun & Silly"}
    ]
}
```

### Frontend Changes

#### 1. Create Card UI

Replace the simple year buttons with a form that includes optional category and title:

```html
<div class="create-card-form">
    <h2>Create New Bingo Card</h2>

    <div class="form-group">
        <label for="card-year">Year</label>
        <select id="card-year">
            <option value="2024">2024</option>
            <option value="2025">2025</option>
        </select>
    </div>

    <div class="form-group">
        <label for="card-title">Title <span class="optional">(optional)</span></label>
        <input type="text" id="card-title"
               placeholder="e.g., Life Goals, Foods to Try"
               maxlength="100">
        <small class="text-muted">Leave blank to use "2024 Bingo Card"</small>
    </div>

    <div class="form-group">
        <label for="card-category">Category <span class="optional">(optional)</span></label>
        <select id="card-category">
            <option value="">None</option>
            <option value="personal">Personal Growth</option>
            <option value="health">Health & Fitness</option>
            <option value="food">Food & Dining</option>
            <option value="travel">Travel & Adventure</option>
            <option value="hobbies">Hobbies & Creativity</option>
            <option value="social">Social & Relationships</option>
            <option value="professional">Professional & Career</option>
            <option value="fun">Fun & Silly</option>
        </select>
    </div>

    <button onclick="App.createCard()">Create Card</button>
</div>
```

#### 2. Dashboard Updates

Show cards with title (or year fallback) and optional category badge:

```html
<div class="card-preview">
    <div class="card-header">
        <h3>Foods to Try</h3>
        <span class="year-badge">2024</span>
        <span class="category-badge category-food">Food & Dining</span>
    </div>
    <p>5/24 completed</p>
    <div class="progress-bar">...</div>
</div>

<!-- For cards without title -->
<div class="card-preview">
    <div class="card-header">
        <h3>2024 Bingo Card</h3>
    </div>
    <p>12/24 completed</p>
</div>
```

Cards grouped by year, with newest first.

#### 3. Card Editor/View Header

Show title prominently with edit capability:

```html
<header class="card-header">
    <div class="title-row">
        <h1 id="card-title">Foods to Try</h1>
        <button class="btn-icon" onclick="App.editCardMeta()" title="Edit title">
            ✏️
        </button>
    </div>
    <div class="meta-row">
        <span class="year-badge">2024</span>
        <span class="category-badge">Food & Dining</span>
    </div>
</header>
```

Edit modal for renaming:
```html
<div class="modal" id="edit-card-meta-modal">
    <h3>Edit Card</h3>
    <div class="form-group">
        <label>Title</label>
        <input type="text" id="edit-title" value="Foods to Try">
    </div>
    <div class="form-group">
        <label>Category</label>
        <select id="edit-category">...</select>
    </div>
    <button onclick="App.saveCardMeta()">Save</button>
</div>
```

#### 4. Archive View

Group by year, show all cards with titles:

```
## 2023
┌─────────────────────────────────────────────────┐
│ Life Goals                          Personal    │
│ 18/24 completed • 3 bingos                      │
├─────────────────────────────────────────────────┤
│ Foods to Try                        Food        │
│ 24/24 completed • 5 bingos ⭐                   │
└─────────────────────────────────────────────────┘

## 2022
┌─────────────────────────────────────────────────┐
│ 2022 Bingo Card                                 │
│ 12/24 completed • 1 bingo                       │
└─────────────────────────────────────────────────┘
```

#### 5. Friend Card View

Update selector to show title + year:

```html
<select id="friend-card-select" onchange="App.switchFriendCard(this.value)">
    <option value="card-id-1">Foods to Try (2024)</option>
    <option value="card-id-2">Life Goals (2024)</option>
    <option value="card-id-3">2023 Bingo Card (archived)</option>
</select>
```

#### 6. Category Styling (Optional)

Add subtle color coding for categories:

```css
.category-badge {
    font-size: 0.75rem;
    padding: 0.25rem 0.5rem;
    border-radius: 0.25rem;
    background: var(--surface);
}

.category-personal { border-left: 3px solid #6366f1; }
.category-health { border-left: 3px solid #10b981; }
.category-food { border-left: 3px solid #f59e0b; }
.category-travel { border-left: 3px solid #3b82f6; }
.category-hobbies { border-left: 3px solid #8b5cf6; }
.category-social { border-left: 3px solid #ec4899; }
.category-professional { border-left: 3px solid #64748b; }
.category-fun { border-left: 3px solid #f43f5e; }
```

---

## Implementation Phases

### Phase 1: Database Migration ✅
1. Create migration to add `category` and `title` columns
2. Update unique constraint
3. Test migration with existing data (existing cards get NULL title/category)

### Phase 2: Backend - Models & Services ✅
1. Update `BingoCard` model with new fields
2. Update `CardService.Create()` to accept optional category/title
3. Remove single-card-per-year restriction
4. Add `CardService.UpdateMeta()` for renaming
5. Add `GetCategories()` endpoint for listing valid categories
6. Update all card queries to include new columns
7. Add new error types (`ErrCardTitleExists`, `ErrInvalidCategory`, `ErrTitleTooLong`)

### Phase 3: Backend - Handlers & Routes ✅
1. Update `CardHandler.Create()` to accept category/title
2. Add `CardHandler.UpdateMeta()` for renaming cards
3. Add `CardHandler.GetCategories()` endpoint
4. Register new routes
5. Add tests for new functionality

### Phase 4: Frontend - Card Creation ✅
1. Replace year-button UI with form (year + title + category)
2. Update `API.cards.create()` to send category/title
3. Add `API.cards.updateMeta()` method
4. Add `API.cards.getCategories()` method
5. Handle validation errors

### Phase 5: Frontend - Card Display ✅
1. Update dashboard to show titles and category badges
2. Update card editor/viewer header with edit button
3. Add edit card metadata modal
4. Update archive view to group by year and show titles
5. Update friend card selector to show titles

### Phase 6: Polish & Testing ✅
1. Add category color styling (subtle badges)
2. Update tests (Go + JS)
3. Manual testing of all flows
4. Update version number (v0.3.0)

---

## Backwards Compatibility

- Existing cards keep `NULL` for category and title
- Display logic: if `title` is NULL, show "{year} Bingo Card"
- All existing features continue to work unchanged
- Users can add title/category to existing cards via edit modal

---

## Files to Modify

### Backend
- `migrations/000011_add_card_category_title.up.sql` (new)
- `migrations/000011_add_card_category_title.down.sql` (new)
- `internal/models/card.go`
- `internal/services/card.go`
- `internal/handlers/card.go`
- `cmd/server/main.go` (new route)

### Frontend
- `web/static/js/api.js`
- `web/static/js/app.js`
- `web/static/css/styles.css`

### Tests
- `internal/services/card_test.go`
- `internal/models/card_test.go`
- `web/static/js/tests/runner.js`

---

## Estimated Scope

| Phase | Files Changed | Complexity |
|-------|---------------|------------|
| Phase 1 | 2 new migration files | Low |
| Phase 2 | 2 files | Medium |
| Phase 3 | 2 files | Medium |
| Phase 4 | 2 files | Medium |
| Phase 5 | 3 files | Medium |
| Phase 6 | 3 files | Low |

**Total**: ~10 files modified, moderate complexity overall.

---

## Future Enhancements (Out of Scope)

- Category-specific suggestions (show relevant suggestions based on selected category)
- Category filtering on dashboard
- Card templates (pre-populated cards for common themes)
- Card duplication (copy last year's card to new year)
