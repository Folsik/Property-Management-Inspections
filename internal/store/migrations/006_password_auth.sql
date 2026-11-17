ALTER TABLE users ADD COLUMN password_hash TEXT;
ALTER TABLE users ADD COLUMN failed_password_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN password_locked_until TIMESTAMPTZ;
CREATE INDEX users_password_phone_idx ON users(phone) WHERE password_hash IS NOT NULL;
