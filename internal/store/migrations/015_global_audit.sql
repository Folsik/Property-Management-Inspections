-- Auth events occur before a user has an organization.
ALTER TABLE audit_log ALTER COLUMN organization_id DROP NOT NULL;
CREATE INDEX audit_log_actor_recent ON audit_log(actor_id,id DESC) WHERE actor_id IS NOT NULL;
