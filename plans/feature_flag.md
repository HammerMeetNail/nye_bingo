# Feature Flag / Entitlement Implementation Plan

## Purpose

Document the premium feature-entitlement pattern used in this repository so premium features can be shipped independently instead of relying on a single global `is_premium` check.

This document reflects the implementation as of February 6, 2026.

## Current Model

### Backend Entitlement Layer

- File: `internal/services/billing/entitlements.go`
- `Feature` enum values currently include:
  - `templates`
  - `edit_after_finalize`
- `FeatureEntitlements` currently includes:
  - `templates: bool`
  - `edit_after_finalize: bool`
- Entitlement API:
  - `IsPremium(user, now)` returns global premium status.
  - `Features(user, now)` returns per-feature flags.
  - `HasFeature(user, now, feature)` is the server-side gate check.

Current mapping:
- `templates` feature is enabled when:
  - `IsPremium(...)` is true, and
  - `FEATURE_TEMPLATES_ENABLED=true`
- `edit_after_finalize` feature is enabled when:
  - `IsPremium(...)` is true, and
  - `FEATURE_EDIT_AFTER_FINALIZE_ENABLED=true`

### Global Feature Switches (Required)

Every premium feature flag must also support a **global runtime switch** so operators can disable a single premium feature without revoking Premium itself.

Required env vars:
- `FEATURE_TEMPLATES_ENABLED` (default `true`)
- `FEATURE_EDIT_AFTER_FINALIZE_ENABLED` (default `true`)

Evaluation rule for server gating:
- `HasFeature(user, now, feature)` must return `true` **only if both are true**:
  - user entitlement for that feature (`IsPremium(...)` + per-feature mapping)
  - global feature switch for that feature

Behavioral expectations:
- A user can still be `is_premium=true` while one or more feature flags are globally disabled.
- In that state, API `features` should expose the disabled feature(s) as `false` so frontend UX stays consistent.
- Server handlers must continue to use `HasFeature(...)` as the source of truth.

### API Exposure

- Billing status payload (`GET /api/billing/status`) includes:
  - `is_premium`
  - `features` object (`templates`, `edit_after_finalize`)
- Auth payloads include:
  - `is_premium`
  - `features` object (`templates`, `edit_after_finalize`)

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

### Edit After Finalize Gating

Edit-after-finalize is gated by feature flag, not directly by global premium:

- Server handler uses `HasFeature(..., FeatureEditAfterFinalize)`:
  - `internal/handlers/card.go`
- Frontend uses feature-aware checks:
  - `App.hasFeature('edit_after_finalize')`
  - `web/static/js/app.js`

## Adding a New Premium Feature Flag

1. Add enum + field in backend entitlement model:
   - `internal/services/billing/entitlements.go`
2. Add global env switch in config (default `true`) and wire startup initialization.
3. Map the new feature in `Features(user, now)` including global switch logic.
4. Gate server endpoints with `HasFeature(...)`.
5. Return feature state through auth and billing responses (already wired to return `features`).
6. Use `App.hasFeature('<feature_name>')` in UI/route/action checks.
7. Add tests for:
   - Entitlement mapping
   - Handler gate behavior
   - Global-switch override behavior (`is_premium=true` but server feature disabled)
   - Frontend route/UI gating behavior
8. Update `web/static/openapi.yaml` with new `features` property schema.

## Guardrail Rules

- Always enforce feature access server-side; frontend checks are UX only.
- Keep free functionality additive-safe (no regressions to existing free workflows).
- Prefer feature-specific checks over broad `is_premium` checks for premium feature surfaces.
- Keep feature names stable and explicit across backend and frontend (`templates`, `ai_enhancements`, etc.).
- Keep global switch env names explicit and stable (`FEATURE_<NAME>_ENABLED`).

## Testing Checklist

- Backend unit tests:
  - `internal/services/billing/entitlements_test.go`
  - Feature-gated handler tests for each premium endpoint.
- Frontend JS tests:
  - Route and navigation gating behavior.
  - Feature entitlement override behavior (`is_premium=true` but feature disabled).
- API contract:
  - `web/static/openapi.yaml` reflects `features` in relevant responses.
