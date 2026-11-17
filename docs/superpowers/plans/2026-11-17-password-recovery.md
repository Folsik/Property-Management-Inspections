# NIST passwords and dual-channel recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans task-by-task. Use TDD and verification-before-completion.

**Goal:** Strong NIST-aligned passwords and email+SMS verification for safe password reset.
**Architecture:** Go API stores Argon2id secrets, PostgreSQL stores one-use recovery challenges, Next.js BFF authenticates origin and serves public reset UX; no SMS-only account bypass.
**Tech Stack:** Go, pgx, PostgreSQL, argon2, NFC, Next.js 15.
**Spec:** docs/superpowers/specs/2026-11-17-password-recovery.md

## Global Constraints
- Existing users and bcrypt credentials remain usable.
- All recovery answers are generic for unknown/unverified users.
- No session granted by password recovery; invalidate current sessions on success.
- Changes first on feat/bootstrap-foundation; merge only after green CI.

## Task 1: Policy/hashing
- [ ] Write failing policy and hashing tests (15 Unicode runes, passphrases, 64+ Unicode, common exact blocklist, bcrypt verification).
- [ ] Implement NFC, Argon2id PHC encode/verify, legacy bcrypt verification.
- [ ] Update setup/change/login paths and frontend password helpers and labels.
- [ ] Verify unit, integration, Next.js checks.

## Task 2: Recovery API and migration
- [ ] Write failing integration test for generic request, two-factor recovery, replay, session invalidation.
- [ ] Add migration 007 and new recovery endpoints with atomic challenge consumption.
- [ ] Integrate Resend and Twilio, dev-only local token/code; limit request/attempts.
- [ ] Verify Go and PG checks.

## Task 3: BFF, pages, integration, delivery
- [ ] Add public /forgot-password and /reset-password pages and no-referrer headers.
- [ ] Wire BFF recovery/request and recovery/complete with origin and developer-code restrictions.
- [ ] Extend Docker Compose E2E smoke; verify error/expiration/replay paths.
- [ ] Update documentation, remove temporary CI format workflow, run all CI checks.
- [ ] PR merge to main and sync integration branch.

## Implementation checkpoint
- Migration 007 stores hashed, expiring recovery capabilities, and auth_epoch on users.
- Go password API has Argon2id+NFC and bcrypt verification compatibility.
- Separate dual-channel recovery endpoints and Next.js pages added.
- Live Docker Compose test to verify reset and previous JWT revocation.
- See CI on integration branch for validation before merge.
