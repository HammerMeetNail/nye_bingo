#!/bin/bash
# Run end-to-end tests against a fresh local stack.
# Usage: ./scripts/e2e.sh [playwright args...]
#
# Useful env vars:
# - PLAYWRIGHT_BROWSERS=firefox[,chromium,webkit]
# - PLAYWRIGHT_WORKERS=auto|N
# - PLAYWRIGHT_HEADLESS=true|false
# - AI_STUB=1 (default) for deterministic AI flows

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

BASE_URL="${BASE_URL:-http://localhost:8080}"
PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-http://app:8080}"
PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL%/}"
PLAYWRIGHT_BROWSERS="${PLAYWRIGHT_BROWSERS:-${BROWSERS:-firefox}}"
PLAYWRIGHT_HEADLESS="${PLAYWRIGHT_HEADLESS:-${HEADLESS:-true}}"
PLAYWRIGHT_WORKERS="${PLAYWRIGHT_WORKERS:-}"
PWDEBUG="${PWDEBUG:-}"
PLAYWRIGHT_OUTPUT_DIR="${PLAYWRIGHT_OUTPUT_DIR:-/test-results}"
PLAYWRIGHT_REPORT_DIR="${PLAYWRIGHT_REPORT_DIR:-/playwright-report}"
HEALTH_ATTEMPTS="${E2E_HEALTH_ATTEMPTS:-60}"
HEALTH_SLEEP="${E2E_HEALTH_SLEEP:-2}"
AI_STUB="${AI_STUB:-1}"
REMINDERS_POLL_INTERVAL="${REMINDERS_POLL_INTERVAL:-1s}"
GOOGLE_OAUTH_ENABLED="${GOOGLE_OAUTH_ENABLED:-true}"
OIDC_CLIENT_ID="${OIDC_CLIENT_ID:-${GOOGLE_OAUTH_CLIENT_ID:-oidc-test}}"
OIDC_CLIENT_SECRET="${OIDC_CLIENT_SECRET:-${GOOGLE_OAUTH_CLIENT_SECRET:-oidc-secret}}"
GOOGLE_OAUTH_CLIENT_ID="${OIDC_CLIENT_ID}"
GOOGLE_OAUTH_CLIENT_SECRET="${OIDC_CLIENT_SECRET}"
GOOGLE_OAUTH_REDIRECT_URL="${PLAYWRIGHT_BASE_URL}/api/auth/google/callback"
GOOGLE_OIDC_ISSUER_URL="${GOOGLE_OIDC_ISSUER_URL:-http://oidc:5555}"
GOOGLE_OIDC_SCOPES="${GOOGLE_OIDC_SCOPES:-openid,email,profile}"
OAUTH_ALLOWED_PROVIDERS="${OAUTH_ALLOWED_PROVIDERS:-google}"
OIDC_ISSUER_URL="${OIDC_ISSUER_URL:-http://oidc:5555}"
OIDC_REDIRECT_URI="${GOOGLE_OAUTH_REDIRECT_URL}"
OIDC_BASE_URL="${OIDC_BASE_URL:-http://oidc:5555}"

cd "$PROJECT_DIR"

echo "================================"
echo "Year of Bingo E2E Runner"
echo "================================"
echo ""
echo "Resetting local stack (destructive: volumes will be removed)."
podman compose down -v

echo ""
echo "Building assets..."
./scripts/build-assets.sh

echo ""
echo "Starting OIDC mock..."
export AI_STUB
export REMINDERS_POLL_INTERVAL
export GOOGLE_OAUTH_ENABLED
export GOOGLE_OAUTH_CLIENT_ID
export GOOGLE_OAUTH_CLIENT_SECRET
export GOOGLE_OAUTH_REDIRECT_URL
export GOOGLE_OIDC_ISSUER_URL
export GOOGLE_OIDC_SCOPES
export OAUTH_ALLOWED_PROVIDERS
export OIDC_ISSUER_URL
export OIDC_CLIENT_ID
export OIDC_CLIENT_SECRET
export OIDC_REDIRECT_URI
export OIDC_BASE_URL
podman compose --profile e2e up -d --build oidc

echo ""
echo "Waiting for OIDC mock..."
for ((i=1; i<=HEALTH_ATTEMPTS; i++)); do
  if curl -fsS "http://localhost:5555/.well-known/openid-configuration" >/dev/null 2>&1; then
    echo "OIDC mock is healthy."
    break
  fi
  if [[ "$i" -eq "$HEALTH_ATTEMPTS" ]]; then
    echo "OIDC mock did not become healthy in time."
    exit 1
  fi
  sleep "$HEALTH_SLEEP"
done

echo ""
echo "Starting containers..."
podman compose --profile e2e up -d --build app postgres redis mailpit

echo ""
echo "Waiting for health check at ${BASE_URL}/health ..."
for ((i=1; i<=HEALTH_ATTEMPTS; i++)); do
  if curl -fsS "${BASE_URL}/health" >/dev/null 2>&1; then
    echo "App is healthy."
    break
  fi
  if [[ "$i" -eq "$HEALTH_ATTEMPTS" ]]; then
    echo "App did not become healthy in time."
    exit 1
  fi
  sleep "$HEALTH_SLEEP"
done

echo ""
echo "Seeding test data..."
./scripts/seed.sh "$BASE_URL"

export PLAYWRIGHT_BASE_URL
export PLAYWRIGHT_BROWSERS
export PLAYWRIGHT_HEADLESS
export PLAYWRIGHT_WORKERS
export PWDEBUG
export PLAYWRIGHT_OUTPUT_DIR
export PLAYWRIGHT_REPORT_DIR

mkdir -p test-results playwright-report

project_args=()
IFS=',' read -r -a browsers <<< "$PLAYWRIGHT_BROWSERS"
for browser in "${browsers[@]}"; do
  trimmed="$(echo "$browser" | xargs)"
  if [[ -n "$trimmed" ]]; then
    project_args+=("--project=${trimmed}")
  fi
done

echo ""
echo "Running Playwright (projects: ${PLAYWRIGHT_BROWSERS}, workers: ${PLAYWRIGHT_WORKERS:-auto})..."
podman compose --profile e2e run --rm playwright \
  /opt/playwright/node_modules/.bin/playwright test \
  "${project_args[@]}" \
  "$@"
