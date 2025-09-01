-- Änderungsprotokoll für Logikal-Importe
CREATE TABLE IF NOT EXISTS project_imports (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source TEXT,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_phases INT NOT NULL DEFAULT 0,
    updated_phases INT NOT NULL DEFAULT 0,
    created_elevations INT NOT NULL DEFAULT 0,
    updated_elevations INT NOT NULL DEFAULT 0,
    created_variants INT NOT NULL DEFAULT 0,
    updated_variants INT NOT NULL DEFAULT 0,
    deleted_variants INT NOT NULL DEFAULT 0,
    materials_replaced_variants INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS project_import_changes (
    id UUID PRIMARY KEY,
    import_id UUID NOT NULL REFERENCES project_imports(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    action TEXT NOT NULL,
    internal_id UUID NULL,
    external_ref TEXT NULL,
    message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_project_imports_project ON project_imports (project_id, imported_at DESC);
CREATE INDEX IF NOT EXISTS idx_project_import_changes_import ON project_import_changes (import_id);

