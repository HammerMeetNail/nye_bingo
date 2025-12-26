#!/bin/bash
# Run all tests in a container matching the production environment
#
# Usage:
#   ./scripts/test.sh               # Run all tests
#   ./scripts/test.sh --coverage    # Run with coverage report (Go only)
#   ./scripts/test.sh --go          # Run Go tests only
#   ./scripts/test.sh --js          # Run JS tests only

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

COVERAGE=""
RUN_GO=""
RUN_JS=""

for arg in "$@"; do
    case "$arg" in
        --coverage)
            COVERAGE="1"
            ;;
        --go)
            RUN_GO="1"
            ;;
        --js)
            RUN_JS="1"
            ;;
        *)
            echo -e "${RED}Unknown option: ${arg}${NC}"
            exit 1
            ;;
    esac
done

if [[ -z "$RUN_GO" && -z "$RUN_JS" ]]; then
    RUN_GO="1"
    RUN_JS="1"
fi

echo -e "${BLUE}Year of Bingo Test Suite${NC}"
echo "================================"
echo ""

# Build test container
echo -e "${BLUE}Building test container...${NC}"
podman build -f "${PROJECT_DIR}/Containerfile.test" -t year_of_bingo_test "${PROJECT_DIR}" >/dev/null 2>&1

# Run tests
echo -e "${BLUE}Running tests...${NC}"
echo ""

podman run --rm \
    -v "${PROJECT_DIR}:/app:ro" \
    -w /app \
    year_of_bingo_test \
    sh -c '
        set -e
        if [ -n "'"$RUN_GO"'" ]; then
            if [ -n "'"$COVERAGE"'" ]; then
                echo "=== Go Tests (with coverage) ==="
                go test -cover ./...
            else
                echo "=== Go Tests ==="
                go test ./...
            fi
            echo ""
        fi
        if [ -n "'"$RUN_JS"'" ]; then
            echo "=== JavaScript Tests ==="
            node web/static/js/tests/runner.js
        fi
    '

EXIT_CODE=$?

echo ""
if [[ $EXIT_CODE -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
else
    echo -e "${RED}Some tests failed.${NC}"
fi

exit $EXIT_CODE
