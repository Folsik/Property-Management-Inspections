-- CRM-lite contacts are organization-owned; property association is optional.
CREATE TABLE contacts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  property_id UUID,
  created_by UUID NOT NULL REFERENCES users(id),
  name TEXT NOT NULL CHECK (char_length(trim(name)) BETWEEN 1 AND 160),
  category TEXT NOT NULL CHECK (category IN ('owner','tenant','vendor','other')),
  email TEXT NOT NULL DEFAULT '',
  phone TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '' CHECK (char_length(notes) <= 2000),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (organization_id,property_id) REFERENCES properties(organization_id,id),
  UNIQUE (organization_id,id)
);
CREATE INDEX contacts_org_category_idx ON contacts(organization_id,category,created_at DESC);
