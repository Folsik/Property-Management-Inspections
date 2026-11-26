# Inspectly staging (not for real customer data)

Copy this directory to /opt/inspectly/staging as deploy. Do not copy existing .env or TLS keys into Git. Use a separate staging bucket and test provider credentials. Run bash scripts/init-tls.sh once; renew the local DB TLS certificate before expiry. The PostgreSQL Alpine image uses UID 70; verify before running the script. The CA private key is discarded, so renewal requires a coordinated replacement of CA and server certificate.

Copy .env.example to .env, chmod 600, fill in the full published commit SHA, three independent hex secrets (database password, BFF and ingress) and persistent Ed25519 private key (raw 64-byte Go Ed25519 private key encoded base64url). Do not use development credentials. For private images authenticate with sudo docker login ghcr.io using a read-only packages credential via stdin. Never put credentials in shell history.

Run sudo docker compose config --quiet; sudo docker compose pull; sudo docker compose up -d. Inspect sudo docker compose ps and logs locally (do not share secrets). Run bash scripts/smoke-test.sh and sudo docker stats --no-stream. Verify native and web authentication rate limiting with distinct source IPs, and spoofed custom headers. Caddy is the only published service and must be the first internet-facing proxy. Do not put a CDN in front without reviewing trusted client IP handling.

Before updating, run bash scripts/backup.sh and record previous IMAGE_TAG securely. Pull new SHA then compose up -d and smoke test. Rollback by restoring previous SHA in .env and running compose up -d ONLY if migrations are backward compatible; otherwise restore a verified pre-update database backup in a separate recovery procedure. Never run compose down -v. Backups are local only; copy offsite securely before relying on them. Test restoration into an isolated database before rollout. No automatic deployment or rollback is enabled.

Resource limits are initial estimates, not validated capacity. API/worker readiness, real provider integrations, migration compatibility, security and recovery require further checks before TestFlight.
