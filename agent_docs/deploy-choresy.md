You are deploying the choresy application (this repository) to a production server
where it will run alongside an existing app (yearofbingo). Your goal is to get
choresy live at `https://choresy.yearofbingo.com` without disrupting yearofbingo.

Work through the steps below in order. Use your tools to read files, run shell
commands, and edit files. Before executing any destructive command, state what
it does and why.

---

## Phase 0 — Understand the choresy app

Before touching the server, read this repository to answer:

1. What `Containerfile` (or `Dockerfile`) is at the root? What port does the app
   listen on inside the container? (Look for `EXPOSE`, `SERVER_PORT`, `PORT`, or
   equivalent in the Containerfile and in any config/env files.)
2. Does the app require postgres, redis, or any other backing services?
3. What environment variables does the app require at runtime? (Check
   `.env.example`, `README`, config files, or `main.go` / `cmd/` entrypoints.)
4. What is the git remote URL for this repo? (You will need it to clone onto the
   server.)

Record your answers — you will need them to fill in the compose file and `.env`
in later steps.

---

## Server access

All commands on the server are run as the `deploy` user over SSH through a
Cloudflare Tunnel. You need:
- SSH key at `~/.ssh/hetzner_yearofbingo_ci` on your local machine
- `cloudflared` CLI available in your PATH

SSH command:
```
ssh -i ~/.ssh/hetzner_yearofbingo_ci \
    -o ProxyCommand="cloudflared access ssh --hostname ssh.yearofbingo.com" \
    -o StrictHostKeyChecking=accept-new \
    deploy@ssh.yearofbingo.com
```

The `deploy` user has passwordless `sudo` (`NOPASSWD: ALL`).

---

## What is already on the server — do not modify any of this

| Item | Detail |
|------|--------|
| OS | Fedora 44 |
| yearofbingo app dir | `/opt/yearofbingo/` |
| yearofbingo host port | **80** — do not use |
| yearofbingo persistent data | `/mnt/data/postgres/`, `/mnt/data/redis/` |
| yearofbingo systemd service | `/etc/systemd/system/yearofbingo.service` |
| Cloudflare Tunnel name | `yearofbingo` |
| Tunnel config | `/home/deploy/.cloudflared/config.yml` |
| Tunnel ingress (current) | `yearofbingo.com → http://localhost:80`, `ssh.yearofbingo.com → ssh://localhost:22` |
| Firewall | No inbound ports are open except SSH. App traffic flows through the outbound Cloudflare Tunnel — no firewall changes are needed. |

## Port assignment for choresy

choresy must use host port **8081**.

The Cloudflare Tunnel will route `choresy.yearofbingo.com → http://localhost:8081`.
cloudflared reaches the app on localhost so no firewall rule is needed.

---

## Phase 1 — Get the source code onto the server

SSH into the server and clone this repository:

```bash
git clone <choresy-repo-url> /home/deploy/choresy-src
```

If the repository is private and SSH keys are not configured on the server, use
rsync from your local machine instead (run this locally):

```bash
rsync -av --exclude='.git' \
  -e 'ssh -i ~/.ssh/hetzner_yearofbingo_ci -o ProxyCommand="cloudflared access ssh --hostname ssh.yearofbingo.com"' \
  ./ \
  deploy@ssh.yearofbingo.com:/home/deploy/choresy-src/
```

---

## Phase 2 — Build the container image on the server

```bash
cd /home/deploy/choresy-src
podman build -t localhost/choresy_app:latest -f Containerfile .
```

Verify:
```bash
podman images localhost/choresy_app:latest
```

---

## Phase 3 — Create the app directory and persistent data directories

```bash
sudo mkdir -p /opt/choresy
sudo chown deploy:deploy /opt/choresy
```

Only create the data directories that choresy actually needs (based on your
Phase 0 findings):

```bash
# If choresy uses postgres:
sudo mkdir -p /mnt/data/choresy/postgres
sudo chown -R 70:deploy /mnt/data/choresy/postgres   # uid 70 = postgres user in alpine image

# If choresy uses redis:
sudo mkdir -p /mnt/data/choresy/redis
sudo chown -R 999:deploy /mnt/data/choresy/redis     # uid 999 = redis user in alpine image
```

---

## Phase 4 — Create `/opt/choresy/compose.yaml`

Write a compose file tailored to what you found in Phase 0. The mandatory
constraint is that the app service maps host port **8081** to the container's
actual port. Use the template below and uncomment/adjust as needed:

```yaml
# /opt/choresy/compose.yaml
services:
  app:
    image: localhost/choresy_app:latest
    ports:
      - "8081:PORT"          # replace PORT with the container port from Phase 0
    environment:
      - APP_ENV=production
      - APP_BASE_URL=https://choresy.yearofbingo.com
      # Add all other required env vars here.
      # Reference secrets from .env with ${VAR_NAME} syntax.
    restart: unless-stopped
    # Uncomment if app depends on postgres/redis:
    # depends_on:
    #   postgres:
    #     condition: service_healthy
    #   redis:
    #     condition: service_healthy

  # Uncomment if choresy needs postgres:
  # postgres:
  #   image: docker.io/library/postgres:16-alpine
  #   environment:
  #     - POSTGRES_USER=choresy
  #     - POSTGRES_PASSWORD=${DB_PASSWORD}
  #     - POSTGRES_DB=choresy
  #   volumes:
  #     - /mnt/data/choresy/postgres:/var/lib/postgresql/data
  #   healthcheck:
  #     test: ["CMD-SHELL", "pg_isready -U choresy -d choresy"]
  #     interval: 5s
  #     timeout: 5s
  #     retries: 5
  #   restart: unless-stopped

  # Uncomment if choresy needs redis:
  # redis:
  #   image: docker.io/library/redis:7-alpine
  #   command: redis-server --requirepass ${REDIS_PASSWORD}
  #   volumes:
  #     - /mnt/data/choresy/redis:/data
  #   healthcheck:
  #     test: ["CMD", "redis-cli", "-a", "${REDIS_PASSWORD}", "ping"]
  #     interval: 5s
  #     timeout: 5s
  #     retries: 5
  #   restart: unless-stopped
```

---

## Phase 5 — Create `/opt/choresy/.env`

Create the file with tight permissions before writing any secrets:

```bash
touch /opt/choresy/.env
chmod 600 /opt/choresy/.env
```

Populate it with every secret referenced via `${VAR}` in compose.yaml.
Generate strong random passwords for any database credentials, e.g.:

```bash
openssl rand -base64 32   # run once per secret to generate a value
```

Example `.env` structure (adjust to what choresy actually needs):
```
DB_PASSWORD=<generated>
REDIS_PASSWORD=<generated>
```

---

## Phase 6 — Smoke-test the stack before wiring up routing

```bash
cd /opt/choresy
podman-compose up -d
podman-compose ps
```

Confirm the app responds on localhost:
```bash
curl -sf http://localhost:8081/health || curl -v http://localhost:8081/
```

If it fails, check logs before proceeding:
```bash
podman-compose logs app
```

Once healthy, bring it back down — the systemd service will manage it from here:
```bash
podman-compose down
```

---

## Phase 7 — Create the systemd service

```bash
sudo tee /etc/systemd/system/choresy.service > /dev/null <<'EOF'
[Unit]
Description=Choresy App
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
User=deploy
WorkingDirectory=/opt/choresy
ExecStart=/usr/bin/podman-compose up -d
ExecStop=/usr/bin/podman-compose down

[Install]
WantedBy=multi-user.target
EOF
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now choresy.service
sudo systemctl status choresy.service
```

Confirm both apps' containers are running:
```bash
podman ps --format 'table {{.Names}}\t{{.Status}}'
```

You should see yearofbingo_app_1, yearofbingo_postgres_1, yearofbingo_redis_1,
and the choresy containers all listed.

---

## Phase 8 — Add choresy to the Cloudflare Tunnel

Edit `/home/deploy/.cloudflared/config.yml`. Insert the choresy ingress rule
**before** the final catch-all line. The file must look like this when done:

```yaml
tunnel: yearofbingo
credentials-file: /home/deploy/.cloudflared/6858ee6a-fee0-4d96-8e20-8f4c7351c57b.json
ingress:
  - hostname: yearofbingo.com
    service: http://localhost:80
  - hostname: ssh.yearofbingo.com
    service: ssh://localhost:22
  - hostname: choresy.yearofbingo.com
    service: http://localhost:8081
  - service: http_status:404
```

Do not remove or reorder the existing yearofbingo.com and ssh entries.

---

## Phase 9 — Create the DNS record

Use the already-authenticated cloudflared CLI to add a CNAME in Cloudflare DNS:

```bash
/usr/local/bin/cloudflared tunnel route dns yearofbingo choresy.yearofbingo.com
```

Expected output:
```
INF Added CNAME choresy.yearofbingo.com which will route to this tunnel tunnelID=6858ee6a-fee0-4d96-8e20-8f4c7351c57b
```

---

## Phase 10 — Restart cloudflared

```bash
sudo systemctl restart cloudflared
sudo systemctl status cloudflared
```

The restart takes only a few seconds; the yearofbingo and SSH ingress rules
reconnect automatically.

---

## Phase 11 — Final verification

```bash
# yearofbingo must still be healthy — if this fails, stop and investigate
curl -sf https://yearofbingo.com/health

# choresy should now be reachable (allow up to 60s for DNS propagation)
curl -sf https://choresy.yearofbingo.com/health || curl -v https://choresy.yearofbingo.com/
```

Both systemd services should be active:
```bash
sudo systemctl status yearofbingo.service choresy.service
```

---

## Rollback if something goes wrong

If choresy breaks yearofbingo at any point, run this to undo all choresy changes:

```bash
sudo systemctl stop choresy.service
sudo systemctl disable choresy.service
sudo rm /etc/systemd/system/choresy.service
sudo systemctl daemon-reload

# Revert /home/deploy/.cloudflared/config.yml to remove the choresy ingress line, then:
sudo systemctl restart cloudflared

# Confirm yearofbingo is still up
curl -sf https://yearofbingo.com/health
podman ps --format 'table {{.Names}}\t{{.Status}}'
```
