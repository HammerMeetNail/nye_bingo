# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

NYE Bingo is a web application for creating and tracking New Year's Resolution Bingo cards. Users create a 5x5 card with 24 personal goals (center is free space), then mark items complete throughout the year. Cards can be shared with friends who can react to completions.

## Tech Stack

- **Backend**: Go 1.23+ using only net/http (no frameworks)
- **Frontend**: Vanilla JavaScript SPA with hash-based routing
- **Database**: PostgreSQL with pgx/v5 driver
- **Cache/Sessions**: Redis with go-redis/v9
- **Migrations**: golang-migrate/migrate/v4
- **Containerization**: Podman with compose.yaml

## Development Commands

```bash
# Start local development environment
podman compose up

# Rebuild after Go changes
podman compose build --no-cache && podman compose up

# Run Go build locally
go build -o server ./cmd/server

# Download dependencies
go mod tidy
```

## Architecture

### Backend Structure

- `cmd/server/main.go` - Application entry point, wires up all dependencies and routes
- `internal/config/` - Environment-based configuration loading
- `internal/database/` - PostgreSQL pool (`postgres.go`), Redis client (`redis.go`), migrations (`migrate.go`)
- `internal/models/` - Data structures (User, Session, BingoCard, BingoItem, Suggestion, Friendship, Reaction)
- `internal/services/` - Business logic layer (UserService, AuthService, CardService, SuggestionService, FriendService, ReactionService)
- `internal/handlers/` - HTTP handlers that call services and return JSON
- `internal/middleware/` - Auth validation, CSRF protection, rate limiting

### Frontend Structure

- `web/templates/index.html` - Single HTML entry point for SPA (main container has `id="main-container"`)
- `web/static/js/api.js` - API client with CSRF token handling, all methods under `API` object
- `web/static/js/app.js` - SPA router and all UI logic under global `App` object
- `web/static/css/styles.css` - Design system with CSS variables, uses OpenDyslexic font for bingo cells

### Frontend Patterns

**App Object**: All frontend logic lives in global `App` object. Key methods:
- `route()` - Hash-based routing, renders appropriate page
- `renderFinalizedCard()` / `renderCardEditor()` - Card views
- `showItemDetailModal()` - Modal for viewing/completing items
- `renderFriends()` / `renderFriendCard()` - Friends list and viewing friend cards
- `openModal()` / `closeModal()` - Generic modal system

**API Object**: Wraps fetch calls with CSRF handling. Namespaced: `API.auth.*`, `API.cards.*`, `API.suggestions.*`, `API.friends.*`, `API.reactions.*`

**Adding New Features**: Add API methods to `api.js`, UI methods to `App` object in `app.js`, styles to `styles.css`

### Key Patterns

**Middleware Chain**: Requests flow through `apiRateLimiter → csrfMiddleware → authMiddleware → handler`

**Session Management**: Sessions stored in Redis first with PostgreSQL fallback. Token stored in HttpOnly cookie, hash stored in database.

**Card State Machine**: Cards start unfinalized (can add/remove/shuffle items), then finalize (locks layout, enables completion marking).

**Grid Positions**: 5x5 grid uses positions 0-24, with position 12 being the center FREE space. Items occupy 24 positions (excluding 12).

**Bingo Card Display**: Grid renders with B-I-N-G-O header row. Cell text is truncated with CSS line-clamp (4 lines desktop, 3 tablet, 2 mobile). Full text shown in modal on click. Finalized card view uses `.finalized-card-view` class with centered grid layout.

### Database Schema

Core tables: `users`, `bingo_cards`, `bingo_items`, `friendships`, `reactions`, `suggestions`, `sessions`

Migrations in `migrations/` directory using numeric prefix ordering.

## API Routes

Auth: `POST /api/auth/{register,login,logout}`, `GET /api/auth/me`

Cards: `POST /api/cards`, `GET /api/cards`, `GET /api/cards/{id}`, `POST /api/cards/{id}/{items,shuffle,finalize}`

Items: `PUT/DELETE /api/cards/{id}/items/{pos}`, `PUT /api/cards/{id}/items/{pos}/{complete,uncomplete,notes}`

Suggestions: `GET /api/suggestions`, `GET /api/suggestions/categories`

Friends: `GET /api/friends`, `GET /api/friends/search`, `POST /api/friends/request`, `PUT /api/friends/{id}/{accept,reject}`, `DELETE /api/friends/{id}`, `GET /api/friends/{id}/card`

Reactions: `POST/DELETE /api/items/{id}/react`, `GET /api/items/{id}/reactions`, `GET /api/reactions/emojis`

## Environment Variables

Server: `SERVER_HOST`, `SERVER_PORT`, `SERVER_SECURE`
Database: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`
Redis: `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB`

## Implementation Status

Phases 1-6 complete:
- Phase 1: Foundation (Go project, PostgreSQL, Redis, migrations, Podman)
- Phase 2: Authentication (register, login, sessions, CSRF, rate limiting)
- Phase 3: Bingo Card API (create, items, shuffle, finalize, suggestions)
- Phase 4: Frontend Card Creation UI (SPA, grid, drag-drop, animations)
- Phase 5: Card Interaction (mark complete, notes, bingo detection)
- Phase 6: Social Features (friends, shared card view, reactions)

**Next: Phase 7 (Archive & History)** - View past years' cards with statistics.

See `plans/bingo.md` for the full implementation plan.
