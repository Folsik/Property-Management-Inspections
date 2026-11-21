# Architecture decision record — initial foundation

- Modular Go monolith, PostgreSQL/pgx and embedded transactional migrations ported from Doit-it.
- JWTs are Ed25519 signed with `sub` as a global user ID, never an organization ID.
- Every tenant-specific query is scoped to the requested organization and membership.
- Roles: owner, manager, inspector. Only owner/manager can create properties. Inspectors can create inspections and items.
- Every inspection item has a client-generated `client_id` unique within an inspection to support offline replay; conflict handling is future work.
- Media uploads require a presigned PUT to an S3-compatible bucket; the API must validate membership and ownership and limit MIME types, keys, sizes and expiry.
- Jobs table uses SKIP LOCKED and lease timestamps. AI/PDF workers are future milestones, not production functionality.
- OTP provider integrations, iOS, Next.js, Stripe, report magic links are not included in this foundation.
- Never port cards, ledger, transfers, KYC or bank-specific database migrations.

## Stage 1 update (November 2026)

The foundation's previous descriptions of Next.js and report guest links as future work are historical; both are present now. The Go API implements device-scoped rotating refresh sessions, and direct phone-based membership addition is disabled. See [auth sessions](auth-sessions.md). Media verification, offline sync and worker jobs remain planned.

## Stage 3 and Stage 4 architecture (November 2026)

The Stage 1 bullets above document historical intentions rather than current implementation. Stage 2 has verified immutable media uploads and tenant-scoped object retrieval. Stage 3 introduced mobile /v1 contracts, keyset pagination, optimistic version conflicts, soft-delete tombstones and transactional offline sync (merged via PR #11).

Stage 4 introduces a separate worker process against the same PostgreSQL database. Worker claims use SKIP LOCKED; timed leases have random attempt tokens, active renewal and fencing. Failed jobs retry with exponential backoff or enter the dead-letter state. Successful transcription schedules extract, then PDF generation, atomically with job completion. All processing artifacts are scoped by organization, inspection and media identifiers. Private API download routes enforce active membership. See [Stage 4 operations](workers-stage4.md) for the STT gateway contract, limitations, recovery and validation.

Do not confuse deterministic transcript line extraction with an AI-confirmed defect assessment. The first PDF renderer is ASCII-oriented and requires internationalization work before global production use. No broker, CRDT, billing or bank/ledger subsystems have been introduced.
