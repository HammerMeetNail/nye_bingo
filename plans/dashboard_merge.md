# Dashboard and Archive Merge Plan

## Current State

### Dashboard (`#dashboard`)
- Shows **all cards** from all years (via `API.cards.list()` → `ListByUser`)
- Features:
  - "Actions" dropdown with Export option
  - "+ New Card" button
  - Simple card previews with inline visibility toggle (eye icon)
  - Delete button per card
  - Progress bar

### Archive (`#archive`)
- Shows only **finalized cards from past years** (via `API.cards.getArchive()`)
- Features:
  - Select All / Deselect All buttons
  - Bulk visibility controls (Make Visible / Make Private)
  - Checkbox selection for each card
  - Visibility badges ("Visible to friends" / "Private")
  - "Archived" badge
  - Nicer card preview layout with hover effects

## Problem
Having both pages is redundant since all cards already appear in the dashboard. The Archive's presentation is superior.

## Proposed Solution

### Single Unified Dashboard

**Layout:**
```
┌─────────────────────────────────────────────────────────────────┐
│ My Bingo Cards                                                  │
│                                                                 │
│ [Select All] [Deselect All]  "3 selected"                       │
│                                                                 │
│ Sort: [Recently Updated ▼]      [Actions ▼]  [+ New Card]       │
│                                                                 │
│ Actions dropdown contains:                                      │
│   - Make Visible    (disabled + tooltip when nothing selected)  │
│   - Make Private    (disabled + tooltip when nothing selected)  │
│   - Delete          (disabled + tooltip when nothing selected)  │
│   - ─────────────                                               │
│   - Export Cards                                                │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│ [Card Preview - Archive style with checkbox]                    │
│ [Card Preview - with "Archived" badge for past years]           │
│ [Card Preview - Archive style with checkbox]                    │
└─────────────────────────────────────────────────────────────────┘
```

**Sort Options:**
- Recently Updated (default)
- Year (newest first)
- Year (oldest first)
- Name (A-Z)
- Name (Z-A)
- Completion % (highest first)
- Completion % (lowest first)

### Key Design Decisions

1. **Button Layout**: Keep "+ New Card" as a standalone primary button (most common action). All other bulk/utility actions go in the Actions dropdown.

2. **Actions Dropdown Contents**:
   - Make Visible (bulk) - with eye icon - *disabled when no selection*
   - Make Private (bulk) - with eye-slash icon - *disabled when no selection*
   - Delete (bulk) - with trash icon - *disabled when no selection*
   - Separator
   - Export Cards - always enabled

3. **Card Presentation**: Use Archive-style card previews for ALL cards:
   - Checkbox for selection
   - Visibility badge (not just an icon)
   - "Archived" badge for past-year finalized cards
   - Progress bar
   - No individual delete button (moved to bulk action)

4. **Sorting**:
   - Default sort: Recently Updated (by `updated_at` descending)
   - Sort selector dropdown with options for year, name, completion %
   - Sort preference persisted in localStorage

5. **Selection State**:
   - Show selection controls (Select All/Deselect All) on their own row
   - Display "X selected" count when cards are selected
   - Bulk actions in dropdown show tooltip "Select cards first" when disabled

## Implementation Steps

### Phase 1: Backend Changes
1. Create new endpoint or modify `GET /api/cards` to return ALL cards with an `is_archived` computed field
   - `is_archived = true` when `year < currentYear AND is_finalized = true`
   - This avoids needing two API calls
2. Add `DELETE /api/cards/bulk` endpoint for bulk card deletion
   - Accepts `{ card_ids: [...] }`
   - Returns count of deleted cards

### Phase 2: Frontend - Unified Dashboard
1. Update `renderDashboard()` to fetch all cards (including archived)
2. Replace card preview rendering with Archive-style previews
3. Add selection state management (rename `selectedArchiveCards` to `selectedCards`)
4. Restructure header layout:
   - Row 1: Title
   - Row 2: Select All / Deselect All / "X selected" count
   - Row 3: Sort dropdown, Actions dropdown, + New Card button
5. Remove individual delete buttons from card previews

### Phase 3: Sorting
1. Add sort dropdown with options:
   - Recently Updated (default)
   - Year (newest/oldest)
   - Name (A-Z/Z-A)
   - Completion % (highest/lowest)
2. Implement client-side sorting function
3. Persist sort preference in localStorage
4. Re-sort when selection changes or cards are modified

### Phase 4: Actions Dropdown Enhancement
1. Add bulk visibility controls to dropdown
2. Add bulk delete to dropdown with confirmation dialog
3. Add separator between bulk actions and export
4. Implement disabled state with tooltip for bulk actions
5. Wire up `bulkDeleteCards()` function

### Phase 5: Navigation Cleanup
1. Remove "Archive" link from navigation
2. Update all `#archive` links to `#dashboard`
3. Redirect `#archive` route to `#dashboard`
4. Update `#archive-card/{id}` to `#card/{id}` (or keep as alias)

### Phase 6: Code Cleanup
1. Remove `renderArchive()` function (or keep as redirect)
2. Merge `renderArchiveCardPreview()` into unified `renderCardPreview()`
3. Consolidate selection-related functions
4. Update CSS - rename archive-specific styles for reuse

### Phase 7: Testing & Polish
1. Test all card states (finalized, unfinalized, current year, past year)
2. Verify bulk operations work correctly (visibility, delete)
3. Test sorting options
4. Test empty state
5. Verify mobile responsiveness
6. Test localStorage persistence of sort preference

## Files to Modify

### Backend
- `internal/handlers/card.go` - Add `is_archived` field to list response, add bulk delete handler
- `internal/services/card.go` - Add `BulkDelete` method
- `cmd/server/main.go` - Register new bulk delete route

### Frontend
- `web/static/js/app.js` - Main UI changes:
  - Rewrite `renderDashboard()` with new layout
  - Add sorting logic and `sortCards()` function
  - Add `bulkDeleteCards()` function
  - Consolidate selection functions
  - Remove/redirect archive functions
- `web/static/js/api.js` - Add `cards.bulkDelete()` method
- `web/static/css/styles.css` - Consolidate/rename archive styles, add sort dropdown styles, add disabled dropdown item styles

### Templates
- `web/templates/index.html` - Remove Archive nav link

### Documentation
- `CLAUDE.md` - Update API routes, remove archive references

## Version
This change warrants a minor version bump (new feature/UI redesign): **v0.9.0**

---

## Implementation Status: COMPLETED

### What Was Implemented

All phases completed with the following adjustments:

#### Phase 1: Backend Changes ✓
- Added `is_archived` boolean column to `bingo_cards` table (migration 000009)
- Added `IsArchived` field to `BingoCard` model
- Added `BulkDelete` and `BulkUpdateArchive` methods to CardService
- Added handlers for `DELETE /api/cards/bulk` and `PUT /api/cards/archive/bulk`

**Key Change from Original Plan:** Archive is now a **manual user action**, not automatically computed. Users explicitly archive/unarchive cards via the dashboard Actions dropdown. This allows users to:
- Keep current year cards archived if desired
- Unarchive past year cards they want to highlight
- Have full control over the "Archived" badge display

#### Phase 2: Frontend - Unified Dashboard ✓
- Rewrote `renderDashboard()` with new header layout
- Added `renderDashboardCardPreview()` using archive-style cards
- Selection state management with `updateDashboardSelection()`
- Checkbox selection for each card

#### Phase 3: Sorting ✓
- Added sort dropdown with 7 options:
  - Recently Updated (default)
  - Year (newest/oldest first)
  - Name (A-Z / Z-A)
  - Completion % (highest/lowest first)
- Client-side sorting via `getSortedCards()`
- Sort preference persisted in localStorage (`dashboard-sort-preference`)
- `changeDashboardSort()` updates preference and re-renders

#### Phase 4: Actions Dropdown Enhancement ✓
- Actions dropdown contains:
  - Archive (bulk) - with archive icon
  - Unarchive (bulk) - with archive icon
  - Make Visible (bulk) - with eye icon
  - Make Private (bulk) - with eye-slash icon
  - Delete (bulk) - with trash icon
  - Separator
  - Export Cards - always enabled
- Disabled state with `.dropdown-item--disabled` class when no cards selected
- Tooltip "Select cards first" shown on hover for disabled items
- `bulkSetArchive()`, `bulkSetVisibility()`, `bulkDeleteCards()` functions

#### Phase 5: Navigation Cleanup ✓
- Removed "Archive" from navigation in `index.html`
- `#archive` route redirects to `#dashboard`
- `#archive-card/{id}` still works for viewing card stats

#### Phase 6: Code Cleanup ✓
- Removed old `renderArchive()` and `renderArchiveCardPreview()` functions
- Removed duplicate archive-related code
- Consolidated selection functions
- Updated CSS with dashboard-specific styles

### Files Modified

**Backend:**
- `migrations/000009_add_is_archived.up.sql` (NEW)
- `migrations/000009_add_is_archived.down.sql` (NEW)
- `internal/models/card.go` - Added `IsArchived` field
- `internal/services/card.go` - Added bulk methods, updated all queries
- `internal/handlers/card.go` - Added handlers and request types
- `cmd/server/main.go` - Registered new routes

**Frontend:**
- `web/static/js/api.js` - Added `bulkDelete()` and `bulkUpdateArchive()` methods
- `web/static/js/app.js` - Major rewrite of dashboard, removed old archive code
- `web/static/css/styles.css` - Added dashboard controls and card preview styles
- `web/templates/index.html` - Removed Archive nav, bumped version to v0.9.0

**Documentation:**
- `CLAUDE.md` - Updated API routes, added Card Archive section
- `README.md` - Updated features and API endpoints
- `plans/dashboard_merge.md` - This file (implementation status)
