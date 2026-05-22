.PHONY: local premium local-billing down build up logs test lint clean assets e2e e2e-headed e2e-debug test-backend test-frontend coverage release stripe-products stripe-listen stripe-stop premium-code premium-code-prod update-prod-os

# Force Podman to use a Linux compose provider (important in WSL where it may
# otherwise auto-detect a Windows Docker Desktop docker-compose binary).
PODMAN_COMPOSE_PROVIDER ?= podman-compose
PODMAN_COMPOSE_PROVIDER_PATH ?= $(shell command -v $(PODMAN_COMPOSE_PROVIDER) 2>/dev/null)
PODMAN_COMPOSE_ENV := PODMAN_COMPOSE_PROVIDER=$(PODMAN_COMPOSE_PROVIDER) PODMAN_COMPOSE_PROVIDER_PATH=$(PODMAN_COMPOSE_PROVIDER_PATH)
PODMAN_COMPOSE := $(PODMAN_COMPOSE_ENV) podman compose

# Tooling versions (override via env, e.g. `make lint GOLANGCI_LINT_VERSION=v2.0.0`)
GOLANGCI_LINT_VERSION ?= v2.6.2

# Stripe CLI PID file for background webhook listener
STRIPE_PID_FILE := .stripe-listen.pid
PREMIUM_DEV_ENV := BILLING_ENABLED=true FEATURE_TEMPLATES_ENABLED=true FEATURE_EDIT_AFTER_FINALIZE_ENABLED=true FEATURE_AI_ENHANCEMENTS_ENABLED=true

# Run full local rebuild: down, build assets, build container, up in background
local: down assets build up
	@echo "Local environment running. Use 'make logs' to view output or 'make down' to stop."
	@echo "For premium/billing testing, use 'make premium' instead."

# Run local environment with Stripe webhook listener and premium feature
# switches enabled.
# Order matters: Stripe listener must start first so STRIPE_WEBHOOK_SECRET is
# available to the app container on boot.
premium:
	@$(MAKE) down
	@$(MAKE) stripe-listen
	@$(MAKE) assets
	@$(MAKE) build
	@$(PREMIUM_DEV_ENV) $(MAKE) up
	@echo "Local environment running with Stripe webhook listener and premium features enabled."
	@echo "Use 'make logs' to view app output or 'make down' to stop (also stops Stripe listener)."

# Backward-compatible alias.
local-billing: premium
	@echo "'make local-billing' is deprecated; use 'make premium'."

# Build hashed assets locally (needed because ./web is volume-mounted)
assets:
	./scripts/build-assets.sh

# Stop and remove containers (also stops Stripe listener if running)
down: stripe-stop
	$(PODMAN_COMPOSE) down

# Build containers
build:
	$(PODMAN_COMPOSE) build app postgres redis mailpit

# Start containers in background
up:
	$(PODMAN_COMPOSE) up -d app postgres redis mailpit

# View logs (follow mode)
logs:
	$(PODMAN_COMPOSE) logs -f

# Run all tests in container
test:
	./scripts/test.sh

# Run Go tests only
test-backend:
	./scripts/test.sh --go

# Run JS tests only
test-frontend:
	./scripts/test.sh --js

# Run Go tests with a coverage summary (writes coverage.out)
coverage:
	@mkdir -p .cache/go-build .cache/go-mod
	@echo "Running Go tests with coverage (may take a couple minutes; includes slow crypto/bcrypt tests)..."
	GOCACHE=$(PWD)/.cache/go-build GOMODCACHE=$(PWD)/.cache/go-mod go test -v -race -coverprofile=coverage.out ./...
	GOCACHE=$(PWD)/.cache/go-build GOMODCACHE=$(PWD)/.cache/go-mod go tool cover -func=coverage.out | tail -n 1

# Run linter
lint:
	@chmod -R u+w .cache 2>/dev/null || true
	rm -rf .cache/go-build .cache/go-mod .cache/golangci-lint
	@mkdir -p .cache/bin .cache/go-build .cache/go-mod .cache/golangci-lint
	@set -e; \
	if command -v golangci-lint >/dev/null 2>&1 && golangci-lint version 2>/dev/null | grep -Eq 'version 2\\.'; then \
		GOLANGCI_LINT=golangci-lint; \
	else \
		echo "Using golangci-lint $(GOLANGCI_LINT_VERSION) (v2 config detected)"; \
		GOBIN=$(PWD)/.cache/bin GOCACHE=$(PWD)/.cache/go-build GOMODCACHE=$(PWD)/.cache/go-mod go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
		GOLANGCI_LINT=$(PWD)/.cache/bin/golangci-lint; \
	fi; \
	GOCACHE=$(PWD)/.cache/go-build GOMODCACHE=$(PWD)/.cache/go-mod GOLANGCI_LINT_CACHE=$(PWD)/.cache/golangci-lint $$GOLANGCI_LINT run

# Clean up everything including volumes
clean:
	$(PODMAN_COMPOSE) down -v

# Run Playwright E2E tests (destructive: resets volumes)
e2e:
	./scripts/e2e.sh

# Run Playwright E2E tests in headed mode
e2e-headed:
	HEADLESS=false ./scripts/e2e.sh

# Run Playwright E2E tests with debug helpers
e2e-debug:
	HEADLESS=false PWDEBUG=1 ./scripts/e2e.sh

# Release helper:
# - Usage: make release 1.2.3   (or: make release v1.2.3)
# - Updates version in web/templates/index.html and web/static/openapi.yaml
# - Commits on main, tags, and pushes
release:
	@bash ./scripts/release.sh $(filter-out $@,$(MAKECMDGOALS))

# Allow `make release 1.2.3` by treating the version argument as a no-op target,
# but only when `release` is among the requested goals.
ifneq (,$(filter release,$(MAKECMDGOALS)))
$(filter-out release,$(MAKECMDGOALS)):
	@:
endif

# ============================================================================
# Production Operations
# ============================================================================

# Apply host OS package updates on the production server.
# Reboots automatically if a new kernel was installed, then verifies
# containers and app health.
# Override SSH key: SSH_KEY=/path/to/key make update-prod-os
update-prod-os:
	@./scripts/update-prod-os.sh

# ============================================================================
# Stripe (Billing) Development Targets
# ============================================================================

# Create Stripe test-mode products and prices, append to .env
stripe-products:
	@echo "Creating Stripe products and prices..."
	@./scripts/stripe-setup.sh >> .env
	@echo "Price IDs appended to .env"

# Start Stripe CLI webhook listener in background
# Requires: stripe CLI installed and authenticated (stripe login)
# The webhook secret is written to .env as STRIPE_WEBHOOK_SECRET
stripe-listen:
	@if [ -f $(STRIPE_PID_FILE) ] && kill -0 $$(cat $(STRIPE_PID_FILE)) 2>/dev/null; then \
		echo "Stripe listener already running (PID $$(cat $(STRIPE_PID_FILE)))"; \
	else \
		echo "Starting Stripe webhook listener..."; \
		stripe listen --forward-to localhost:8080/api/billing/webhook > .stripe-listen.log 2>&1 & \
		echo $$! > $(STRIPE_PID_FILE); \
		sleep 2; \
		WEBHOOK_SECRET=$$(grep -o 'whsec_[a-zA-Z0-9]*' .stripe-listen.log | head -1); \
		if [ -n "$$WEBHOOK_SECRET" ]; then \
			if grep -q '^STRIPE_WEBHOOK_SECRET=' .env 2>/dev/null; then \
				sed -i.bak "s/^STRIPE_WEBHOOK_SECRET=.*/STRIPE_WEBHOOK_SECRET=$$WEBHOOK_SECRET/" .env && rm -f .env.bak; \
			else \
				echo "STRIPE_WEBHOOK_SECRET=$$WEBHOOK_SECRET" >> .env; \
			fi; \
			echo "Stripe listener started (PID $$(cat $(STRIPE_PID_FILE)))"; \
			echo "Webhook secret written to .env: $$WEBHOOK_SECRET"; \
		else \
			echo "Warning: Could not extract webhook secret from Stripe CLI output"; \
			echo "Check .stripe-listen.log for details"; \
		fi; \
	fi

# Stop Stripe CLI webhook listener
stripe-stop:
	@if [ -f $(STRIPE_PID_FILE) ]; then \
		PID=$$(cat $(STRIPE_PID_FILE)); \
		if kill -0 $$PID 2>/dev/null; then \
			echo "Stopping Stripe listener (PID $$PID)..."; \
			kill $$PID 2>/dev/null || true; \
		fi; \
		rm -f $(STRIPE_PID_FILE); \
	fi

# ============================================================================
# Premium Code Generation (Owner/Admin)
# ============================================================================

# Local code generation (writes hashed codes to the configured DB, prints plaintext codes to stdout).
# Examples:
#   make premium-code PREMIUM_CODE_COUNT=5 PREMIUM_CODE_DURATION_DAYS=30
#   make premium-code PREMIUM_CODE_COUNT=1 PREMIUM_CODE_LIFETIME=1
#
# Defaults target the local dev DB from compose.yaml (override via env if needed).
PREMIUM_CODE_COUNT ?= 1
PREMIUM_CODE_DURATION_DAYS ?= 30
PREMIUM_CODE_LIFETIME ?= 0
PREMIUM_CODE_EXPIRES_DAYS ?= 0

premium-code:
	@bash -euo pipefail -c '\
	FLAGS="--count $(PREMIUM_CODE_COUNT)"; \
	if [ "$(PREMIUM_CODE_LIFETIME)" = "1" ]; then \
		FLAGS="$$FLAGS --lifetime"; \
	elif [ "$(PREMIUM_CODE_DURATION_DAYS)" != "0" ]; then \
		FLAGS="$$FLAGS --duration_days $(PREMIUM_CODE_DURATION_DAYS)"; \
	fi; \
	if [ "$(PREMIUM_CODE_EXPIRES_DAYS)" != "0" ]; then \
		FLAGS="$$FLAGS --expires_days $(PREMIUM_CODE_EXPIRES_DAYS)"; \
	fi; \
	echo "Generating Premium code(s) in DB (prints codes to stdout)..."; \
	DB_HOST="$${DB_HOST:-localhost}" \
	DB_PORT="$${DB_PORT:-5432}" \
	DB_USER="$${DB_USER:-bingo}" \
	DB_PASSWORD="$${DB_PASSWORD:-bingo_dev_password}" \
	DB_NAME="$${DB_NAME:-nye_bingo}" \
	DB_SSLMODE="$${DB_SSLMODE:-disable}" \
	go run ./scripts/create_premium_codes $$FLAGS \
	'

# Production-ish generation via SSH tunnel.
# This runs the generator locally, but connects to the production DB through an SSH port forward.
#
# Required:
#   PREMIUM_CODE_SSH_HOST=user@prod-host
#
# Optional (adjust depending on where Postgres is reachable FROM the SSH host):
#   PREMIUM_CODE_SSH_DB_HOST=127.0.0.1
#   PREMIUM_CODE_SSH_DB_PORT=5432
#   PREMIUM_CODE_LOCAL_PORT=15432
#
# Usage:
#   make premium-code-prod PREMIUM_CODE_SSH_HOST=user@prod-host DB_USER=... DB_PASSWORD=... DB_NAME=...
PREMIUM_CODE_SSH_HOST ?=
PREMIUM_CODE_SSH_DB_HOST ?= 127.0.0.1
PREMIUM_CODE_SSH_DB_PORT ?= 5432
PREMIUM_CODE_LOCAL_PORT ?= 15432

premium-code-prod:
	@bash -euo pipefail -c '\
	if [ -z "$(PREMIUM_CODE_SSH_HOST)" ]; then \
		echo "PREMIUM_CODE_SSH_HOST is required (e.g. user@prod-host)"; \
		exit 1; \
	fi; \
	if [ -z "$${DB_PASSWORD:-}" ]; then \
		echo "DB_PASSWORD is required for premium-code-prod (pass it in your shell env)"; \
		exit 1; \
	fi; \
	FLAGS="--count $(PREMIUM_CODE_COUNT)"; \
	if [ "$(PREMIUM_CODE_LIFETIME)" = "1" ]; then \
		FLAGS="$$FLAGS --lifetime"; \
	elif [ "$(PREMIUM_CODE_DURATION_DAYS)" != "0" ]; then \
		FLAGS="$$FLAGS --duration_days $(PREMIUM_CODE_DURATION_DAYS)"; \
	fi; \
	if [ "$(PREMIUM_CODE_EXPIRES_DAYS)" != "0" ]; then \
		FLAGS="$$FLAGS --expires_days $(PREMIUM_CODE_EXPIRES_DAYS)"; \
	fi; \
	echo "Opening SSH tunnel to Postgres via $(PREMIUM_CODE_SSH_HOST) (localhost:$(PREMIUM_CODE_LOCAL_PORT) -> $(PREMIUM_CODE_SSH_DB_HOST):$(PREMIUM_CODE_SSH_DB_PORT))..."; \
	ssh -o ExitOnForwardFailure=yes -N -L $(PREMIUM_CODE_LOCAL_PORT):$(PREMIUM_CODE_SSH_DB_HOST):$(PREMIUM_CODE_SSH_DB_PORT) $(PREMIUM_CODE_SSH_HOST) & \
	TUNNEL_PID="$$!"; \
	trap "kill \"$$TUNNEL_PID\" >/dev/null 2>&1 || true" EXIT; \
	sleep 0.5; \
	echo "Generating Premium code(s) in production DB (prints codes to stdout)..."; \
	DB_HOST="localhost" \
	DB_PORT="$(PREMIUM_CODE_LOCAL_PORT)" \
	DB_USER="$${DB_USER:-bingo}" \
	DB_PASSWORD="$$DB_PASSWORD" \
	DB_NAME="$${DB_NAME:-nye_bingo}" \
	DB_SSLMODE="$${DB_SSLMODE:-disable}" \
	go run ./scripts/create_premium_codes $$FLAGS \
	'
