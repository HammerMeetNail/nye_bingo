#!/usr/bin/env bash
# update-prod-os.sh — Apply host OS package updates on the production server.
#
# Steps:
#   1. Preflight checks (local tools, SSH connectivity)
#   2. Snapshot current kernel/OS state
#   3. Run: sudo dnf upgrade --refresh -y
#   4. Determine whether a reboot is needed (new kernel installed)
#   5. Reboot if needed, then wait for the server to come back
#   6. Verify containers and app health
#
# Usage:
#   ./scripts/update-prod-os.sh
#   make update-prod-os
#
# Override SSH key path:
#   SSH_KEY=/path/to/key make update-prod-os

set -uo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
SSH_KEY="${SSH_KEY:-$HOME/.ssh/hetzner_yearofbingo_ci}"
SSH_HOST="ssh.yearofbingo.com"
SSH_USER="deploy"
HEALTH_URL="https://yearofbingo.com/health"

REBOOT_INITIAL_WAIT=30   # seconds to wait after issuing reboot before polling
REBOOT_POLL_INTERVAL=15  # seconds between SSH reconnect attempts
REBOOT_POLL_MAX=24       # max poll attempts (24 × 15s = 6 minutes)

# ---------------------------------------------------------------------------
# Logging helpers
# ---------------------------------------------------------------------------
_ts() { date '+%H:%M:%S'; }

log()  { printf '\033[0;34m[%s]\033[0m %s\n'        "$(_ts)" "$*"; }
ok()   { printf '\033[0;32m[%s] ✓\033[0m %s\n'      "$(_ts)" "$*"; }
warn() { printf '\033[1;33m[%s] !\033[0m %s\n'      "$(_ts)" "$*"; }
die()  { printf '\033[0;31m[%s] ✗\033[0m %s\n'      "$(_ts)" "$*" >&2; exit 1; }
sep()  { printf '\033[0;90m%s\033[0m\n' "────────────────────────────────────────────"; }

# ---------------------------------------------------------------------------
# SSH helper — runs commands on the production server
# ---------------------------------------------------------------------------
prod_ssh() {
  ssh \
    -i "$SSH_KEY" \
    -o ProxyCommand="cloudflared access ssh --hostname ${SSH_HOST}" \
    -o StrictHostKeyChecking=accept-new \
    -o ConnectTimeout=15 \
    -o LogLevel=ERROR \
    "${SSH_USER}@${SSH_HOST}" \
    "$@"
}

# Like prod_ssh but tolerates a failed connection (used in polling loop).
prod_ssh_try() {
  ssh \
    -i "$SSH_KEY" \
    -o ProxyCommand="cloudflared access ssh --hostname ${SSH_HOST}" \
    -o StrictHostKeyChecking=accept-new \
    -o ConnectTimeout=10 \
    -o LogLevel=ERROR \
    "${SSH_USER}@${SSH_HOST}" \
    "$@" 2>/dev/null
}

# ---------------------------------------------------------------------------
# Phase 1 — Preflight
# ---------------------------------------------------------------------------
sep
log "Phase 1 — Preflight checks"

if ! command -v cloudflared &>/dev/null; then
  die "'cloudflared' not found in PATH. Install it and authenticate before running this script."
fi
ok "cloudflared found: $(cloudflared --version 2>&1 | head -1)"

if [[ ! -f "$SSH_KEY" ]]; then
  die "SSH key not found: ${SSH_KEY} (override with SSH_KEY=/path/to/key)"
fi
ok "SSH key: ${SSH_KEY}"

log "Testing SSH connectivity..."
if ! prod_ssh "echo connected" &>/dev/null; then
  die "Could not connect to ${SSH_USER}@${SSH_HOST}. Check that cloudflared access is active."
fi
ok "SSH connection OK"

# ---------------------------------------------------------------------------
# Phase 2 — Pre-update snapshot
# ---------------------------------------------------------------------------
sep
log "Phase 2 — Current server state"

read -r PRE_KERNEL PRE_OS PRE_UPTIME < <(prod_ssh \
  'printf "%s %s %s" "$(uname -r)" "$(rpm -E %fedora)" "$(uptime -p)"')

log "  OS:      Fedora ${PRE_OS}"
log "  Kernel:  ${PRE_KERNEL}"
log "  Uptime:  ${PRE_UPTIME}"

# ---------------------------------------------------------------------------
# Phase 3 — Apply updates
# ---------------------------------------------------------------------------
sep
log "Phase 3 — Running: sudo dnf upgrade --refresh -y"
log "(streaming output from server...)"
echo

# Run upgrade; allow non-zero exit only for "nothing to do" (exit 0 anyway on Fedora dnf)
prod_ssh "sudo dnf upgrade --refresh -y"
echo

ok "dnf upgrade completed"

# ---------------------------------------------------------------------------
# Phase 4 — Determine whether a reboot is needed
# ---------------------------------------------------------------------------
sep
log "Phase 4 — Checking whether reboot is required"

# Get the version string of the most recently installed kernel package.
# rpm --last lists packages in install-time order, newest first.
LATEST_KERNEL=$(prod_ssh \
  "rpm -q kernel-core --last 2>/dev/null | head -1 | awk '{print \$1}' | sed 's/^kernel-core-//'")

log "  Running kernel:   ${PRE_KERNEL}"
log "  Installed kernel: ${LATEST_KERNEL}"

if [[ "$PRE_KERNEL" == "$LATEST_KERNEL" ]]; then
  ok "Kernel unchanged — no reboot required"
  NEEDS_REBOOT=false
else
  warn "New kernel detected (${PRE_KERNEL} → ${LATEST_KERNEL}) — reboot required"
  NEEDS_REBOOT=true
fi

# ---------------------------------------------------------------------------
# Phase 5 — Reboot (if needed) and wait for server to return
# ---------------------------------------------------------------------------
if [[ "$NEEDS_REBOOT" == true ]]; then
  sep
  log "Phase 5 — Rebooting server"

  prod_ssh "sudo systemctl reboot" || true   # connection closes mid-command; that's expected

  log "Reboot issued. Waiting ${REBOOT_INITIAL_WAIT}s for server to go down..."
  sleep "$REBOOT_INITIAL_WAIT"

  log "Polling for SSH connectivity (every ${REBOOT_POLL_INTERVAL}s, up to $((REBOOT_POLL_MAX * REBOOT_POLL_INTERVAL))s)..."

  RECONNECTED=false
  for i in $(seq 1 "$REBOOT_POLL_MAX"); do
    printf '\033[0;34m[%s]\033[0m   attempt %d/%d...\n' "$(_ts)" "$i" "$REBOOT_POLL_MAX"
    if prod_ssh_try "echo up" | grep -q "up"; then
      RECONNECTED=true
      break
    fi
    sleep "$REBOOT_POLL_INTERVAL"
  done

  if [[ "$RECONNECTED" != true ]]; then
    die "Server did not come back within $((REBOOT_POLL_MAX * REBOOT_POLL_INTERVAL))s. Investigate manually."
  fi

  ok "Server is back online"
else
  log "Phase 5 — Skipped (no reboot needed)"
fi

# ---------------------------------------------------------------------------
# Phase 6 — Post-update verification
# ---------------------------------------------------------------------------
sep
log "Phase 6 — Verification"

# Give systemd a moment to start services if we just rebooted
if [[ "$NEEDS_REBOOT" == true ]]; then
  sleep 5
fi

POST_KERNEL=$(prod_ssh "uname -r")
POST_OS=$(prod_ssh "rpm -E %fedora")
POST_UPTIME=$(prod_ssh "uptime -p")

log "  OS:      Fedora ${POST_OS}"
log "  Kernel:  ${POST_KERNEL}"
log "  Uptime:  ${POST_UPTIME}"

if [[ "$NEEDS_REBOOT" == true ]]; then
  if [[ "$POST_KERNEL" == "$LATEST_KERNEL" ]]; then
    ok "Running on new kernel: ${POST_KERNEL}"
  else
    warn "Expected kernel ${LATEST_KERNEL} but got ${POST_KERNEL} — may need another reboot"
  fi
fi

log "Checking container status..."
prod_ssh "podman ps --format 'table {{.Names}}\t{{.Status}}'"
echo

log "Checking app health (${HEALTH_URL})..."
HEALTH_RESPONSE=$(curl -sf --max-time 15 "$HEALTH_URL" || true)
if echo "$HEALTH_RESPONSE" | grep -q '"status":"healthy"'; then
  ok "App is healthy: ${HEALTH_RESPONSE}"
else
  die "Health check failed. Response: '${HEALTH_RESPONSE}'. Investigate: prod_ssh podman-compose -f /opt/yearofbingo/compose.yaml logs"
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
sep
if [[ "$NEEDS_REBOOT" == true ]]; then
  ok "Production OS update complete. Rebooted ${PRE_KERNEL} → ${POST_KERNEL}."
else
  ok "Production OS update complete. No reboot was required."
fi
sep
