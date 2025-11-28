# Export Bingo Cards as CSV

## Overview

Add the ability to export Bingo cards as CSV files. Users can export one or more cards from the Dashboard. Exports are always delivered as a ZIP file containing one CSV per card.

## Requirements

1. **Dashboard UI**: Add a dropdown/menu button to the left of "New Card" button
2. **Export Option**: The dropdown has an "Export" item (designed for future expansion)
3. **Card Selection**: Users can select one or many cards to export
4. **Export Format**: One CSV file per card, always delivered as a ZIP
5. **Scope**: Only the user's own cards (current year + archived)
6. **Delivery**: Client-side download only (no server storage)

## CSV Format

Each CSV file contains one row per bingo item with the following columns:

| Column | Description |
|--------|-------------|
| card_title | Card title or "YYYY Bingo Card" if no title |
| year | Card year |
| category | Card category (e.g., "Personal Growth") or empty |
| position | Grid position (0-24, excluding 12 which is FREE) |
| item_text | The goal/item text |
| completed | "yes" or "no" |
| completion_date | ISO date (YYYY-MM-DD) or empty |
| notes | Completion notes or empty |

## Implementation Plan

### Phase 1: Backend API Endpoint

**File: `internal/handlers/card.go`**

Add a new endpoint to fetch all exportable cards with full item details:

```
GET /api/cards/export
```

Response:
```json
{
  "cards": [
    {
      "id": "uuid",
      "title": "My Goals",
      "year": 2025,
      "category": "personal",
      "is_finalized": true,
      "items": [...]
    }
  ]
}
```

This endpoint returns:
- All current year cards (finalized and unfinalized)
- All archived cards (from past years)

**File: `cmd/server/main.go`**
- Add route: `GET /api/cards/export` → `cardHandler.ListExportable`

### Phase 2: Frontend - Dropdown Menu Component

**File: `web/static/js/app.js`**

Add dropdown menu in `renderDashboard()`:

```html
<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem;">
  <h2>My Bingo Cards</h2>
  <div style="display: flex; gap: 0.5rem;">
    <div class="dropdown">
      <button class="btn btn-secondary dropdown-toggle" id="actions-dropdown">
        Actions ▾
      </button>
      <div class="dropdown-menu" id="actions-menu">
        <button class="dropdown-item" onclick="App.showExportModal()">
          Export Cards
        </button>
      </div>
    </div>
    <button class="btn btn-primary" onclick="App.showCreateCardModal()">+ New Card</button>
  </div>
</div>
```

**File: `web/static/css/styles.css`**

Add dropdown styles:
- `.dropdown` - container with relative positioning
- `.dropdown-menu` - absolutely positioned menu, hidden by default
- `.dropdown-menu--visible` - shows the menu
- `.dropdown-item` - individual menu items

### Phase 3: Frontend - Export Modal

**File: `web/static/js/app.js`**

Add `showExportModal()` method:
1. Fetch all exportable cards via `API.cards.getExportable()`
2. Display modal with card list and checkboxes
3. "Select All" / "Deselect All" toggle
4. "Download" button (disabled until at least one card selected)

Modal layout:
```
┌─────────────────────────────────────┐
│ Export Cards                    [X] │
├─────────────────────────────────────┤
│ Select cards to export:             │
│                                     │
│ [✓] Select All                      │
│                                     │
│ [✓] 2025 Bingo Card (24 items)     │
│ [✓] Food Adventures 2025 (18 items)│
│ [ ] Travel Goals 2024 (Archived)   │
│ [ ] 2023 Bingo Card (Archived)     │
│                                     │
├─────────────────────────────────────┤
│ [Cancel]              [Download ZIP]│
└─────────────────────────────────────┘
```

### Phase 4: Frontend - CSV & ZIP Generation

**File: `web/static/js/app.js`**

Add export utilities:

1. `generateCSV(card)` - Creates CSV string for a single card
   - Proper CSV escaping (quotes, commas, newlines)
   - UTF-8 BOM for Excel compatibility
   - Returns string content

2. `generateExportZip(cards)` - Creates ZIP with all CSVs
   - Uses JSZip library (loaded from CDN or bundled)
   - Filename format: `{year}_{sanitized_title}.csv`
   - ZIP filename: `yearofbingo_export_{timestamp}.zip`

3. `downloadZip(blob, filename)` - Triggers browser download
   - Creates temporary anchor element
   - Uses URL.createObjectURL
   - Cleans up after download

**File: `web/templates/index.html`**

Add JSZip library:
```html
<script src="https://cdnjs.cloudflare.com/ajax/libs/jszip/3.10.1/jszip.min.js"></script>
```

### Phase 5: Frontend - API Client Update

**File: `web/static/js/api.js`**

Add to `cards` object:
```javascript
async getExportable() {
  return API.request('GET', '/api/cards/export');
}
```

## File Changes Summary

| File | Changes |
|------|---------|
| `internal/handlers/card.go` | Add `ListExportable` handler |
| `cmd/server/main.go` | Add `/api/cards/export` route |
| `web/static/js/api.js` | Add `getExportable()` method |
| `web/static/js/app.js` | Add dropdown, export modal, CSV/ZIP generation |
| `web/static/css/styles.css` | Add dropdown and export modal styles |
| `web/templates/index.html` | Add JSZip script tag |

## Testing

1. **Manual Testing**:
   - Export single card → ZIP with one CSV
   - Export multiple cards → ZIP with multiple CSVs
   - Verify CSV opens correctly in Excel/Google Sheets
   - Verify special characters (quotes, commas) are escaped
   - Test with cards that have no items vs. full cards
   - Test with archived cards

2. **Edge Cases**:
   - No cards to export → Show message, disable download
   - Cards with same title/year → Unique filenames (add suffix)
   - Very long titles → Truncate in filename
   - Empty notes/completion dates → Empty CSV cells

## Version

Increment to v0.4.0 (new feature).
