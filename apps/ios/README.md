# iOS client

Planned: SwiftUI offline-first inspections, photo capture, PencilKit and voice notes. Backend is authoritative; the Stage 3 /v1 sync contract is implemented and merged. Native SwiftUI is the next milestone.

## Authentication integration

The Go API now implements a [refresh-token rotation protocol](../../docs/auth-sessions.md) ready for the upcoming SwiftUI client. Store refresh credentials in iOS Keychain, not UserDefaults. Serialize refresh attempts, and clear stored secrets if a device session is revoked. No iOS networking code exists yet.

## Planned media upload contract

Follow the documented [presign → background PUT → confirm](../../docs/media-lifecycle.md) sequence. iOS must maintain a retryable queue of local asset IDs while offline; signed tickets expire after five minutes and must be renewed on retry. Only confirmed uploads are considered complete; verified downloads use short-lived signed GET URLs. SwiftUI upload screens are not yet implemented.

## Mobile data/sync integration (Stage 3)

Use the [v1 mobile sync contract](../../docs/api-v1-mobile.md) for cursor-based local database replication, queued idempotent item mutations and conflict resolution. Apply each pull page and persist its next cursor in one SwiftData/SQLite transaction. Client-generated item IDs and stable mutation UUIDs survive restarts; do not silently overwrite server changes. Stage 3 is present in `main` (PR #11), and the Stage 4 worker foundation is present in `main` (PR #12).
