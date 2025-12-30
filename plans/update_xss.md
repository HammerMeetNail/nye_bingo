# Plan: XSS Hardening + CSP Tightening (Future Session)

Goal: eliminate practical XSS vectors in the SPA and tighten Content Security Policy by removing reliance on inline JavaScript (`onclick`, `onsubmit`, etc.) and unsafe DOM injection patterns. This plan is written to be implementable end-to-end by an AI agent.

## Scope and current state
- Frontend is a vanilla JS SPA with heavy use of `innerHTML` template strings in `web/static/js/app.js`.
- The app currently uses inline event handlers in many templates (e.g., `onclick="App.*"` and `onsubmit="App.*"`).
- CSP in `internal/middleware/security.go` currently includes `script-src 'unsafe-inline' 'unsafe-hashes' ...`, which greatly reduces CSP’s XSS protection.
- Some code paths insert **unescaped** `error.message` into `innerHTML` (e.g., friends search catch path), which is a real XSS risk if server error strings can contain HTML-ish content.

## Threat model / what “fixed” means
### Must prevent
- Stored XSS from user-controlled fields (username, card title, bingo item content/notes, category labels) appearing in the UI.
- Reflected XSS via server-provided error messages rendered into the DOM.
- Attribute/JS-string injection (e.g., interpolating user strings into `onclick="... '${x}' ..."`).

### Non-goals for this pass
- Rewriting the entire UI framework; keep the existing `App` object structure.
- Removing all `innerHTML` usage everywhere (but we should reduce it in high-risk places and standardize safe patterns).

## Implementation plan (step-by-step)

### 1) Inventory and classify risky sinks (automated grep)
Run these searches and record results as a checklist:
- Inline handlers:
  - `rg -n "on(click|submit|change|input|key)" web/static/js/app.js`
  - `rg -n "onclick=\\\"|onsubmit=\\\"|onchange=\\\"|oninput=\\\"" web/static/js/app.js`
- HTML sinks:
  - `rg -n "innerHTML\\s*=|insertAdjacentHTML\\(|outerHTML\\s*=" web/static/js/app.js`
- Unescaped variables in templates (high priority):
  - `rg -n "\\$\\{error\\.message\\}" web/static/js/app.js`
  - `rg -n "\\$\\{[^}]*username[^}]*\\}" web/static/js/app.js`
  - `rg -n "\\$\\{[^}]*content[^}]*\\}" web/static/js/app.js`

For each match, classify:
- **T1 (Untrusted)**: anything from API responses, user input, URL params, `error.message`.
- **T2 (Semi-trusted)**: UUIDs from API, enum-like values, known server-controlled constants.
- **T3 (Trusted)**: hardcoded UI strings.

### 2) Establish safe rendering rules (project conventions)
Add/standardize these conventions in `web/static/js/app.js`:
- Never interpolate **T1** into an HTML attribute inside an `innerHTML` string.
- Prefer `textContent` for T1 text rendering.
- Prefer `element.setAttribute(...)` for attributes, after creating the element.
- Avoid inline handler attributes entirely; attach handlers with `addEventListener` or event delegation.

If needed, add small helper utilities near existing helpers (end of file):
- `App.setText(el, text)` => `el.textContent = text ?? ''`
- `App.qs(id)` helper for `document.getElementById`
- Optional: `App.el(tag, attrs)` + `App.append(parent, child)` to reduce template strings in high-risk sections.

### 3) Remove inline handlers (enables strict CSP)
For each UI section, replace inline handler attributes with one of:

**A) Event delegation (preferred for lists)**
- Render buttons/links with `data-action`, `data-id`, etc.
- Attach a single `click` handler to the parent container once.
- Use `event.target.closest('button[data-action]')` to route.

**B) Direct listeners (preferred for single controls/modals)**
- Render element with a stable `id` (or `data-role`).
- After `innerHTML` is injected, do `document.getElementById(...).addEventListener(...)`.

Concretely, update these common patterns in `web/static/js/app.js`:
- `onclick="App.*(...)"` => remove and move logic to:
  - existing `setup*Events()` functions, or
  - a new `setup*ActionHandlers()` per page/section.
- `onsubmit="App.handle*(event)"` => remove, attach listener:
  - `form.addEventListener('submit', (e) => App.handle*(e))`

Checklist: ensure `setupNavigation()` and `renderX()` paths call the relevant `setup...` method after rendering.

### 4) Fix known unsafe error rendering (must-do)
Replace all `innerHTML = \`...${error.message}...\`` with safe DOM operations:
- Minimal change:
  - `el.innerHTML = \`<p class="text-muted">${this.escapeHtml(error.message)}</p>\``
- Better:
  - `el.textContent = error.message`
  - and apply styling via className or wrapping `<p>` created via DOM API.

Search term: `\${error.message}` is the fast way to find likely issues.

### 5) Fix remaining inline-handler injection risks
Even if values look safe today (UUIDs, generated tokens), remove patterns that embed values in inline JS strings:
- Invite link “Copy” button currently uses inline `onclick` with an interpolated URL string. Replace with:
  - a button with `id="copy-invite-link-btn"` and an `addEventListener`, or
  - a button with `data-url` + delegated handler.
- Invite “Revoke” currently uses `onclick="App.revokeInvite('${invite.id}')"`; replace with `data-invite-id` + delegated handler on `#invite-list`.

Also scan for any remaining instances of:
- `onclick="... '${this.escapeHtml(...)}' ..."` (especially unsafe)
- `onclick="... ${this.escapeHtml(...)} ..."` inside attributes

### 6) Add Playwright regression tests for XSS-sensitive cases
Add E2E tests under `tests/e2e/` that prove:
- A username containing quotes and angle brackets does not break the UI and cannot inject script.
  - Example username: `xss'\"<b>test</b>`
- Actions still work (click “Block”, “Remove”, “Unblock”, “Revoke invite”, “Copy invite link”).

Implementation notes:
- The existing helpers generate usernames; update helpers to allow overriding the username or provide a `buildUser` option to inject a crafted username.
- Avoid checking “no alert popped” (fragile); instead:
  - ensure the page still renders,
  - actions remain clickable,
  - and text appears as escaped text (e.g., contains literal `<b>` not bolded).

Update `plans/playwright.md` to list the new XSS-focused spec(s).

### 7) Tighten CSP once inline JS is eliminated
After step (3) removes inline handlers across the app (not just friends/invites), update CSP in `internal/middleware/security.go`:
- Remove `script-src 'unsafe-inline'` and likely remove `'unsafe-hashes'`.
- Ensure scripts are loaded via external files only.
- If any inline scripts remain in templates, move them into an external JS file or use a nonce-based CSP (nonce requires template support).

Update `internal/middleware/security_test.go` expected directives accordingly.

Optional: add a CSP “report-only” mode first (header `Content-Security-Policy-Report-Only`) to validate in production before enforcing.

### 8) Consider username validation (optional product decision)
This is not strictly required if output encoding is correct, but it reduces risk:
- In `internal/handlers/auth.go`, restrict usernames to a safe subset (e.g., letters/numbers/space/`_-.`) and reject `<`, `>`, quotes, control chars.
- WARNING: may break existing users unless you only enforce on registration and allow existing usernames to remain (and still must be safely encoded everywhere).

### 9) Validation checklist before shipping
Run:
- `./scripts/test.sh --coverage`
- `make e2e` (destructive) or targeted e2e specs if supported.

Manual checks:
- Navigate through core pages (home/login/register/dashboard/card editor/friends/invites/profile).
- Confirm there are no inline handler attributes left in rendered DOM for those pages:
  - Quick devtools check: search DOM for `onclick=` / `onsubmit=`.
- Confirm CSP header no longer includes `'unsafe-inline'` and the app still functions.

## Files likely to change
- `web/static/js/app.js` (main work: remove inline handlers, safe rendering, fix error rendering)
- `internal/middleware/security.go` + `internal/middleware/security_test.go` (CSP tightening)
- `tests/e2e/*.spec.js` + `plans/playwright.md` (regression coverage)
- Optional: `internal/handlers/auth.go` (username validation policy)

## Notes for the implementing agent
- Be systematic: convert one page/section at a time and keep behavior identical.
- Prefer event delegation for lists and repeated controls to avoid re-binding handlers on rerender.
- When you must use `innerHTML` for layout, keep interpolations limited to trusted constants; set untrusted values via DOM APIs after insertion.
- Don’t rely on `escapeHtml()` to make values safe inside inline JS; treat attribute/JS contexts separately or avoid them.

