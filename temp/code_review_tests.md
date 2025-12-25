# Test Quality Remediation Plan (PR #24)

This plan captures the test-quality issues identified in PR #24 and lays out concrete work to (1) remove low-value/coverage-driven tests, (2) strengthen assertions around outcomes and side effects, and (3) reduce the risk of tests passing while behavior is wrong (e.g., incorrect SQL, missing gating, wrong error mapping).

## Goals

- Prefer fewer, higher-signal tests over many status-only or “callable” tests.
- Make tests fail for the *right reasons*: wrong output, wrong side effects, wrong arguments, wrong error mapping.
- Avoid real network dependencies in unit tests (no “real” Redis client dialing a host/port).
- Reduce brittleness from asserting incidental details (unless they’re part of a stable contract).

## Non-Goals

- No refactors purely to raise coverage.
- No broad product changes unless required to enable meaningful testing (e.g., injecting a dependency via an interface).

## Execution Checklist (Phased)

### Phase 0 — Baseline + guardrails

- [x] Run `./scripts/test.sh` and record current failures/flakes (if any).
  - Notes: baseline run passed (no failures/flakes).
- [x] Identify which tests are newly added vs. refactored (already mostly done via `git diff --name-only main..HEAD | rg _test.go`).
- [x] Agree on “test contract” expectations for key endpoints:
  - `POST /api/ai/generate`: response JSON shape, quota behavior, and error codes/messages.
  - Auth endpoints: error response schema and cookies/session side effects.

Deliverable: PR comment (or notes) stating the intended contracts we’re testing.

---

## Workstream A — Remove/Replace Coverage-Driven Tests

### A1. Delete coverage-only Redis adapter test

Problem: `internal/handlers/support_test.go` contains a explicitly coverage-driven test that ignores errors and instantiates a real Redis client.

- [x] Remove `TestSupportHandler_RedisAdapter_Coverage` (`internal/handlers/support_test.go:140`).
- [ ] Replace with one of the following higher-signal alternatives (pick one):
  1) **Preferred (unit-only)**: adjust `NewSupportHandler` to accept a `RateLimitStore` interface (or accept `rateLimiter` directly), and test that the handler uses the limiter (already well-covered by `checkRateLimit` tests).
  2) **If adapter must remain**: isolate the adapter into its own type/file and unit test it by stubbing the minimal Redis operations via a small interface (no real `*redis.Client`).

Notes: adapter wiring was not separately re-tested; existing `checkRateLimit` coverage kept as the unit signal.

Acceptance criteria:
- No unit test dials a real host/port.
- Adapter wiring is either removed from tests or validated via stubs with meaningful assertions (key prefix/expiry usage and error propagation).

---

## Workstream B — Fix “Non-Test” Tests (Cannot Fail)

### B1. Enforce a real contract for bcrypt “too long” passwords

Problem: `TestPasswordComplexity_TooLong` logs and passes even if behavior is wrong (`internal/services/auth_test.go:198`).

Decide and implement one contract:

Option 1 (recommended for clarity/safety):
- [x] Add an explicit length check in `AuthService.HashPassword` for bcrypt’s effective limit (72 bytes, not runes).
- [x] Return a stable error (e.g., `ErrPasswordTooLong`) and assert it.
- [x] Update the test to require an error (no `t.Log` escape hatch).

Option 2 (if you do not want to enforce this in app code):
- [ ] Delete the test entirely (since it provides no confidence).

Acceptance criteria:
- The test fails if the contract is violated.
- The contract is documented in the code and tested deterministically.

---

## Workstream C — Upgrade Status-Only Handler Tests Into Contract Tests

### C1. AI handler tests: assert response + side effects, not just `w.Code`

Problem: `internal/handlers/ai_test.go` largely asserts only status codes (e.g., `:378`), missing response-body contract and quota side effects.

Implementation steps:
- [x] Add helpers in tests for:
  - decoding JSON response into a typed struct or `map[string]any`
  - asserting standard error response schema (e.g., `ErrorResponse{Error: ...}`) and `Content-Type`
- [x] For success cases, assert:
  - response contains goals and expected count
  - `Content-Type` is JSON (if that is the contract)
- [x] For unverified users:
  - assert `ConsumeUnverifiedFreeGeneration` is called exactly once
  - assert `GenerateGoals` is not called if quota is exhausted
- [x] For validation failures:
  - assert `GenerateGoals` and quota-consume methods are not called (use call counters or `t.Fatal` on invocation)

Acceptance criteria:
- Tests fail if the handler returns the wrong JSON payload, forgets quota consumption, or calls provider methods on invalid input.

### C2. Card/Auth/Friend handler tests: add meaningful assertions to “invalid body”/“unauthenticated” cases

Targets (examples):
- `internal/handlers/card_test.go:32` and similar: currently only checks status.
- `internal/handlers/auth_test.go:152` and similar: many cases check status only.
- `internal/handlers/friend_test.go:16`: mixed scenarios in one test; also checks status without response contract.

Implementation steps:
- [x] For each validation/unauthenticated test, assert:
  - response error payload (message/code)
  - no downstream service calls were made (use mocks that track calls; `t.Fatal` if invoked on invalid requests)
- [x] Split “multi-scenario” tests into smaller, named subtests where that improves clarity (e.g., friend search short query vs. service error).

Acceptance criteria:
- Validation tests enforce both *output* and *lack of side effects*.
- Tests are readable: one behavior per subtest, no “scripted” multi-step flows without assertions in between.

---

## Workstream D — Make Service Tests Detect Wrong SQL / Wrong Args

Problem: many service tests pass even if SQL strings/argument ordering is wrong because the DB fakes accept any query.

Targets (high value first):
- `internal/services/api_token_test.go`:
  - `UpdateLastUsed` success (`:84`) should assert SQL contains the expected update + where clause and args include the token ID.
  - `Delete/DeleteAll` should assert correct scoping to user (where applicable).
- `internal/services/suggestion_service_test.go`: assert correct table and filters (e.g., active-only if that’s intended).
- `internal/services/friend_service_test.go`: assert existence-check and insert/update queries include expected predicates.

Implementation steps:
- [x] Extend the existing `fakeDB` (in `internal/services/dbiface_test.go`) or add a small “recording DB” wrapper that:
  - records each `Exec/Query/QueryRow` call (sql + args)
  - optionally enforces a per-test expectation list
- [x] Update service tests to verify:
  - SQL contains stable fragments (`FROM ...`, `WHERE user_id = $1`, etc.)
  - args match expected values (IDs, category strings, etc.)

Notes: used per-test SQL/args capture via existing `fakeDB` hooks instead of introducing a new recording wrapper.

Acceptance criteria:
- A swapped argument order or missing `WHERE user_id = ...` causes the test to fail.
- SQL assertions avoid over-specifying formatting (use `strings.Contains` on key fragments).

---

## Workstream E — Replace Weak Error Assertions With Contractual Ones

Problem: patterns like `if err == nil || err.Error() == ""` are low-signal and don’t validate wrapping or error identity.

Targets:
- `internal/database/postgres_test.go:12`, `:25`, `:56`
- `internal/database/redis_test.go:12`

Implementation steps:
- [x] Assert error identity (`errors.Is`) where possible.
- [x] If wrapping strings are part of the contract, assert `strings.Contains` on expected wrapper prefix + root cause.

Acceptance criteria:
- Errors are asserted meaningfully (cause + context), not just “non-empty string”.

---

## Workstream F — Remove/Reduce Trivial, Language-Guarantee, or “Doesn’t Panic” Tests

Targets to evaluate and likely remove or fold into stronger tests:
- `internal/testutil/testutil_test.go:12` (`TestAssertHelpers`): tests test helpers with tautological inputs.
- `internal/models/card_test.go:87` (`TestCardStats_ZeroValues`): asserts Go zero values.
- `internal/database/postgres_test.go:143` (`Close_NilPool`) and `internal/database/redis_test.go:109` (`CloseNil`): “doesn’t panic” checks.
- `internal/handlers/context_test.go:71` (`TestContextKey_UniqueType`): largely re-tests Go’s typed-key behavior; keep only if it’s guarding a prior regression.

Implementation steps:
- [x] Delete tests that don’t encode app-specific behavior.
- [ ] If you keep a “no panic” test, convert it into a real contract test:
  - e.g., `Close()` returns `nil` and is idempotent (if that is intended and relied upon).

Acceptance criteria:
- Each remaining test asserts meaningful business logic, contract behavior, or regression-prone behavior.

---

## Optional Workstream G — Targeted Integration Tests (High Confidence, Low Count)

If `./scripts/test.sh` already runs containers (Postgres/Redis), consider adding a *small number* of integration tests that replace many low-signal unit tests:

- [ ] One integration test for rate limiting middleware with real Redis (verifies increments, expiry, and 429 behavior).
- [ ] One integration test for a critical handler/service flow that touches DB constraints (e.g., token creation + validation).
  - Notes: not added; scope kept to unit-level improvements per plan focus.

Guidelines:
- Keep integration tests few and focused.
- Ensure they are deterministic and can run in the project’s container test harness.

Acceptance criteria:
- Integration tests catch real wiring/config/query issues that unit tests cannot.

---

## Definition of Done

- [x] Coverage-driven tests removed or upgraded to assert outcomes/side effects.
- [x] Status-only tests upgraded to validate response bodies and “no unexpected calls”.
- [x] Service tests assert SQL fragments + args for correctness and scoping.
- [x] Non-failing tests fixed or removed.
- [x] `./scripts/test.sh` passes.

## Suggested Implementation Order (to minimize churn)

1) A1 (delete/replace Redis adapter coverage test)
2) B1 (fix the non-failing bcrypt-too-long test)
3) C1 (AI handler contract + side effects)
4) D (service SQL/args assertions)
5) E (error assertion improvements)
6) F (delete trivial/noise tests)
7) Optional G (integration tests)
