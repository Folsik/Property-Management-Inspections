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
