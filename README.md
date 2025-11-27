# Year of Bingo

[![CI](https://github.com/HammerMeetNail/yearofbingo/actions/workflows/ci.yaml/badge.svg)](https://github.com/HammerMeetNail/yearofbingo/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/HammerMeetNail/yearofbingo/graph/badge.svg)](https://codecov.io/gh/HammerMeetNail/yearofbingo)
[![Go Version](https://img.shields.io/github/go-mod/go-version/HammerMeetNail/yearofbingo)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**[yearofbingo.com](https://yearofbingo.com)**

A web application for creating and tracking annual Bingo cards. Create a 5x5 card with 24 personal goals, then mark items complete throughout the year as you achieve them.

## Features

- **Create Bingo Cards**: Build a personalized 5x5 bingo card with 24 goals (center is a free space)
- **Curated Suggestions**: Browse 80+ goal suggestions across 8 categories to inspire your resolutions
- **Track Progress**: Mark goals complete with optional notes about how you achieved them
- **Celebrate Wins**: Get notified when you complete a row, column, or diagonal bingo
- **Social Features**: Add friends, view their cards, and react to their achievements with emojis
- **Card Archive**: View past years' cards with completion statistics and bingo counts
- **Accessible Design**: Uses OpenDyslexic font for improved readability

## Tech Stack

- **Backend**: Go 1.24+ with net/http (no frameworks)
- **Frontend**: Vanilla JavaScript SPA with hash-based routing
- **Database**: PostgreSQL 15+
- **Cache/Sessions**: Redis 7+
- **Containerization**: Podman/Docker with Compose

## Quick Start

### Prerequisites

- [Podman](https://podman.io/) or [Docker](https://www.docker.com/)
- Podman Compose or Docker Compose

### Running Locally

```bash
# Clone the repository
git clone https://github.com/yourusername/nye_bingo.git
cd nye_bingo

# Start the application
podman compose up

# Or with Docker
docker compose up
```

The application will be available at http://localhost:8080

### Development

```bash
# Rebuild after code changes
podman compose build --no-cache && podman compose up

# Run Go build locally (requires Go 1.24+)
go build -o server ./cmd/server

# Download dependencies
go mod tidy
```

### Testing

Tests run in containers to match the production environment:

```bash
# Run all tests (Go + JavaScript)
./scripts/test.sh

# Run with coverage report
./scripts/test.sh --coverage
```

Or run locally:

```bash
# Go tests
go test ./...

# JavaScript tests (requires Node.js, no npm dependencies)
node web/static/js/tests/runner.js
```

## Project Structure

```
nye_bingo/
├── cmd/server/          # Application entry point
├── internal/
│   ├── config/          # Environment configuration
│   ├── database/        # PostgreSQL and Redis clients
│   ├── handlers/        # HTTP request handlers
│   ├── middleware/      # Auth, CSRF, security headers, compression, logging
│   ├── logging/         # Structured JSON logging
│   ├── models/          # Data structures
│   └── services/        # Business logic
├── migrations/          # Database migrations
├── web/
│   ├── static/
│   │   ├── css/         # Stylesheets
│   │   └── js/          # Frontend JavaScript
│   └── templates/       # HTML templates
├── scripts/             # Development and testing scripts
├── compose.yaml         # Container orchestration
├── Containerfile        # Container build instructions
└── CLAUDE.md            # AI assistant guidance
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_HOST` | Server bind address | `0.0.0.0` |
| `SERVER_PORT` | Server port | `8080` |
| `SERVER_SECURE` | Enable secure cookies | `false` |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL user | `bingo` |
| `DB_PASSWORD` | PostgreSQL password | `bingo` |
| `DB_NAME` | PostgreSQL database | `bingo` |
| `DB_SSLMODE` | PostgreSQL SSL mode | `disable` |
| `REDIS_HOST` | Redis host | `localhost` |
| `REDIS_PORT` | Redis port | `6379` |
| `REDIS_PASSWORD` | Redis password | (empty) |
| `REDIS_DB` | Redis database number | `0` |

## API Endpoints

### Authentication
- `POST /api/auth/register` - Create new account
- `POST /api/auth/login` - Sign in
- `POST /api/auth/logout` - Sign out
- `GET /api/auth/me` - Get current user

### Cards
- `POST /api/cards` - Create new card
- `GET /api/cards` - List user's cards
- `GET /api/cards/archive` - List archived cards from past years
- `GET /api/cards/{id}` - Get card details
- `GET /api/cards/{id}/stats` - Get card statistics (completion rate, bingos)
- `POST /api/cards/{id}/items` - Add item to card
- `POST /api/cards/{id}/shuffle` - Shuffle card items
- `POST /api/cards/{id}/finalize` - Lock card for play

### Items
- `PUT /api/cards/{id}/items/{pos}` - Update item
- `DELETE /api/cards/{id}/items/{pos}` - Remove item
- `PUT /api/cards/{id}/items/{pos}/complete` - Mark complete
- `PUT /api/cards/{id}/items/{pos}/uncomplete` - Mark incomplete

### Suggestions
- `GET /api/suggestions` - Get all suggestions
- `GET /api/suggestions/categories` - Get grouped by category

### Friends
- `GET /api/friends` - List friends and requests
- `GET /api/friends/search?q=` - Search users
- `POST /api/friends/request` - Send friend request
- `PUT /api/friends/{id}/accept` - Accept request
- `PUT /api/friends/{id}/reject` - Reject request
- `DELETE /api/friends/{id}` - Remove friend
- `GET /api/friends/{id}/card` - View friend's current card
- `GET /api/friends/{id}/cards` - View all friend's cards (with year selector)

### Reactions
- `POST /api/items/{id}/react` - React to completed item
- `DELETE /api/items/{id}/react` - Remove reaction
- `GET /api/items/{id}/reactions` - Get item reactions

## Scripts

Development and testing scripts are located in the `scripts/` directory. All scripts use the API (not direct database access) and require `curl` and `jq`.

### seed.sh

Seeds the database with test data for development and testing.

```bash
# Use default URL (http://localhost:8080)
./scripts/seed.sh

# Use custom URL
./scripts/seed.sh http://localhost:3000
```

**Creates:**
- 3 test users with password `Password1`:
  - `alice@test.com` (Alice Anderson)
  - `bob@test.com` (Bob Builder)
  - `carol@test.com` (Carol Chen)
- Alice's cards:
  - 2025: 6/24 completed (current year)
  - 2024: 18/24 completed, 2 bingos (archived)
  - 2023: 12/24 completed, 1 bingo (archived)
- Bob's cards:
  - 2025: 6/24 completed (current year)
  - 2024: 24/24 completed, 12 bingos - perfect year! (archived)
- Friendships: Alice ↔ Bob (accepted), Carol → Alice (pending)
- Emoji reactions from Bob on Alice's completed items

**Idempotent behavior:** If users/cards already exist, the script logs in and fetches existing data rather than failing.

### cleanup.sh

Removes friendships and reactions for test accounts via the API.

```bash
# Use default URL (http://localhost:8080)
./scripts/cleanup.sh

# Use custom URL
./scripts/cleanup.sh http://localhost:3000
```

**Note:** User accounts and cards remain after cleanup (no delete API). For a complete reset:

```bash
podman compose down -v && podman compose up
```

### test-archive.sh

Tests the archive and statistics endpoints.

```bash
# Use default URL (http://localhost:8080)
./scripts/test-archive.sh

# Use custom URL
./scripts/test-archive.sh http://localhost:3000
```

**Tests:**
- `GET /api/cards/archive` - Lists cards from past years
- `GET /api/cards/{id}/stats` - Card statistics including completion rate and bingo count

**Note:** Archive only shows cards from past years. Cards created by `seed.sh` (2025) won't appear in archive until 2026.

### build-assets.sh

Minifies CSS and JavaScript files for production deployment.

```bash
./scripts/build-assets.sh
```

**Output:**
- Creates minified files in `web/static/dist/`
- Reports size reduction for each file (typically 8-20% reduction)

**Note:** For production deployments, consider using proper minification tools like esbuild, terser, or csso for better compression.

### Adding New Scripts

When adding new scripts to this directory:
1. Use the API, not direct database access
2. Include a header comment explaining purpose and usage
3. Support a custom base URL as the first argument
4. Use the shared patterns: `get_csrf()`, `login_user()`, `logout_user()`
5. Log to stderr (`>&2`) so function return values aren't polluted
6. Handle "already exists" cases gracefully for idempotency

## CI/CD

The project uses GitHub Actions for continuous integration and deployment.

### Pipeline Stages

1. **Lint** - Code quality checks with golangci-lint
2. **Test (Go)** - Unit tests with race detection and coverage
3. **Test (JS)** - Frontend JavaScript tests
4. **Build** - Compile Go binary
5. **Build & Scan Image** - Build container, run Trivy security scan
6. **Push** - Push to container registry (only after scan passes)

### Container Images

Multi-architecture images (linux/amd64 and linux/arm64) are published to [quay.io/yearofbingo/yearofbingo](https://quay.io/repository/yearofbingo/yearofbingo):
- `quay.io/yearofbingo/yearofbingo:latest` - Latest main branch build
- `quay.io/yearofbingo/yearofbingo:<sha>` - Specific commit builds

### Running the Production Image

```bash
# Run the production image from quay.io
podman compose -f compose.prod.yaml up

# Or with Docker
docker compose -f compose.prod.yaml up
```

This pulls the pre-built image from quay.io and runs it with local PostgreSQL and Redis containers.

### Running CI Locally

```bash
# Run linting (requires golangci-lint)
golangci-lint run

# Run all tests
./scripts/test.sh

# Build container image locally
podman build -f Containerfile -t yearofbingo .
```

## Security & Performance

- **Security Headers**: CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy
- **HTTPS**: HSTS enabled when `SERVER_SECURE=true`
- **Compression**: Gzip for text responses
- **Caching**: Appropriate cache headers for static assets and API responses
- **Logging**: Structured JSON request logs with timing and status

## Accessibility

- Skip links for keyboard navigation
- ARIA labels on interactive elements
- Focus visible styles for keyboard users
- Reduced motion support (`prefers-reduced-motion`)
- OpenDyslexic font for improved readability

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
