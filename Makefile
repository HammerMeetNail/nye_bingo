.PHONY: local down build up logs test lint clean assets e2e e2e-headed e2e-debug test-backend test-frontend release

# Run full local rebuild: down, build assets, build container, up in background
local: down assets build up
	@echo "Local environment running. Use 'make logs' to view output or 'make down' to stop."

# Build hashed assets locally (needed because ./web is volume-mounted)
assets:
	./scripts/build-assets.sh

# Stop and remove containers
down:
	podman compose down

# Build containers
build:
	podman compose build

# Start containers in background
up:
	podman compose up -d

# View logs (follow mode)
logs:
	podman compose logs -f

# Run all tests in container
test:
	./scripts/test.sh

# Run Go tests only
test-backend:
	./scripts/test.sh --go

# Run JS tests only
test-frontend:
	./scripts/test.sh --js

# Run linter
lint:
	@chmod -R u+w .cache 2>/dev/null || true
	rm -rf .cache
	@mkdir -p .cache/go-build .cache/go-mod .cache/golangci-lint
	GOCACHE=$(PWD)/.cache/go-build GOMODCACHE=$(PWD)/.cache/go-mod GOLANGCI_LINT_CACHE=$(PWD)/.cache/golangci-lint golangci-lint run

# Clean up everything including volumes
clean:
	podman compose down -v

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
