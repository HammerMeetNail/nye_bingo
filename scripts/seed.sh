#!/bin/bash
# Seed the application with test data via the API
# Usage: ./scripts/seed.sh [base_url]
#
# Creates:
#   - 3 test users (alice, bob, carol) with password "Password1"
#   - Finalized bingo cards for alice and bob (2025)
#   - Some completed items on each card
#   - Friendships: alice <-> bob (accepted), carol -> alice (pending)
#   - Reactions from bob on alice's completed items

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

# Register a user
register_user() {
    local email="$1"
    local password="$2"
    local display_name="$3"
    local csrf=$(get_csrf)

    local response=$(curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X POST "$BASE_URL/api/auth/register" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $csrf" \
        -d "{\"email\":\"$email\",\"password\":\"$password\",\"display_name\":\"$display_name\"}")

    local user_id=$(echo "$response" | jq -r '.user.id // empty')
    local error=$(echo "$response" | jq -r '.error // empty')

    if [ -n "$user_id" ]; then
        log_info "Created user: $email"
        echo "$user_id"
    elif [[ "$error" == *"already"* ]]; then
        log_warn "User already exists: $email - logging in instead"
        login_user "$email" "$password"
    else
        log_error "Failed to create user $email: $error"
        return 1
    fi
}

# Login as user
login_user() {
    local email="$1"
    local password="$2"
    local csrf=$(get_csrf)

    local response=$(curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X POST "$BASE_URL/api/auth/login" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $csrf" \
        -d "{\"email\":\"$email\",\"password\":\"$password\"}")

    local user_id=$(echo "$response" | jq -r '.user.id // empty')

    if [ -n "$user_id" ]; then
        log_info "Logged in as: $email"
        echo "$user_id"
    else
        log_error "Failed to login as $email: $(echo "$response" | jq -r '.error // empty')"
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

# Create a card
create_card() {
    local year="$1"
    local csrf=$(get_csrf)

    local response=$(curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X POST "$BASE_URL/api/cards" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $csrf" \
        -d "{\"year\":$year}")

    local card_id=$(echo "$response" | jq -r '.card.id // empty')
    local error=$(echo "$response" | jq -r '.error // empty')

    if [ -n "$card_id" ]; then
        log_info "Created card for year $year"
        echo "$card_id"
    elif [[ "$error" == *"already"* ]]; then
        log_warn "Card already exists for $year - fetching existing"
        curl -s -b "$COOKIE_JAR" "$BASE_URL/api/cards" | jq -r ".cards[] | select(.year == $year) | .id"
    else
        log_error "Failed to create card: $error"
        return 1
    fi
}

# Add item to card
add_item() {
    local card_id="$1"
    local content="$2"
    local csrf=$(get_csrf)

    curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X POST "$BASE_URL/api/cards/$card_id/items" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $csrf" \
        -d "{\"content\":\"$content\"}" > /dev/null
}

# Complete item
complete_item() {
    local card_id="$1"
    local position="$2"
    local notes="$3"
    local csrf=$(get_csrf)

    local body="{}"
    if [ -n "$notes" ]; then
        body="{\"notes\":\"$notes\"}"
    fi

    curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X PUT "$BASE_URL/api/cards/$card_id/items/$position/complete" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $csrf" \
        -d "$body" > /dev/null
}

# Finalize card
finalize_card() {
    local card_id="$1"
    local csrf=$(get_csrf)

    curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X POST "$BASE_URL/api/cards/$card_id/finalize" \
        -H "X-CSRF-Token: $csrf" > /dev/null

    log_info "Finalized card"
}

# Send friend request
send_friend_request() {
    local friend_id="$1"
    local csrf=$(get_csrf)

    curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X POST "$BASE_URL/api/friends/request" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $csrf" \
        -d "{\"friend_id\":\"$friend_id\"}" > /dev/null

    log_info "Sent friend request"
}

# Accept friend request
accept_friend_request() {
    local friendship_id="$1"
    local csrf=$(get_csrf)

    curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X PUT "$BASE_URL/api/friends/$friendship_id/accept" \
        -H "X-CSRF-Token: $csrf" > /dev/null

    log_info "Accepted friend request"
}

# Get pending friend requests
get_pending_requests() {
    curl -s -b "$COOKIE_JAR" "$BASE_URL/api/friends" | jq -r '.requests[0].id // empty'
}

# React to item
react_to_item() {
    local item_id="$1"
    local emoji="$2"
    local csrf=$(get_csrf)

    curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
        -X POST "$BASE_URL/api/items/$item_id/react" \
        -H "Content-Type: application/json" \
        -H "X-CSRF-Token: $csrf" \
        -d "{\"emoji\":\"$emoji\"}" > /dev/null
}

# Get friend's card and items
get_friend_card_items() {
    local friendship_id="$1"
    curl -s -b "$COOKIE_JAR" "$BASE_URL/api/friends/$friendship_id/card" | jq -r '.card.items[] | select(.is_completed == true) | .id'
}

# Main seeding logic
echo "=========================================="
echo "  NYE Bingo - Seed Script"
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

# Alice's goals
ALICE_GOALS=(
    "Run a 5K"
    "Read 12 books"
    "Learn to cook 5 new recipes"
    "Start a meditation practice"
    "Visit a new country"
    "Save \$1000 emergency fund"
    "Learn basic guitar chords"
    "Declutter closet"
    "Take a photography class"
    "Volunteer monthly"
    "Learn a new language basics"
    "Do a digital detox weekend"
    "Start a journal"
    "Try rock climbing"
    "Read one non-fiction book per month"
    "Meal prep for a month"
    "Take a solo trip"
    "Learn to make sourdough"
    "Complete a 30-day fitness challenge"
    "Organize digital photos"
    "Learn basic car maintenance"
    "Host a dinner party"
    "Complete an online course"
    "Write letters to old friends"
)

# Bob's goals
BOB_GOALS=(
    "Build a piece of furniture"
    "Run a marathon"
    "Learn woodworking basics"
    "Read 24 books"
    "Visit all state parks in region"
    "Max out 401k contribution"
    "Learn to play piano"
    "Renovate bathroom"
    "Take a welding class"
    "Coach little league"
    "Learn Spanish"
    "Complete a home improvement project monthly"
    "Start a workshop YouTube channel"
    "Go camping once a month"
    "Read about home electrical"
    "Build a workbench"
    "Take a road trip"
    "Brew beer at home"
    "Train for Tough Mudder"
    "Organize the garage"
    "Restore an old tool"
    "Host a BBQ competition"
    "Get a contractor license"
    "Teach kids to build something"
)

# Create users
log_info "Creating users..."
ALICE_ID=$(register_user "alice@test.com" "Password1" "Alice Anderson")
logout_user

BOB_ID=$(register_user "bob@test.com" "Password1" "Bob Builder")
logout_user

CAROL_ID=$(register_user "carol@test.com" "Password1" "Carol Chen")
logout_user

echo ""

# Create Alice's card
log_info "Creating Alice's card..."
login_user "alice@test.com" "Password1" > /dev/null
ALICE_CARD=$(create_card 2025)

if [ -n "$ALICE_CARD" ]; then
    log_info "Adding items to Alice's card..."
    for goal in "${ALICE_GOALS[@]}"; do
        add_item "$ALICE_CARD" "$goal"
    done

    finalize_card "$ALICE_CARD"

    # Complete some items (positions 0, 2, 5, 7, 11, 18)
    log_info "Completing some of Alice's items..."
    complete_item "$ALICE_CARD" 0 "Completed the Turkey Trot!"
    complete_item "$ALICE_CARD" 2 "Made pad thai, ramen, and more"
    complete_item "$ALICE_CARD" 5 ""
    complete_item "$ALICE_CARD" 7 "Donated 3 bags!"
    complete_item "$ALICE_CARD" 11 "So refreshing!"
    complete_item "$ALICE_CARD" 18 "Finally got a good rise!"
fi
logout_user

echo ""

# Create Bob's card
log_info "Creating Bob's card..."
login_user "bob@test.com" "Password1" > /dev/null
BOB_CARD=$(create_card 2025)

if [ -n "$BOB_CARD" ]; then
    log_info "Adding items to Bob's card..."
    for goal in "${BOB_GOALS[@]}"; do
        add_item "$BOB_CARD" "$goal"
    done

    finalize_card "$BOB_CARD"

    # Complete some items (positions 0, 2, 7, 11, 16, 20)
    log_info "Completing some of Bob's items..."
    complete_item "$BOB_CARD" 0 "Made a bookshelf from scratch"
    complete_item "$BOB_CARD" 2 ""
    complete_item "$BOB_CARD" 7 "New tiles look great!"
    complete_item "$BOB_CARD" 11 ""
    complete_item "$BOB_CARD" 16 "Heavy duty!"
    complete_item "$BOB_CARD" 20 "Finally found the floor!"
fi

# Bob sends friend request to Alice
log_info "Bob sending friend request to Alice..."
send_friend_request "$ALICE_ID"
logout_user

echo ""

# Alice accepts Bob's request
log_info "Alice accepting Bob's friend request..."
login_user "alice@test.com" "Password1" > /dev/null
FRIENDSHIP_ID=$(get_pending_requests)
if [ -n "$FRIENDSHIP_ID" ]; then
    accept_friend_request "$FRIENDSHIP_ID"
fi
logout_user

echo ""

# Carol sends friend request to Alice (will remain pending)
log_info "Carol sending friend request to Alice..."
login_user "carol@test.com" "Password1" > /dev/null
send_friend_request "$ALICE_ID"
logout_user

echo ""

# Bob adds reactions to Alice's completed items
log_info "Bob adding reactions to Alice's items..."
login_user "bob@test.com" "Password1" > /dev/null

# Get friendship ID for Alice
ALICE_FRIENDSHIP=$(curl -s -b "$COOKIE_JAR" "$BASE_URL/api/friends" | jq -r '.friends[0].id')

if [ -n "$ALICE_FRIENDSHIP" ]; then
    EMOJIS=("🎉" "👏" "🔥" "❤️" "⭐")
    COMPLETED_ITEMS=$(get_friend_card_items "$ALICE_FRIENDSHIP")

    i=0
    for item_id in $COMPLETED_ITEMS; do
        emoji="${EMOJIS[$((i % 5))]}"
        react_to_item "$item_id" "$emoji"
        ((i++))
    done
    log_info "Added reactions to ${i} items"
fi
logout_user

echo ""
echo "=========================================="
echo "  Seed Complete!"
echo "=========================================="
echo ""
echo "Test accounts created:"
echo "  - alice@test.com / Password1"
echo "    - Has 2025 card with 6 completed items"
echo "    - Friends with Bob"
echo "    - Pending request from Carol"
echo ""
echo "  - bob@test.com / Password1"
echo "    - Has 2025 card with 6 completed items"
echo "    - Friends with Alice"
echo "    - Has reacted to Alice's completed items"
echo ""
echo "  - carol@test.com / Password1"
echo "    - No card yet"
echo "    - Pending friend request to Alice"
echo ""
