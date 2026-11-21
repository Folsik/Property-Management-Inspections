# Inspectly mobile API v1 — Stage 3 contract

Status: **Stage 3 approved and merged to `main` via PR #11 on 2026-11-20**. Stage 4 was approved and merged to `main` via PR #12 on 2026-11-21.

## Authentication and tenancy

Every endpoint is under `/v1`. Protected requests send `Authorization: Bearer <access_token>`; a short-lived token can be renewed with the previously implemented rotating refresh-token protocol. `{org}` is a UUID. Go verifies current organization membership on **every** sync and entity route; a valid token from another tenant does not grant access. Only a member of an organization may pull its private changes. Roles owner, manager and inspector can mutate inspection items only while their inspection is draft/in progress.

Media URLs, refresh secrets and S3 object keys are not included in sync payloads. `media_attachment` changes contain public attachment metadata only. To download media, use the authenticated, tenant-scoped download URL endpoint and respect its 60-second expiry.

## Entity version contract

`property`, `inspection`, `inspection_item` and `media_attachment` entities now have:

- `updated_at` — server-assigned UTC timestamp, returned as an ISO 8601 time.
- `version` — positive 64-bit server revision, initial 1, incremented on any server-side update.
- `deleted_at` on properties, inspections and items; media already has `state:deleted` and `deleted_at`.
- **Tombstone** — `deleted:true` in `sync/pull`; clients must delete/invalidate their local copy, never resurrect an item from a stale offline draft.

Old web PATCH clients can temporarily omit `version` for compatibility. **New mobile clients MUST send the last observed `version`**; outdated versions receive HTTP 409 with `{ "error": "...", "current": <server entity> }`. Direct DELETE of an item always requires a version. The server is authoritative; there is no last-write-wins timestamp race or CRDT at this stage.

The initial migration backfills `sync_changes` for existing properties, inspections, items and media, so a new device can bootstrap its local store without special server migration logic.

## Cursor pagination (server keyset, not OFFSET)

The following authenticated list routes support `?limit=N&cursor=<opaque>`:

- GET `/v1/organizations/{org}/properties` (default/max 100/200)
- GET `/v1/organizations/{org}/inspections` (default/max 200/200)
- GET `/v1/organizations/{org}/inspections/{inspection}/items` (default/max 500/500)
- GET `/v1/organizations/{org}/inspections/{inspection}/media` (default/max 200/200)

Response remains `{"items":[...],"next_cursor":"...","has_more":false}` for compatibility. The pagination key is `(created_at,id)` descending and unique; cursor binds tenant and collection scope. Never parse or edit the cursor in an iOS app. Use `has_more` to decide whether to continue.

`limit` must be 1..max, otherwise HTTP 400. An invalid or cross-tenant collection cursor is HTTP 400. Existing list routes for members, contacts and jobs retain their older limits; they will receive pagination when their mobile workflows are implemented.

## Change feed: GET /v1/organizations/{org}/sync/pull

Query:

- `cursor` omitted/empty = initial history (including migration backfill).
- `limit` = 1..500; default 100.

Response:

```json
{
  "changes":[
    {"seq":1,"entity":"inspection_item","id":"<uuid>","version":2,
     "deleted":false,"data":{"id":"<uuid>","status":"resolved","version":2},
     "updated_at":"2026-11-20T00:00:00Z"}
  ],
  "next_cursor":"<opaque>",
  "has_more":false
}
```

`entity` is `property`, `inspection`, `inspection_item` or `media_attachment`. Each change has its full current row snapshot (safe subset for media). Changes are sorted by increasing server sequence. The server uses per-organization transaction-scoped advisory locks before version updates and event allocation, preventing visible out-of-order commits for one organization.

**Device algorithm:** open a local database transaction → apply changes in order (including tombstones) → save `next_cursor` within the same local transaction → commit. A crash before commit must reapply the same page (upsert by `entity,id,version`). Continue while `has_more=true`. A client must not advance its cursor before writing entities. Pull requests are authenticated each time; there is no public feed or account-wide cross-tenant cursor.

Change events currently have indefinite retention. When retention and compacted snapshots are implemented, cursor-expiry semantics must be versioned and explicitly documented before pruning. Do not silently discard tombstones needed by offline clients.

## Upload pending mutations: POST /v1/organizations/{org}/sync/push

The JSON body has 1..50 `mutations`. Stage 3 supports **inspection_item** `create`, `update`, `delete` operations (other entity mutations use their existing online REST routes). Each mutation MUST have a new stable UUID `mutation_id` for each distinct intent, an `entity:"inspection_item"`, `action`, and `inspection_id`.

Create (idempotent with a device-generated `client_id` UUID):

```json
{"mutations":[{"mutation_id":"<uuid>","entity":"inspection_item","action":"create",
 "inspection_id":"<uuid>","client_id":"<uuid>","title":"Sink inspection",
 "notes":"Water pressure low","severity":"normal"}]}
```

Update (optimistic concurrency):

```json
{"mutations":[{"mutation_id":"<uuid>","entity":"inspection_item","action":"update",
 "inspection_id":"<uuid>","id":"<uuid>","version":3,"status":"resolved"}]}
```

Delete (soft-delete tombstone):

```json
{"mutations":[{"mutation_id":"<uuid>","entity":"inspection_item","action":"delete",
 "inspection_id":"<uuid>","id":"<uuid>","version":3}]}
```

Valid sync/push batches return HTTP 200 with ordered per-mutation `results`: each has `mutation_id`, `entity`, `action`, numeric `status` (200/201/409/404), `id` and `version` on success, and `current` on a 409 stale-version conflict. Each operation has its own DB transaction and idempotency record. **A batch is not all-or-nothing.** The client must inspect all results and retain unresolved queue entries. Syntactically invalid requests return HTTP 400; reusing the same `mutation_id` with different content returns HTTP 409 for the request.

An idempotency key is bound to the authenticated actor, tenant and canonical mutation body (SHA-256), and its committed result is stored in PostgreSQL. Retrying **the exact same mutation** after timeout returns the same result and does not apply it twice. Another operation with the same ID and modified payload is rejected; to resolve a 409, the user reviews the `current` server entity and submits a **new** mutation ID with the newly observed version. Do not silently overwrite a teammate's work.

Direct `POST /inspections/{inspection}/items` with the same client_id and identical contents now returns the existing ID/version without updating the row or writing a duplicate sync event.

## HTTP errors

- 400 — invalid JSON, cursor, limit, missing version or unsupported mutation.
- 401 — missing/invalid/expired bearer credential.
- 403 — nonmember or insufficient organization role.
- 404 — absent/deleted entity in its scoped organization.
- 409 — outdated version, idempotency key reuse, existing client_id mismatch, or invalid state transition. Version conflicts on item REST PATCH include the current server entity.
- 503 — temporary storage/database/backend failure. Retry only safe idempotent requests with their original mutation IDs.

## Data and release operations

The database migration is additive (`010_mobile_sync.sql`). It backfills existing objects and increments `schema_migrations` to 10. **Branch isolation does not isolate the local PostgreSQL schema**; before running a newer branch against valuable data, take a database backup. The index and trigger changes can lock tables while migrations run; production rollout needs a maintenance window or online migration plan.

This contract defines **online-first mobile readiness with an incremental offline item queue**, not general CRDT syncing of arbitrary records, conflict-free collaborative editing, automatic purge/rebuild, or a complete SwiftUI implementation. Billing remains a separate roadmap stage. Worker transcription and minimal PDF generation are implemented in Stage 4; advanced AI extraction remains planned.
