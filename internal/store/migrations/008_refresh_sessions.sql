-- Session credentials are opaque random secrets. Only their SHA256 digests are stored.
CREATE TABLE auth_sessions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 current_token_hash TEXT NOT NULL UNIQUE,
 auth_epoch BIGINT NOT NULL,
 generation BIGINT NOT NULL DEFAULT 1,
 device_name TEXT NOT NULL DEFAULT 'Unknown device',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL,
 revoked_at TIMESTAMPTZ,
 revocation_reason TEXT
);
CREATE INDEX auth_sessions_user_recent ON auth_sessions(user_id,created_at DESC);
CREATE INDEX auth_sessions_expiry ON auth_sessions(expires_at);
CREATE TABLE used_refresh_tokens (
 token_hash TEXT PRIMARY KEY,
 session_id UUID NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
 used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX used_refresh_tokens_expiry ON used_refresh_tokens(expires_at);
-- Shared across Go replicas (not per-process) for authentication endpoint limits.
CREATE TABLE auth_rate_limits (
 scope_hash TEXT PRIMARY KEY,
 window_started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 hit_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX auth_rate_limits_cleanup ON auth_rate_limits(window_started_at);
