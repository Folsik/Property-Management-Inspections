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

- Guided phone OTP → PIN → verified email → owner/company workspace → first property onboarding
- Invited employee quick path (PIN, email verification, accept invitation) without creating a second workspace
- Phone OTP sign-in, PIN sign-in, logout
- Organization creation and switcher
- Portfolio overview with actual counts and activity
- Property creation/list/search
- Inspection creation/list/details/status transitions
- Punch-list creation, severity and status updates
- Member listing and role-limited invitations by verified email, one-time 7-day links, pending list and revocation
- S3/R2 signed direct media transfer (upload confirmation pipeline pending)
- Background jobs listing (no fabricated AI/PDF reports)
- Print-ready inspection reports and secure time-limited, revocable guest report links
- User profile name update and first-time PIN setup

## Infrastructure dependencies

- Existing Go API and PostgreSQL; no static fallback data
- A production OTP delivery provider is required (Twilio credentials); local Go development can provide a dev OTP outside the web UI
- S3/R2 credentials and CORS policy for browser direct PUT are required for media upload
- AI/PDF worker and customer report guest links are *not implemented*; the UI truthfully communicates unavailable functionality
- Temporary access-token expiry currently requires signing in again; refresh sessions are future work

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

Owner can invite managers and inspectors; manager can invite inspectors. The invitee must authenticate with phone OTP/PIN and verify **exactly the recipient email** to accept. Invitation tokens are random, hashed in PostgreSQL, single-use, expiring after 7 days, and revocable. Links shown after creation cannot be recovered from the invitation list. All access remains organization scoped.
