-- Existing challenges were historically committed only after delivery.
ALTER TABLE otp_challenges ADD COLUMN delivery_confirmed_at TIMESTAMPTZ;
UPDATE otp_challenges SET delivery_confirmed_at=created_at;

ALTER TABLE email_challenges ADD COLUMN delivery_confirmed_at TIMESTAMPTZ;
UPDATE email_challenges SET delivery_confirmed_at=created_at;
