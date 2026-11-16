CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  phone TEXT NOT NULL UNIQUE CHECK (phone ~ '^[+][1-9][0-9]{7,14}$'),
  pin_hash TEXT,
  full_name TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE otp_challenges (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  phone TEXT NOT NULL,
  code_hash TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX otp_challenges_phone_idx ON otp_challenges(phone, created_at DESC);
CREATE TABLE revoked_tokens (
  jti TEXT PRIMARY KEY,
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE organizations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL CHECK (char_length(trim(name)) BETWEEN 1 AND 120),
  created_by UUID NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE organization_members (
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('owner','manager','inspector')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id,user_id)
);
CREATE INDEX organization_members_user_idx ON organization_members(user_id);
CREATE TABLE properties (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name TEXT NOT NULL CHECK (char_length(trim(name)) BETWEEN 1 AND 160),
  address JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(address) = 'object'),
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (organization_id,id)
);
CREATE INDEX properties_org_idx ON properties(organization_id,created_at DESC);
CREATE TABLE inspections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL,
  property_id UUID NOT NULL,
  created_by UUID NOT NULL REFERENCES users(id),
  inspection_type TEXT NOT NULL CHECK (inspection_type IN ('move_in','move_out','turnover','maintenance')),
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','in_progress','completed','cancelled')),
  occurred_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (organization_id,property_id) REFERENCES properties(organization_id,id),
  UNIQUE (organization_id,id)
);
CREATE INDEX inspections_org_property_idx ON inspections(organization_id,property_id,created_at DESC);
CREATE TABLE inspection_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL,
  inspection_id UUID NOT NULL,
  client_id UUID NOT NULL,
  title TEXT NOT NULL CHECK (char_length(trim(title)) BETWEEN 1 AND 500),
  notes TEXT NOT NULL DEFAULT '',
  severity TEXT NOT NULL DEFAULT 'normal' CHECK (severity IN ('low','normal','high','critical')),
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','resolved')),
  annotations JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(annotations)='object'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (organization_id,inspection_id) REFERENCES inspections(organization_id,id) ON DELETE CASCADE,
  UNIQUE (inspection_id,client_id),
  UNIQUE (organization_id,id)
);
CREATE INDEX inspection_items_inspection_idx ON inspection_items(inspection_id,created_at DESC);
CREATE TABLE media_attachments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL,
  inspection_id UUID NOT NULL,
  item_id UUID,
  uploaded_by UUID NOT NULL REFERENCES users(id),
  object_key TEXT NOT NULL UNIQUE,
  mime_type TEXT NOT NULL CHECK (mime_type IN ('image/jpeg','image/png','image/heic','audio/mp4','audio/mpeg')),
  size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 52428800),
  state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','uploaded','rejected')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (organization_id,inspection_id) REFERENCES inspections(organization_id,id) ON DELETE CASCADE,
  FOREIGN KEY (organization_id,item_id) REFERENCES inspection_items(organization_id,id)
);
CREATE INDEX media_inspection_idx ON media_attachments(inspection_id);
CREATE TABLE jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK (kind IN ('transcribe','extract','generate_pdf')),
  state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','running','succeeded','failed')),
  payload JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(payload)='object'),
  attempts INTEGER NOT NULL DEFAULT 0,
  run_after TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_until TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX jobs_ready_idx ON jobs(run_after,created_at) WHERE state='pending';
CREATE TABLE idempotency_keys (
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  actor_id UUID NOT NULL REFERENCES users(id),
  key TEXT NOT NULL,
  request_hash BYTEA NOT NULL,
  response_code INTEGER,
  response_body JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id,actor_id,key)
);
