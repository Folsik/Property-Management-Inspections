# Property-Management-Inspections

AI-assisted property inspections and punch lists for property managers and short-term rental operators.

## Architecture
- **Go 1.23 modular monolith:** `cmd/api`, `internal/{app,api,auth,config,store,media,jobs}`.
- **PostgreSQL + pgx:** embedded, transactionally applied migrations and org-scoped data.
- **Auth:** phone-first OTP (Twilio SMS in production), PIN, Ed25519 access JWT with revocation; roles are owner, manager and inspector.
- **Media:** S3-compatible storage / Cloudflare R2 direct uploads (presigned requests).
- **Clients:** Next.js 15 / Tailwind CSS 4 / shadcn-style web workspace implemented; SwiftUI iOS remains planned.
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

## Web workspace

Start with `docker compose up --build`, then open **http://localhost:3000**.

The Next.js client uses a same-origin BFF and server-side HTTP-only session cookie. No production user data is embedded in the frontend or seeded in the database. The UI provides:

- Phone OTP and PIN login, logout, initial PIN creation and profile name settings
- Tenant organization switcher, creation and membership management
- Live portfolio dashboard, properties and search
- Inspections, status transitions, punch-list creation and resolution
- S3/R2 photo/audio direct upload tickets (remote browser PUT requires bucket CORS)
- Printable inspection reports from real records; time-limited public guest report links with revocation
- Background jobs list (AI transcription and PDF workers are **not yet deployed**)

For real SMS delivery set `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_FROM_NUMBER`; see [OTP integration](docs/otp-provider.md). Local Go development-only OTP is available through the API; no fabricated domain records are used.

See [web documentation](apps/web/README.md) for architecture and operational requirements.

**Not complete yet:** AI/PDF workers, media verification and retrieval, Stripe billing, offline iOS synchronization, session renewal and password/PIN recovery. This web foundation does not claim those integrations are finished.
