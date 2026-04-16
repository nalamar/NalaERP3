CREATE TABLE IF NOT EXISTS quote_imports (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    contact_id UUID NULL REFERENCES contacts(id) ON DELETE SET NULL,
    source_kind TEXT NOT NULL,
    source_filename TEXT NOT NULL,
    source_document_id TEXT NOT NULL,
    status TEXT NOT NULL,
    parser_version TEXT NOT NULL DEFAULT '',
    detected_format TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_quote_id UUID NULL REFERENCES quotes(id) ON DELETE SET NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_quote_imports_project_uploaded
    ON quote_imports (project_id, uploaded_at DESC);

CREATE INDEX IF NOT EXISTS idx_quote_imports_contact_uploaded
    ON quote_imports (contact_id, uploaded_at DESC);

CREATE INDEX IF NOT EXISTS idx_quote_imports_status_uploaded
    ON quote_imports (status, uploaded_at DESC);
