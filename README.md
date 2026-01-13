# Year of Bingo

[![CI](https://github.com/HammerMeetNail/yearofbingo/actions/workflows/ci.yaml/badge.svg)](https://github.com/HammerMeetNail/yearofbingo/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/HammerMeetNail/yearofbingo/graph/badge.svg)](https://codecov.io/gh/HammerMeetNail/yearofbingo)
[![Go Version](https://img.shields.io/github/go-mod/go-version/HammerMeetNail/yearofbingo)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**[yearofbingo.com](https://yearofbingo.com)**

A web application for creating and tracking annual Bingo cards. Create a Bingo card (2x2, 3x3, 4x4, or 5x5), fill it with personal goals, then mark items complete throughout the year as you achieve them.

## Features

- Create and customize annual bingo cards (2x2–5x5, optional FREE space)
- Edit via drag/drop (desktop) and touch-friendly interactions (mobile)
- Track progress with optional notes and bingo notifications
- Suggestions + AI-assisted goal generation (email-verification gated)
- Social features: friends, reactions, share links, and privacy controls
- Archive + export (stats + CSV export)
- Email auth flows (verification, magic links, reset password)

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
git clone https://github.com/HammerMeetNail/yearofbingo.git
cd yearofbingo

# Start the application
make local
```

The application will be available at http://localhost:8080

### Development

A `Makefile` provides convenient commands for common tasks:

```bash
# Full local rebuild: stop, build assets, build container, start
make local

# View container logs
make logs

# Stop containers
make down

# Run linting (requires golangci-lint)
make lint

# Run all tests in container (wraps ./scripts/test.sh)
make test

# Check code coverage
make coverage

# Clean up everything including volumes (full reset, destructive)
make clean
```

### Testing

Tests run in containers to match the production environment:

```bash
# Run all tests (Go + JavaScript)
make test

# Run Go-only / JS-only
make test-backend
make test-frontend
```

Or run locally:

```bash
# Go tests
go test ./...

# JavaScript tests (requires Node.js, no npm dependencies)
node web/static/js/tests/runner.js
```

#### E2E (Playwright)

Playwright runs in a container (no npm install on the host) and uses Mailpit (SMTP) for email flows.

```bash
# Full E2E run (destructive: resets volumes, reseeds data)
make e2e

# Headed mode / debug helpers
make e2e-headed
make e2e-debug
```

Artifacts:
- HTML report: `playwright-report/`
- Raw results: `test-results/`

Notes:
- E2E runs with `AI_STUB=1` by default so AI wizard tests are deterministic (no network/API keys).
- Specs live in `tests/e2e/*.spec.js` (with shared helpers in `tests/e2e/helpers.js`).
- For a current “coverage map” of workflows, see `plans/playwright.md`.

## Project Structure

```
yearofbingo/
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
├── Containerfile.test   # Test container build instructions
└── AGENTS.md            # AI assistant guidance
```

## Documentation

- API docs: `/api/docs` (Swagger UI) and `web/static/openapi.yaml`
- Architectural notes and “how we work”: `agent_docs/`
- Feature specs and implementation notes: `plans/`

## Configuration

Configuration is via environment variables (see `.env.example`). For local development, Compose files (`compose.yaml`, `compose.prod.yaml`) provide sensible defaults.

Set `DEBUG=true` to enable debug-level logs. In `APP_ENV=development`, this may log Gemini prompt/response text during AI requests; don’t enable in production.

## Scripts

Helper scripts live in `scripts/` (generally API-driven; many require `curl` + `jq`). Common entrypoints:

- `./scripts/test.sh` (used by `make test`; supports flags like `--coverage`, `--go`, `--js`)
- `./scripts/seed.sh` (seed local dev data)
- `./scripts/build-assets.sh` (build content-hashed frontend assets)

## CI/CD

CI runs via GitHub Actions (`.github/workflows/ci.yaml`) and exercises lint, unit/integration tests, and container builds. Release notes/process live in `agent_docs/ops.md`.

## Security & Performance

- **Security Headers**: CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy
- **HTTPS**: HSTS enabled when `SERVER_SECURE=true`
- **Compression**: Gzip for text responses
- **Caching**: Content-hashed assets cached immutably (1 year); API responses not cached
- **Logging**: Structured JSON request logs with timing and status

## Accessibility

- Skip links for keyboard navigation
- ARIA labels on interactive elements
- Focus visible styles for keyboard users
- Reduced motion support (`prefers-reduced-motion`)
- OpenDyslexic font for improved readability

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
