# Refactor Plan: Reduce Monolith Risk in Frontend + Server

## Context

Large files are increasing change risk despite decent test coverage:

- `web/static/js/app.js` (~10k LOC)
- `internal/handlers/card.go` (~1.5k LOC)
- `internal/services/card.go` (~2k LOC)
- `cmd/server/main.go` (~640 LOC, low package coverage concentration)

Primary risks:
- high blast radius for edits
- harder reviews and debugging
- accidental coupling between unrelated features
- regression risk when touching routing/startup wiring

## Goals

1. Split monolithic files by domain without behavior changes.
2. Make route/service wiring composable and easier to test.
3. Improve testability of orchestration code (`cmd/server`) and feature modules.
4. Keep deploy behavior and API contracts unchanged.

## Non-Goals

- No feature redesign.
- No framework migration (keep `net/http` + vanilla JS).
- No breaking changes to API routes, auth behavior, or CSP model.
- No broad style/lint churn beyond touched files.

## Constraints

- Frontend asset pipeline currently hashes individual JS files (no bundler).
- CSP disallows inline scripts/handlers.
- Existing E2E and unit tests must stay green.

---

## Workstream A: Frontend (`web/static/js/app.js`)

## Target End State

Break `App` into domain files that still compose into the same global `App` object.

Proposed structure:

- `web/static/js/app-core.js` (state, init, routing shell, shared helpers)
- `web/static/js/app-auth.js`
- `web/static/js/app-cards.js`
- `web/static/js/app-friends.js`
- `web/static/js/app-notifications.js`
- `web/static/js/app-reminders.js`
- `web/static/js/app-billing.js`
- `web/static/js/app-templates.js`
- `web/static/js/app-ai.js`
- `web/static/js/app-modals.js`
- `web/static/js/app-actions.js` (delegated action router)
- `web/static/js/app.js` (thin bootstrap/composition entrypoint)

## Phases

### A1. Create composition pattern (no behavior change)

- Add module pattern: each file exports methods by mutating `window.App` (`Object.assign(App, {...})`).
- Keep existing initialization order and `App.init()` entrypoint.
- Keep old `app.js` as source of truth initially; add composition scaffolding first.

### A2. Extract low-risk shared pieces

- Move pure helpers/constants first:
  - text/escape helpers
  - route parsing helpers
  - generic UI helper methods (`qs`, `setText`, etc.)
- Move modal primitives and action delegation maps into dedicated files.

### A3. Extract domain renderers + handlers incrementally

Recommended order (lowest coupling first):
1. support/profile tokens
2. notifications/reminders
3. friends/invites/blocks
4. billing/premium
5. templates/rollover
6. card editor/finalized card flows
7. AI flows

After each extraction:
- run JS tests
- run targeted E2E specs for moved domain

### A4. Bootstrap and asset wiring update

- Update template/page data and manifest accessors to load new JS files in deterministic order.
- Ensure hashed asset lookup supports new script names:
  - extend `internal/assets/assets.go`
  - extend page template data + `internal/handlers/pages.go`
  - include new `<script src=...>` tags in `web/templates/index.html`

### A5. Frontend regression net expansion

- Expand `web/static/js/tests/runner.js` coverage around routing/action dispatch and extracted helpers.
- Add/adjust E2E smoke by domain to verify module boundaries did not alter behavior.

## Exit Criteria (A)

- `web/static/js/app.js` reduced to bootstrap/composition only (target < 600 LOC).
- Domain modules are independently readable and scoped.
- `node web/static/js/tests/runner.js` passes.
- Existing E2E suite passes.

---

## Workstream B: Card Handler/Service split

## Target End State

Keep same package/API types but split by concern.

### Handler files (`internal/handlers/`)

- `card_create_read.go`
- `card_update.go`
- `card_finalize_completion.go`
- `card_bulk.go`
- `card_import_export.go`
- `card_conflict.go`
- `card_validation.go`
- keep `card.go` as minimal type/constructor file or remove if redundant

### Service files (`internal/services/`)

- `card_read.go`
- `card_write.go`
- `card_finalize.go`
- `card_completion.go`
- `card_bulk.go`
- `card_import_export.go`
- `card_conflict.go`
- `card_permissions.go`
- keep `card.go` for struct/type wiring only

## Phases

### B1. Mechanical split with zero logic change

- Move methods into new files in same package.
- Preserve public method signatures and constructor behavior.
- Keep helper functions private and colocated with their domain.

### B2. Isolate validation and mapping logic

- Centralize request validation helpers per domain.
- Centralize service-to-handler error mapping (avoid duplicated `if errors.Is(...)` ladders).

### B3. Improve focused tests around extracted domains

- Add/expand tests per extracted file domain:
  - import/export
  - bulk operations
  - finalize/edit flows
  - conflict handling

## Exit Criteria (B)

- No single card handler/service file > 700 LOC.
- All card-related handler/service tests pass.
- No API behavior changes in existing E2E card scenarios.

---

## Workstream C: `cmd/server/main.go` composable startup + routes

## Target End State

Move `run()` orchestration into testable builders and route modules.

Proposed layout (same package `cmd/server`):

- `main.go` (entrypoint only: `main`, `run`)
- `bootstrap.go` (config + infra clients + service/handler wiring)
- `routes_api.go` (API route registration)
- `routes_web.go` (static/pages/share/og/docs routes)
- `middleware_chain.go` (middleware wiring/order)
- `ratelimits.go` (rate limiter setup + auth limit structs)
- `background_jobs.go` (notification/reminder loops)
- `server_http.go` (http.Server construction + graceful shutdown)

## Phases

### C1. Extract pure route registration functions

- Introduce `registerAuthRoutes`, `registerCardRoutes`, `registerFriendRoutes`, etc.
- Keep existing paths/middleware wrappers unchanged.

### C2. Introduce app wiring structs

- Add structs like:
  - `Dependencies` (db/redis/config/logger)
  - `Services`
  - `Handlers`
  - `Middleware`
- Build each via dedicated functions.

### C3. Extract background jobs and shutdown orchestration

- Move reminder/notification ticker goroutines to dedicated functions.
- Keep cancellation semantics identical.

### C4. Expand server package tests

- Route registration tests for key path existence + method binding.
- Middleware order tests (security-critical order assertions).
- Unit tests for extracted builders where possible (env parsing, rate limit config, poll interval).

## Exit Criteria (C)

- `cmd/server/main.go` reduced to orchestration shell (target < 250 LOC).
- Route registration logic is split by domain files.
- `cmd/server` tests cover route/middleware composition significantly better than baseline.

---

## Cross-Cutting Quality Gates

Run at each phase boundary:

1. `go test ./...`
2. `node web/static/js/tests/runner.js`
3. `make lint`
4. targeted `make e2e` or domain-specific E2E spec runs for touched areas

Hard guardrails:

- No API contract drift without explicit OpenAPI updates.
- No CSP relaxation.
- No change to auth/session/CSRF semantics.
- No hidden behavior changes in route paths or method verbs.

---

## Sequencing (Recommended)

1. Workstream C (route extraction first)  
Reason: lowers merge conflict risk for backend feature work and improves startup clarity.

2. Workstream B (card handler/service split)  
Reason: biggest backend blast-radius reducer after route modularization.

3. Workstream A (frontend split)  
Reason: largest change surface; do after backend structure is stable.

---

## Milestones

### Milestone 1
- `cmd/server` route/middleware composition split complete.
- No behavior changes, all tests green.

### Milestone 2
- card handler/service split complete with focused tests.
- Existing card E2E flows unchanged.

### Milestone 3
- frontend app split into domain files + bootstrap entrypoint.
- existing route/action behavior preserved.

### Milestone 4
- cleanup: remove dead helpers, tighten docs, update architecture docs with new file map.

---

## Definition of Done

- Monolithic hot spots reduced and domain boundaries explicit.
- PRs touching one domain do not require editing giant unrelated files.
- CI (unit/lint/e2e) passes without baseline regressions.
- Team docs reflect new structure and extension points.

---

## Implementation Backlog (PR-Sized)

Legend:
- Size: `S` (~0.5-1 day), `M` (~1-2 days), `L` (~2-4 days)
- Type: `Mechanical` (no behavior change expected), `Behavioral` (requires closer verification)

### Track C: Server Composition (`cmd/server`)

- [x] `C1` Extract API route registration into domain functions (`M`, `Mechanical`)
  - Scope:
    - Move route wiring from `run()` into `registerAuthRoutes`, `registerCardRoutes`, etc.
    - Keep wrappers and middleware application exactly as-is.
  - Files:
    - `cmd/server/main.go`
    - `cmd/server/routes_api.go` (new)
  - Validation:
    - `go test ./cmd/server`
    - `go test ./...`

- [x] `C2` Extract web/static/share/docs route registration (`S`, `Mechanical`)
  - Scope:
    - Move static/page/share/OG/docs route wiring to `registerWebRoutes`.
  - Files:
    - `cmd/server/routes_web.go` (new)
  - Validation:
    - `go test ./cmd/server`
    - `go test ./...`

- [x] `C3` Extract middleware chain builder (`S`, `Mechanical`)
  - Scope:
    - Introduce `buildMiddlewareChain(...)` with explicit order.
  - Files:
    - `cmd/server/middleware_chain.go` (new)
  - Validation:
    - `go test ./cmd/server`
    - `go test ./...`

- [x] `C4` Extract rate limit builders/config (`M`, `Mechanical`)
  - Scope:
    - Move auth/AI/redeem limiter setup and env parsing helpers to `ratelimits.go`.
    - Keep defaults and development overrides unchanged.
  - Files:
    - `cmd/server/ratelimits.go` (new)
  - Validation:
    - `go test ./cmd/server`
    - `go test ./...`

- [x] `C5` Extract reminder/notification background jobs (`M`, `Mechanical`)
  - Scope:
    - Move ticker goroutines + cancellation wiring to `background_jobs.go`.
  - Files:
    - `cmd/server/background_jobs.go` (new)
  - Validation:
    - `go test ./cmd/server`
    - `go test ./...`

- [x] `C6` Add route/middleware composition tests (`M`, `Behavioral`)
  - Scope:
    - Add tests asserting key routes exist and middleware order remains security-correct.
  - Files:
    - `cmd/server/*_test.go`
  - Validation:
    - `go test ./cmd/server -count=1`

### Track B: Card Handler/Service Split

- [x] `B1` Split handler: read/create/update paths (`M`, `Mechanical`)
  - Scope:
    - Move methods from `internal/handlers/card.go` into domain files without logic edits.
  - Files:
    - `internal/handlers/card_create_read.go` (new)
    - `internal/handlers/card_update.go` (new)
  - Validation:
    - `go test ./internal/handlers -count=1`

- [x] `B2` Split handler: finalize/completion flows (`M`, `Mechanical`)
  - Scope:
    - Move finalize/complete/uncomplete/notes/edit methods.
  - Files:
    - `internal/handlers/card_finalize_completion.go` (new)
  - Validation:
    - `go test ./internal/handlers -count=1`

- [x] `B3` Split handler: bulk/import/export/conflict (`L`, `Mechanical`)
  - Scope:
    - Move bulk/archive/export/import/conflict handling methods and helpers.
  - Files:
    - `internal/handlers/card_bulk.go` (new)
    - `internal/handlers/card_import_export.go` (new)
    - `internal/handlers/card_conflict.go` (new)
  - Validation:
    - `go test ./internal/handlers -count=1`
    - `go test ./...`

- [x] `B4` Split service: read/write/finalize/completion domains (`L`, `Mechanical`)
  - Scope:
    - Move methods from `internal/services/card.go` into domain files with zero signature changes.
  - Files:
    - `internal/services/card_read.go` (new)
    - `internal/services/card_write.go` (new)
    - `internal/services/card_finalize.go` (new)
    - `internal/services/card_completion.go` (new)
  - Validation:
    - `go test ./internal/services -count=1`

- [x] `B5` Split service: bulk/import/export/conflict/permissions (`L`, `Mechanical`)
  - Scope:
    - Move remaining card service domains and local helpers.
  - Files:
    - `internal/services/card_bulk.go` (new)
    - `internal/services/card_import_export.go` (new)
    - `internal/services/card_conflict.go` (new)
    - `internal/services/card_permissions.go` (new)
  - Validation:
    - `go test ./internal/services -count=1`
    - `go test ./...`

- [x] `B6` Add focused tests for extracted card domains (`M`, `Behavioral`)
  - Scope:
    - Add/expand tests per extracted area to reduce future regression risk.
  - Validation:
    - `go test ./internal/handlers ./internal/services -count=1`

### Track A: Frontend Decomposition (`web/static/js/app.js`)

- [x] `A1` Add composition scaffold (`M`, `Mechanical`)
  - Scope:
    - Introduce `app-core.js` + module composition pattern with no behavior changes.
    - Keep `app.js` as active source, scaffold only.
  - Files:
    - `web/static/js/app-core.js` (new)
    - `web/static/js/app.js`
  - Validation:
    - `node web/static/js/tests/runner.js`

- [x] `A2` Extract shared helpers + action routing shell (`M`, `Mechanical`)
  - Scope:
    - Move helper utilities and action delegation maps.
  - Files:
    - `web/static/js/app-actions.js` (new)
    - `web/static/js/app-modals.js` (new)
  - Validation:
    - `node web/static/js/tests/runner.js`

- [x] `A3` Extract notifications/reminders modules (`L`, `Behavioral`)
  - Scope:
    - Move render + action flows for notifications and reminders.
  - Files:
    - `web/static/js/app-notifications.js` (new)
    - `web/static/js/app-reminders.js` (new)
  - Validation:
    - `node web/static/js/tests/runner.js`
    - targeted E2E: notifications/reminders specs

- [x] `A4` Extract friends/invites/blocks modules (`L`, `Behavioral`)
  - Files:
    - `web/static/js/app-friends.js` (new)
  - Validation:
    - JS tests
    - targeted E2E: friend/invite specs

- [ ] `A5` Extract billing/templates/AI modules (`L`, `Behavioral`)
  - Files:
    - `web/static/js/app-billing.js` (new)
    - `web/static/js/app-templates.js` (new)
    - `web/static/js/app-ai.js` (new)
  - Validation:
    - JS tests
    - targeted E2E: billing/templates/ai specs

- [ ] `A6` Extract cards/auth/profile modules (`L`, `Behavioral`)
  - Files:
    - `web/static/js/app-cards.js` (new)
    - `web/static/js/app-auth.js` (new)
  - Validation:
    - JS tests
    - targeted E2E: card/auth/profile specs

- [ ] `A7` Wire multi-script loading + hashed manifest support (`M`, `Behavioral`)
  - Scope:
    - Add manifest getters + template vars for new script files.
    - Ensure deterministic script load order.
  - Files:
    - `internal/assets/assets.go`
    - `internal/handlers/pages.go`
    - `web/templates/index.html`
    - `scripts/build-assets.sh` (if needed for manifest mapping)
  - Validation:
    - `go test ./internal/assets ./internal/handlers`
    - JS tests
    - smoke E2E

- [ ] `A8` Expand frontend unit test coverage for extracted modules (`M`, `Behavioral`)
  - Scope:
    - Add tests for module boundaries and dispatch behavior.
  - Files:
    - `web/static/js/tests/runner.js`
  - Validation:
    - `node web/static/js/tests/runner.js`

### Stabilization / Rollout Tasks

- [ ] `R1` Repo-wide verification gate before merge (`S`)
  - Run:
    - `go test ./...`
    - `node web/static/js/tests/runner.js`
    - `make lint`
    - `make e2e` (or agreed targeted subset per PR)

- [ ] `R2` Documentation updates (`S`)
  - Scope:
    - Update `agent_docs/architecture.md` with new file map and extension points.
    - Link this plan from relevant roadmap docs.

---

## Suggested PR Batching

Recommended batching for manageable review size:

1. `PR-1` = `C1 + C2`
2. `PR-2` = `C3 + C4`
3. `PR-3` = `C5 + C6`
4. `PR-4` = `B1 + B2`
5. `PR-5` = `B3`
6. `PR-6` = `B4 + B5`
7. `PR-7` = `B6`
8. `PR-8` = `A1 + A2`
9. `PR-9` = `A3`
10. `PR-10` = `A4`
11. `PR-11` = `A5`
12. `PR-12` = `A6 + A7`
13. `PR-13` = `A8 + R1 + R2`

Each PR should include:
- explicit “no behavior change intended” statement (or behavior changes called out)
- before/after touched file map
- test output summary
