-- Mandanten-/Standort-Scoping fuer Material/Lager (ADR 0002, Micro-Subtask
-- 0.2.1.2.4). Nur die natuerlichen Anker-Tabellen bekommen eigene Spalten:
--   - materials: company_id (mandantenweit, kein branch_id, da Artikelstamm
--     nicht je Standort dupliziert wird)
--   - warehouses: company_id + branch_id (ein Lager IST die physische
--     Standort-Einheit)
-- locations (ueber warehouse_id), batches (ueber material_id) und
-- stock_movements (ueber warehouse_id) bekommen bewusst KEINE eigene Spalte
-- - sie erben den Scope ueber ihren bereits vorhandenen Fremdschluessel
-- (siehe Praezisierung in docs/adr/0002-mandanten-standort-scoping.md).
--
-- Wie bei 054/055/056: company_id bleibt NULLABLE (Expand-Contract, kein
-- sofortiges SET NOT NULL), da der Anwendungscode company_id erst mit
-- Subtask 0.2.2.1 setzt.

ALTER TABLE materials
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id);
UPDATE materials SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_materials_company_id ON materials(company_id);

ALTER TABLE warehouses
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);
UPDATE warehouses SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_warehouses_company_id ON warehouses(company_id);
CREATE INDEX IF NOT EXISTS idx_warehouses_branch_id ON warehouses(branch_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE materials DROP COLUMN IF EXISTS company_id;
--   ALTER TABLE warehouses DROP COLUMN IF EXISTS branch_id, DROP COLUMN IF EXISTS company_id;
-- DATENVERLUSTRISIKO: Sobald ein zweiter Mandant real angelegt und
-- bestehende Artikel/Lager diesem zugeordnet wurden, geht bei einem
-- Downgrade die Mandantenzuordnung unwiderruflich verloren.
