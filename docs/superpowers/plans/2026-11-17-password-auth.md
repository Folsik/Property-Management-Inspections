# Sign in / Sign up with passwords

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Test-first.

**Goal:** Replace public PIN login and account PIN onboarding with password-based credentials.
**Architecture:** Go/PostgreSQL is verifier, Next.js BFF stores JWT HttpOnly cookie, UI routes sign-up to existing OTP onboarding.
**Tech Stack:** Go bcrypt/pgx, PostgreSQL 17, Next.js 15 React 19.
**Spec:** docs/superpowers/specs/2026-11-17-password-auth.md

## Global Constraints
- Maintain existing organization data and legacy user access through phone OTP migration.
- No password bypass after password enrollment. No account enumeration.
- Rate-limit failed authentication and protect cookie/session and BFF origin.
- User-requested complexity policy plus a common-password blocklist; separate note NIST recommends longer passwords without composition rules.

## Review Focus
- Existing password user cannot sign in through OTP sign-up bypass.
- Old six-digit PIN cannot unlock account after password enrollment.
- Password identifier supports E.164 or verified email only; unverified email fails.
- Incorrect attempts lock account; valid credentials after expiry succeed.
- Short/common/72-byte exceeding passwords rejected; no plaintext storage.

### Task 1: Backend and data
**Files:** create internal/store/migrations/006_password_auth.sql, internal/api/password.go, internal/api/password_test.go; modify internal/api/server.go, internal/api/onboarding.go, internal/api/integration_test.go
- [ ] Add failing password-policy unit tests and new integration tests for password setup/login/email/phone/replay/lock.
- [ ] Verify failures in CI.
- [ ] Implement policy, bcrypt hash, login lockout, login routing; update onboarding status and completion.
- [ ] Verify tests, gofmt, commit.

### Task 2: Next.js BFF and UX
**Files:** modify apps/web/app/login/page.tsx, apps/web/components/onboarding.tsx, apps/web/app/api/auth/[action]/route.ts, apps/web/app/api/backend/[...path]/route.ts, apps/web/lib/types.ts
- [ ] Extend Docker registration smoke test for signup password and email/phone login, and verify error/blocked OTP bypass.
- [ ] Observe failure prior to implementation.
- [ ] Replace tabs with Sign in / Sign up, create password step in onboarding and password rules UI.
- [ ] Run typecheck/lint/build and full real Docker smoke.

### Task 3: Documentation and promotion
**Files:** README.md, apps/web/README.md, docs/superpowers/
- [ ] Describe credential migration and provider requirements.
- [ ] Confirm all four checks green on feature HEAD.
- [ ] PR into main and synchronize integration branch.

## Implementation checkpoint
- Backend migration 006, password set/login/change, legacy PIN safeguards implemented.
- New Sign in / Sign up login and password onboarding implemented.
- Existing profile and property records retained; migration does not discard old PINs.
- Unit and PostgreSQL integration tests plus Docker registration smoke cover new password path; CI gates determine promotion to main.
