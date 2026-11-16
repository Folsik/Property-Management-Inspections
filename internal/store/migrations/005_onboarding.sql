-- Preserve existing users and their working sessions during onboarding rollout.
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN onboarding_completed_at TIMESTAMPTZ;
UPDATE users SET onboarding_completed_at = now() WHERE onboarding_completed_at IS NULL;
CREATE UNIQUE INDEX users_verified_email_unique ON users(lower(email)) WHERE email_verified_at IS NOT NULL;

ALTER TABLE organizations ADD COLUMN kind TEXT NOT NULL DEFAULT 'company'
    CHECK (kind IN ('individual','company'));
ALTER TABLE organizations ADD COLUMN country TEXT NOT NULL DEFAULT 'GB'
    CHECK (country ~ '^[A-Z]{2}$');

CREATE TABLE email_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX email_challenges_recent_idx ON email_challenges(user_id, created_at DESC);

CREATE TABLE organization_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('manager','inspector')),
    token_hash TEXT NOT NULL UNIQUE,
    created_by UUID NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    accepted_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX organization_invitations_by_org_idx ON organization_invitations(organization_id,created_at DESC);
