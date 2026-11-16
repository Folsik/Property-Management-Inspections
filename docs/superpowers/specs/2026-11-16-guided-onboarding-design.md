# Guided onboarding and team invitations — design

## Intent
First-time Inspectly users (individual owners and companies) complete phone verification, PIN setup, verified email, organization creation and first property entry before seeing the dashboard. Existing accounts and team invitees must retain access.

## UX
- OTP sign-in/create account reuses current phone selection screen. After verification navigate to /onboarding; PIN login also routes through onboarding status.
- Progress is server-derived: PIN → email → organization → property → complete; verified users can resume any time, no client-stored secret.
- For private owners: organization kind 'individual', name (e.g. My properties), country, owner full name.
- For businesses: kind 'company', business name, country, contact full name; no legal ID/tax number required now.
- Property: nickname, street, city, country (unit optional). Success redirects dashboard. Existing organizations/properties stay intact; migrated legacy users grandfathered complete.
- Invited employee: verifies phone, sets PIN and email, then accepts the invite and is redirected dashboard, no duplicate org/property.
- Members: owner or manager invites by email. A short-lived (7 days), bearer capability URL is shown once and emailed by configured provider; invitee must authenticate AND verify the *same email* before acceptance. Pending invite list with revoke/reinvite; role owner-only manager invites. All token storage hashed, acceptance transactional, original token never returned in list.
- Real emails require RESEND_API_KEY and EMAIL_FROM. Local development can expose an email OTP only via same-origin loopback BFF. Never expose codes on public production endpoints.
- Public invitation preview only displays organization name and role, not recipient email, and does not grant authorization.

## Security & compatibility
- Server validates roles, membership, token expiry, email claim, one-use acceptance, and rate limits. Browser BFF limits allowed paths and validates Origin on writes.
- Go backend is authoritative for status, state transitions and access; no fake success states.
- Existing registered users marked onboarding complete during migration. New users complete steps in order and may return; existing registered member invitees not forced to create property.
- Do not force second-factor actions for already-authenticated legacy users or lose existing organizations.
- Use HTTP local developer OTP only under APP_ENV=development and WEB_LOCAL_OTP=true with loopback Host, not for public deployments.

## Delivery
Migrations and API → BFF → onboarding UI + dashboard gate → team invites + acceptance → integration and Compose smoke tests. Work first in feat/bootstrap-foundation, then PR to main after green CI.
