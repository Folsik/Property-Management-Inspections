# Stage 4 — Workers and operational quality

Implementation branch: `feat/bootstrap-foundation`. Stage 3 was merged to `main` in PR #11; do **not** promote Stage 4 without owner verification.

## Deployment

A separate `worker` service runs `cmd/worker` in Docker Compose. Worker and API share PostgreSQL; neither requires Redis or an in-memory queue. The worker runs additive migrations 011 and 012 on startup, serialized by the existing migration advisory lock. Back up the database before applying migrations. `main` does not receive Stage 4 schema changes until approval.

Configure `S3_ENDPOINT`, `S3_BUCKET`, `S3_REGION`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` for verified audio storage. A production transcription adapter requires `TRANSCRIPTION_ENDPOINT` (HTTPS) and `TRANSCRIPTION_API_KEY`. The adapter sends `Authorization: Bearer <key>` and a JSON POST body `{"audio_base64":"...", "mime_type":"audio/mp4"}`; the response must be JSON `{"text":"..."}`. **This is a custom adapter contract, not a direct OpenAI/Whisper API contract.** No fake transcript is produced when the provider is absent. Configure an audited STT gateway with security, privacy, deletion, retention and regional processing guarantees before using real recordings. Raw audio and secrets must not be logged.

## Lifecycle

1. Authenticated tenant member submits `POST /v1/organizations/{org}/inspections/{inspection}/transcriptions` with UUIDs `media_id` and `idempotency_key`. Source must be a verified, undeleted MP4/MPEG audio object for this organization and inspection. The same key returns the same job only if the payload matches; conflicting use is HTTP 409.
2. A worker atomically claims the earliest eligible `pending` job or expired `running` job via `FOR UPDATE SKIP LOCKED` and assigns a random per-attempt lease token. Each attempt increments `attempts`. Lease renewal is every third of the lease duration, and completion uses a compare-and-swap on the current token and unexpired lease.
3. On transient failure, the worker stores a bounded error and retries at 5, 10, 20, 40 seconds, etc., up to `max_attempts` (default 5). Exhausted jobs and expired final-attempt leases transition to `failed` (dead letter). Successful jobs are marked `succeeded`. Events are persisted in `job_events`.
4. A successful `transcribe` task atomically enqueues `extract`; successful `extract` atomically enqueues `generate_pdf`. Each next job has a unique stage idempotency key derived from its predecessor, preventing duplicate chaining. Handler output is written with a tenant-and-source primary key, so a crash between artifact persistence and job acknowledgement can safely repeat.
5. A `generate_pdf` job writes an inspection PDF report to `inspection_generated_reports`. Authenticated members may download via `GET /v1/organizations/{org}/inspections/{inspection}/reports/{media}/pdf`, with no-store response headers. The PDF is a minimal one-page ASCII-compatible rendering of the persisted verbatim extraction notes. Non-ASCII characters are replaced; **this renderer is a functional baseline, not production-ready internationalized PDF layout**.
6. Owner or manager may requeue a `failed` job with `POST /v1/organizations/{org}/jobs/{job}/retry`; the action is recorded in `audit_log`. Do not requeue indefinitely without investigating the failure. Tenant membership and role are enforced by API middleware and organization-scoped SQL queries.

The extraction implementation is intentionally deterministic: it stores verbatim non-empty transcript lines (up to 100). It does **not** infer defect severity or fabricate inspection items. A schema-validated, consent-aware structured AI extraction can replace this adapter after review.

## Reliability and security

- At-least-once execution: do not assume exactly-once external provider invocation, especially after an expired lease or crash. Internal writes are upserts. Workload-specific side effects must remain idempotent.
- Avoid placing signed S3 URLs, access keys, original audio or refresh tokens in `jobs.payload`, `job_events.detail`, logs or audit metadata.
- Leases are fenced by `lease_token` and expiry. A worker that loses its lease may finish external work, but may not commit success or schedule downstream work.
- Worker logs are structured JSON. Use `job_id`, `kind` and `attempt` for correlation. A tenant-scoped JSON metrics endpoint exists at `GET /v1/organizations/{org}/jobs/metrics` (owner/manager), including backlog counts and oldest pending age. Historical metrics can be derived from `job_events`, job duration and run_after lag. A Prometheus/OpenTelemetry scrape endpoint is not yet provided.
- `job_events` is a processing history; `audit_log` stores user-initiated transcription request and requeue actions (in the same transaction as their state changes). Broader API activity auditing remains a separate security-hardening task.
- No automatic deletion of job/audit rows is configured. Set retention rules based on legal and privacy requirements; do not silently truncate offline sync tombstones.

## Validation

Run `go test ./...`, `go vet ./...`, `go build ./cmd/api ./cmd/worker` and Docker Compose smoke. With a disposable PostgreSQL database, set `RUN_DB_INTEGRATION=1` and `DATABASE_URL`; worker tests cover lease fencing, retries, and maximum-attempt crash recovery. For full end-to-end transcribe → extract → PDF validation, configure a controlled STT mock gateway and verified S3 audio fixture. Never point destructive integration tests at production data.
