# Feature Flag / Entitlement Implementation Plan

## Purpose

Document the premium feature-entitlement pattern used in this repository so premium features can be shipped independently instead of relying on a single global `is_premium` check.

This document reflects the implementation as of February 5, 2026.

## Current Model

### Backend Entitlement Layer

- File: `internal/services/billing/entitlements.go`
- `Feature` enum values currently include:
  - `templates`
- `FeatureEntitlements` currently includes:
  - `templates: bool`
- Entitlement API:
  - `IsPremium(user, now)` returns global premium status.
  - `Features(user, now)` returns per-feature flags.
  - `HasFeature(user, now, feature)` is the server-side gate check.

Current mapping:
- `templates` feature is enabled when `IsPremium(...)` is true.

### API Exposure

- Billing status payload (`GET /api/billing/status`) includes:
  - `is_premium`
  - `features` object (`templates` currently)
- Auth payloads include:
  - `is_premium`
  - `features` object (`templates` currently)

Files:
- `internal/services/billing/types.go`
- `internal/services/billing/service.go`
- `internal/handlers/auth.go`
- `internal/handlers/billing.go`

### Templates Gating

Templates are gated by feature flag, not directly by global premium:

- Server handlers use `HasFeature(..., FeatureTemplates)`:
  - `internal/handlers/templates.go`
- Frontend uses feature-aware checks:
  - `App.hasFeature('templates')`
  - `App.requireFeature('templates', ...)`
  - `web/static/js/app.js`

## Adding a New Premium Feature Flag

1. Add enum + field in backend entitlement model:
   - `internal/services/billing/entitlements.go`
2. Map the new feature in `Features(user, now)`.
3. Gate server endpoints with `HasFeature(...)`.
4. Return feature state through auth and billing responses (already wired to return `features`).
5. Use `App.hasFeature('<feature_name>')` in UI/route/action checks.
6. Add tests for:
   - Entitlement mapping
   - Handler gate behavior
   - Frontend route/UI gating behavior
7. Update `web/static/openapi.yaml` with new `features` property schema.

## Guardrail Rules

- Always enforce feature access server-side; frontend checks are UX only.
- Keep free functionality additive-safe (no regressions to existing free workflows).
- Prefer feature-specific checks over broad `is_premium` checks for premium feature surfaces.
- Keep feature names stable and explicit across backend and frontend (`templates`, `ai_enhancements`, etc.).

## Testing Checklist

- Backend unit tests:
  - `internal/services/billing/entitlements_test.go`
  - Feature-gated handler tests for each premium endpoint.
- Frontend JS tests:
  - Route and navigation gating behavior.
  - Feature entitlement override behavior (`is_premium=true` but feature disabled).
- API contract:
  - `web/static/openapi.yaml` reflects `features` in relevant responses.
