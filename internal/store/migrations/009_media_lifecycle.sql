-- Media lifecycle and auditable verification state. Existing pending tickets are preserved.
ALTER TABLE media_attachments DROP CONSTRAINT IF EXISTS media_attachments_state_check;
ALTER TABLE media_attachments ADD CONSTRAINT media_attachments_state_check
 CHECK (state IN ('pending','uploaded','rejected','deleted'));
ALTER TABLE media_attachments
 ADD COLUMN verified_at TIMESTAMPTZ,
 ADD COLUMN deleted_at TIMESTAMPTZ,
 ADD COLUMN rejected_at TIMESTAMPTZ,
 ADD COLUMN verification_error TEXT;
CREATE INDEX media_pending_gc_idx ON media_attachments(created_at,id) WHERE state='pending';
CREATE INDEX media_visible_idx ON media_attachments(organization_id,inspection_id,created_at DESC)
 WHERE state <> 'deleted';
