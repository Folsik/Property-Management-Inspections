-- Stage 3: versioned durable mobile sync without breaking existing web clients.
ALTER TABLE properties ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE properties ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0);
ALTER TABLE properties ADD COLUMN deleted_at timestamptz;
ALTER TABLE inspections ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE inspections ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0);
ALTER TABLE inspections ADD COLUMN deleted_at timestamptz;
ALTER TABLE inspection_items ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE inspection_items ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0);
ALTER TABLE inspection_items ADD COLUMN deleted_at timestamptz;

CREATE TABLE sync_changes (
 seq bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 entity_type text NOT NULL CHECK(entity_type IN ('property','inspection','inspection_item')),
 entity_id uuid NOT NULL,
 version bigint NOT NULL,
 deleted boolean NOT NULL,
 payload jsonb NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sync_changes_org_seq ON sync_changes(organization_id,seq);
-- Prevent missing events when concurrent transactions commit out of order:
-- each tenant's mutations acquire a transaction-scoped lock before versions
-- and event sequence allocation. Adjacent tenants proceed independently.
CREATE FUNCTION sync_version_stamp() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.organization_id::text, 314159));
 IF TG_OP='UPDATE' THEN
  NEW.version:=OLD.version+1;
 END IF;
 NEW.updated_at:=clock_timestamp();
 RETURN NEW;
END $$;
CREATE FUNCTION sync_emit_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE category text;
BEGIN
 category:=CASE TG_TABLE_NAME WHEN 'properties' THEN 'property'
 WHEN 'inspections' THEN 'inspection'
 WHEN 'inspection_items' THEN 'inspection_item' END;
 INSERT INTO sync_changes(organization_id,entity_type,entity_id,version,deleted,payload,updated_at)
 VALUES(NEW.organization_id,category,NEW.id,NEW.version,NEW.deleted_at IS NOT NULL,
 to_jsonb(NEW),NEW.updated_at);
 RETURN NULL;
END $$;
CREATE TRIGGER properties_version BEFORE INSERT OR UPDATE ON properties
 FOR EACH ROW EXECUTE FUNCTION sync_version_stamp();
CREATE TRIGGER inspections_version BEFORE INSERT OR UPDATE ON inspections
 FOR EACH ROW EXECUTE FUNCTION sync_version_stamp();
CREATE TRIGGER inspection_items_version BEFORE INSERT OR UPDATE ON inspection_items
 FOR EACH ROW EXECUTE FUNCTION sync_version_stamp();
CREATE TRIGGER properties_changed AFTER INSERT OR UPDATE ON properties
 FOR EACH ROW EXECUTE FUNCTION sync_emit_change();
CREATE TRIGGER inspections_changed AFTER INSERT OR UPDATE ON inspections
 FOR EACH ROW EXECUTE FUNCTION sync_emit_change();
CREATE TRIGGER inspection_items_changed AFTER INSERT OR UPDATE ON inspection_items
 FOR EACH ROW EXECUTE FUNCTION sync_emit_change();

-- Existing records must appear on the first sync, even if never edited.
-- Migrations execute serially, so initial event order is deterministic.
INSERT INTO sync_changes(organization_id,entity_type,entity_id,version,deleted,payload,updated_at)
 SELECT organization_id,'property',id,version,false,to_jsonb(p),updated_at FROM properties p
 ORDER BY created_at,id;
INSERT INTO sync_changes(organization_id,entity_type,entity_id,version,deleted,payload,updated_at)
 SELECT organization_id,'inspection',id,version,false,to_jsonb(i),updated_at FROM inspections i
 ORDER BY created_at,id;
INSERT INTO sync_changes(organization_id,entity_type,entity_id,version,deleted,payload,updated_at)
 SELECT organization_id,'inspection_item',id,version,false,to_jsonb(x),updated_at FROM inspection_items x
 ORDER BY created_at,id;
CREATE INDEX properties_active_seek ON properties(organization_id,created_at DESC,id DESC) WHERE deleted_at IS NULL;
CREATE INDEX inspections_active_seek ON inspections(organization_id,created_at DESC,id DESC) WHERE deleted_at IS NULL;
CREATE INDEX inspection_items_active_seek ON inspection_items(organization_id,inspection_id,created_at DESC,id DESC) WHERE deleted_at IS NULL;
