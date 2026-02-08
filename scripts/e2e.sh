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
# Normalize to avoid double-slashes when building URLs.
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

# Billing E2E defaults:
# - We do not rely on the Stripe CLI listener.
# - A local mock Stripe API runs as a container and we post signed webhook events directly from Playwright.
APP_BASE_URL="${APP_BASE_URL:-$PLAYWRIGHT_BASE_URL}"
BILLING_ENABLED="${BILLING_ENABLED:-true}"
# Avoid picking up a developer's local Stripe env vars; E2E uses a deterministic mock setup.
STRIPE_SECRET_KEY="${E2E_STRIPE_SECRET_KEY:-sk_test_mock}"
STRIPE_WEBHOOK_SECRET="${E2E_STRIPE_WEBHOOK_SECRET:-whsec_test}"
STRIPE_API_BASE_URL="${STRIPE_API_BASE_URL:-http://host.containers.internal:12111}"
STRIPE_MOCK_PUBLIC_BASE_URL="${STRIPE_MOCK_PUBLIC_BASE_URL:-http://host.containers.internal:12111}"
STRIPE_PREMIUM_PRICE_MONTHLY="${STRIPE_PREMIUM_PRICE_MONTHLY:-price_premium_monthly}"
STRIPE_PREMIUM_PRICE_YEARLY="${STRIPE_PREMIUM_PRICE_YEARLY:-price_premium_yearly}"
STRIPE_PREMIUM_PRICE_LIFETIME="${STRIPE_PREMIUM_PRICE_LIFETIME:-price_premium_lifetime}"
STRIPE_TIP_PRICE_5="${STRIPE_TIP_PRICE_5:-price_tip_5}"
STRIPE_TIP_PRICE_10="${STRIPE_TIP_PRICE_10:-price_tip_10}"
STRIPE_TIP_PRICE_20="${STRIPE_TIP_PRICE_20:-price_tip_20}"

OIDC_CLIENT_ID="${OIDC_CLIENT_ID:-}"
OIDC_CLIENT_SECRET="${OIDC_CLIENT_SECRET:-}"
GOOGLE_OAUTH_CLIENT_ID="${GOOGLE_OAUTH_CLIENT_ID:-}"
GOOGLE_OAUTH_CLIENT_SECRET="${GOOGLE_OAUTH_CLIENT_SECRET:-}"

if [[ -z "$OIDC_CLIENT_ID" && -z "$GOOGLE_OAUTH_CLIENT_ID" ]]; then
  OIDC_CLIENT_ID="oidc-test"
  GOOGLE_OAUTH_CLIENT_ID="$OIDC_CLIENT_ID"
elif [[ -z "$OIDC_CLIENT_ID" ]]; then
  OIDC_CLIENT_ID="$GOOGLE_OAUTH_CLIENT_ID"
elif [[ -z "$GOOGLE_OAUTH_CLIENT_ID" ]]; then
  GOOGLE_OAUTH_CLIENT_ID="$OIDC_CLIENT_ID"
elif [[ "$OIDC_CLIENT_ID" != "$GOOGLE_OAUTH_CLIENT_ID" ]]; then
  echo "OIDC_CLIENT_ID and GOOGLE_OAUTH_CLIENT_ID must match for E2E." >&2
  exit 1
fi

if [[ -z "$OIDC_CLIENT_SECRET" && -z "$GOOGLE_OAUTH_CLIENT_SECRET" ]]; then
  OIDC_CLIENT_SECRET="oidc-secret"
  GOOGLE_OAUTH_CLIENT_SECRET="$OIDC_CLIENT_SECRET"
elif [[ -z "$OIDC_CLIENT_SECRET" ]]; then
  OIDC_CLIENT_SECRET="$GOOGLE_OAUTH_CLIENT_SECRET"
elif [[ -z "$GOOGLE_OAUTH_CLIENT_SECRET" ]]; then
  GOOGLE_OAUTH_CLIENT_SECRET="$OIDC_CLIENT_SECRET"
elif [[ "$OIDC_CLIENT_SECRET" != "$GOOGLE_OAUTH_CLIENT_SECRET" ]]; then
  echo "OIDC_CLIENT_SECRET and GOOGLE_OAUTH_CLIENT_SECRET must match for E2E." >&2
  exit 1
fi

GOOGLE_OAUTH_REDIRECT_URL="${PLAYWRIGHT_BASE_URL}/api/auth/google/callback"
GOOGLE_OIDC_ISSUER_URL="${GOOGLE_OIDC_ISSUER_URL:-http://oidc:5555}"
GOOGLE_OIDC_SCOPES="${GOOGLE_OIDC_SCOPES:-openid,email,profile}"
OAUTH_ALLOWED_PROVIDERS="${OAUTH_ALLOWED_PROVIDERS:-google}"
OIDC_ISSUER_URL="${OIDC_ISSUER_URL:-http://oidc:5555}"
OIDC_REDIRECT_URI="${GOOGLE_OAUTH_REDIRECT_URL}"
OIDC_BASE_URL="${OIDC_BASE_URL:-http://oidc:5555}"

# podman-compose reads .env from the repo by default; isolate E2E from local billing secrets/config.
E2E_ENV_FILE="$(mktemp -t yearofbingo-e2e.XXXXXX.env)"
trap 'rm -f "$E2E_ENV_FILE"' EXIT
COMPOSE_E2E_ARGS=(--env-file "$E2E_ENV_FILE" --profile e2e)

cat >"$E2E_ENV_FILE" <<EOF
E2E_DEBUG_STRIPE_SIG=1
AI_STUB=$AI_STUB
REMINDERS_POLL_INTERVAL=$REMINDERS_POLL_INTERVAL

GOOGLE_OAUTH_ENABLED=$GOOGLE_OAUTH_ENABLED
GOOGLE_OAUTH_CLIENT_ID=$GOOGLE_OAUTH_CLIENT_ID
GOOGLE_OAUTH_CLIENT_SECRET=$GOOGLE_OAUTH_CLIENT_SECRET
GOOGLE_OAUTH_REDIRECT_URL=$GOOGLE_OAUTH_REDIRECT_URL
GOOGLE_OIDC_ISSUER_URL=$GOOGLE_OIDC_ISSUER_URL
GOOGLE_OIDC_SCOPES=$GOOGLE_OIDC_SCOPES
OAUTH_ALLOWED_PROVIDERS=$OAUTH_ALLOWED_PROVIDERS

OIDC_ISSUER_URL=$OIDC_ISSUER_URL
OIDC_CLIENT_ID=$OIDC_CLIENT_ID
OIDC_CLIENT_SECRET=$OIDC_CLIENT_SECRET
OIDC_REDIRECT_URI=$OIDC_REDIRECT_URI
OIDC_BASE_URL=$OIDC_BASE_URL

APP_BASE_URL=$APP_BASE_URL
BILLING_ENABLED=$BILLING_ENABLED
STRIPE_SECRET_KEY=$STRIPE_SECRET_KEY
STRIPE_WEBHOOK_SECRET=$STRIPE_WEBHOOK_SECRET
STRIPE_API_BASE_URL=$STRIPE_API_BASE_URL
STRIPE_MOCK_PUBLIC_BASE_URL=$STRIPE_MOCK_PUBLIC_BASE_URL
STRIPE_PREMIUM_PRICE_MONTHLY=$STRIPE_PREMIUM_PRICE_MONTHLY
STRIPE_PREMIUM_PRICE_YEARLY=$STRIPE_PREMIUM_PRICE_YEARLY
STRIPE_PREMIUM_PRICE_LIFETIME=$STRIPE_PREMIUM_PRICE_LIFETIME
STRIPE_TIP_PRICE_5=$STRIPE_TIP_PRICE_5
STRIPE_TIP_PRICE_10=$STRIPE_TIP_PRICE_10
STRIPE_TIP_PRICE_20=$STRIPE_TIP_PRICE_20
EOF

cd "$PROJECT_DIR"

PROJECT_NAME="${COMPOSE_PROJECT_NAME:-$(basename "$PROJECT_DIR")}"
NETWORK_NAME="${PROJECT_NAME}_default"

echo "================================"
echo "Year of Bingo E2E Runner"
echo "================================"
echo ""
echo "Resetting local stack (destructive: volumes will be removed)."
"${PROJECT_DIR}/scripts/podman-compose.sh" --env-file "$E2E_ENV_FILE" down -v

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
export APP_BASE_URL
export BILLING_ENABLED
export STRIPE_SECRET_KEY
export STRIPE_WEBHOOK_SECRET
export STRIPE_API_BASE_URL
export STRIPE_MOCK_PUBLIC_BASE_URL
export STRIPE_PREMIUM_PRICE_MONTHLY
export STRIPE_PREMIUM_PRICE_YEARLY
export STRIPE_PREMIUM_PRICE_LIFETIME
export STRIPE_TIP_PRICE_5
export STRIPE_TIP_PRICE_10
export STRIPE_TIP_PRICE_20
"${PROJECT_DIR}/scripts/podman-compose.sh" "${COMPOSE_E2E_ARGS[@]}" up -d --build oidc stripe-mock

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
echo "Waiting for Stripe mock..."
for ((i=1; i<=HEALTH_ATTEMPTS; i++)); do
  if curl -fsS "http://localhost:12111/health" >/dev/null 2>&1; then
    echo "Stripe mock is healthy."
    break
  fi
  if [[ "$i" -eq "$HEALTH_ATTEMPTS" ]]; then
    echo "Stripe mock did not become healthy in time."
    exit 1
  fi
  sleep "$HEALTH_SLEEP"
done

STRIPE_MOCK_CONTAINER="${PROJECT_NAME}_stripe-mock_1"
STRIPE_MOCK_IP="$(podman inspect -f "{{(index .NetworkSettings.Networks \"${NETWORK_NAME}\").IPAddress}}" "$STRIPE_MOCK_CONTAINER" 2>/dev/null || true)"
if [[ -n "$STRIPE_MOCK_IP" ]]; then
  STRIPE_API_BASE_URL="http://${STRIPE_MOCK_IP}:12111"
  STRIPE_MOCK_PUBLIC_BASE_URL="http://${STRIPE_MOCK_IP}:12111"
  sed -i.bak "s|^STRIPE_API_BASE_URL=.*|STRIPE_API_BASE_URL=${STRIPE_API_BASE_URL}|" "$E2E_ENV_FILE"
  sed -i.bak "s|^STRIPE_MOCK_PUBLIC_BASE_URL=.*|STRIPE_MOCK_PUBLIC_BASE_URL=${STRIPE_MOCK_PUBLIC_BASE_URL}|" "$E2E_ENV_FILE"
  rm -f "${E2E_ENV_FILE}.bak"
else
  echo "Warning: unable to resolve stripe-mock container IP; continuing with STRIPE_API_BASE_URL=$STRIPE_API_BASE_URL" >&2
fi

echo ""
echo "Starting containers..."
"${PROJECT_DIR}/scripts/podman-compose.sh" --env-file "$E2E_ENV_FILE" up -d --build app postgres redis mailpit

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
echo "Building Playwright container..."
"${PROJECT_DIR}/scripts/podman-compose.sh" "${COMPOSE_E2E_ARGS[@]}" build playwright

echo ""
echo "Running Playwright (projects: ${PLAYWRIGHT_BROWSERS}, workers: ${PLAYWRIGHT_WORKERS:-auto})..."

podman run --rm \
  --pull=never \
  --net "$NETWORK_NAME" \
  -e PLAYWRIGHT_BASE_URL="$PLAYWRIGHT_BASE_URL" \
  -e PLAYWRIGHT_BROWSERS="$PLAYWRIGHT_BROWSERS" \
  -e PLAYWRIGHT_HEADLESS="$PLAYWRIGHT_HEADLESS" \
  -e PLAYWRIGHT_WORKERS="${PLAYWRIGHT_WORKERS:-}" \
  -e PWDEBUG="${PWDEBUG:-}" \
  -e PLAYWRIGHT_OUTPUT_DIR="$PLAYWRIGHT_OUTPUT_DIR" \
  -e PLAYWRIGHT_REPORT_DIR="$PLAYWRIGHT_REPORT_DIR" \
  -e OIDC_BASE_URL="$OIDC_BASE_URL" \
  -e STRIPE_WEBHOOK_SECRET="$STRIPE_WEBHOOK_SECRET" \
  -e STRIPE_MOCK_BASE_URL="$STRIPE_MOCK_PUBLIC_BASE_URL" \
  -e STRIPE_PREMIUM_PRICE_MONTHLY="$STRIPE_PREMIUM_PRICE_MONTHLY" \
  -e STRIPE_PREMIUM_PRICE_YEARLY="$STRIPE_PREMIUM_PRICE_YEARLY" \
  -e STRIPE_PREMIUM_PRICE_LIFETIME="$STRIPE_PREMIUM_PRICE_LIFETIME" \
  -e STRIPE_TIP_PRICE_5="$STRIPE_TIP_PRICE_5" \
  -e STRIPE_TIP_PRICE_10="$STRIPE_TIP_PRICE_10" \
  -e STRIPE_TIP_PRICE_20="$STRIPE_TIP_PRICE_20" \
  -v "${PROJECT_DIR}:/app:ro" \
  -v "${PROJECT_DIR}/test-results:/test-results" \
  -v "${PROJECT_DIR}/playwright-report:/playwright-report" \
  -w /app \
  --shm-size 1gb \
  localhost/yearofbingo_playwright:latest \
  /opt/playwright/node_modules/.bin/playwright test \
  "${project_args[@]}" \
  "$@"
