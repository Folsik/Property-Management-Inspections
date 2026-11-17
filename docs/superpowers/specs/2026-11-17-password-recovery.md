# Password hardening and verified account recovery (17 November 2026)

## Passwords (NIST SP 800-63B-4)
- Require >=15 Unicode code points for single-factor login; accept 64+ code points and spaces, punctuation, Unicode, paste and password managers; no uppercase/digit/symbol requirements.
- Reject entire passwords found in a local common/compromised password blocklist, and passwords equal to user identifiers; no substring dictionary filters.
- Apply NFC Unicode normalization before hashing and verifying. Newly enrolled passwords use Argon2id with per-user random salt and constant-time digest comparison; retain support for existing bcrypt hashes on sign-in and convert upon password change.
- Cap 128 runes and 1024 UTF-8 bytes to avoid abuse. No periodic changes.

## Separate recovery
- Initiate only for accounts with verified email and enrolled password. User enters verified email, receives short-lived, cryptographically random one-use recovery link via Resend and a distinct six-digit code via Twilio to enrolled phone; both must be verified together before password is reset. Neither email link nor SMS alone grants a session or changes credentials.
- Success response is identical for existing and nonexistent email; no account enumeration in UI. Per-account request limits, code attempt limits, consumed/expired tokens and one active request per account. No raw tokens/OTP stored, no tokens in logs, no session issued on reset.
- For local Docker development, tokens and SMS code are surfaced only on loopback through BFF and only with WEB_LOCAL_OTP; production requires configured email and SMS providers.
- On completion reset password, invalidate all prior JWTs using user credential epoch/version check; direct user to normal sign-in. Notify verified email.
- Existing PIN-only accounts continue old migration flow. Accounts lacking verified email cannot self-reset; need assisted identity review (not an SMS-only bypass).
- Public recovery pages have no-referrer/noindex/no-store headers.

## Backend
- New migration 007: password_reset_requests (user_id, hash of URL token, hash of SMS code, attempts, expiry, used_at, request metadata), users credential epoch or revision.
- POST /v1/auth/recovery/request {email} -> always 202 generic; POST /v1/auth/recovery/complete {token,code,password} -> 200 or generic error; no implicit auth.
- Sensitive actions protected from CSRF in Next.js BFF; requests length limited.

## Acceptance
- Tests reject SMS-only, email-only, incorrect, expired and replay attempts and prove session revocation.
- Tests new passphrase with spaces, all lowercase, Unicode, 15 runes and >72-byte input, and reject common exact passwords.
- Existing account can still sign in with unchanged legacy hash.
- All CI workflows pass before promotion from feat/bootstrap-foundation to main.
