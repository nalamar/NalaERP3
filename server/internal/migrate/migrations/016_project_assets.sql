-- Projekt-Assets (z. B. Emfs/Rtfs Bilder aus Logikal)
CREATE TABLE IF NOT EXISTS project_assets (
  id text PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  rel_path text NOT NULL,
  gridfs_id text NOT NULL,
  filename text NOT NULL,
  content_type text NOT NULL,
  length bigint NOT NULL DEFAULT 0,
  uploaded_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, rel_path)
);

-- Bildpfad an Elevations speichern (relativ, z. B. 'Emfs/xyz.emf')
ALTER TABLE project_elevations ADD COLUMN IF NOT EXISTS picture1_relpath text;
