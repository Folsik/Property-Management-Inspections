# Property-Management-Inspections

AI-assisted property inspections and punch lists for property managers and short-term rental operators.

## Architecture
- **Go 1.23 modular monolith:** `cmd/api`, `internal/{app,api,auth,config,store,media,jobs}`.
- **PostgreSQL + pgx:** embedded, transactionally applied migrations and org-scoped data.
- **Auth:** phone-first OTP, PIN, Ed25519 access JWT with revocation; roles are owner, manager and inspector.
- **Media:** S3-compatible storage / Cloudflare R2 direct uploads (presigned requests).
- **Clients (planned):** offline-first SwiftUI iOS and Next.js web.
- **Workers (planned):** transcription, structured extraction, PDF report generation using PostgreSQL SKIP LOCKED.
- **Integrations (planned):** Whisper/LLM, Stripe, Resend/Twilio and guest report magic links.

## Quick start
```sh
docker compose up --build
curl http://localhost:8080/healthz
```
Or run `docker compose up -d postgres` and `go run ./cmd/api`, with `DATABASE_URL` set from `.env.example`.

Environment variables are *not* automatically loaded by the Go process: export them or use your local environment loader. Never use the development credentials in production.

## Origin and differences from Doit-it
Reusable code was ported, rather than reimplemented: Ed25519 JWT token manager and tests, PostgreSQL connection/migration runner, configuration, process lifecycle and shutdown, Docker and CI patterns. The fintech ledger, accounts, transfers, cards and KYC were intentionally omitted. The replacement schema provides organizations, role memberships, properties, inspections, punch-list items, media attachments and a job table.

## Security and roadmap
All tenant routes must verify membership for each organization. Client-generated IDs support future offline replay but do not yet constitute a complete synchronization protocol. Production OTP delivery needs a provider; no fake production OTP delivery. All cloud integrations require explicit credentials and validation. See [architecture](docs/architecture.md) and [Superpowers](docs/superpowers.md).

## License
No license has been granted for this new repository.
