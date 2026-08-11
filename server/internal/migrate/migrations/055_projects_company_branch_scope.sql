-- Mandanten-/Standort-Scoping fuer projects (ADR 0002, Micro-Subtask 0.2.1.2.2)
--
-- company_id bleibt bewusst NULLABLE (kein SET NOT NULL), siehe Lektion aus
-- 054_contacts_company_branch_scope.sql: server/internal/projects setzt
-- company_id noch nicht beim Anlegen (erst Subtask 0.2.2.1). Expand-Contract:
-- Spalte + Backfill jetzt, NOT NULL erst in einer spaeteren Migration.

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);

UPDATE projects SET company_id = 'default' WHERE company_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_projects_company_id ON projects(company_id);
CREATE INDEX IF NOT EXISTS idx_projects_branch_id ON projects(branch_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE projects DROP COLUMN IF EXISTS branch_id;
--   ALTER TABLE projects DROP COLUMN IF EXISTS company_id;
-- DATENVERLUSTRISIKO: Sobald ein zweiter Mandant real angelegt und
-- bestehende projects diesem zugeordnet wurden, geht bei einem Downgrade die
-- Mandantenzuordnung unwiderruflich verloren.
