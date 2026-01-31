#!/bin/sh
#
# Wrapper around `podman compose` that forces a Linux compose provider.
#
# In WSL, Podman may auto-detect a Windows Docker Desktop `docker-compose` binary
# (via /mnt/c/...) as its external compose provider, which fails inside the distro.
#
# This script forces Podman to use `podman-compose` when available.

set -eu

if ! command -v podman >/dev/null 2>&1; then
  echo "podman is required but was not found in PATH." >&2
  exit 127
fi

provider="${PODMAN_COMPOSE_PROVIDER:-podman-compose}"
provider_path="${PODMAN_COMPOSE_PROVIDER_PATH:-}"

if [ -z "$provider_path" ] && command -v "$provider" >/dev/null 2>&1; then
  provider_path="$(command -v "$provider")"
fi

case "$provider_path" in
  /mnt/*)
    # In WSL, a Windows Docker Desktop shim may appear on PATH. Ignore it.
    provider_path=""
    ;;
esac

if [ -n "$provider_path" ]; then
  export PODMAN_COMPOSE_PROVIDER="$provider"
  export PODMAN_COMPOSE_PROVIDER_PATH="$provider_path"
fi

exec podman compose "$@"
