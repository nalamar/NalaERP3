-- Verknüpfungstabelle Kontakt -> Dokument (GridFS)

CREATE TABLE IF NOT EXISTS contact_documents (
  id text PRIMARY KEY,
  contact_id text NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
  document_id text NOT NULL,
  filename text NOT NULL DEFAULT '',
  content_type text NOT NULL DEFAULT '',
  length bigint NOT NULL DEFAULT 0,
  uploaded_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(contact_id, document_id)
);

CREATE INDEX IF NOT EXISTS idx_contact_documents_contact
  ON contact_documents(contact_id, uploaded_at DESC);
