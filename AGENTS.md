# Year of Bingo - Developer Agent Context

## Project Overview
Year of Bingo is a Go/Vanilla JS web app for creating annual Bingo cards.
Cards support predefined grid sizes (2x2, 3x3, 4x4, 5x5) with an optional FREE space.
**Domain**: yearofbingo.com

## Tech Stack
- **Backend**: Go 1.24+ (std lib only), pgx/v5, go-redis/v9.
- **Frontend**: Vanilla JS SPA (Hash routing), CSS variables.
- **Infra**: Podman, PostgreSQL, Redis.

## Core Workflow
- **Start Dev**: `make local` (or `podman compose up`)
- **Test**: `./scripts/test.sh` (Runs Go + JS tests in container)
- **Lint**: `make lint`

## Documentation Map & Constraints
You are **REQUIRED** to read the specific documentation below if your task involves these keywords:

- **Release, Tag, Push, Versioning, CI/CD** -> `read_file agent_docs/ops.md`
  - **CRITICAL**: Do not tag/push without following the "Creating a Release" steps in this file (e.g. updating `index.html` footer).
- **Database, Migrations, Schema, Redis** -> `read_file agent_docs/database.md`
  - **CRITICAL**: Review schema before writing queries.
- **Architecture, Patterns, Styles** -> `read_file agent_docs/architecture.md`
  - **CRITICAL**: Follow the `App` object pattern for JS and Service layer pattern for Go.
- **New Endpoints, Auth, Tokens** -> `read_file agent_docs/api.md`
  - **CRITICAL**: New endpoints must be registered in `main.go` and documented in `openapi.yaml`.
- **Cards, Grid Size, FREE Space, Header, Clone** -> `read_file plans/flexible_cards.md`
  - **CRITICAL**: Card config is draft-only; once finalized, the card is immutable. FREE defaults on and must not move during shuffle.
- **AI, Gemini, LLM, Wizard, Rate Limit** -> `read_file plans/ai_goals.md`
  - **CRITICAL**: AI API work must also update `web/static/openapi.yaml`, keep the endpoint session-only (no API tokens), and preserve the verified-email gating (5 free generations per unverified user tracked via `users.ai_free_generations_used`).

## Version Info
- App release version is shown in the footer (`web/templates/index.html`).
- API spec version lives in `web/static/openapi.yaml` (keep it in sync with releases).
- **Testing Strategies** -> `read_file agent_docs/testing.md`
  - **CRITICAL**: Use `./scripts/test.sh` for accurate results (runs in container).
- **Roadmap, Status, New Features** -> `read_file agent_docs/roadmap.md`
  - **CRITICAL**: Check `plans/` directory for detailed specs before starting major features.

## Universal Conventions
- **Style**: Mimic existing Go (idiomatic, no frameworks) and JS (vanilla, ES6 modules) code.
- **Safety**: Never commit secrets. Explain destructive shell commands before running.
