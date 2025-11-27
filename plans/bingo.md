# NYE Bingo - Implementation Plan

A web application for creating and tracking New Year's Resolution Bingo cards.

## Overview

Users create a 5x5 Bingo card on New Year's Eve with 24 personal goals (center square is free). Throughout the year, they mark items complete and track progress. Cards can be shared with friends who can view and react to completions.

---

## Architecture

### Tech Stack
- **Backend**: Go (net/http, no framework)
- **Frontend**: Vanilla JavaScript, HTML5, CSS3
- **Database**: PostgreSQL
- **Cache**: Redis (session storage, rate limiting)
- **Containerization**: Podman (local dev), container images on quay.io
- **CI/CD**: GitHub Actions
- **Deployment**: GitOps (declarative, container-based)

### Infrastructure Cost Estimate
- VPS (2GB RAM): ~$6-12/mo (DigitalOcean, Hetzner, or Fly.io)
- Managed PostgreSQL: Free tier or ~$5/mo
- Redis: In-memory on same VPS or free tier (Upstash)
- **Total**: ~$6-15/mo for modest traffic

---

## Phase 1: Foundation (Core Infrastructure)

### 1.1 Project Structure
```
nye_bingo/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   ├── database/
│   ├── handlers/
│   ├── middleware/
│   ├── models/
│   ├── services/
│   └── templates/
├── web/
│   ├── static/
│   │   ├── css/
│   │   ├── js/
│   │   └── images/
│   └── templates/
├── migrations/
├── scripts/
├── plans/
├── Containerfile
├── compose.yaml
├── go.mod
└── README.md
```

### 1.2 Database Schema (PostgreSQL)

```sql
-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Bingo Cards
CREATE TABLE bingo_cards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    year INTEGER NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, year)
);

-- Bingo Items (24 per card + 1 free space)
CREATE TABLE bingo_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id UUID REFERENCES bingo_cards(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0 AND position <= 24),
    content TEXT NOT NULL,
    is_completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMPTZ,
    notes TEXT,
    proof_url TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(card_id, position)
);

-- Friend Relationships
CREATE TABLE friendships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    friend_id UUID REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'pending', -- pending, accepted, rejected
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, friend_id)
);

-- Reactions to completions
CREATE TABLE reactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID REFERENCES bingo_items(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    emoji VARCHAR(10) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(item_id, user_id)
);

-- Curated Suggestions
CREATE TABLE suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true
);

-- Sessions (Redis-backed, but fallback table)
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 1.3 Development Environment

- `compose.yaml` with PostgreSQL, Redis, and app service
- Hot reload for Go using `air` or `watchexec`
- Environment configuration via `.env` files

### 1.4 Deliverables
- [x] Go project initialization with modules
- [x] Database connection pool setup
- [x] Redis client setup
- [x] Database migrations system (golang-migrate)
- [x] Initial schema migrations
- [x] Podman compose configuration
- [x] Basic health check endpoint
- [x] Configuration management (env vars)

---

## Phase 2: Authentication & User Management

### 2.1 Security Requirements
- Password hashing with bcrypt (cost factor 12+)
- Secure session tokens (32 bytes, crypto/rand)
- CSRF protection for all forms
- Rate limiting on auth endpoints (Redis-backed)
- Secure cookie settings (HttpOnly, Secure, SameSite=Strict)
- Input validation and sanitization

### 2.2 Auth Endpoints
```
POST /api/auth/register     - Create account
POST /api/auth/login        - Login, create session
POST /api/auth/logout       - Destroy session
GET  /api/auth/me           - Get current user
POST /api/auth/password     - Change password
```

### 2.3 Session Management
- Sessions stored in Redis with 30-day expiry
- Session token in HttpOnly cookie
- Sliding expiration on activity

### 2.4 Deliverables
- [x] User registration with email validation
- [x] Password hashing and verification
- [x] Session creation and management
- [x] Login/logout functionality
- [x] Auth middleware for protected routes
- [x] CSRF token generation and validation
- [x] Rate limiting middleware
- [ ] Password reset flow (email-based)

---

## Phase 3: Bingo Card Creation

### 3.1 Card Creation Flow
1. User clicks "Create Card for [Year]"
2. Card grid displayed (5x5, center marked FREE)
3. Input field + suggestion panel shown
4. User types or clicks suggestions to add items
5. Each item animates onto a random empty square
6. After 24 items, shuffle option appears
7. User can drag-and-drop or click "Shuffle" until satisfied
8. "Finalize Card" locks the layout

### 3.2 Suggestion System
Pre-curated categories:
- Health & Fitness (exercise, diet, sleep)
- Career & Learning (skills, certifications, reading)
- Relationships (family, friends, community)
- Hobbies & Creativity (art, music, crafts)
- Finance (savings, investments, debt)
- Travel & Adventure (trips, experiences)
- Personal Growth (habits, mindfulness)
- Home & Organization (declutter, projects)

### 3.3 API Endpoints
```
POST   /api/cards                - Create new card for year
GET    /api/cards                - List user's cards
GET    /api/cards/:id            - Get card details
POST   /api/cards/:id/items      - Add item to card
PUT    /api/cards/:id/items/:pos - Update item position/content
DELETE /api/cards/:id/items/:pos - Remove item
POST   /api/cards/:id/shuffle    - Randomize item positions
POST   /api/cards/:id/finalize   - Lock card layout
GET    /api/suggestions          - Get curated suggestions
```

### 3.4 Deliverables
- [x] Card creation API
- [x] Item management (add, edit, remove, reposition)
- [x] Random position assignment algorithm
- [x] Shuffle functionality
- [x] Card finalization (lock editing)
- [x] Suggestions API with categories
- [x] Seed data for suggestions

---

## Phase 4: Frontend - Card Creation UI

### 4.1 Design Principles
- New Year's Eve theme (dark background, gold/silver accents, confetti)
- Mobile-first responsive design
- Satisfying micro-interactions
- Accessible (WCAG 2.1 AA)

### 4.2 Pages
- Landing page (marketing, login/register)
- Dashboard (current card, past cards)
- Card creation wizard
- Card view (interactive)
- Friends list
- Shared card view

### 4.3 Card Creation UI
- 5x5 CSS Grid layout
- Empty squares have subtle pulse/glow
- Items animate in with confetti burst
- Drag-and-drop using native HTML5 API
- Touch support for mobile
- Shuffle button with satisfying animation

### 4.4 Deliverables
- [x] CSS design system (variables, typography, colors)
- [x] Responsive 5x5 bingo grid component
- [x] Item input with autocomplete suggestions
- [x] Suggestion panel with categories
- [x] Add-item animation (item flies to square)
- [x] Drag-and-drop reordering
- [x] Shuffle animation
- [x] Mobile touch interactions
- [x] Card creation wizard flow

---

## Phase 5: Card Interaction & Tracking

### 5.1 Marking Items Complete
- Click/tap square to mark complete
- Satisfying "stamp" or "dauber" animation
- Optional modal for notes/proof
- Completion timestamp recorded
- Bingo detection (5 in a row)

### 5.2 Completion Modal
- Appears after marking complete
- Optional text note field
- Optional image upload (stored in object storage or base64)
- "Skip" and "Save" buttons
- Can be accessed later to add notes

### 5.3 API Endpoints
```
PUT  /api/cards/:id/items/:pos/complete   - Mark complete
PUT  /api/cards/:id/items/:pos/uncomplete - Unmark
PUT  /api/cards/:id/items/:pos/notes      - Update notes/proof
```

### 5.4 Deliverables
- [x] Click-to-complete interaction
- [x] Stamp/dauber visual effect
- [x] Completion modal with notes
- [ ] Image upload for proof (optional)
- [x] Bingo detection algorithm
- [x] Bingo celebration animation
- [x] Progress indicator (X/24 complete)

---

## Phase 6: Social Features

### 6.1 Friend System
- Search users by email or display name
- Send friend request
- Accept/reject requests
- Remove friends
- View friends' active cards

### 6.2 Sharing
- Friends can view your card (read-only)
- See completion progress
- React to completed items with emojis

### 6.3 Reactions
- Predefined emoji set (celebrate, clap, fire, heart, star)
- One reaction per user per item
- Can change reaction
- Shows reaction count on items

### 6.4 API Endpoints
```
GET    /api/friends              - List friends
POST   /api/friends/request      - Send request
PUT    /api/friends/:id/accept   - Accept request
DELETE /api/friends/:id          - Remove/reject
GET    /api/friends/:id/card     - View friend's card
POST   /api/items/:id/react      - Add/change reaction
DELETE /api/items/:id/react      - Remove reaction
```

### 6.5 Deliverables
- [ ] Friend search and discovery
- [ ] Friend request flow
- [ ] Friends list UI
- [ ] Shared card view (read-only)
- [ ] Reaction picker component
- [ ] Reaction display on items
- [ ] Notification for reactions (optional)

---

## Phase 7: Archive & History

### 7.1 Card Archive
- View past years' cards
- Cards become read-only after year ends
- Statistics: completion rate, bingos achieved
- Year-over-year comparison (future enhancement)

### 7.2 API Endpoints
```
GET /api/cards/archive          - List past cards
GET /api/cards/:id/stats        - Card statistics
```

### 7.3 Deliverables
- [ ] Archive page listing past cards
- [ ] Read-only historical card view
- [ ] Completion statistics
- [ ] Visual distinction for archived cards

---

## Phase 8: Polish & Production Readiness

### 8.1 Performance
- Asset minification (CSS, JS)
- Image optimization
- Gzip compression
- Cache headers for static assets
- Database query optimization
- Connection pooling tuning

### 8.2 Security Hardening
- Security headers (CSP, HSTS, X-Frame-Options)
- SQL injection prevention (parameterized queries)
- XSS prevention (proper escaping)
- Rate limiting on all endpoints
- Input validation everywhere
- Dependency vulnerability scanning

### 8.3 Monitoring & Logging
- Structured logging (JSON)
- Request logging middleware
- Error tracking
- Health check endpoints
- Basic metrics (request count, latency)

### 8.4 Deliverables
- [ ] Asset build pipeline (esbuild or similar)
- [ ] Security headers middleware
- [ ] Structured logging
- [ ] Error pages (404, 500)
- [ ] Loading states and error handling in UI
- [ ] Accessibility audit and fixes
- [ ] Cross-browser testing (Firefox, Chrome, Safari)
- [ ] Mobile testing (iOS Safari, Android Chrome)

---

## Phase 9: Containerization & CI/CD

### 9.1 Container Setup
```dockerfile
# Multi-stage build
FROM golang:1.22-alpine AS builder
# ... build steps

FROM alpine:3.19
# ... minimal runtime
```

### 9.2 Local Development
- `compose.yaml` for full stack
- Volume mounts for live reload
- Consistent environment parity

### 9.3 CI Pipeline (GitHub Actions)
```yaml
- Lint (golangci-lint, eslint)
- Test (go test, integration tests)
- Build container image
- Push to quay.io
- Security scan (trivy)
```

### 9.4 GitOps Deployment
- Kubernetes manifests or Docker Compose for deployment
- ArgoCD or Flux for GitOps (or simple pull-based deployment)
- Environment-specific configs via Kustomize or envsubst

### 9.5 Deliverables
- [ ] Production Containerfile (multi-stage)
- [ ] compose.yaml for local development
- [ ] GitHub Actions workflow for CI
- [ ] Container image push to quay.io
- [ ] Deployment manifests
- [ ] Environment configuration management
- [ ] Database migration in deployment pipeline
- [ ] Rollback strategy

---

## Phase 10: Launch Preparation

### 10.1 Pre-Launch Checklist
- [ ] End-to-end testing
- [ ] Load testing (basic)
- [ ] Backup strategy for database
- [ ] SSL/TLS certificate (Let's Encrypt)
- [ ] Domain configuration
- [ ] Privacy policy and terms of service
- [ ] Cookie consent (if required)
- [ ] Email delivery setup (password reset)

### 10.2 Launch
- [ ] Deploy to production
- [ ] Monitor for issues
- [ ] Gather user feedback

---

## API Summary

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/auth/register | Create account |
| POST | /api/auth/login | Login |
| POST | /api/auth/logout | Logout |
| GET | /api/auth/me | Current user |
| POST | /api/cards | Create card |
| GET | /api/cards | List user's cards |
| GET | /api/cards/:id | Get card |
| POST | /api/cards/:id/items | Add item |
| PUT | /api/cards/:id/items/:pos | Update item |
| DELETE | /api/cards/:id/items/:pos | Remove item |
| POST | /api/cards/:id/shuffle | Shuffle items |
| POST | /api/cards/:id/finalize | Lock card |
| PUT | /api/cards/:id/items/:pos/complete | Mark complete |
| PUT | /api/cards/:id/items/:pos/notes | Add notes |
| GET | /api/suggestions | Get suggestions |
| GET | /api/friends | List friends |
| POST | /api/friends/request | Send request |
| PUT | /api/friends/:id/accept | Accept request |
| DELETE | /api/friends/:id | Remove friend |
| GET | /api/friends/:id/card | View friend's card |
| POST | /api/items/:id/react | React to item |

---

## Non-Functional Requirements

### Performance Targets
- Page load: < 2s on 3G
- API response: < 200ms p95
- Lighthouse score: > 90

### Security
- OWASP Top 10 compliance
- Regular dependency updates
- Secure by default configuration

### Accessibility
- WCAG 2.1 AA compliance
- Keyboard navigation
- Screen reader support

---

## Future Enhancements (Post-Launch)
- Email reminders for incomplete items
- Achievement badges
- Public card sharing (with unique URL)
- Import/export cards
- Dark/light theme toggle
- Push notifications for reactions
- Year-end summary/wrap-up
- Social sharing images (OG tags)

---

## Development Order Summary

1. **Foundation** - Project setup, database, containers
2. **Auth** - Registration, login, sessions
3. **Card Creation API** - Backend for cards and items
4. **Card Creation UI** - Frontend wizard and interactions
5. **Card Interaction** - Marking complete, notes, celebrations
6. **Social** - Friends and reactions
7. **Archive** - Historical cards and stats
8. **Polish** - Performance, security, accessibility
9. **CI/CD** - Pipeline, containers, deployment
10. **Launch** - Production deployment
