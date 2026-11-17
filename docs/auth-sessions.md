# Authentication sessions and security — Stage 1

## Go API contract

Phone OTP verification and password login return access_token, token_type, expires_at, refresh_token, refresh_expires_at and session_id.
Access JWTs are Ed25519 signed (default 15-minute lifetime), contain sid, session generation sg and auth epoch ae.
Each login starts an independent refresh session with a 256-bit random opaque credential. PostgreSQL stores only its SHA-256 digest.

- POST /v1/auth/refresh accepts {"refresh_token":"..."}, consumes the old secret, rotates atomically, returns a new pair, increments JWT generation and retains the original **absolute 30-day expiration**.
- Reuse of a consumed refresh token triggers a 401 and revokes the entire device session, including its current refresh and access credentials.
- POST /v1/auth/refresh/logout revokes a device using the current opaque refresh token even if its access JWT has expired.
- POST /v1/auth/logout revokes the device associated with the access JWT.
- GET /v1/auth/sessions lists active sessions and identifies the current session.
- DELETE /v1/auth/sessions/{uuid} immediately revokes a session owned by the current user; others cannot be accessed.
- Password change and password recovery invalidate all device sessions via auth_epoch; legacy JWTs without a session ID remain usable only until their short original expiration.

## Web BFF

Two HttpOnly SameSite=Lax cookies (inspectly_session / inspectly_refresh), Secure when serving HTTPS except explicit localhost Docker override.
Protected-page Next.js middleware renews near-expiry access and forwards the renewed cookie to server-rendered requests. The allowlisted BFF retries exactly once after 401 and forwards rotated cookies to the browser. Logout clears both cookies and revokes the server session.
Same-process concurrent refresh calls use a short-lived single-flight cache to avoid accidental rotation replay. Deploying multiple Next.js replicas requires distributed refresh coordination or sticky routing; otherwise racing requests may revoke a session.
No client-side localStorage session secrets.

## iOS integration

Store the opaque refresh token in iOS Keychain with suitable data-protection accessibility; hold the access JWT in memory. Serialize refresh calls and atomically replace credentials on success; clear both and prompt sign-in on 401/replay or revocation. Never log tokens or persist them in UserDefaults.
With strict one-use rotation, a crash between a refresh request and writing the response may require the user to sign in again. Mobile clients should design around this.

## Auth throttling

PostgreSQL-backed fixed-window counters are shared across Go replicas, keyed by route and observed peer IP (IPv6 /64). Arbitrary X-Forwarded-For values are intentionally ignored.
Because the Next.js BFF is a shared gateway, these backend-IP thresholds are defense in depth only. For production, apply stronger client-IP limits via a trusted ingress/WAF on SMS, password login, recovery and refresh endpoints. Existing phone/email OTP attempt limits and password lockouts remain.
Legacy direct phone-based member addition POST /v1/organizations/{org}/members is disabled with HTTP 410; invitations require matching verified recipient email.

## Retention and cleanup (Stage 4)

Migration 008 retains old data and creates auth_sessions, used_refresh_tokens and auth_rate_limits. Schedule cleanup of expired rows:

    DELETE FROM used_refresh_tokens WHERE expires_at < now();
    DELETE FROM auth_sessions WHERE expires_at < now() - interval '7 days';
    DELETE FROM auth_rate_limits WHERE window_started_at < now() - interval '2 days';

Do not delete active sessions. Run cleanup with observable metrics and tested database backups.

## Testing

PostgreSQL integration exercises issuance, rotation, old JWT invalidation, reuse detection, device revocation and DB-backed throttling. Docker Compose smoke covers refresh cookies, BFF renewal, onboarding, invitations, password recovery and logout.