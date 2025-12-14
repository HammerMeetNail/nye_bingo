# Flexible Cards Plan (Predefined Square Grids)

## Overview

Allow users to create bingo cards with predefined square grid sizes (2x2, 3x3, 4x4, 5x5), while retaining the existing “position is an integer” model.

This avoids rectangular grid complexity and keeps mobile rendering predictable.

## Goals

- Users can choose a grid size: **2x2, 3x3, 4x4, 5x5**
- Header labels are configurable **before finalization** (immutable after finalize)
  - Header length must be **1..N** where `N == grid_size` (cannot exceed columns)
- FREE space is **default ON** for new cards
- FREE can only be toggled during creation/draft editing (immutable after finalize)
- FREE placement is set by rules, then can be moved via drag/drop in draft; FREE cannot be moved by shuffle
- Existing cards continue working unchanged (defaults map to current 5x5 behavior)
- Users can “Clone & Edit” to create a new card when they want a different number of cells

## Non-Goals

- Rectangular grids (NxM)
- Grids larger than 5x5

## User Stories

1. As a user, I can choose a grid size (2/3/4/5) when creating a card
2. As a user, FREE space is enabled by default, but I can turn it off during creation
3. As a user, I can customize the header before finalizing
4. As a user, once a card is finalized, its header and layout rules are immutable
5. As a user, if I want a different size later, I can clone a card into a new draft and edit that instead

## Definitions

- `grid_size` = N (2, 3, 4, or 5)
- Total cell positions = `0..(N*N - 1)` in row-major order
- Item capacity:
  - If FREE ON: `N*N - 1`
  - If FREE OFF: `N*N`

## Header Rules

- Stored as `header_text` (string)
- Normalized for display: trim and uppercase
- Validation: `1 <= len(header_text) <= grid_size`
- Default header for new cards: `"BINGO"` truncated to `grid_size`
  - 2 → `BI`
  - 3 → `BIN`
  - 4 → `BING`
  - 5 → `BINGO`

Rendering rule:
- Render exactly `grid_size` header cells; if `header_text` is shorter, render blanks for remaining header cells.

Mutability rule:
- Header can be edited only when `is_finalized == false`.

## FREE Space Rules

FREE is **default ON** when creating a new card.

### Placement Rules

- For **3x3** and **5x5**: FREE position is always center (`(N*N)/2`)
  - 3x3 → 4
  - 5x5 → 12
- For **2x2** and **4x4**: FREE position is chosen randomly **among currently empty cells**

### Toggle Rules (Draft Only)

FREE can only be toggled while `is_finalized == false`.

Enabling FREE:
- Choose `freePos` using the placement rules.
- If `freePos` is occupied by an item:
  - If there is at least one empty position: move that occupied item to a randomly-chosen empty position.
  - If there are no empty positions (card full): block and prompt:
    - “Your card is full. Remove an item to add a FREE space.”
- Mark `freePos` as the FREE cell; it cannot hold an item.

Disabling FREE:
- Clear `free_space_position` and treat the previously-FREE cell as a normal empty cell.

### Moving FREE (Drag & Drop, Draft Only)

Once FREE is enabled and `free_space_position` is set, the user can move the FREE cell via drag and drop while `is_finalized == false`.

Rules:
- FREE movement is done by “dropping” the FREE cell onto a target position.
- If the target position is empty: move FREE to that position.
- If the target position has an item:
  - If there is at least one empty position: move the occupied item to a randomly-chosen empty position, then move FREE to the target position.
  - If there are no empty positions (card full): block and prompt:
    - “Your card is full. Remove an item to move the FREE space.”

Important constraint:
- FREE must **not** move due to shuffle. Shuffle only shuffles items around the FREE cell (FREE stays in place).

## Data Model Changes

Add configuration fields to the card (recommended fields):

- `grid_size` (small int, 2/3/4/5)
- `header_text` (string, max 5)
- `has_free_space` (bool)
- `free_space_position` (nullable int)

These fields must be returned in card APIs and used everywhere we currently assume 5x5.

## Database Plan

### Migration 1: Add configuration columns to `bingo_cards`

```sql
ALTER TABLE bingo_cards
  ADD COLUMN grid_size SMALLINT NOT NULL DEFAULT 5,
  ADD COLUMN header_text VARCHAR(5) NOT NULL DEFAULT 'BINGO',
  ADD COLUMN has_free_space BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN free_space_position INTEGER;

ALTER TABLE bingo_cards
  ADD CONSTRAINT bingo_cards_valid_grid_size
    CHECK (grid_size IN (2,3,4,5)),
  ADD CONSTRAINT bingo_cards_header_len
    CHECK (char_length(header_text) >= 1 AND char_length(header_text) <= grid_size),
  ADD CONSTRAINT bingo_cards_free_pos_null_when_disabled
    CHECK ((has_free_space = false AND free_space_position IS NULL) OR has_free_space = true),
  ADD CONSTRAINT bingo_cards_free_pos_in_range
    CHECK (free_space_position IS NULL OR (free_space_position >= 0 AND free_space_position < (grid_size * grid_size)));
```

Backfill strategy:
- Existing cards keep default behavior: `grid_size=5`, `header_text='BINGO'`, `has_free_space=true`
- Set `free_space_position` to 12 for existing cards (or compute on read); pick one approach and keep it consistent.

### Keep existing `bingo_items.position` constraint

The existing DB constraint limiting positions to `0..24` remains compatible with max 5x5.

## API Plan

### Extend `POST /api/cards`

Request fields (additive):
- `grid_size` (optional, defaults to 5)
- `header_text` (optional, defaults to truncated BINGO for size)
- `has_free_space` (optional, defaults to true)

Server responsibilities:
- Validate `grid_size` and `header_text` length
- Default FREE to ON when omitted
- Compute and store `free_space_position` when `has_free_space==true`
  - For 2/4, choose random among empties (creation time: all empty)

### Draft-only config update endpoint

Add draft-only endpoint for header + FREE toggling (recommended to keep separate from title/category meta):

- `PUT /api/cards/{id}/config`
  - Body: `{ "header_text": "...", "has_free_space": true/false }`
  - Enforce: card owned by user, `is_finalized == false`
  - Apply FREE toggle rules, including “move occupied item” and “block if full”

### Swap endpoint behavior (FREE-aware)

The existing swap endpoint (`POST /api/cards/{id}/swap`) should support moving FREE via drag and drop:

- Accept `position1` / `position2` that may include the current `free_space_position` or the target FREE position.
- If neither position involves FREE: keep existing “swap/move items” semantics.
- If one position is the FREE cell (or the swap would move FREE):
  - Update `bingo_cards.free_space_position` to the new position.
  - If the target is occupied and there is an empty cell, relocate the displaced item to a randomly-chosen empty position.
  - If there is no empty cell, return a clear error that the frontend can turn into the “remove an item” prompt.

### Clone endpoint

Add:
- `POST /api/cards/{id}/clone`

Clone request fields:
- `year` (optional default: source year)
- `title` (optional default: source title + “ (Copy)”)
- `category` (optional default: source category)
- `grid_size` (required; or default to source grid size)
- `has_free_space` (optional default: true)
- `header_text` (optional default: truncated BINGO for size)

Clone behavior:
- Create a new draft card with chosen config.
- Copy item **content** from source into new card (uncompleted).
- Place copied items into random available positions (excluding FREE position if enabled).
- If source item count exceeds new capacity:
  - Copy only up to capacity and include a message in the response indicating truncation occurred.
  - (Future enhancement) allow selecting which items to copy.

### OpenAPI updates

Update `web/static/openapi.yaml`:
- Card schema includes `grid_size`, `header_text`, `has_free_space`, `free_space_position`
- Item `position` description becomes `0..(grid_size^2 - 1)` (and “FREE position is reserved when enabled”)
- Stats/progress denominators are based on capacity, not hardcoded 24

## Frontend Plan

### Creation UX

Update the “Create Card” modal to include:
- Grid size selector (2/3/4/5)
- FREE toggle (default ON)
- Header input (max length = selected grid size)

After creation, draft editor should:
- Render grid based on `grid_size`
- Render FREE cell when enabled
- Use computed capacity for “items added” and finalize gating

### Draft-only config editing

In the draft editor, allow:
- Editing header text (disabled once finalized)
- Toggling FREE (disabled once finalized)
  - If blocked because full: show modal prompting the user to remove an item, then retry
- Moving the FREE cell via drag & drop (disabled once finalized)
  - If blocked because full: show modal prompting the user to remove an item, then retry

### Finalized UX

Finalized view remains interactive for marking completion, but:
- No header edits
- No FREE toggle
- No grid size changes

### Bingo detection toast

Replace hardcoded 5x5 logic with NxN logic:
- Check all N rows, N columns, and 2 diagonals
- Treat FREE as completed if enabled

### Anonymous mode

Update `web/static/js/anonymous-card.js` to store and enforce:
- `grid_size`, `header_text`, `has_free_space`, `free_space_position`
- Capacity checks and FREE placement rules
- Drag/drop FREE movement rules (and “not via shuffle” rule)

## Backend Algorithm Notes

### Validating positions

For a given card:
- Valid positions are `0..(grid_size*grid_size - 1)`
- If FREE enabled: the FREE position is not valid for item placement

### Counting bingos

For grid size N:
- Rows: N
- Cols: N
- Diagonals: 2
- Total possible bingos: `2N + 2`

## Phased Approach

### Phase 0: Audit & Alignment

1. Enumerate all hardcoded `5`, `24`, `25`, `12` usages in Go + JS
2. Confirm the “header shorter than grid size renders blanks” rule
3. Confirm clone truncation behavior (“copy up to capacity + message”)

### Phase 1: Database Migration

1. Add `grid_size`, `header_text`, `has_free_space`, `free_space_position` to `bingo_cards`
2. Backfill existing cards to preserve current 5x5 behavior
3. Add constraints for allowed grid sizes and header length

### Phase 2: Backend Core

1. Extend `models.BingoCard` to include configuration fields
2. Update card creation to accept config fields + defaults (FREE default ON)
3. Update item operations validation (add/update/swap/shuffle) to respect per-card bounds and FREE
4. Update stats and bingo counting to be dynamic NxN
5. Update shuffle so it never moves FREE (items shuffle around the FREE cell)
6. Update swap so it supports moving FREE (FREE-aware swap semantics)

### Phase 3: API + Spec

1. Update handlers for create and card responses
2. Add `PUT /api/cards/{id}/config` (draft-only)
3. Add `POST /api/cards/{id}/clone`
4. Update `web/static/openapi.yaml`

### Phase 4: Frontend Draft Editing

1. Update create card modal for size/FREE/header
2. Update grid render to use `grid_size` and FREE position
3. Replace all “/24” and capacity gating with computed capacity
4. Add draft-only header + FREE toggles (and “full card” prompt)
5. Enable dragging the FREE cell in draft mode and wire it to the FREE-aware swap behavior

### Phase 5: Frontend Finalized + Bingo Toast

1. Dynamic completion progress denominators
2. Dynamic NxN bingo detection toasts
3. Ensure finalized cards cannot change config

### Phase 6: Clone & Edit

1. Add “Clone & Edit” UI entry point
2. Implement clone modal and clone call
3. Display truncation message when applicable

### Phase 7: Testing & Polish

1. Unit tests for position validation and bingo counting for N=2,3,4,5
2. Handler tests for create/config/clone success + error cases
3. Manual UX verification on mobile for all grid sizes
