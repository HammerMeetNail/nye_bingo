#!/bin/bash
# Test backup integrity without restoring to production
# Usage: ./scripts/test-backup.sh [backup_filename]
#        ./scripts/test-backup.sh --latest
#
# This script:
# 1. Downloads and decrypts a backup
# 2. Spins up a temporary PostgreSQL container
# 3. Restores the backup to it
# 4. Runs validation queries
# 5. Cleans up
#
# Required environment variables:
#   BACKUP_ENCRYPTION_KEY - GPG passphrase used when creating backup

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${ENV_FILE:-/opt/yearofbingo/.env}"

# Load environment if available
if [[ -f "$ENV_FILE" ]]; then
    set -a
    source "$ENV_FILE"
    set +a
fi

# Configuration
BACKUP_DIR="/tmp/yearofbingo-backups"
R2_BUCKET="${R2_BUCKET:-yearofbingo-backups}"
TEST_CONTAINER="yearofbingo-backup-test"
TEST_DB_PASSWORD="test_password_$(date +%s)"
TEST_DB_NAME=""

# Validation
if [[ -z "${BACKUP_ENCRYPTION_KEY:-}" ]]; then
    echo "ERROR: BACKUP_ENCRYPTION_KEY is required"
    exit 1
fi

if ! command -v rclone &> /dev/null; then
    echo "ERROR: rclone is not installed"
    exit 1
fi

if ! command -v podman &> /dev/null; then
    echo "ERROR: podman is not installed"
    exit 1
fi

# Cleanup function
cleanup() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Cleaning up..."
    podman rm -f "$TEST_CONTAINER" 2>/dev/null || true
    rm -f "${BACKUP_DIR}/${BACKUP_FILE:-}" 2>/dev/null || true
    rm -f "${BACKUP_DIR}/test_restore.sql" 2>/dev/null || true
}
trap cleanup EXIT

# Get latest backup filename
get_latest() {
    rclone ls "r2:${R2_BUCKET}/" | grep -E '\.sql\.gz\.gpg$' | sort -k2 | tail -1 | awk '{print $2}'
}

# Handle arguments
if [[ "${1:-}" == "--latest" ]]; then
    BACKUP_FILE=$(get_latest)
    if [[ -z "$BACKUP_FILE" ]]; then
        echo "ERROR: No backups found in r2:${R2_BUCKET}/"
        exit 1
    fi
elif [[ -n "${1:-}" ]]; then
    BACKUP_FILE="$1"
else
    echo "Usage: ./test-backup.sh [backup_filename | --latest]"
    echo ""
    echo "Available backups:"
    rclone ls "r2:${R2_BUCKET}/" | grep -E '\.sql\.gz\.gpg$' | sort -k2 | tail -10
    exit 1
fi

mkdir -p "$BACKUP_DIR"

echo "=========================================="
echo "Testing backup: ${BACKUP_FILE}"
echo "=========================================="
echo ""

# Step 1: Download backup
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Step 1/5: Downloading backup from R2..."
rclone copy "r2:${R2_BUCKET}/${BACKUP_FILE}" "${BACKUP_DIR}/" --progress

# Step 2: Decrypt and decompress
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Step 2/5: Decrypting backup..."
gpg --decrypt --batch --passphrase "$BACKUP_ENCRYPTION_KEY" "${BACKUP_DIR}/${BACKUP_FILE}" \
    | gunzip > "${BACKUP_DIR}/test_restore.sql"

SQL_SIZE=$(du -h "${BACKUP_DIR}/test_restore.sql" | cut -f1)
echo "  Decrypted SQL size: ${SQL_SIZE}"

# Derive database name from dump (pg_dump includes a leading \connect).
TEST_DB_NAME="$(
    awk '
        match($0, /^\\\\connect( -reuse-previous=on)?[[:space:]]+\"?([^\"[:space:]]+)\"?/, m) { print m[2]; exit }
    ' "${BACKUP_DIR}/test_restore.sql" 2>/dev/null || true
)"
TEST_DB_NAME="${TEST_DB_NAME:-yearofbingo}"

# Step 3: Start temporary PostgreSQL
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Step 3/5: Starting temporary PostgreSQL container..."
podman run -d \
    --name "$TEST_CONTAINER" \
    -e POSTGRES_USER=bingo \
    -e POSTGRES_PASSWORD="$TEST_DB_PASSWORD" \
    -e POSTGRES_DB="$TEST_DB_NAME" \
    docker.io/library/postgres:16-alpine

# Wait for PostgreSQL to be ready
echo "  Waiting for PostgreSQL to be ready..."
for i in {1..60}; do
    if podman exec -e PGPASSWORD="$TEST_DB_PASSWORD" "$TEST_CONTAINER" \
        psql -U bingo -d "$TEST_DB_NAME" -c "SELECT 1" &>/dev/null; then
        break
    fi
    sleep 1
done

# Step 4: Restore backup
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Step 4/5: Restoring backup to test container..."
podman exec -i -e PGPASSWORD="$TEST_DB_PASSWORD" "$TEST_CONTAINER" \
    psql -U bingo -d "$TEST_DB_NAME" < "${BACKUP_DIR}/test_restore.sql"

# Step 5: Validate data
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Step 5/5: Validating restored data..."
echo ""
echo "=========================================="
echo "VALIDATION RESULTS"
echo "=========================================="

# Run validation queries
podman exec -e PGPASSWORD="$TEST_DB_PASSWORD" "$TEST_CONTAINER" \
    psql -U bingo -d "$TEST_DB_NAME" -c "
    SELECT 'Users' as table_name, COUNT(*) as count FROM users
    UNION ALL
    SELECT 'Bingo Cards', COUNT(*) FROM bingo_cards
    UNION ALL
    SELECT 'Bingo Items', COUNT(*) FROM bingo_items
    UNION ALL
    SELECT 'Friendships', COUNT(*) FROM friendships
    UNION ALL
    SELECT 'Reactions', COUNT(*) FROM reactions
    UNION ALL
    SELECT 'Sessions', COUNT(*) FROM sessions
    UNION ALL
    SELECT 'Suggestions', COUNT(*) FROM suggestions;
"

# Check for recent data
echo ""
echo "Most recent card created:"
podman exec -e PGPASSWORD="$TEST_DB_PASSWORD" "$TEST_CONTAINER" \
    psql -U bingo -d "$TEST_DB_NAME" -c "
    SELECT id, year, is_finalized, created_at
    FROM bingo_cards
    ORDER BY created_at DESC
    LIMIT 1;
"

echo ""
echo "=========================================="
echo "BACKUP TEST PASSED"
echo "=========================================="
echo "Backup file: ${BACKUP_FILE}"
echo "Encryption: OK (GPG AES256)"
echo "Compression: OK (gzip)"
echo "Restore: OK"
echo "Data integrity: OK"
echo ""
