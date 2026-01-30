# Premium Templates + 1‑Click New Year Rollover Plan

## Overview

Add a Premium-only “Templates” feature that lets users:
1) save reusable card templates, and
2) create a new year’s card from a template (or roll over an existing card) in one click.

This must be **100% additive**: free users can still create cards, clone cards, and manage cards exactly as today.

**Prerequisites**
- Billing + entitlements foundation exists (see `plans/monetization.md`), including a reliable server-side `IsPremium(user)` check.

---

## Goals

- Premium users can create, edit, and delete templates.
- Premium users can generate a new card from a template with a **fast, friendly UX**.
- Premium users can “roll over” a card to the next year with options like “carry over incomplete”.
- All template/rollover operations:
  - are **authorized** (user owns resource),
  - are **server-enforced** for Premium entitlement,
  - validate grid rules from `plans/flexible_cards.md`,
  - never break existing card flows.
- Strong testing coverage (service + handler), written TDD-first.

## Decisions (Proposed Defaults)

These defaults remove ambiguity so an agent can implement without guesswork.

### Downgrade Behavior (No “Hostage Data”)
- If a user loses Premium, they can still **view** their templates and **see** template contents.
- Premium is required to **create/edit/delete** templates and to **create cards from templates** or **roll over**.

### Rollover Data Rules
- Rollover always resets:
  - `is_completed=false`
  - `completed_at=NULL`
  - `notes=NULL`
  - `proof_url=NULL`
- Rollover is allowed from finalized cards or drafts that contain at least 1 item.

### Limits (Reasonable, Non-Predatory)
- Max templates per user: **50**
- Template item max length: **500 chars** (match existing card item limit)
- Template item count: `<= capacity` (same as cards; depends on grid size and FREE)

---

## Implementation Details (Concrete Enough for Agentic Implementation)

This section is intentionally copy/paste oriented for a less-capable implementation agent.

### File Changes Summary

**Backend**
- Add migrations:
  - `migrations/000015_templates.up.sql`
  - `migrations/000015_templates.down.sql`
- Add models:
  - `internal/models/template.go`
- Add service:
  - `internal/services/template.go`
- Add handler:
  - `internal/handlers/templates.go`
- Modify existing:
  - `cmd/server/main.go` (routes)
  - `web/static/openapi.yaml` (docs)
  - `web/static/js/api.js` (client)
  - `web/static/js/app.js` (UI + route)

### Migration Details (Include updated_at trigger)

In the `up.sql`, after creating `card_templates`, add:
```sql
CREATE TRIGGER update_card_templates_updated_at
  BEFORE UPDATE ON card_templates
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();
```

### Models (Exact Structs)

Create `internal/models/template.go`:
```go
package models

import (
  "time"

  "github.com/google/uuid"
)

type CardTemplate struct {
  ID                     uuid.UUID `json:"id"`
  UserID                 uuid.UUID `json:"user_id"`
  Name                   string    `json:"name"`
  Category               *string   `json:"category,omitempty"`
  GridSize               int       `json:"grid_size"`
  HeaderText             string    `json:"header_text"`
  HasFreeSpace           bool      `json:"has_free_space"`
  DefaultVisibleToFriends bool     `json:"default_visible_to_friends"`
  CreatedAt              time.Time `json:"created_at"`
  UpdatedAt              time.Time `json:"updated_at"`
}

type CardTemplateItem struct {
  ID         uuid.UUID `json:"id"`
  TemplateID uuid.UUID `json:"template_id"`
  SortOrder  int       `json:"sort_order"`
  Content    string    `json:"content"`
  CreatedAt  time.Time `json:"created_at"`
}

type TemplateWithItems struct {
  Template CardTemplate       `json:"template"`
  Items    []CardTemplateItem `json:"items"`
}
```

### Service (Key Algorithms + Suggested SQL)

#### List templates
```sql
SELECT id, user_id, name, category, grid_size, header_text, has_free_space, default_visible_to_friends, created_at, updated_at
FROM card_templates
WHERE user_id = $1
ORDER BY updated_at DESC;
```

#### Get template + items
```sql
-- template
SELECT id, user_id, name, category, grid_size, header_text, has_free_space, default_visible_to_friends, created_at, updated_at
FROM card_templates
WHERE id = $1 AND user_id = $2;

-- items
SELECT id, template_id, sort_order, content, created_at
FROM card_template_items
WHERE template_id = $1
ORDER BY sort_order ASC;
```

#### Replace items (transaction)
```sql
DELETE FROM card_template_items WHERE template_id = $1;
-- then insert rows with sort_order 0..n-1
INSERT INTO card_template_items (template_id, sort_order, content) VALUES ($1, $2, $3);
```

#### Create card from template (algorithm)
1. Load template + items.
2. Validate item count <= capacity.
3. Create destination card using `CardService.Create`:
   - Year/title/category taken from request (title should be non-NULL by default).
   - Grid/header/free from template.
   - `visible_to_friends` set from template default.
4. Compute positions:
   - `total := gridSize*gridSize`
   - `freePos := card.FreeSpacePos` (from created card)
   - `available := []int{0..total-1 excluding freePos}`
   - if shuffle ON: shuffle `available`
   - select first `len(items)` positions
5. Insert items into selected positions **atomically** (single transaction):
   - recommended: implement `CardService.AddItemsBatch(...)` helper that:
     - verifies ownership, not finalized, positions valid
     - inserts all `bingo_items` in one transaction

If `AddItemsBatch` is not implemented, the minimum acceptable alternative is:
- Begin a DB transaction in `TemplateService` and do direct inserts into `bingo_items` for the new card.

#### Rollover card (algorithm)
1. Load source card via `CardService.GetByID` and verify ownership.
2. Determine copied items:
   - if “all items”: copy all item contents
   - if “incomplete only”: copy only `is_completed=false`
3. Create destination card:
   - Year = target year
   - Title:
     - if source title exists: keep
     - else: set title to `fmt.Sprintf("%d Bingo Card", targetYear)` (non-NULL)
   - Category: copy
   - Grid/header/free: copy
4. Placement:
   - if shuffle ON: randomize positions (same as template create)
   - if shuffle OFF: keep original positions for copied items
5. Reset completions and notes (always):
   - new items are inserted with defaults: `is_completed=false`, `completed_at=NULL`, `notes=NULL`, `proof_url=NULL`
6. Handle conflict:
   - call `CardService.CheckForConflict(userID, year, title)` before create
   - on conflict, return 409 with `suggested_title` = `"<title> (Copy)"` (and if still conflicts, `"(Copy 2)"`, etc.)

### Handlers (Request/Response Shapes)

Create `internal/handlers/templates.go` with:
- `ListTemplates`
- `GetTemplate`
- `CreateTemplate`
- `UpdateTemplate`
- `ReplaceTemplateItems`
- `DeleteTemplate`
- `CreateCardFromTemplate`
- `RolloverCard`

Each premium endpoint must:
1. Require auth (`user != nil`)
2. Enforce Premium entitlement (server-side)
3. Parse/validate JSON with `MaxBytesReader` + `DisallowUnknownFields`

Suggested request bodies:
```json
POST /api/templates
{
  "name": "My Template",
  "category": "personal",
  "grid_size": 5,
  "header_text": "BINGO",
  "has_free_space": true,
  "default_visible_to_friends": true,
  "items": ["Goal 1", "Goal 2"]
}
```

```json
POST /api/templates/{id}/create-card
{
  "year": 2026,
  "title": "My 2026 Goals",
  "category": "personal",
  "shuffle_layout": true,
  "visible_to_friends": true
}
```

```json
POST /api/cards/{id}/rollover
{
  "year": 2026,
  "carry_over": "incomplete_only",
  "shuffle_layout": true,
  "title": "Optional override"
}
```

### Route Registration Sketch
Register routes in `cmd/server/main.go` (match existing `mux.Handle(..., requireRead/requireWrite/requireSession(...))` patterns):
```go
mux.Handle("GET /api/templates", requireRead(http.HandlerFunc(templatesHandler.ListTemplates)))
mux.Handle("GET /api/templates/{id}", requireRead(http.HandlerFunc(templatesHandler.GetTemplate)))

// premium writes
mux.Handle("POST /api/templates", requireWrite(http.HandlerFunc(templatesHandler.CreateTemplate)))
mux.Handle("PUT /api/templates/{id}", requireWrite(http.HandlerFunc(templatesHandler.UpdateTemplate)))
mux.Handle("PUT /api/templates/{id}/items", requireWrite(http.HandlerFunc(templatesHandler.ReplaceTemplateItems)))
mux.Handle("DELETE /api/templates/{id}", requireWrite(http.HandlerFunc(templatesHandler.DeleteTemplate)))
mux.Handle("POST /api/templates/{id}/create-card", requireWrite(http.HandlerFunc(templatesHandler.CreateCardFromTemplate)))
mux.Handle("POST /api/cards/{id}/rollover", requireWrite(http.HandlerFunc(templatesHandler.RolloverCard)))
```
Notes:
- These routes are *not* session-only: cookie sessions and API tokens with sufficient scope can call them.
- Premium entitlement checks happen inside the handlers/services; route middleware remains scope-based.

### OpenAPI Updates (Required)
File: `web/static/openapi.yaml`
- Add `CardTemplate` / `CardTemplateItem` schemas.
- Add endpoints:
  - `GET /templates`, `GET /templates/{id}`
  - `POST /templates`, `PUT /templates/{id}`, `PUT /templates/{id}/items`, `DELETE /templates/{id}`
  - `POST /templates/{id}/create-card`
  - `POST /cards/{id}/rollover`

Concrete OpenAPI sketch (copy/paste shape):
```yaml
  /templates:
    get:
      summary: List templates
      responses:
        '200':
          description: Templates
    post:
      summary: Create template (Premium)
      responses:
        '201':
          description: Created

  /templates/{id}/create-card:
    post:
      summary: Create card from template (Premium)
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [year]
              properties:
                year: { type: integer }
                title: { type: string }
                category: { type: string }
                shuffle_layout: { type: boolean, default: true }
                visible_to_friends: { type: boolean }
```

### Frontend (Concrete UI Plan)

Add route `#templates`:
- If not Premium: render an upgrade CTA (no nagging modal).
- If Premium:
  - Fetch templates list
  - Render list with actions

Suggested `API.templates` client surface:
```js
API.templates = {
  async list() { return API.request('GET', '/api/templates'); },
  async get(id) { return API.request('GET', `/api/templates/${id}`); },
  async create(payload) { return API.request('POST', '/api/templates', payload); },
  async update(id, payload) { return API.request('PUT', `/api/templates/${id}`, payload); },
  async replaceItems(id, items) { return API.request('PUT', `/api/templates/${id}/items`, { items }); },
  async del(id) { return API.request('DELETE', `/api/templates/${id}`); },
  async createCard(id, payload) { return API.request('POST', `/api/templates/${id}/create-card`, payload); },
  async rollover(cardId, payload) { return API.request('POST', `/api/cards/${cardId}/rollover`, payload); },
};
```

### Testing Matrix (TDD)

Minimum recommended tests:
- Template validation:
  - invalid grid size/header length rejected
  - item length > 500 rejected
  - item count > capacity rejected
- Create card from template:
  - shuffle ON produces items in valid positions excluding FREE
  - shuffle OFF produces deterministic placement (if implemented)
- Rollover:
  - incomplete-only filters properly
  - notes/proof/completions reset
  - conflict returns 409 with suggested title
- Premium enforcement:
  - non-premium gets 403 for write endpoints

## Non-Goals

- Team/shared templates (Premium is per-account only).
- Public template marketplace.
- Automated reminders/notifications (if desired, create `plans/premium_reminders.md`).

---

## User Stories

1. As a Premium user, I can save a card as a template so I can reuse it later.
2. As a Premium user, I can create a new card from a template for a chosen year.
3. As a Premium user, I can roll over my 2025 card into a 2026 card, optionally carrying over incomplete items only.
4. As a Premium user, I can edit my template items and template defaults without affecting existing cards.
5. As any user (free or premium), I can still create cards manually and use existing clone/import/export flows unchanged.

---

## UX / Product Design

### Entry Points (No Nagging)
- Dashboard: add an “Actions” menu item `Templates` (visible to all, but gated on click if not Premium).
- Profile: show “Premium Features” section with “Templates” link/button.

### Templates Screen (`#templates`)
**Layout**
- Header: “Templates”
- List: template cards with name, grid size, item count, and “Create Card” button
- Secondary actions: “Edit”, “Delete”
- Primary CTA: “New Template”

**Non-premium experience**
- Screen shows brief description + benefits + “Upgrade to Premium” CTA.

### Create Card From Template Flow
Modal fields:
- Year (default current year)
- Title (default to template name; editable; always send a non-NULL title to avoid null-title uniqueness edge cases)
- Category (optional; default from template)
- Visibility default (optional; default from template or user default)
- “Shuffle layout” toggle (default ON)
- “Create Draft” button (default)
  - Optionally add “Create & Finalize” later (out of scope v1; requires a dedicated plan for safe finalize UX).

### Rollover Flow (from a card)
On card views, Premium users see:
- “Rollover to {card.year + 1}” button (only if card is finalized or has >=1 item)

Modal options:
- Target year (default `card.year + 1`)
- Carry over:
  - “All items (reset completion)” (default)
  - “Incomplete items only (reset completion)”
- Title strategy:
  - Default: keep same title (if conflict, auto-suffix “(Copy)” and prompt)
- “Shuffle layout” toggle (default ON)

Conflict handling:
- If a card already exists with the same `(user, year, title)` (or null-title rules), show a friendly prompt:
  - “You already have a card named ‘X’ for 2026. Choose a new title.”
  - Provide an auto-suggested title with suffix.

---

## Data Model

Use separate template tables (do not overload `bingo_cards`).

### Migration (suggested): `000015_templates.*.sql`
If billing migration is `000014_*`, use `000015_*`. Otherwise choose the next number.

#### Table: `card_templates`
```sql
CREATE TABLE card_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    category VARCHAR(50),

    grid_size SMALLINT NOT NULL DEFAULT 5,
    header_text VARCHAR(5) NOT NULL DEFAULT 'BINGO',
    has_free_space BOOLEAN NOT NULL DEFAULT true,

    default_visible_to_friends BOOLEAN NOT NULL DEFAULT true,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT card_templates_valid_grid_size CHECK (grid_size IN (2,3,4,5)),
    CONSTRAINT card_templates_header_len CHECK (char_length(header_text) >= 1 AND char_length(header_text) <= grid_size)
);

CREATE INDEX idx_card_templates_user_id ON card_templates(user_id);
```

#### Table: `card_template_items`
```sql
CREATE TABLE card_template_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES card_templates(id) ON DELETE CASCADE,
    sort_order INT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(template_id, sort_order)
);

CREATE INDEX idx_card_template_items_template_id ON card_template_items(template_id);
```

Notes:
- Template items are ordered by `sort_order` (not grid positions).
- Enforce item count <= capacity in application validation (capacity depends on `grid_size` + `has_free_space`).

---

## Backend Design (Services + Handlers)

### New Models
- `internal/models/template.go`:
  - `CardTemplate`, `CardTemplateItem`
  - `TemplateWithItems` helper struct for API responses

### New Service: `TemplateService`
File: `internal/services/template.go`

Responsibilities:
- CRUD templates and template items
- Create card from template (delegates card creation to `CardService`)
- Create rollover card from an existing card (delegates to `CardService`)

Key methods (concrete enough to implement):
```go
type TemplateService struct {
  db *pgxpool.Pool
  cardService *CardService
  entitlements Entitlements // interface that can answer IsPremium(userID)
}

func (s *TemplateService) List(ctx context.Context, userID uuid.UUID) ([]models.CardTemplate, error)
func (s *TemplateService) Get(ctx context.Context, userID, templateID uuid.UUID) (*models.TemplateWithItems, error)
func (s *TemplateService) CreateFromCard(ctx context.Context, userID, cardID uuid.UUID, name string) (*models.TemplateWithItems, error)
func (s *TemplateService) Create(ctx context.Context, userID uuid.UUID, params models.CreateTemplateParams) (*models.TemplateWithItems, error)
func (s *TemplateService) Update(ctx context.Context, userID, templateID uuid.UUID, params models.UpdateTemplateParams) (*models.TemplateWithItems, error)
func (s *TemplateService) ReplaceItems(ctx context.Context, userID, templateID uuid.UUID, items []string) (*models.TemplateWithItems, error)
func (s *TemplateService) Delete(ctx context.Context, userID, templateID uuid.UUID) error

func (s *TemplateService) CreateCardFromTemplate(ctx context.Context, userID, templateID uuid.UUID, params models.TemplateCreateCardParams) (*models.BingoCard, error)
func (s *TemplateService) RolloverCard(ctx context.Context, userID, cardID uuid.UUID, params models.RolloverParams) (*models.BingoCard, error)
```

### Validation Rules
- `name`: 1..100 chars
- `category`: optional, 1..50 chars
- Template config must match card config rules from `plans/flexible_cards.md`:
  - `grid_size` in (2,3,4,5)
  - `header_text` length 1..grid_size
  - `has_free_space` boolean
- Items:
  - each item trimmed, non-empty, max length 500 chars
  - count must be `<= capacity`:
    - capacity = `grid_size*grid_size` minus 1 if `has_free_space`

### Premium Enforcement (Server Side)
Any template/rollover write operation requires Premium:
- Create template
- Edit template
- Delete template
- Create card from template
- Rollover card

Implementation pattern:
- In handlers: check `entitlements.IsPremium(userID)` early; if false return `403 {"error":"Premium required"}`.
- Do not rely on frontend gating.

### Handlers
New file: `internal/handlers/templates.go`

Endpoints (prefer token-scope compatible; Premium check is independent):
- `GET /api/templates` (requireRead)
- `GET /api/templates/{id}` (requireRead)
- `POST /api/templates` (requireWrite + Premium)
  - supports either:
    - `{ "from_card_id": "uuid", "name": "..." }`, or
    - `{ "name": "...", "grid_size": 5, "header_text": "BINGO", "has_free_space": true, "items": [...] }`
- `PUT /api/templates/{id}` (requireWrite + Premium) (meta/config only)
- `PUT /api/templates/{id}/items` (requireWrite + Premium) (replace list)
- `DELETE /api/templates/{id}` (requireWrite + Premium)
- `POST /api/templates/{id}/create-card` (requireWrite + Premium)
- `POST /api/cards/{id}/rollover` (requireWrite + Premium)

Conflict behavior:
- If new card conflicts with the unique indexes on `bingo_cards`, return `409` with:
  - `{"error":"Card conflict","conflict":{"year":2026,"title":"..."},"suggested_title":"... (Copy)"}`
Frontend uses this to prompt for a new title.

### Route Registration
Register new routes in `cmd/server/main.go` and document them in `web/static/openapi.yaml`.

---

## Frontend Implementation (Vanilla JS, App Object)

### API Client
File: `web/static/js/api.js`
- Add `API.templates.*` methods for each endpoint.

### UI + Routing
File: `web/static/js/app.js`
- Add route `#templates` → `renderTemplates()`.
- Add modals:
  - Create template (from card or blank)
  - Edit template
  - Create card from template
  - Rollover card
- Add Premium gating modal that reuses the billing upgrade CTA from `plans/monetization.md`.

### Template Editing UX (Simple, List-Based)
Templates are edited as a **list**, not a grid:
- Add item text box + “Add”
- List of items with drag/drop reordering (optional v1; if too big, defer and create `plans/template_reorder.md`)
- “Save” replaces entire item list (single API call).

---

## Testing Plan (TDD)

### Go Unit Tests
1. `TemplateService` validation:
   - grid/header rules
   - item trimming, max lengths
   - capacity enforcement for 2/3/4/5 with/without free
2. Create-from-template behavior:
   - items placed into available positions excluding free cell
   - shuffle flag changes placement but preserves item content
3. Rollover behavior:
   - “incomplete only” correctly filters source items
   - completion reset
   - conflict returns 409 with suggested title
4. Handler auth + premium enforcement:
   - non-premium receives 403 for premium endpoints

**Make It Unit-Testable (No DB Required)**
The repo’s Go tests run without a database (see `scripts/test.sh`). Keep most logic testable without pgx by:
- putting validation into small pure functions (inputs → error),
- extracting placement into a deterministic helper that accepts an injected RNG.

Placement helper sketch:
```go
func choosePositions(gridSize int, freePos *int, itemCount int, shuffle bool, rng *rand.Rand) ([]int, error) {
  total := gridSize * gridSize
  available := make([]int, 0, total)
  for pos := 0; pos < total; pos++ {
    if freePos != nil && pos == *freePos {
      continue
    }
    available = append(available, pos)
  }
  if itemCount > len(available) {
    return nil, fmt.Errorf("itemCount exceeds capacity")
  }
  if shuffle {
    rng.Shuffle(len(available), func(i, j int) { available[i], available[j] = available[j], available[i] })
  }
  return available[:itemCount], nil
}
```
Then tests can use `rand.New(rand.NewSource(1))` for deterministic expectations.

### JavaScript Tests (optional but recommended)
- Add small utility tests for:
  - suggested title suffixing
  - input validation helpers (length trimming)

### Run Tests
- `./scripts/test.sh` (Go + JS in container)

---

## Rollout

1. Ship endpoints + UI behind Premium entitlement checks.
2. Soft launch:
   - enable Premium for a few accounts via manual grant/code
   - validate usability + conflicts
3. Public launch once Stripe billing is live.
