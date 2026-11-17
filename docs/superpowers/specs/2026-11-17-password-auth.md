# Password-based sign-in and sign-up

- Public login page offers Sign in and Sign up, not SMS code vs PIN.
- Sign up: phone country picker -> SMS OTP -> /onboarding password setup -> verified email -> organization -> property.
- Sign in: verified email or E.164 phone + password; never create accounts through sign-in.
- Existing accounts: preserve orgs/properties; users with old PIN can verify phone to establish a password. Existing password accounts cannot bypass password through OTP signup.
- Password: min 12 characters, at least one uppercase Latin letter, a number and a symbol (user request), cap 72 UTF-8 bytes for bcrypt. Reject common breached-style patterns and repeated/keyboard sequences, password matching email/phone. Confirm password; allow paste/password managers. Messages avoid user enumeration.
- Bcrypt at rest; no plaintext logging; failed login counter and temporary lockout. Legacy PIN login disabled once a password is set.
- API: POST /v1/auth/password (authenticated initial setup), POST /v1/auth/password/login (anonymous). New status password_set; completion requires password.
- No password reset via email in this change: existing verified-phone OTP is available for legacy migration only. Full recovery will require a separate secure process.
- All code goes to feat/bootstrap-foundation, CI green prior to main.
