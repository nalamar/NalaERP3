-- Projekte Grundschema
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY,
    nummer TEXT NOT NULL,
    name TEXT NOT NULL,
    kunde_id UUID NULL,
    status TEXT NOT NULL DEFAULT 'neu',
    angelegt_am TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_projects_nummer ON projects (nummer);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects (status);

-- Optional: FK zu contacts (falls vorhanden); tolerieren, wenn Tabelle nicht existiert
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables WHERE table_name='contacts'
    ) THEN
        ALTER TABLE projects
        ADD CONSTRAINT fk_projects_kunde FOREIGN KEY (kunde_id) REFERENCES contacts(id) ON DELETE SET NULL;
    END IF;
EXCEPTION WHEN duplicate_object THEN
    -- Constraint bereits vorhanden
    NULL;
END$$;

