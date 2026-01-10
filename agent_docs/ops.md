# Operations & Deployment

## Versioning & Releases

The application version is displayed in the footer and must be updated with each release. The version number is located in `web/templates/index.html` in the footer element.

The API spec version must also be kept in sync with releases and is located in `web/static/openapi.yaml` under `info.version`.

**Version format**: Follow [Semantic Versioning](https://semver.org/) (MAJOR.MINOR.PATCH)
- MAJOR: Breaking changes or significant new features
- MINOR: New features, backwards compatible
- PATCH: Bug fixes, backwards compatible

**Version increment rules**:
- Bug fixes → increment PATCH (e.g., 1.0.4 → 1.0.5)
- New features → increment MINOR, reset PATCH (e.g., 1.0.5 → 1.1.0)
- Breaking changes → increment MAJOR, reset MINOR and PATCH (e.g., 1.1.0 → 2.0.0)

### Creating a Release

When ready to release, follow these steps:

1. **Update the version**:
   - `web/templates/index.html` footer link (uses `vX.Y.Z`)
   - `web/static/openapi.yaml` `info.version` (uses `X.Y.Z`)
2. **Commit the change**:
   ```bash
   git add web/templates/index.html web/static/openapi.yaml
   git commit -m "Release v1.0.5"
   ```
3. **Create and push a version tag**:
   ```bash
   git tag v1.0.5
   git push origin main
   git push origin v1.0.5
   ```

Alternatively, you can use the release helper:
```bash
make release 1.0.5
```

This triggers the CI pipeline which will:
- Build and test the code
- Build multi-arch container images
- Tag the container with the version (e.g., `quay.io/yearofbingo/yearofbingo:1.0.5`)
- Create a GitHub Release with auto-generated changelog
- Deploy to production

**Container tags created**:
- `:1.0.5` - Exact version (immutable)
- `:latest` - Latest release (floating)
- `:<commit-sha>` - Specific commit

**Production deploys** use the image digest (not tag) for reproducibility. The compose.yaml on the server includes the tag as a comment above the image line for easy reference.

**GitHub Releases**: Each version tag creates a GitHub Release at https://github.com/yearofbingo/yearofbingo/releases with auto-generated release notes based on commits since the last tag

## CI/CD (Phase 9)

GitHub Actions workflow in `.github/workflows/ci.yaml`:

**Pipeline triggers:**
- Push to `main` branch - lint, test, build binary only (no container push or deploy)
- Version tags (`v*`) - full pipeline: lint, test, build, container push, release, deploy
- Pull requests - lint and test only
- Manual dispatch - deploy existing image

**Pipeline stages:**
1. **Secret Scan** - Gitleaks scans for accidentally committed secrets (runs on ALL changes)
2. **Lint** - golangci-lint with config in `.golangci.yaml`
3. **Test (Go)** - `go test -v -race -coverprofile=coverage.out ./...`
4. **Test (JS)** - `node web/static/js/tests/runner.js`
5. **Build** - Compile binary, upload as artifact
6. **Build Image** - Parallel builds on native runners (amd64 + arm64)
7. **Scan & Push** - Trivy security scan, Cosign signing, push multi-arch manifest
8. **Release** - Creates GitHub Release (only for version tags)
9. **Deploy** - Deploys to production server

**Path filtering:** Lint, test, build, and deploy jobs only run when code changes. Documentation-only changes (README, markdown files) skip these jobs but still run secret scanning.

**Container registry:** [quay.io/yearofbingo/yearofbingo](https://quay.io/repository/yearofbingo/yearofbingo)
- Multi-arch: `linux/amd64` and `linux/arm64`
- `:latest` - Latest main branch build
- `:<sha>` - Specific commit builds
- `:<version>` - Release versions (e.g., `:1.0.5`)

**Running production image locally:**
```bash
# Run pre-built image from quay.io (auto-selects correct arch)
podman compose -f compose.prod.yaml up
```

**Local CI/dev commands:**
```bash
# Run linting
golangci-lint run

# Run all tests in container
./scripts/test.sh

# Build container locally
podman build -f Containerfile -t yearofbingo .

# Run local dev build
podman compose up
```

**Secret scanning (pre-commit hook):**
```bash
# Install pre-commit (once)
brew install pre-commit

# Install hooks (run after cloning)
pre-commit install

# Test hook manually
pre-commit run --all-files
```

Gitleaks runs automatically on every commit to prevent accidentally committing secrets. Configuration in `.pre-commit-config.yaml`.

## Environment Variables

Server: `SERVER_HOST`, `SERVER_PORT`, `SERVER_SECURE`
Database: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`
Redis: `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB`
Email: `EMAIL_PROVIDER`, `RESEND_API_KEY`, `EMAIL_FROM_ADDRESS`, `APP_BASE_URL`
Backup: `BACKUP_ENCRYPTION_KEY`, `R2_BUCKET` (default: yearofbingo-backups), `BACKUP_NOTIFY_EMAILS`

## Database Backups

PostgreSQL backups are stored in Cloudflare R2 (S3-compatible, 10GB free tier). Redis is not backed up as it's only used for session caching with PostgreSQL fallback.

**Retention:** R2 lifecycle policy protects backups from deletion for 30 days, then auto-deletes at 31 days. This prevents attackers from deleting backups if the server is compromised.

**Backup scripts:**
- `./scripts/backup.sh` - Create encrypted backup and upload to R2
- `./scripts/restore.sh` - Download and restore from R2 backup
- `./scripts/verify-backup.sh` - Verify backup can be restored (runs daily)
- `./scripts/test-backup.sh` - Interactive backup test with detailed output

**Automation:** Systemd timers run daily (see `cloud-init.yaml`):
- 3:00 AM - Backup (`yearofbingo-backup.timer`)
- 4:00 AM - Verification (`yearofbingo-verify-backup.timer`)

**Verification:** If daily verification fails, an error file `BACKUP_VERIFICATION_FAILED_*.txt` is written to the R2 bucket with details. Check the bucket periodically or set up Cloudflare notifications.

**Email notifications (Resend):**
- Set `BACKUP_NOTIFY_EMAILS` to a comma-separated list of recipients (requires `RESEND_API_KEY` and `EMAIL_FROM_ADDRESS`).
- Backup success emails are sent by default; set `BACKUP_NOTIFY_SUCCESS=0` to disable.
- Verification success emails are off by default; set `BACKUP_NOTIFY_VERIFY_SUCCESS=1` to enable. Verification failures always email (when `BACKUP_NOTIFY_EMAILS` is set).

Check status:
```bash
systemctl status yearofbingo-backup.timer
journalctl -u yearofbingo-backup.service
```

If verification fails with a restore error, run a manual test for full output:
```bash
./scripts/test-backup.sh --latest
```

Note: `pg_dump` plain SQL includes a leading `\connect <db>`; if `/opt/yearofbingo/.env` has a stale `DB_NAME` (e.g. `nye_bingo`), backups may target a different database than expected.

**Manual backup:**
```bash
./scripts/backup.sh
```

**Restore from backup:**
```bash
./scripts/restore.sh --list     # List available backups
./scripts/restore.sh --latest   # Restore most recent
./scripts/restore.sh <filename> # Restore specific backup
```

**Test backup integrity:**
```bash
./scripts/test-backup.sh --latest
```

**Security:** Backups are encrypted with GPG (AES-256) before upload. The `BACKUP_ENCRYPTION_KEY` must be stored securely and separately from backups.

**Disaster Recovery:**
1. Provision new server with `cloud-init.yaml`
2. Run CI deploy (configures secrets and rclone)
3. Start timers: `sudo systemctl start yearofbingo-backup.timer yearofbingo-verify-backup.timer`
4. Restore: `./scripts/restore.sh --latest`
