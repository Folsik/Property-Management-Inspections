# Inspectly iOS — Sprint 1 foundation

Native SwiftUI application targeting iOS 17+. This branch (`feat/ios-foundation`) is an initial, intentionally narrow vertical slice. Stage 1 includes email/phone password login to the Go `/v1` API, secure session restoration, refresh-token rotation, profile and organization listing and sign-out.

## Open in Xcode

1. Install Xcode 16+ and [XcodeGen](https://github.com/yonaskolb/XcodeGen) on macOS (for example, `brew install xcodegen`).
2. From `apps/ios`, run `xcodegen generate --spec project.yml`.
3. Open `Inspectly.xcodeproj`, choose an iOS 17+ Simulator, and run `Inspectly`.
4. Debug uses `http://localhost:8080`; start the existing Go API/Docker Compose service on your Mac first. On a physical iPhone, use a reachable HTTPS host, not localhost.
5. Release currently uses the deliberate invalid placeholder `https://api.example.invalid`. Configure a real HTTPS API and signing/team settings before any distribution.

The project is generated, not checked in; source of truth is `project.yml`. GitHub Actions builds with the iOS Simulator SDK without code signing. This branch remains isolated from `main` until native CI and owner review.

## Security model

- Access JWT remains **in memory only**.
- Each device's opaque refresh token is persisted in **ThisDeviceOnly Keychain**, accessible after first device unlock. No `UserDefaults`, local storage or token logging.
- An actor owns the token state and serializes refresh requests. Each authenticated read retries at most once on HTTP 401; an invalid refresh (401) clears local credentials and requires sign-in.
- Transient backend/network failures do not falsely sign the user out or erase a potentially valid token.
- Sign-out revokes the current device's refresh session server-side before removing credentials. If revocation is unavailable, the UI reports the failure instead of falsely claiming logout.
- The Go server remains authoritative for JWT validity, membership, tenant access and authorization.

## Next implementation slices

- Onboarding: phone OTP, passphrase creation, verified email and organization join/create using existing documented endpoints.
- Organization/inspection list and details, local SwiftData or SQLite storage, cursor-based `sync/pull` with atomic cursor commits, offline `sync/push` queue with stable mutation UUIDs and explicit HTTP 409 conflict UI.
- Camera and voice notes using the existing verified S3 presign → PUT → confirm contract, with persisted iOS background tasks.
- PDF preview, accessibility, localization, Face ID convenience unlock and structured telemetry.

The current sprint does **not** yet include complete onboarding, offline sync, media capture, push notifications or App Store delivery. Do not enable production service endpoints or ship without testing in Xcode and on devices.

References: [mobile API](../../docs/api-v1-mobile.md), [device sessions](../../docs/auth-sessions.md), [media lifecycle](../../docs/media-lifecycle.md).
