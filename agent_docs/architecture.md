# Architecture & Patterns

## Backend Structure

- `cmd/server/main.go` - Application entry point, wires up all dependencies and routes
- `internal/config/` - Environment-based configuration loading
- `internal/database/` - PostgreSQL pool (`postgres.go`), Redis client (`redis.go`), migrations (`migrate.go`)
- `internal/models/` - Data structures (User, Session, BingoCard, BingoItem, Suggestion, Friendship, Reaction)
- `internal/services/` - Business logic layer (UserService, AuthService, CardService, SuggestionService, FriendService, ReactionService)
- `internal/handlers/` - HTTP handlers that call services and return JSON
- `internal/middleware/` - Auth validation, CSRF protection, security headers, compression, caching, request logging
- `internal/logging/` - Structured JSON logging
- `scripts/` - Development/testing scripts (seed.sh, cleanup.sh, test-archive.sh) - use API, not direct DB access

## Frontend Structure

- `web/templates/index.html` - Single HTML entry point for SPA (main container has `id="main-container"`)
- `web/static/js/api.js` - API client with CSRF token handling, all methods under `API` object
- `web/static/js/app.js` - SPA router and all UI logic under global `App` object
- `web/static/css/styles.css` - Design system with CSS variables, uses OpenDyslexic font for bingo cells

**External Dependencies**: FontAwesome 6.5 loaded from cdnjs.cloudflare.com for icons (eye, eye-slash for visibility toggles). CSP allows cdnjs.cloudflare.com for script-src, style-src, and font-src.

## Frontend Patterns

**App Object**: All frontend logic lives in global `App` object. Key methods:
- `route()` - Hash-based routing, renders appropriate page
- `renderDashboard()` - Unified dashboard showing all cards (current year and archived) with sorting, selection, and bulk actions
- `renderFinalizedCard()` / `renderCardEditor()` - Card views (handles both authenticated and anonymous modes)
- `showItemDetailModal()` - Modal for viewing/completing items
- `renderFriends()` / `renderFriendCard()` - Friends list and viewing friend cards (with year selector for multiple cards)
- `renderArchiveCard()` - View for individual archived cards with stats
- `renderProfile()` - Account settings page (email verification status, privacy settings, change password)
- `renderAbout()` - Origin story and open source info
- `renderFAQ()` - Frequently asked questions (linked from navbar)
- `renderTerms()` / `renderPrivacy()` / `renderSecurity()` - Legal pages linked from footer
- `renderSupport()` - Contact form for support requests
- `openModal()` / `closeModal()` - Generic modal system
- `fillEmptySpaces()` - Auto-fill empty card slots with random suggestions

**API Object**: Wraps fetch calls with CSRF handling. Namespaced: `API.auth.*`, `API.cards.*`, `API.suggestions.*`, `API.friends.*`, `API.reactions.*`, `API.support.*`

**Adding New Features**: Add API methods to `api.js`, UI methods to `App` object in `app.js`, styles to `styles.css`

### Key Patterns

**Middleware Chain**: Requests flow through `requestLogger → securityHeaders → compress → cacheControl → csrfMiddleware → authMiddleware → handler`

**Rate Limiting**: Implemented at the application level for selected abuse/cost-sensitive paths. Auth endpoints use Redis-backed per-IP and per-email limits (with relaxed values in `APP_ENV=development` for E2E), AI endpoints and billing code redemption are rate-limited via middleware, and support form submissions have an IP-based Redis limiter. Upstream rate limiting (CDN/gateway) is still recommended as a second layer.

**Session Management**: Sessions stored in Redis first with PostgreSQL fallback. Token stored in HttpOnly cookie, hash stored in database.

**Privacy Model**: Friend search is opt-in. Users must enable "searchable" in their profile to appear in friend search results. Search only matches username (not email). Registration includes a checkbox for opting into discoverability.

**Card Visibility**: Cards have a `visible_to_friends` flag (default: true). Users can set individual cards as private or visible to friends. Private cards are completely hidden from friend views (no indication they exist). Visibility can be toggled via bulk actions on the dashboard or on individual card views during finalization.

**Card Archive**: Cards have an `is_archived` flag that users can toggle manually via the dashboard Actions menu. Archived cards display an "Archived" badge. This is a user action, not automatic based on year. The `#archive-card/{id}` route shows detailed stats for any card.

**Card Export**: Export uses the dashboard selection. Users select cards via checkboxes, then click Actions → Export Cards to download a ZIP file containing CSV files for each selected card. The export is disabled when no cards are selected.

**Card State Machine**: Cards start unfinalized (can add/remove/shuffle items), then finalize (locks layout, enables completion marking).

**Grid Positions**: 5x5 grid uses positions 0-24, with position 12 being the center FREE space. Items occupy 24 positions (excluding 12).

**Bingo Card Display**: Grid renders with B-I-N-G-O header row. Cell text is truncated with CSS line-clamp (4 lines desktop, 3 tablet, 2 mobile). Full text shown in modal on click. Finalized card view uses `.finalized-card-view` class with centered grid layout.

**Card Editor Layout**: The unfinalized card editor uses `.card-editor-layout` with responsive behavior:
- **Desktop** (>900px): Two-column CSS Grid - bingo grid on left (`.editor-grid`), sidebar on right (`.editor-sidebar`) containing input, action buttons, and suggestions. Sidebar has `margin-top` to align with first row of cells (below B-I-N-G-O header).
- **Mobile** (≤900px): Single-column flexbox with reordered elements using CSS `order`. The `.editor-sidebar` uses `display: contents` to "unwrap" so its children participate in parent flex ordering. Order: input (1) → grid (2) → actions (3) → suggestions (4) → delete (5).
- **Key classes**: `.editor-grid`, `.editor-sidebar`, `.editor-input`, `.editor-actions`, `.editor-suggestions`, `.editor-delete`

**Drag and Drop**: Unfinalized cards support drag and drop to rearrange items:
- **Desktop**: Native HTML5 drag and drop with `draggable="true"` attribute
- **Mobile**: Touch events with 300ms long-press to initiate drag, visual clone follows finger
- **API**: `POST /api/cards/{id}/swap` swaps two positions atomically (handles both filled and empty cells)
- **CSS**: `.bingo-cell[draggable="true"]` has `user-select: none` and `touch-action: none` to prevent text selection
- **Event delegation**: Listeners attached to grid element, not individual cells - no re-attachment needed after DOM updates

## Help & Legal Pages

Navbar contains FAQ link (`#faq`), footer contains About and legal pages (`#about`, `#terms`, `#privacy`, `#security`):
- **FAQ** (navbar) - Frequently asked questions about using the site, finding friends, managing cards
- **About** - Origin story, open source info, GitHub repository link
- **Terms of Service** - Account usage, acceptable use, content ownership, disclaimers
- **Privacy Policy** - Data collection, cookies (strictly necessary only), GDPR rights, international transfers
- **Security** - Infrastructure security, application security measures, responsible disclosure

**Cookie Policy**: Only strictly necessary cookies are used (session authentication, CSRF protection, Cloudflare security). No tracking or advertising cookies. Cloudflare Web Analytics is cookie-free. No cookie consent banner required under GDPR.

## Security Features (Phase 8)

- **Security Headers**: CSP (includes cdnjs.cloudflare.com for JSZip and FontAwesome in script-src, style-src, font-src), X-Frame-Options, X-Content-Type-Options, X-XSS-Protection, Referrer-Policy, Permissions-Policy, HSTS (in secure mode)
- **Compression**: Gzip compression for responses (with pool for efficiency)
- **Cache Control**: Content-hashed assets in `/static/dist/` get immutable cache (1 year); non-hashed assets use short cache with revalidation
- **Structured Logging**: JSON-formatted request logs with timing, status, and context

## Accessibility (Phase 8)

- Skip links for keyboard navigation
- ARIA labels and roles on interactive elements
- Focus visible styles for keyboard users
- Reduced motion support for users who prefer it
- Improved color contrast (WCAG 2.1 AA)
- Screen reader friendly loading states and toasts
