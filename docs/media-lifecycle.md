# Media Service — stage 2 (development branch)

## Security model

Buckets must remain **private**. Browser/mobile clients never receive the S3 access key or secret. They receive short-lived server-scoped, SigV4-signed URLs, never arbitrary object names.

The API supports JPEG, PNG, HEIC, AAC/M4A audio (audio/mp4), and MP3 (audio/mpeg) up to **50 MiB**. Video uploads are intentionally not enabled.

**Upload lifecycle:**
1. Authenticate as an organization owner, manager, or inspector. POST `/v1/organizations/{org}/inspections/{inspection}/media/presign` with `{"mime_type":"image/jpeg","size_bytes":1234}`. The API checks membership, inspection scope, type/size and generates a server-side randomized staging key. It returns `id`, a PUT URL valid for 300 seconds, and exact Content-Type header.
2. PUT the file bytes directly to that URL using the returned headers. On iOS, use a background URLSession upload task and persist the media ID locally until confirmed.
3. POST `/v1/organizations/{org}/inspections/{inspection}/media/{id}/confirm`. The Go API makes a **signed S3 HEAD** request to check physical existence, Content-Length and Content-Type. It makes a signed **ranged GET** for 64 bytes to validate file signature (JPEG, PNG, HEIC, MP4/MP3). It rejects non-matching MIME/signatures/sizes.
4. After validation, the Go API executes a server-authorized **S3 CopyObject** into a new random **verified-only key**, rechecks the copy and only then marks the database row `uploaded`. **No client ever receives a PUT for the verified key.** Replaying a staging PUT after confirmation cannot alter the downloadable object.
5. Authorized clients request GET `/v1/organizations/{org}/inspections/{inspection}/media/{id}/download`. The API returns a GET URL valid for **60 seconds** and a MIME type. Ineligible tenants get HTTP 403 or 404. Pending/rejected objects are never downloadable.
6. DELETE `/v1/organizations/{org}/inspections/{inspection}/media/{id}` performs a signed storage deletion and marks the row `deleted`, with deleted timestamp. Repeated deletion returns a success status; deleted rows are excluded from media listings.

**Cleanup:** the Go server starts a maintenance loop at startup and repeats every five minutes. With `FOR UPDATE SKIP LOCKED`, it processes at most 50 abandoned tickets per sweep across replicas. Pending tickets and invalid/rejected staging objects are deleted **after 20 minutes** (beyond the 5-minute PUT expiry). Verified files retain their immutable copy; their original staging objects are deleted after the PUT has expired. Storage errors are retryable and never silently mark a still-stored object as deleted.

## Configure S3/R2

The Go API requires `S3_ENDPOINT=https://...`, `S3_BUCKET`, `S3_REGION`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`. Do not enable public bucket listing or reads. Only allow trusted service credentials to CopyObject and DeleteObject.

Configure S3/R2 CORS to allow the **frontend origin** to PUT directly with `Content-Type`. The Go API conducts HEAD, ranged GET, CopyObject and DELETE server-to-server; those operations do not need browser CORS permissions.

Recommended bucket lifecycle rules: retain verified objects under the product retention policy; expire temporary staging prefixes and any orphaned server-side copies after a documented reconciliation window. A failed network or database commit can leave an unreferenced object; bucket reconciliation is needed for strict orphan cleanup.

## iOS integration

Persist the media ID and local draft in a durable upload queue. Use a background URLSession to PUT the file, then call confirm. When the app restarts, retry confirm on its saved ID; an already verified attachment returns HTTP 200. Expired pending tickets return 409 and must be restarted with a new presign. Never assume a successful PUT means the media is visible to others. Downloads and deletions require a fresh bearer access JWT; reuse the existing refresh-session mechanism.

## Residual limitations

Header-based magic-byte validation is not antivirus scanning or a complete codec decoder. For arbitrary public media sharing, add antivirus/metadata sanitization and content-disarm pipelines before exposure. S3/R2 account lifecycle rules and managed-backup policies are deployment configuration, not baked into Docker Compose. The development Docker environment intentionally does not configure S3 credentials; presign responds with 503 until storage is supplied.

## Validation

`go test ./internal/api` with `RUN_DB_INTEGRATION=1` checks simulated HTTPS S3: missing object, valid upload, immutable verified copy, MIME spoofing and size mismatch rejection, idempotent confirmation, tenant isolation, signed retrieval, explicit deletion, and stale staged-object cleanup. Unit tests cover SigV4 presign URL shape and media header signatures. Normal Docker Compose smoke still covers onboarding and authentication without requiring S3 credentials.
