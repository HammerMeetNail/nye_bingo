#!/bin/bash
# Clean up test accounts via the API
# Usage: ./scripts/cleanup.sh [base_url]
#
# This script logs into each test account and deletes all their data
# by removing cards, friends, etc. Since there's no delete account API,
# the accounts will remain but be empty.
#
# Note: For a complete cleanup, you may need to restart the containers
# to clear Redis session data, or just recreate the database:
#   podman compose down -v && podman compose up

set -e

BASE_URL="${1:-http://localhost:8080}"
COOKIE_JAR=$(mktemp)
trap "rm -f $COOKIE_JAR" EXIT

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1" >&2; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1" >&2; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

# Get CSRF token
get_csrf() {
    curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$BASE_URL/api/csrf" | jq -r '.token'
}

# Login as user
login_user() {
    local email="$1"
    local password="$2"
    local csrf=$(get_csrf)

    local response=$(curl -s -w "\n%{http_code}" -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X POST "$BASE_URL/api/auth/login" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $csrf" \
        -d "{\"email\":\"$email\",\"password\":\"$password\"}")

    local http_code=$(echo "$response" | tail -1)

    if [ "$http_code" = "200" ]; then
        return 0
    else
        return 1
    fi
}

# Logout
logout_user() {
    local csrf=$(get_csrf)
    curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X POST "$BASE_URL/api/auth/logout" \
        -H "X-CSRF-Token: $csrf" > /dev/null
}

# Remove all friends
remove_all_friends() {
    local csrf=$(get_csrf)

    # Get all friendships (friends + sent requests)
    local friends=$(curl -s -b "$COOKIE_JAR" "$BASE_URL/api/friends")

    # Remove accepted friends
    echo "$friends" | jq -r '.friends[]?.id // empty' | while read -r id; do
        if [ -n "$id" ]; then
            curl -s -b "$COOKIE_JAR" -X DELETE "$BASE_URL/api/friends/$id" \
                -H "X-CSRF-Token: $(get_csrf)" > /dev/null
        fi
    done

    # Cancel sent requests
    echo "$friends" | jq -r '.sent[]?.id // empty' | while read -r id; do
        if [ -n "$id" ]; then
            curl -s -b "$COOKIE_JAR" -X DELETE "$BASE_URL/api/friends/$id/cancel" \
                -H "X-CSRF-Token: $(get_csrf)" > /dev/null
        fi
    done

    # Reject received requests
    echo "$friends" | jq -r '.requests[]?.id // empty' | while read -r id; do
        if [ -n "$id" ]; then
            curl -s -b "$COOKIE_JAR" -X PUT "$BASE_URL/api/friends/$id/reject" \
                -H "X-CSRF-Token: $(get_csrf)" > /dev/null
        fi
    done
}

# Clean up user data (friends, etc.)
cleanup_user() {
    local email="$1"
    local password="$2"

    if login_user "$email" "$password"; then
        log_info "Cleaning up: $email"
        remove_all_friends
        logout_user
        return 0
    else
        log_warn "Could not login as $email (may not exist)"
        return 1
    fi
}

# Main cleanup logic
echo "=========================================="
echo "  NYE Bingo - Cleanup Script"
echo "=========================================="
echo ""
log_info "Using base URL: $BASE_URL"
echo ""

# Check if server is running
if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
    log_error "Server not reachable at $BASE_URL"
    log_error "Make sure the application is running: podman compose up"
    exit 1
fi

# Test accounts to clean up
TEST_ACCOUNTS=(
    "alice@test.com:Password1"
    "bob@test.com:Password1"
    "carol@test.com:Password1"
)

for account in "${TEST_ACCOUNTS[@]}"; do
    email="${account%%:*}"
    password="${account##*:}"
    cleanup_user "$email" "$password" || true
done

echo ""
echo "=========================================="
echo "  Cleanup Complete!"
echo "=========================================="
echo ""
echo "Note: User accounts still exist but their friends/reactions are removed."
echo "Cards cannot be deleted via the API."
echo ""
echo "For a complete reset, recreate the database:"
echo "  podman compose down -v && podman compose up"
echo ""
