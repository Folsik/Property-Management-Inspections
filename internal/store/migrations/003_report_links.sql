-- A public guest sees only a time-limited, explicitly shared inspection report.
-- Bearer secrets are never stored in plaintext.
CREATE TABLE report_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL,
  inspection_id UUID NOT NULL,
  created_by UUID NOT NULL REFERENCES users(id),
  token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (organization_id, inspection_id)
    REFERENCES inspections(organization_id,id) ON DELETE CASCADE,
  UNIQUE (organization_id,id)
);
CREATE INDEX report_links_inspection_idx
  ON report_links(organization_id,inspection_id,created_at DESC);
