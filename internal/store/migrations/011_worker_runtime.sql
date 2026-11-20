-- Stage 4: durable worker lifecycle; additive, retains existing jobs.
ALTER TABLE jobs ADD COLUMN lease_token UUID;
ALTER TABLE jobs ADD COLUMN finished_at TIMESTAMPTZ;
ALTER TABLE jobs ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE jobs ADD COLUMN max_attempts INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts BETWEEN 1 AND 25);
ALTER TABLE jobs ADD COLUMN idempotency_key TEXT;
ALTER TABLE jobs ADD COLUMN result JSONB CHECK (result IS NULL OR jsonb_typeof(result)='object');
ALTER TABLE jobs ADD CONSTRAINT jobs_state_v2_check CHECK (state IN ('pending','running','succeeded','failed'));
CREATE UNIQUE INDEX jobs_dedupe_idx ON jobs(organization_id,kind,idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX jobs_reclaim_idx ON jobs(locked_until) WHERE state='running';
CREATE INDEX jobs_deadletter_idx ON jobs(organization_id,finished_at DESC) WHERE state='failed';
CREATE TABLE job_events (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
 organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 event TEXT NOT NULL CHECK (event IN ('claimed','retried','succeeded','dead_lettered','requeued','lease_lost')),
 attempt INTEGER NOT NULL,
 detail TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX job_events_job_idx ON job_events(job_id,id);
CREATE TABLE audit_log (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 actor_id UUID REFERENCES users(id),
 action TEXT NOT NULL,
 resource_type TEXT NOT NULL,
 resource_id UUID,
 metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_tenant_idx ON audit_log(organization_id,id DESC);
