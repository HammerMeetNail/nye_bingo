# NYE Bingo

A web application for creating and tracking New Year's Resolution Bingo cards. Create a 5x5 card with 24 personal goals, then mark items complete throughout the year as you achieve them.

## Features

- **Create Bingo Cards**: Build a personalized 5x5 bingo card with 24 goals (center is a free space)
- **Curated Suggestions**: Browse 80+ goal suggestions across 8 categories to inspire your resolutions
- **Track Progress**: Mark goals complete with optional notes about how you achieved them
- **Celebrate Wins**: Get notified when you complete a row, column, or diagonal bingo
- **Social Features**: Add friends, view their cards, and react to their achievements with emojis
- **Accessible Design**: Uses OpenDyslexic font for improved readability

## Tech Stack

- **Backend**: Go 1.23+ with net/http (no frameworks)
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

# Run Go build locally (requires Go 1.23+)
go build -o server ./cmd/server

# Download dependencies
go mod tidy
```

## Project Structure

```
nye_bingo/
├── cmd/server/          # Application entry point
├── internal/
│   ├── config/          # Environment configuration
│   ├── database/        # PostgreSQL and Redis clients
│   ├── handlers/        # HTTP request handlers
│   ├── middleware/      # Auth, CSRF protection
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
- `GET /api/cards/{id}` - Get card details
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
- `GET /api/friends/{id}/card` - View friend's card

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
- Finalized 2025 bingo cards for Alice and Bob with 24 unique goals each
- 6 completed items on each card with notes
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

### Adding New Scripts

When adding new scripts to this directory:
1. Use the API, not direct database access
2. Include a header comment explaining purpose and usage
3. Support a custom base URL as the first argument
4. Use the shared patterns: `get_csrf()`, `login_user()`, `logout_user()`
5. Log to stderr (`>&2`) so function return values aren't polluted
6. Handle "already exists" cases gracefully for idempotency

## License

MIT
