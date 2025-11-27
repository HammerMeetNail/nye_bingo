#!/bin/bash
# Run all tests in a container matching the production environment
#
# Usage:
#   ./scripts/test.sh            # Run all tests
#   ./scripts/test.sh --coverage # Run with coverage report

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

COVERAGE=""
if [[ "$1" == "--coverage" ]]; then
    COVERAGE="1"
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

if [[ -n "$COVERAGE" ]]; then
    podman run --rm \
        -v "${PROJECT_DIR}:/app:ro" \
        -w /app \
        year_of_bingo_test \
        sh -c '
            echo "=== Go Tests (with coverage) ==="
            go test -cover ./...
            echo ""
            echo "=== JavaScript Tests ==="
            node web/static/js/tests/runner.js
        '
else
    podman run --rm \
        -v "${PROJECT_DIR}:/app:ro" \
        -w /app \
        year_of_bingo_test \
        sh -c '
            echo "=== Go Tests ==="
            go test ./...
            echo ""
            echo "=== JavaScript Tests ==="
            node web/static/js/tests/runner.js
        '
fi

EXIT_CODE=$?

echo ""
if [[ $EXIT_CODE -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
else
    echo -e "${RED}Some tests failed.${NC}"
fi

exit $EXIT_CODE
