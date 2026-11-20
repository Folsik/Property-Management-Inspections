# iOS client

Planned: SwiftUI offline-first inspections, photo capture, PencilKit and voice notes. Backend is authoritative; sync protocol is not yet implemented.

## Authentication integration

The Go API now implements a [refresh-token rotation protocol](../../docs/auth-sessions.md) ready for the upcoming SwiftUI client. Store refresh credentials in iOS Keychain, not UserDefaults. Serialize refresh attempts, and clear stored secrets if a device session is revoked. No iOS networking code exists yet.

## Planned media upload contract

Follow the documented [presign → background PUT → confirm](../../docs/media-lifecycle.md) sequence. iOS must maintain a retryable queue of local asset IDs while offline; signed tickets expire after five minutes and must be renewed on retry. Only confirmed uploads are considered complete; verified downloads use short-lived signed GET URLs. SwiftUI upload screens are not yet implemented.
