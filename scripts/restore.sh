#!/bin/bash
# Restore PostgreSQL from Cloudflare R2 backup
# Usage: ./scripts/restore.sh [backup_filename]
#        ./scripts/restore.sh --list        # List available backups
#        ./scripts/restore.sh --latest      # Restore most recent backup
#
# Required environment variables (or in /opt/yearofbingo/.env):
#   DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
#   BACKUP_ENCRYPTION_KEY - GPG passphrase used when creating backup
#
# WARNING: This will DROP and recreate the database!

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

# Database connection
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-bingo}"
DB_NAME="${DB_NAME:-yearofbingo}"

# Validation
if [[ -z "${DB_PASSWORD:-}" ]]; then
    echo "ERROR: DB_PASSWORD is required"
    exit 1
fi

if [[ -z "${BACKUP_ENCRYPTION_KEY:-}" ]]; then
    echo "ERROR: BACKUP_ENCRYPTION_KEY is required"
    exit 1
fi

if ! command -v rclone &> /dev/null; then
    echo "ERROR: rclone is not installed"
    exit 1
fi

# List backups
list_backups() {
    echo "Available backups in r2:${R2_BUCKET}/"
    echo "----------------------------------------"
    rclone ls "r2:${R2_BUCKET}/" | sort -k2 | tail -20
    echo "----------------------------------------"
    echo "Use: ./restore.sh <filename> to restore"
    echo "Use: ./restore.sh --latest to restore most recent"
}

# Get latest backup filename
get_latest() {
    rclone ls "r2:${R2_BUCKET}/" | sort -k2 | tail -1 | awk '{print $2}'
}

# Handle arguments
if [[ "${1:-}" == "--list" ]] || [[ "${1:-}" == "-l" ]]; then
    list_backups
    exit 0
fi

if [[ "${1:-}" == "--latest" ]]; then
    BACKUP_FILE=$(get_latest)
    if [[ -z "$BACKUP_FILE" ]]; then
        echo "ERROR: No backups found in r2:${R2_BUCKET}/"
        exit 1
    fi
    echo "Latest backup: $BACKUP_FILE"
elif [[ -n "${1:-}" ]]; then
    BACKUP_FILE="$1"
else
    list_backups
    exit 1
fi

mkdir -p "$BACKUP_DIR"

# Detect target container for warning
if [[ "$DB_HOST" == "localhost" ]] || [[ "$DB_HOST" == "127.0.0.1" ]]; then
    TARGET_CONTAINER=$(podman ps --format '{{.Names}}' 2>/dev/null | grep -E 'postgres' | head -1)
else
    TARGET_CONTAINER="remote: ${DB_HOST}:${DB_PORT}"
fi

echo ""
echo "=========================================="
echo "!!! DANGER: PRODUCTION DATABASE RESTORE !!!"
echo "=========================================="
echo ""
echo "This will PERMANENTLY REPLACE ALL DATA in:"
echo "  Database: ${DB_NAME}"
echo "  Target:   ${TARGET_CONTAINER:-direct connection}"
echo "  Backup:   ${BACKUP_FILE}"
echo ""
echo "=========================================="
echo ""
read -p "Type 'yes-restore-production' to continue: " CONFIRM

if [[ "$CONFIRM" != "yes-restore-production" ]]; then
    echo "Aborted (must type exactly: yes-restore-production)"
    exit 1
fi

# Double confirmation
echo ""
read -p "Are you ABSOLUTELY SURE? Type the database name '${DB_NAME}' to confirm: " CONFIRM_DB

if [[ "$CONFIRM_DB" != "$DB_NAME" ]]; then
    echo "Aborted (database name did not match)"
    exit 1
fi

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Downloading backup..."
rclone copy "r2:${R2_BUCKET}/${BACKUP_FILE}" "${BACKUP_DIR}/" --progress

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Decrypting and decompressing..."

# For containerized PostgreSQL
if [[ "$DB_HOST" == "localhost" ]] || [[ "$DB_HOST" == "127.0.0.1" ]]; then
    POSTGRES_CONTAINER=$(podman ps --format '{{.Names}}' 2>/dev/null | grep -E 'postgres' | head -1)
    APP_CONTAINER=$(podman ps --format '{{.Names}}' 2>/dev/null | grep -E 'app' | head -1)

    if [[ -n "$POSTGRES_CONTAINER" ]]; then
        echo "Found PostgreSQL container: $POSTGRES_CONTAINER"

        if [[ -n "$APP_CONTAINER" ]]; then
            echo "Stopping app container: $APP_CONTAINER"
            podman stop "$APP_CONTAINER" 2>/dev/null || true
        fi

        echo "Restoring via podman exec..."
        gpg --decrypt --batch --pinentry-mode loopback --passphrase-fd 3 3<<<"$BACKUP_ENCRYPTION_KEY" "${BACKUP_DIR}/${BACKUP_FILE}" \
            | gunzip \
            | podman exec -i -e PGPASSWORD="$DB_PASSWORD" "$POSTGRES_CONTAINER" \
                psql -U "$DB_USER" -d "$DB_NAME"

        if [[ -n "$APP_CONTAINER" ]]; then
            echo "Starting app container: $APP_CONTAINER"
            podman start "$APP_CONTAINER" 2>/dev/null || true
        fi
    else
        gpg --decrypt --batch --pinentry-mode loopback --passphrase-fd 3 3<<<"$BACKUP_ENCRYPTION_KEY" "${BACKUP_DIR}/${BACKUP_FILE}" \
            | gunzip \
            | PGPASSWORD="$DB_PASSWORD" psql \
                -h "$DB_HOST" \
                -p "$DB_PORT" \
                -U "$DB_USER" \
                -d "$DB_NAME"
    fi
else
    gpg --decrypt --batch --pinentry-mode loopback --passphrase-fd 3 3<<<"$BACKUP_ENCRYPTION_KEY" "${BACKUP_DIR}/${BACKUP_FILE}" \
        | gunzip \
        | PGPASSWORD="$DB_PASSWORD" psql \
            -h "$DB_HOST" \
            -p "$DB_PORT" \
            -U "$DB_USER" \
            -d "$DB_NAME"
fi

# Clean up
rm -f "${BACKUP_DIR}/${BACKUP_FILE}"

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Restore completed successfully"
echo ""
echo "Verify the restore by checking the app or running:"
echo "  podman exec \$(podman ps --format '{{.Names}}' | grep postgres) psql -U $DB_USER -d $DB_NAME -c 'SELECT COUNT(*) FROM users;'"
