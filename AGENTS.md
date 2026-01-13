# Year of Bingo — Agent Guide (AGENTS.md)

This file is intentionally short: it’s an index for **progressive disclosure**. Read deeper docs only when your task requires them.

## WHY (what is this repo?)
Year of Bingo (yearofbingo.com) is a Go + vanilla JS web app for creating annual Bingo cards and tracking completion (2x2–5x5 with optional FREE space).

## WHAT (where things live)
- Start here for human-facing orientation: `README.md`
- Backend entrypoint: `cmd/server/main.go` (Go 1.24+, `net/http`, no frameworks)
- Backend code: `internal/` (handlers, services, middleware, database, models)
- DB migrations: `migrations/`
- Frontend SPA: `web/templates/` + `web/static/` (vanilla JS, hash routing)
- Docs/specs: `agent_docs/` (how we work) and `plans/` (feature specs)
- Tests: `make test` (wraps `./scripts/test.sh`), `tests/e2e/`, `web/static/js/tests/`

## HOW (common workflows)
- Dev: `make local` (or `podman compose up` / `docker compose up`)
- Tests (preferred): `make test` (container; wraps `./scripts/test.sh`)
- Targeted tests: `make test-backend`, `make test-frontend` (or run `./scripts/test.sh` with flags like `--coverage`)
- Lint: `make lint`
- E2E: `make e2e` (destructive: resets volumes, reseeds; Playwright runs in a container) plus `make e2e-headed` / `make e2e-debug`
- Debug: `DEBUG=true` (in development this may log Gemini prompt/response text; don’t enable in prod)

## Non‑negotiables (security + correctness)
- No inline `<script>` and no HTML event handlers (`onclick=`, `onsubmit=`, etc.); use existing `data-action` delegation in `web/static/js/app.js`.
- Treat all server/user data as untrusted; prefer DOM APIs (`textContent`, `createElement`) or escape with `App.escapeHtml` (avoid `innerHTML`).
- For values used in routing decisions, class names, or `data-*` attributes, prefer whitelists/mappings over “escaping”.
- Preserve strict CSP (no `unsafe-inline` / `unsafe-hashes`); if CSP changes, update/add tests accordingly.
- When rendering user-controlled content, add/extend Playwright XSS regression coverage (assert payload is rendered as text, no DOM nodes created).
- Keep tests passing and don’t reduce coverage.
- Never commit secrets; explain destructive shell commands before running them.

## Progressive Disclosure Index (open only when relevant)
- Releases, tagging, CI/CD, version bumps: `agent_docs/ops.md`
- Architecture/conventions (Go service layer, JS `App` object pattern): `agent_docs/architecture.md`
- Database/migrations/schema/Redis: `agent_docs/database.md`
- Adding/changing endpoints, auth, tokens: `agent_docs/api.md` (also update `web/static/openapi.yaml`)
- Testing strategy/containers/coverage: `agent_docs/testing.md`
- Roadmap/status/new features: `agent_docs/roadmap.md`

### Feature Specs (`plans/`)
- Cards (grid sizes, FREE, finalize/immutability, clone): `plans/flexible_cards.md`
- Sharing + OpenGraph (`/s/{token}`, `/og/...`): `plans/card_share.md`
- Export: `plans/export.md`
- Account deletion: `plans/delete_account.md`
- AI (wizard/assist, gating, rate limits): `plans/ai_goals.md` and `plans/ai_guide.md`
- Playwright “coverage map”: `plans/playwright.md`
- XSS hardening notes/checklist: `plans/update_xss.md`
