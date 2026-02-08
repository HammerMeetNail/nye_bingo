# Roadmap & Status

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
- Phase 14: Public API Access (API tokens, OpenAPI spec, Swagger UI)

See `plans/bingo.md` for the full implementation plan and `plans/auth.md` for email authentication details.

## Active Maintenance Plans

- **`plans/refactor.md`** - Monolith risk reduction refactor for server composition (`cmd/server`), card handler/service domain splits, and frontend `App` decomposition with deterministic multi-script asset loading. Use this plan as the source of truth for incremental PR batching and validation gates.

## Pending Plans

The following plans are ready for implementation:

- **`plans/tracing.md`** - OpenTelemetry tracing with Honeycomb (free tier). Adds distributed tracing, service-level instrumentation, database query tracing, and log correlation. 5 phases, can be done incrementally.

- **`plans/increase_test_coverage_via_interfaces.md`** - Interface-based dependency injection to enable comprehensive unit testing. Introduces interfaces between handlers and services, enabling mock injection. Target: 70%+ handler coverage (currently ~31%).

- **`plans/flexible_cards.md`** - Custom card dimensions beyond 5x5 BINGO. Header word determines columns (2-10 chars), rows configurable (2-10). Optional user-placed FREE space. Classic BINGO cards preserved as separate type. Significant impact on rendering and UI.
