-- Verknüpfungstabelle Material -> Dokument (GridFS)

CREATE TABLE IF NOT EXISTS material_documents (
  id text PRIMARY KEY,
  material_id text NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
  document_id text NOT NULL,
  filename text NOT NULL DEFAULT '',
  content_type text NOT NULL DEFAULT '',
  length bigint NOT NULL DEFAULT 0,
  uploaded_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(material_id, document_id)
);

