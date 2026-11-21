# Pre-iOS backend and frontend review (2026-11-21)

Scope: Go 1.23 API, PostgreSQL migrations, Stage 4 jobs, Next.js 15.5 BFF/auth sessions, Stage 3 mobile sync, media access and deployment pipelines. Review branch: `fix/pre-ios-security-audit` (PR #13), not merged automatically.

## Verified changes

- Concurrent API and worker startup caused an intermittent `pg_type_typname_nsp_index` collision while creating `schema_migrations`; migration bootstrap now takes the shared PostgreSQL advisory lock (Stage 4 promotion PR #12).
- The background runner contained a `ReapExhausted` routine but did not call it, leaving crashed last-attempt jobs indefinitely running; the runner now invokes the dead-letter reaper.
- Stage 4 endpoints (transcription, metrics, retry and PDF) were missing from the closed Next.js backend proxy allowlist. Routes are added with existing authentication and cross-origin write checks.
- PDF reports were accidentally passed through `response.text()`. The BFF now preserves binary `application/pdf` payloads and enforces a 1 MiB report cap and no-store headers.
- PostgreSQL CI now runs real job recovery tests before API integration tests. Running independent packages concurrently against the *same* database caused the worker lease test to claim a different package's pending transcription task. CI is serialized by package to prevent shared-fixture interference.
- Stage 4 integration tests check tenant membership, duplicate-job idempotency, byte-accurate PDF download and denial after a media object is deleted.

## Architecture / security posture

- Primary design: organization-scoped PostgreSQL modular monolith; short-lived access JWT + rotating device refresh tokens; versioned mobile /v1 sync with optimistic conflict resolution; verified immutable media; separate PostgreSQL-backed worker.
- Authenticated processing/report routes perform organization-member/role checks, and queries scope by organization and inspection; report download checks uploaded media is not deleted.
- Next.js proxies only explicitly allowlisted paths; cross-origin non-GET requests are blocked, auth credentials use HttpOnly cookies, and unexpected JSON output is not trusted.
- As of this review, package-lock resolves Next.js 15.5.29 and React 19.3.1. These are newer than the published October 2026 security patch floors (Next.js 15.5.28, React 19.3.1); this is not a substitute for continuous vulnerability scanning.

## Remaining items before production security certification

1. **AI/voice privacy and abuse:** transcription still uses a custom HTTPS STT gateway, not a fully deployed, evaluated provider; implement provider-specific authorization, egress controls, content deletion/retention, redaction and robust error classification. Raw audio is personally sensitive, so enforce contractual processing region and consent.
2. **PDF accessibility/internationalization:** current one-page ASCII fallback replaces non-ASCII characters. Replace it with a Unicode-font PDF renderer and multi-page layout before using reports for international users.
3. **Observability:** JSON metrics and slog exist, but there is no comprehensive Prometheus/OpenTelemetry exporter, trace propagation or configured alerts; add SLO alerting and dashboard.
4. **Dependency governance:** periodically run `npm audit --omit=dev`, `govulncheck ./...`, secret scanning and container image scanning in CI with pinned action versions. This review is targeted code+CI analysis, not a formal penetration test.
5. **Infrastructure:** production reverse proxy, TLS, WAF, edge client IP attribution, backup restore drill and GDPR retention enforcement require deployment-specific verification.
6. **End-to-end real provider:** STT → verbatim extraction → PDF must be exercised using an isolated test object store and controlled HTTPS gateway (mocked and real provider scenarios).

## iOS readiness

The mobile API, auth refresh/session contract, cursor sync and media lifecycle are sufficiently specified for a native iOS foundation. Begin with SwiftUI app skeleton, encrypted Keychain-based refresh token management, one in-flight refresh per device, URLSession API client, persistent offline mutation queue and safe pull cursor transactions. Do not treat completion of the current review as proof of production readiness; manual Swift/Xcode build, simulator/device and background-upload tests remain mandatory.
