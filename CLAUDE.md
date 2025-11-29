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

**Drag and Drop**: Unfinalized cards support drag and drop to rearrange items:
- **Desktop**: Native HTML5 drag and drop with `draggable="true"` attribute
- **Mobile**: Touch events with 300ms long-press to initiate drag, visual clone follows finger
- **API**: `POST /api/cards/{id}/swap` swaps two positions atomically (handles both filled and empty cells)
- **CSS**: `.bingo-cell[draggable="true"]` has `user-select: none` and `touch-action: none` to prevent text selection
- **Event delegation**: Listeners attached to grid element, not individual cells - no re-attachment needed after DOM updates

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

Items: `PUT/DELETE /api/cards/{id}/items/{pos}`, `POST /api/cards/{id}/swap`, `PUT /api/cards/{id}/items/{pos}/{complete,uncomplete,notes}`

Suggestions: `GET /api/suggestions`, `GET /api/suggestions/categories`

Friends: `GET /api/friends`, `GET /api/friends/search`, `POST /api/friends/request`, `PUT /api/friends/{id}/{accept,reject}`, `DELETE /api/friends/{id}`, `GET /api/friends/{id}/card`, `GET /api/friends/{id}/cards`

Reactions: `POST/DELETE /api/items/{id}/react`, `GET /api/items/{id}/reactions`, `GET /api/reactions/emojis`

Support: `POST /api/support`

## Versioning & Releases

The application version is displayed in the footer and must be updated with each release. The version number is located in `web/templates/index.html` in the footer element.

**Version format**: Follow [Semantic Versioning](https://semver.org/) (MAJOR.MINOR.PATCH)
- MAJOR: Breaking changes or significant new features
- MINOR: New features, backwards compatible
- PATCH: Bug fixes, backwards compatible

**Version increment rules**:
- Bug fixes → increment PATCH (e.g., 1.0.4 → 1.0.5)
- New features → increment MINOR, reset PATCH (e.g., 1.0.5 → 1.1.0)
- Breaking changes → increment MAJOR, reset MINOR and PATCH (e.g., 1.1.0 → 2.0.0)

### Creating a Release

When ready to release, follow these steps:

1. **Update the version** in `web/templates/index.html` footer
2. **Commit the change**:
   ```bash
   git add web/templates/index.html
   git commit -m "Release v1.0.5"
   ```
3. **Create and push a version tag**:
   ```bash
   git tag v1.0.5
   git push && git push --tags
   ```

This triggers the CI pipeline which will:
- Build and test the code
- Build multi-arch container images
- Tag the container with the version (e.g., `quay.io/yearofbingo/yearofbingo:1.0.5`)
- Create a GitHub Release with auto-generated changelog
- Deploy to production

**Container tags created**:
- `:1.0.5` - Exact version (immutable)
- `:1.0` - Major.minor (points to latest patch)
- `:latest` - Latest release
- `:<commit-sha>` - Specific commit

**Production deploys** use the image digest (not tag) for reproducibility. The compose.yaml on the server includes the tag as a comment above the image line for easy reference.

**GitHub Releases**: Each version tag creates a GitHub Release at https://github.com/yearofbingo/yearofbingo/releases with auto-generated release notes based on commits since the last tag

## Environment Variables

Server: `SERVER_HOST`, `SERVER_PORT`, `SERVER_SECURE`
Database: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`
Redis: `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB`
Email: `EMAIL_PROVIDER`, `RESEND_API_KEY`, `EMAIL_FROM_ADDRESS`, `APP_BASE_URL`
Backup: `BACKUP_ENCRYPTION_KEY`, `R2_BUCKET` (default: yearofbingo-backups)

## Database Backups

PostgreSQL backups are stored in Cloudflare R2 (S3-compatible, 10GB free tier). Redis is not backed up as it's only used for session caching with PostgreSQL fallback.

**Retention:** R2 lifecycle policy protects backups from deletion for 30 days, then auto-deletes at 31 days. This prevents attackers from deleting backups if the server is compromised.

**Backup scripts:**
- `./scripts/backup.sh` - Create encrypted backup and upload to R2
- `./scripts/restore.sh` - Download and restore from R2 backup
- `./scripts/verify-backup.sh` - Verify backup can be restored (runs daily)
- `./scripts/test-backup.sh` - Interactive backup test with detailed output

**Automation:** Systemd timers run daily (see `cloud-init.yaml`):
- 3:00 AM - Backup (`yearofbingo-backup.timer`)
- 4:00 AM - Verification (`yearofbingo-verify-backup.timer`)

**Verification:** If daily verification fails, an error file `BACKUP_VERIFICATION_FAILED_*.txt` is written to the R2 bucket with details. Check the bucket periodically or set up Cloudflare notifications.

Check status:
```bash
systemctl status yearofbingo-backup.timer
journalctl -u yearofbingo-backup.service
```

**Manual backup:**
```bash
./scripts/backup.sh
```

**Restore from backup:**
```bash
./scripts/restore.sh --list     # List available backups
./scripts/restore.sh --latest   # Restore most recent
./scripts/restore.sh <filename> # Restore specific backup
```

**Test backup integrity:**
```bash
./scripts/test-backup.sh --latest
```

**Security:** Backups are encrypted with GPG (AES-256) before upload. The `BACKUP_ENCRYPTION_KEY` must be stored securely and separately from backups.

**Disaster Recovery:**
1. Provision new server with `cloud-init.yaml`
2. Run CI deploy (configures secrets and rclone)
3. Start timers: `sudo systemctl start yearofbingo-backup.timer yearofbingo-verify-backup.timer`
4. Restore: `./scripts/restore.sh --latest`

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
- Phase 11: Legal Pages (Terms of Service, Privacy Policy, Security page in footer)
- Phase 12: About Page (origin story, open source info, GitHub link)
- Phase 13: FAQ Page (frequently asked questions in navbar, Friends page search improvements)

See `plans/bingo.md` for the full implementation plan and `plans/auth.md` for email authentication details.

## Help & Legal Pages

Navbar contains FAQ link (`#faq`), footer contains About and legal pages (`#about`, `#terms`, `#privacy`, `#security`):
- **FAQ** (navbar) - Frequently asked questions about using the site, finding friends, managing cards
- **About** - Origin story, open source info, GitHub repository link
- **Terms of Service** - Account usage, acceptable use, content ownership, disclaimers
- **Privacy Policy** - Data collection, cookies (strictly necessary only), GDPR rights, international transfers
- **Security** - Infrastructure security, application security measures, responsible disclosure

**Cookie Policy**: Only strictly necessary cookies are used (session authentication, CSRF protection, Cloudflare security). No tracking or advertising cookies. Cloudflare Web Analytics is cookie-free. No cookie consent banner required under GDPR.

## CI/CD (Phase 9)

GitHub Actions workflow in `.github/workflows/ci.yaml`:

**Pipeline triggers:**
- Push to `main` branch - builds, tests, deploys to production
- Version tags (`v*`) - builds, tests, creates GitHub Release, deploys to production
- Pull requests - runs tests only (no deploy)

**Pipeline stages:**
1. **Secret Scan** - Gitleaks scans for accidentally committed secrets (runs on ALL changes)
2. **Lint** - golangci-lint with config in `.golangci.yaml`
3. **Test (Go)** - `go test -v -race -coverprofile=coverage.out ./...`
4. **Test (JS)** - `node web/static/js/tests/runner.js`
5. **Build** - Compile binary, upload as artifact
6. **Build Image** - Parallel builds on native runners (amd64 + arm64)
7. **Scan & Push** - Trivy security scan, Cosign signing, push multi-arch manifest
8. **Release** - Creates GitHub Release (only for version tags)
9. **Deploy** - Deploys to production server

**Path filtering:** Lint, test, build, and deploy jobs only run when code changes. Documentation-only changes (README, markdown files) skip these jobs but still run secret scanning.

**Container registry:** [quay.io/yearofbingo/yearofbingo](https://quay.io/repository/yearofbingo/yearofbingo)
- Multi-arch: `linux/amd64` and `linux/arm64`
- `:latest` - Latest main branch build
- `:<sha>` - Specific commit builds
- `:<version>` - Release versions (e.g., `:1.0.5`, `:1.0`)

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

**Secret scanning (pre-commit hook):**
```bash
# Install pre-commit (once)
brew install pre-commit

# Install hooks (run after cloning)
pre-commit install

# Test hook manually
pre-commit run --all-files
```

Gitleaks runs automatically on every commit to prevent accidentally committing secrets. Configuration in `.pre-commit-config.yaml`.

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
