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

- Phone OTP sign-in, PIN sign-in, logout
- Organization creation and switcher
- Portfolio overview with actual counts and activity
- Property creation/list/search
- Inspection creation/list/details/status transitions
- Punch-list creation, severity and status updates
- Member listing and authorized member addition
- S3/R2 signed direct media transfer (upload confirmation pipeline pending)
- Background jobs listing (no fabricated AI/PDF reports)
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
