-- Credential revisions invalidate all stateless sessions after account recovery.
ALTER TABLE users ADD COLUMN auth_epoch BIGINT NOT NULL DEFAULT 0;
CREATE TABLE password_recoveries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  sms_code_hash TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX password_recoveries_user_created ON password_recoveries(user_id,created_at DESC);
