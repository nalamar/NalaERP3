-- Lose / Phases je Projekt
CREATE TABLE IF NOT EXISTS project_phases (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    nummer TEXT NOT NULL, -- z.B. "1", "2"
    name TEXT NOT NULL,   -- z.B. "Haus A"
    beschreibung TEXT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    angelegt_am TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_phase_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_project_phases_project ON project_phases (project_id, sort_order);

