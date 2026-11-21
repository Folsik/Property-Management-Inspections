# Inspectly iOS — Sprint 1 foundation

Native SwiftUI application targeting iOS 17+. This branch (`feat/ios-foundation`) is an initial, intentionally narrow vertical slice. The current vertical slice includes phone OTP registration, password enrollment, email/phone password login, secure session restoration, refresh-token rotation, profile and organization management, property and inspection lists, inspection creation, issue creation and server-versioned status updates via Go `/v1`.

## Open in Xcode

1. Install Xcode 26.2 (your installed version is supported; the CI runner may use another compatible Xcode) and [XcodeGen](https://github.com/yonaskolb/XcodeGen) on macOS (for example, `brew install xcodegen`).
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

- Onboarding follow-up: verified email, invitations and recovery UI. Phone OTP, password enrollment and organization creation are implemented.
- Organization/inspection list and details, local SwiftData or SQLite storage, cursor-based `sync/pull` with atomic cursor commits, offline `sync/push` queue with stable mutation UUIDs and explicit HTTP 409 conflict UI.
- Camera and voice notes using the existing verified S3 presign → PUT → confirm contract, with persisted iOS background tasks.
- PDF preview, accessibility, localization, Face ID convenience unlock and structured telemetry.

The current sprint has phone signup and online property/inspection workflows. It does **not** yet include offline sync, camera/media capture, full email/invitation onboarding, push notifications or App Store delivery. Do not enable production service endpoints or ship without testing in Xcode and on devices.

References: [mobile API](../../docs/api-v1-mobile.md), [device sessions](../../docs/auth-sessions.md), [media lifecycle](../../docs/media-lifecycle.md).

## Xcode 26.2 and simulator configuration

The XcodeGen spec declares Swift 5.9 language mode and iOS 17 as a minimum deployment target. This is intentional for compatibility and is not a requirement to use Xcode 16. Xcode 26.2 can open the generated project. Configure a Personal Team in Signing & Capabilities for on-device testing; GitHub Actions uses `CODE_SIGNING_ALLOWED=NO` only for simulator builds.

The local API default is `http://localhost:8080`, valid **only in the simulator running on the same Mac as Docker/Go**. On a real iPhone, `localhost` points at the phone. Use a reachable HTTPS origin and update `InspectlyAPIBaseURL` in the XcodeGen build configuration, then regenerate the project. Production uses a deliberately invalid placeholder until a trusted deployment address is configured.

There are no fabricated demo records: the lists come from authenticated Go endpoints. A missing tenant or property is represented by an empty state. The account created through phone OTP requires the production Twilio provider in nondevelopment deployments; local dev API may return a test OTP for simulator use only.
