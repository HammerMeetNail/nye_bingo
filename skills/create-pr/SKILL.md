---
name: create-pr
description: Create a GitHub pull request from the current non-main branch in this repo by running lint/tests/e2e, staging only safe files (never secrets or troubleshooting images), committing with a detailed message, pushing, and opening/updating a PR via the gh CLI with a reviewer-ready description. Use for requests like "/create-pr", "create a PR", "push and open a PR", or "prep this branch for review".
---

# Create PR

## Preconditions

- Be on a feature branch (not `main`).
- Have a working `gh` CLI auth session (`gh auth status`).
- Know whether `make e2e` is acceptable to run right now (it is destructive in this repo).
- Decide whether you want to rebase/merge on top of the latest `origin/main` before opening the PR.

## 1) Verify branch and repo state

1. Get current branch:
   - `git branch --show-current`
2. Stop if on `main`:
   - If branch is `main`, create/switch to a new branch before continuing.
3. Sync refs and understand delta vs `main`:
   - `git fetch origin`
   - `git log --oneline --decorate origin/main..HEAD`
   - `git diff --stat origin/main...HEAD`
4. (Optional but recommended) Rebase/merge onto latest `origin/main`:
   - Prefer `git rebase origin/main` for linear history, unless the repo/process prefers merges.
   - If you already pushed and rebased, you will need `git push --force-with-lease` (do not use plain `--force`).
5. Confirm what will be included:
   - `git status --porcelain`
   - `git diff`

## 2) Decide which checks to run (based on relevant file changes)

1. Compute changed files vs `origin/main`:
   - `git fetch origin`
   - `changed=$(git diff --name-only origin/main...HEAD)`
2. Decide what’s “relevant”:
   - **Go-relevant** (run coverage/lint/backend tests): any changes to `*.go`, `go.mod`, `go.sum`, `cmd/`, `internal/`, `migrations/`
   - **JS/asset-relevant** (run frontend tests): any changes under `web/` or `web/static/js/`, or JS/CSS/HTML files
   - **E2E-relevant** (consider running Playwright): any changes under `web/`, `tests/e2e/`, `playwright.config.js`, or backend HTTP surfaces (`cmd/`, `internal/handlers/`, `internal/middleware/`, `internal/services/`, `migrations/`)
3. Optional override:
   - If the user wants the “full” safety net regardless of change type, run all checks anyway.

## 3) Run validations (only when relevant)

1. If Go-relevant: ensure coverage does not decrease vs `origin/main`:
   - Compute baseline coverage on `origin/main` (recommended: temporary worktree so you don’t lose state):

```bash
tmp=$(mktemp -d)
git worktree add --detach "$tmp" origin/main
base_cov=$((cd "$tmp" && make coverage) | tail -n 1 | rg -o '[0-9]+(\\.[0-9]+)?%' | tr -d '%')
git worktree remove --force "$tmp"
head_cov=$(make coverage | tail -n 1 | rg -o '[0-9]+(\\.[0-9]+)?%' | tr -d '%')
BASE_COV="$base_cov" HEAD_COV="$head_cov" python3 - <<'PY'
import os
base = float(os.environ["BASE_COV"])
head = float(os.environ["HEAD_COV"])
print(f"origin/main: {base:.2f}%")
print(f"HEAD:        {head:.2f}%")
if head + 1e-9 < base:
  raise SystemExit("Coverage decreased. Add/improve tests before proceeding.")
PY
```

2. If Go-relevant: run lint:
   - `make lint`
3. Run tests based on relevance:
   - If Go-relevant **and** JS/asset-relevant: `make test`
   - If Go-relevant only: `make test-backend`
   - If JS/asset-relevant only: `make test-frontend`
4. If E2E-relevant: confirm with the user, then run e2e:
   - Explain that `make e2e` is destructive (resets volumes/reseeds) and can take a while.
   - Run: `make e2e`

If any validation step fails:
- Fix failures automatically (update code/config/tests as needed).
- Re-run the failed command until it passes.
- After fixes, restart this validation section from the beginning (coverage/lint/tests/e2e as applicable) before proceeding to staging/commit/PR.

## 4) Stage changes safely (no secrets, no troubleshooting images)

1. Enumerate changed/untracked files:
   - `git status --porcelain`
2. Identify files to explicitly exclude from commit:
   - Any secrets/credentials (tokens, private keys, API keys).
   - Local env/config/log artifacts (examples: `.env`, `.stripe-listen.log`, `coverage.out`, `playwright-report/`, `test-results/`), unless there is an explicit reason to commit them.
   - Any image added only for debugging/troubleshooting (common extensions: `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`), unless the change intentionally adds product assets.
3. Stage the intended change set:
   - Prefer staging explicitly by file path(s) you intend to include.
   - For careful staging, use `git add -p` to avoid bundling drive-by changes.
   - If staging everything for convenience (`git add -A`), immediately unstage excluded files (`git restore --staged <path>`), and/or delete unintended untracked artifacts.
4. Review staged changes carefully:
   - `git diff --cached --stat`
   - `git diff --cached`
5. Confirm nothing important was accidentally left out:
   - `git status --porcelain` should show either a fully staged change set (ready to commit) or only intentionally untracked/ignored artifacts.
6. Do a quick secret sanity check before committing:
   - Search staged diff for common secret markers (examples): `git diff --cached | rg -n '(BEGIN (RSA|EC|OPENSSH) PRIVATE KEY|PRIVATE KEY-----|STRIPE_SECRET_KEY|AWS_SECRET_ACCESS_KEY|GH_TOKEN|xox[baprs]-)'`
   - If any match is found, do not commit; remove/redact and rotate secrets if needed.

## 5) Commit with a detailed message

Create a commit message that is detailed enough for review and future archaeology.

1. Choose a clear title (imperative, ≤ 72 chars).
2. In the body, include:
   - Why: what problem this change solves
   - What: key changes grouped by area (backend/frontend/db)
   - Risk: any tricky parts or migration notes
   - Tests: explicitly list which checks ran (and outcomes), and which were skipped (and why)

Commit using a multi-line message, for example:

```bash
git commit -m "<title>" \
  -m "Why: ..." \
  -m "What: ..." \
  -m "Tests: (ran) ...; (skipped) ... (reason: ...)"
```

## 6) Push branch

- Push and set upstream:
  - `git push -u origin HEAD`
- If you rebased after pushing:
  - Push safely: `git push --force-with-lease`

## 7) Create or update PR with a reviewer-ready description

1. Determine branch name:
   - `branch=$(git branch --show-current)`
2. If an open PR already exists for this branch, prefer updating it:
   - `gh pr view --head "$branch"` (if this fails, create a new PR)
   - Update title/body when needed: `gh pr edit --title "<title>" --body-file - <<'EOF' ... EOF`
3. Create PR targeting `main` (adjust base if requested):

```bash
gh pr create --base main --head "$branch" --title "<title>" --body-file - <<'EOF'
## Summary
- ...

## Changes
- ...

## How to review
1. ...
2. ...

## How to test
- (Ran) ...
- (Skipped) ... (reason: ...)

## Notes / risks
- ...
EOF
```

4. (Optional) Add reviewers/assignees/labels if the user provides them:
   - `gh pr edit --add-reviewer user1,user2`
   - `gh pr edit --add-assignee user1`
   - `gh pr edit --add-label "bug","enhancement"`
5. Provide the PR URL and (optionally) open it in a browser:
   - `gh pr view --web`
6. After opening, watch CI status (optional but convenient):
   - `gh pr checks --watch`
