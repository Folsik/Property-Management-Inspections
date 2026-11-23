-- Encrypt rotated responses at the application layer; never store plaintext credentials.
ALTER TABLE used_refresh_tokens ADD COLUMN replay_response BYTEA;
ALTER TABLE used_refresh_tokens ADD COLUMN replay_until TIMESTAMPTZ;
CREATE INDEX used_refresh_tokens_replay_until ON used_refresh_tokens(replay_until) WHERE replay_response IS NOT NULL;
