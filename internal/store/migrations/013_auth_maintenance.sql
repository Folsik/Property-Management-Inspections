CREATE INDEX IF NOT EXISTS revoked_tokens_expiry ON revoked_tokens(expires_at);
CREATE INDEX IF NOT EXISTS otp_challenges_expiry ON otp_challenges(expires_at);
CREATE INDEX IF NOT EXISTS email_challenges_expiry ON email_challenges(expires_at);
CREATE INDEX IF NOT EXISTS password_recoveries_expiry ON password_recoveries(expires_at);
