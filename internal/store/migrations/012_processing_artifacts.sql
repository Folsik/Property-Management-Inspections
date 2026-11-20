-- Stage 4 artifacts scoped to the same organization and inspection as their sources.
CREATE TABLE inspection_transcripts (
 organization_id UUID NOT NULL,
 inspection_id UUID NOT NULL,
 media_id UUID NOT NULL REFERENCES media_attachments(id),
 text TEXT NOT NULL CHECK (char_length(text)<=65536),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id,inspection_id,media_id),
 FOREIGN KEY (organization_id,inspection_id) REFERENCES inspections(organization_id,id) ON DELETE CASCADE
);
CREATE TABLE inspection_extractions (
 organization_id UUID NOT NULL,
 inspection_id UUID NOT NULL,
 media_id UUID NOT NULL,
 notes JSONB NOT NULL CHECK (jsonb_typeof(notes)='array'),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id,inspection_id,media_id),
 FOREIGN KEY (organization_id,inspection_id,media_id) REFERENCES inspection_transcripts(organization_id,inspection_id,media_id) ON DELETE CASCADE
);
CREATE TABLE inspection_generated_reports (
 organization_id UUID NOT NULL,
 inspection_id UUID NOT NULL,
 media_id UUID NOT NULL,
 content BYTEA NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 1048576),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY (organization_id,inspection_id,media_id),
 FOREIGN KEY (organization_id,inspection_id,media_id) REFERENCES inspection_transcripts(organization_id,inspection_id,media_id) ON DELETE CASCADE
);
