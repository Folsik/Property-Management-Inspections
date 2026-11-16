# Guided Onboarding and Team Invitations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans task-by-task. TDD and CI mandatory.

**Goal:** Guided first-run onboarding and secure self-service member invitations.
**Architecture:** Go/PostgreSQL owns state; Next.js pages orchestrate existing Go API through HttpOnly BFF; signed invite bearer links are hashed at rest.
**Tech Stack:** Go 1.23, pgx, PostgreSQL 17, Next.js 15, React 19, Tailwind 4.
**Spec:** docs/superpowers/specs/2026-11-16-guided-onboarding-design.md

## Global Constraints
- Work in feat/bootstrap-foundation; merge to main only after tests.
- Keep legacy accounts and existing APIs working. Real data only.
- Security: no production dev OTP; no unverified member grants; no raw invite tokens at rest.

## Review Focus
- Wrong/replayed/expired OTP cannot verify another email.
- Invite token cannot be used by a different verified email or non-authenticated requester.
- Only owner can invite a manager; managers can invite inspectors.
- Interrupted onboarding resumes without duplicate organization/property.
- Repeated/concurrent invite acceptance does not create duplicate membership.

### Task 1: Data model and core onboarding API
Files: create 005_onboarding.sql, internal/api/onboarding.go, internal/api/email.go; modify server.go, integration_test.go.
- [ ] Add migration for email challenges, verified user email, business metadata, completion marker and invitations; update count test.
- [ ] Write failing integration tests for email OTP and onboarding status/completion; observe failure.
- [ ] Implement GET /v1/onboarding, POST /v1/onboarding/complete, POST /v1/email/request, POST /v1/email/verify; validated provider delivery.
- [ ] Run Go tests, commit.

### Task 2: Team invitations API
Files: internal/api/invitations.go, server.go, integration_test.go
- [ ] Write failing integration tests for invitation create/list/revoke and acceptance permission/expiry checks.
- [ ] Implement secure hashed token creation and acceptance, deliver email and allow local one-time copy.
- [ ] Run Go tests, commit.

### Task 3: BFF, guided UI and dashboard gate
Files: apps/web/app/onboarding/page.tsx, apps/web/components/onboarding.tsx, apps/web/app/invite/[token]/page.tsx, apps/web/app/(protected)/dashboard/page.tsx, apps/web/app/login/page.tsx, apps/web/app/api/backend/[...path]/route.ts, apps/web/lib/types.ts
- [ ] Add failing Compose registration journey test for PIN/email/org/property and invited staff.
- [ ] Expose BFF routes, redirect newly authenticated users to onboarding, use real steps and server status.
- [ ] Implement verification UI, organization kind/name/country, property entry, invite acceptance, progress and error states.
- [ ] Run web unit/typecheck/lint/build plus Compose E2E; commit.

### Task 4: Team UI
Files: apps/web/components/workspace.tsx, apps/web/lib/api.ts, apps/web/tests/registration-smoke.mjs
- [ ] Extend smoke test for invite flow; see failure.
- [ ] Replace existing registered-phone-only team add dialog with invite email + role; pending list and revocation, copy link.
- [ ] Verify backend permissions and Next.js E2E + CI, commit.
