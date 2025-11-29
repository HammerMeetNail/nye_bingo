# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Year of Bingo is a web application for creating and tracking annual Bingo cards. Users create a 5x5 card with 24 personal goals (center is free space), then mark items complete throughout the year. Cards can be shared with friends who can react to completions.

**Domain**: yearofbingo.com

## Tech Stack

- **Backend**: Go 1.24+ using only net/http (no frameworks)
- **Frontend**: Vanilla JavaScript SPA with hash-based routing
- **Database**: PostgreSQL with pgx/v5 driver
- **Cache/Sessions**: Redis with go-redis/v9
- **Migrations**: golang-migrate/migrate/v4
- **Containerization**: Podman with compose.yaml

## Development Commands

```bash
# Full local rebuild (recommended)
make local

# View container logs
make logs

# Stop containers
make down

# Run linting
make lint

# Run all tests in container
make test

# Clean up everything including volumes
make clean
```

### Manual Commands

```bash
# Start local development environment
podman compose up

# Rebuild after Go changes
podman compose build --no-cache && podman compose up

# Run Go build locally
go build -o server ./cmd/server

# Download dependencies
go mod tidy

# Seed database with test data (includes archive cards)
./scripts/seed.sh

# Clean up test friendships
./scripts/cleanup.sh

# Test archive functionality
./scripts/test-archive.sh

# Build content-hashed assets for production (cache busting)
./scripts/build-assets.sh

# Full database reset
podman compose down -v && podman compose up

# Run all tests in container (recommended)
./scripts/test.sh

# Run Go unit tests locally
go test ./...

# Run Go tests with coverage locally
go test -cover ./...
```

## Testing

Tests run in containers to match the production environment.

### Running Tests

```bash
# Run all tests (Go + JS) in container
./scripts/test.sh

# Run with coverage report
./scripts/test.sh --coverage
```

### Go Tests
Unit tests are in `*_test.go` files alongside the source code:
- `internal/config/config_test.go` - Configuration loading tests
- `internal/models/card_test.go` - Grid position and bingo detection tests
- `internal/middleware/*_test.go` - CSRF, auth, security, compression, caching tests
- `internal/services/auth_test.go` - Password hashing and session token tests
- `internal/services/card_test.go` - Bingo counting and position finding tests
- `internal/handlers/health_test.go` - Health check endpoint tests
- `internal/handlers/context_test.go` - User context tests
- `internal/testutil/testutil.go` - Test helper functions

### Frontend Tests
JavaScript tests in `web/static/js/tests/runner.js`:
- Tests for utility functions (escapeHtml, truncateText, parseHash)
- Tests for bingo detection algorithm
- Tests for grid position calculations
- Tests for progress calculations

Tests use only Node.js built-in modules (no npm dependencies).

## Architecture

### Backend Structure

- `cmd/server/main.go` - Application entry point, wires up all dependencies and routes
- `internal/config/` - Environment-based configuration loading
- `internal/database/` - PostgreSQL pool (`postgres.go`), Redis client (`redis.go`), migrations (`migrate.go`)
- `internal/models/` - Data structures (User, Session, BingoCard, BingoItem, Suggestion, Friendship, Reaction)
- `internal/services/` - Business logic layer (UserService, AuthService, CardService, SuggestionService, FriendService, ReactionService)
- `internal/handlers/` - HTTP handlers that call services and return JSON
- `internal/middleware/` - Auth validation, CSRF protection, security headers, compression, caching, request logging
- `internal/logging/` - Structured JSON logging
- `scripts/` - Development/testing scripts (seed.sh, cleanup.sh, test-archive.sh) - use API, not direct DB access

### Frontend Structure

- `web/templates/index.html` - Single HTML entry point for SPA (main container has `id="main-container"`)
- `web/static/js/api.js` - API client with CSRF token handling, all methods under `API` object
- `web/static/js/app.js` - SPA router and all UI logic under global `App` object
- `web/static/css/styles.css` - Design system with CSS variables, uses OpenDyslexic font for bingo cells

**External Dependencies**: FontAwesome 6.5 loaded from cdnjs.cloudflare.com for icons (eye, eye-slash for visibility toggles). CSP allows cdnjs.cloudflare.com for script-src, style-src, and font-src.

### Frontend Patterns

**App Object**: All frontend logic lives in global `App` object. Key methods:
- `route()` - Hash-based routing, renders appropriate page
- `renderDashboard()` - Unified dashboard showing all cards (current year and archived) with sorting, selection, and bulk actions
- `renderFinalizedCard()` / `renderCardEditor()` - Card views (handles both authenticated and anonymous modes)
- `showItemDetailModal()` - Modal for viewing/completing items
- `renderFriends()` / `renderFriendCard()` - Friends list and viewing friend cards (with year selector for multiple cards)
- `renderArchiveCard()` - View for individual archived cards with stats
- `renderProfile()` - Account settings page (email verification status, privacy settings, change password)
- `openModal()` / `closeModal()` - Generic modal system
- `fillEmptySpaces()` - Auto-fill empty card slots with random suggestions

**API Object**: Wraps fetch calls with CSRF handling. Namespaced: `API.auth.*`, `API.cards.*`, `API.suggestions.*`, `API.friends.*`, `API.reactions.*`

**Adding New Features**: Add API methods to `api.js`, UI methods to `App` object in `app.js`, styles to `styles.css`

### Key Patterns

**Middleware Chain**: Requests flow through `requestLogger → securityHeaders → compress → cacheControl → csrfMiddleware → authMiddleware → handler`

**Rate Limiting**: Not implemented at the application level. Rate limiting should be handled by upstream infrastructure (load balancer, API gateway, CDN) in production environments.

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

### Database Schema

Core tables: `users`, `bingo_cards`, `bingo_items`, `friendships`, `reactions`, `suggestions`, `sessions`

Email verification tables: `email_verification_tokens`, `magic_link_tokens`, `password_reset_tokens`

**Users table key columns:**
- `username` - Unique (case-insensitive) user display name
- `searchable` - Boolean, opt-in flag for appearing in friend search (default: false)

Migrations in `migrations/` directory using numeric prefix ordering.

## API Routes

Auth: `POST /api/auth/{register,login,logout}`, `GET /api/auth/me`, `POST /api/auth/password`, `PUT /api/auth/searchable`
Email Auth: `POST /api/auth/{verify-email,resend-verification,magic-link,forgot-password,reset-password}`, `GET /api/auth/magic-link/verify`

Cards: `POST /api/cards`, `GET /api/cards`, `GET /api/cards/archive`, `GET /api/cards/export`, `GET /api/cards/{id}`, `GET /api/cards/{id}/stats`, `POST /api/cards/{id}/{items,shuffle,finalize}`, `PUT /api/cards/{id}/visibility`, `PUT /api/cards/visibility/bulk`, `PUT /api/cards/archive/bulk`, `DELETE /api/cards/bulk`

Items: `PUT/DELETE /api/cards/{id}/items/{pos}`, `PUT /api/cards/{id}/items/{pos}/{complete,uncomplete,notes}`

Suggestions: `GET /api/suggestions`, `GET /api/suggestions/categories`

Friends: `GET /api/friends`, `GET /api/friends/search`, `POST /api/friends/request`, `PUT /api/friends/{id}/{accept,reject}`, `DELETE /api/friends/{id}`, `GET /api/friends/{id}/card`, `GET /api/friends/{id}/cards`

Reactions: `POST/DELETE /api/items/{id}/react`, `GET /api/items/{id}/reactions`, `GET /api/reactions/emojis`

## Versioning

The application version is displayed in the footer and must be updated with each release. The version number is located in `web/templates/index.html` in the footer element.

**Version format**: Follow [Semantic Versioning](https://semver.org/) (MAJOR.MINOR.PATCH)
- MAJOR: Breaking changes or significant new features
- MINOR: New features, backwards compatible
- PATCH: Bug fixes, backwards compatible

**When to update**: Increment the version when committing and pushing changes:
- Bug fixes → increment PATCH (e.g., 0.1.0 → 0.1.1)
- New features → increment MINOR, reset PATCH (e.g., 0.1.1 → 0.2.0)
- Breaking changes → increment MAJOR, reset MINOR and PATCH (e.g., 0.2.0 → 1.0.0)

## Environment Variables

Server: `SERVER_HOST`, `SERVER_PORT`, `SERVER_SECURE`
Database: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`
Redis: `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB`
Email: `EMAIL_PROVIDER`, `RESEND_API_KEY`, `EMAIL_FROM_ADDRESS`, `APP_BASE_URL`

## Email Service

**Provider**: [Resend](https://resend.com) - Domain verified for yearofbingo.com
**Local Development**: Use Mailpit (SMTP capture) or console logging
**See**: `plans/auth.md` for full email authentication implementation plan

## Implementation Status

Phases 1-10 complete, ongoing enhancements:
- Phase 1: Foundation (Go project, PostgreSQL, Redis, migrations, Podman)
- Phase 2: Authentication (register, login, sessions, CSRF)
- Phase 2.5: Email Authentication (email verification, magic link login, password reset, profile page)
- Phase 3: Bingo Card API (create, items, shuffle, finalize, suggestions)
- Phase 4: Frontend Card Creation UI (SPA, grid, drag-drop, animations)
- Phase 4.5: New User Experience (anonymous card creation, Fill Empty button)
- Phase 5: Card Interaction (mark complete, notes, bingo detection)
- Phase 6: Social Features (friends, shared card view, reactions)
- Phase 7: Archive & History (past years cards, completion statistics, bingo counts)
- Phase 8: Polish & Production Readiness (security headers, compression, caching, logging, error pages, accessibility)
- Phase 9: CI/CD (GitHub Actions, container builds, security scanning)
- Phase 10: Card Visibility (per-card privacy controls, bulk visibility management)

See `plans/bingo.md` for the full implementation plan and `plans/auth.md` for email authentication details.

## CI/CD (Phase 9)

GitHub Actions workflow in `.github/workflows/ci.yaml`:

**Pipeline stages:**
1. **Lint** - golangci-lint with config in `.golangci.yaml`
2. **Test (Go)** - `go test -v -race -coverprofile=coverage.out ./...`
3. **Test (JS)** - `node web/static/js/tests/runner.js`
4. **Build** - Compile binary, upload as artifact
5. **Build Image** - Parallel builds on native runners (amd64 + arm64)
6. **Scan & Push** - Trivy security scan, then push multi-arch manifest

**Container registry:** [quay.io/yearofbingo/yearofbingo](https://quay.io/repository/yearofbingo/yearofbingo)
- Multi-arch: `linux/amd64` and `linux/arm64`
- `:latest` - Latest main branch build
- `:<sha>` - Specific commit builds

**Running production image locally:**
```bash
# Run pre-built image from quay.io (auto-selects correct arch)
podman compose -f compose.prod.yaml up
```

**Local CI/dev commands:**
```bash
# Run linting
golangci-lint run

# Run all tests in container
./scripts/test.sh

# Build container locally
podman build -f Containerfile -t yearofbingo .

# Run local dev build
podman compose up
```

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
