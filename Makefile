.PHONY: local local-billing down build up logs test lint clean assets e2e e2e-headed e2e-debug test-backend test-frontend coverage release stripe-products stripe-listen stripe-stop

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

# Run full local rebuild: down, build assets, build container, up in background
local: down assets build up
	@echo "Local environment running. Use 'make logs' to view output or 'make down' to stop."
	@echo "For billing testing, use 'make local-billing' instead."

# Run local environment with Stripe webhook listener (for billing development)
local-billing: down assets build up stripe-listen
	@echo "Local environment running with Stripe webhook listener."
	@echo "Use 'make logs' to view app output or 'make down' to stop (also stops Stripe listener)."

# Build hashed assets locally (needed because ./web is volume-mounted)
assets:
	./scripts/build-assets.sh

# Stop and remove containers (also stops Stripe listener if running)
down: stripe-stop
	$(PODMAN_COMPOSE) down

# Build containers
build:
	$(PODMAN_COMPOSE) build

# Start containers in background
up:
	$(PODMAN_COMPOSE) up -d

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
