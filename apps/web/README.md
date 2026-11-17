# Inspectly Web

Next.js App Router, Tailwind CSS v4 and shadcn/ui (Radix-based, repository-owned component sources). All operational data comes from the Go API and PostgreSQL; there are no mocked records or fake analytics.

## Start

From repository root:

```sh
docker compose up --build
```

Visit http://localhost:3000 . The web container talks privately to Go on `http://api:8080`. From `apps/web`, run `npm install && GO_API_URL=http://localhost:8080 npm run dev` for local development.

**Environment:** `GO_API_URL` is a server-side-only origin for Go. Never expose backend access tokens in `NEXT_PUBLIC_*` variables. The Next.js BFF stores Ed25519 bearer tokens in a HttpOnly/SameSite cookie and forwards authenticated API calls. It rejects cross-origin writes and unsupported API paths.

## Screen coverage

- Guided phone OTP → password → verified email → owner/company workspace → first property onboarding
- Invited employee quick path (password, email verification, accept invitation) without creating a second workspace
- Email/phone + password sign-in; SMS OTP sign-up, password change and logout
- Organization creation and switcher
- Portfolio overview with actual counts and activity
- Property creation/list/search
- Inspection creation/list/details/status transitions
- Punch-list creation, severity and status updates
- Member listing and role-limited invitations by verified email, one-time 7-day links, pending list and revocation
- S3/R2 signed direct media transfer (upload confirmation pipeline pending)
- Background jobs listing (no fabricated AI/PDF reports)
- Print-ready inspection reports and secure time-limited, revocable guest report links
- User profile name update and password management

## Infrastructure dependencies

- Existing Go API and PostgreSQL; no static fallback data
- A production OTP delivery provider is required (Twilio credentials); local Go development can provide a dev OTP outside the web UI
- S3/R2 credentials and CORS policy for browser direct PUT are required for media upload
- AI/PDF worker and customer report guest links are *not implemented*; the UI truthfully communicates unavailable functionality
- Short-lived access tokens renew automatically via 30-day rotating refresh sessions in separate HttpOnly cookies. Device revocation is available in Account settings.

## Quality checks

```sh
npm run typecheck
npm run lint
npm run build
```

### Local authentication

The root Docker Compose binds the web UI to `127.0.0.1:3000` and enables `WEB_LOCAL_OTP=true` with HTTP cookies for development only. When Twilio is not configured, the locally generated OTP is shown only on loopback requests to complete a real registration flow. **Do not use these environment overrides on public servers.**

Production deployments must use HTTPS, secure cookies (omit `SESSION_COOKIE_SECURE=false`) and Twilio credentials; set `WEB_LOCAL_OTP=false` or omit it. The web app never fabricates domain data.

### Email verification and invitations

For production supply `RESEND_API_KEY`, `EMAIL_FROM` (verified sender), and `PUBLIC_WEB_URL` (an HTTPS origin). The API sends verified-email OTP and invitation links through Resend. Do not expose API keys to the web client.

For local testing, when the provider is unset and the Go API runs with `APP_ENV=development`, the BFF reveals the email verification code **only** when `WEB_LOCAL_OTP=true` and the browser's public Host is loopback. Invitation links are also returned once to the authorized owner/manager in this environment; copy the link to invite a colleague. **Never publish local developer settings externally.**

The organization types are `individual` (private property owner) and `company`, with a country code. No company registration number or tax ID is required at signup. Existing users are grandfathered into completed onboarding by migration 005; their data is retained.

### Team access

Owner can invite managers and inspectors; manager can invite inspectors. The invitee must authenticate with phone OTP and password and verify **exactly the recipient email** to accept. Invitation tokens are random, hashed in PostgreSQL, single-use, expiring after 7 days, and revocable. Links shown after creation cannot be recovered from the invitation list. All access remains organization scoped.

## Password authentication and account recovery (Nov 2026)

- **Sign up**: international phone number -> SMS OTP -> password -> verified email -> organization and first property.
- **Sign in**: verified email or E.164 phone + password. Existing PIN-only accounts still migrate through the phone verification route without losing their workspace.
- **Password policy**: at least **15 Unicode characters**, up to **128 characters / 1024 UTF-8 bytes**. Accept Unicode and spaces; **do not require** uppercase letters, numbers or symbols. Compare complete candidate passwords with a local blocklist of known common passwords and disallow exact user email/phone identifiers. A larger compromised-password corpus integration is recommended before high-scale production.
- **Storage**: newly enrolled / updated passwords use Argon2id (19 MiB, 2 iterations, 1 lane) with random salts and NFC normalization; legacy bcrypt hashes are still verified.
- **Login defence**: five incorrect attempts temporarily lock password sign-in for 15 minutes. Legacy PINs and OTP signup cannot bypass established passwords.
- **Forgot password**: `/forgot-password` always returns a generic message (whether or not the email exists). Eligible users must receive a **random 256-bit one-use reset link** by Resend on their **verified email** AND a separate **SMS OTP** by Twilio on their pre-existing registered phone. Codes and links expire after **10 minutes**; verification has a **five-attempt limit** and requests are limited per account. Email-only or SMS-only cannot change the password. Reused links are rejected.
- Successful reset never creates a login session. It increases the user's credential epoch, making all existing access JWTs invalid, and sends a post-reset email notice. Users then sign in normally.
- Users without access to both established channels must request **assisted identity review**; no SMS-only recovery is available.
- **Production**: set `RESEND_API_KEY`, `EMAIL_FROM`, `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_FROM_NUMBER`, and `PUBLIC_WEB_URL=https://...`. Also configure edge WAF / IP-level request limiting for recovery and login. In development, `WEB_LOCAL_OTP=true` reveals reset URL and SMS OTP only on a loopback Host.
- **Security limitations**: the current local common-password blocklist is small and should be supplemented with a well-maintained breached-password dataset for production. Recovery request processing is synchronous and a timing side channel may remain despite generic responses; a durable asynchronous dispatch queue is recommended before public launch. If one verification channel is lost, assisted/manual recovery is not yet implemented.

See [NIST SP 800-63B-4](https://pages.nist.gov/800-63-4/sp800-63b.html) and [OWASP Forgot Password Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html).

### Session renewal

Next.js middleware renews protected pages near expiry; the BFF retries once on 401, rotates the refresh cookie and never exposes secrets to client JavaScript. Account settings show active device sessions. See [auth sessions](../../docs/auth-sessions.md).
