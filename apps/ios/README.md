# iOS client

Planned: SwiftUI offline-first inspections, photo capture, PencilKit and voice notes. Backend is authoritative; sync protocol is not yet implemented.

## Authentication integration

The Go API now implements a [refresh-token rotation protocol](../../docs/auth-sessions.md) ready for the upcoming SwiftUI client. Store refresh credentials in iOS Keychain, not UserDefaults. Serialize refresh attempts, and clear stored secrets if a device session is revoked. No iOS networking code exists yet.
