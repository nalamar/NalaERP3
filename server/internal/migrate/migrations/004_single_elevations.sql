-- Ausführungsvarianten je Kalkulationsposition
CREATE TABLE IF NOT EXISTS project_single_elevations (
    id UUID PRIMARY KEY,
    elevation_id UUID NOT NULL,
    name TEXT NOT NULL,         -- z.B. "linksdrehend" / "rechtsdrehend"
    beschreibung TEXT NULL,
    menge REAL NOT NULL DEFAULT 1,
    selected BOOLEAN NOT NULL DEFAULT false, -- optionale Markierung "gewählte Variante"
    external_guid TEXT NULL,     -- Referenz nach Logikal
    angelegt_am TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_single_elev FOREIGN KEY (elevation_id) REFERENCES project_elevations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_single_elevations_elev ON project_single_elevations (elevation_id);

