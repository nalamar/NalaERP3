-- Kalkulationspositionen (Elevations) je Los
CREATE TABLE IF NOT EXISTS project_elevations (
    id UUID PRIMARY KEY,
    phase_id UUID NOT NULL,
    nummer TEXT NOT NULL,     -- Positionsnummer, z.B. "1", "2"
    name TEXT NOT NULL,       -- z.B. "10 Fenster"
    beschreibung TEXT NULL,
    menge REAL NOT NULL DEFAULT 1,
    width_mm REAL NULL,
    height_mm REAL NULL,
    external_guid TEXT NULL,  -- Referenz nach Logikal (z.B. xGUID)
    angelegt_am TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_elev_phase FOREIGN KEY (phase_id) REFERENCES project_phases(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_project_elevations_phase ON project_elevations (phase_id);

