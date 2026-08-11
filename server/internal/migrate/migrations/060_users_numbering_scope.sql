-- Mandanten-/Standort-Scoping fuer users; Vorbereitung fuer number_sequences
-- (ADR 0002, Subtask 0.2.1.3 - letzte Subtask von Task 0.2.1).
--
-- users: company_id + branch_id wie bei den anderen Kopf-Tabellen
-- (Expand-Contract, company_id vorerst NULLABLE).
--
-- number_sequences: NUR company_id-Spalte + Backfill + unterstuetzender
-- Index in dieser Migration. Der Primary Key wird bewusst NICHT auf
-- (company_id, entity) umgestellt - siehe Praezisierung in
-- docs/adr/0002-mandanten-standort-scoping.md: ein zusammengesetzter PK
-- wuerde company_id sofort NOT NULL erzwingen (Postgres erlaubt keine
-- NULL-Werte in PK-Spalten), und settings.NumberingService filtert aktuell
-- ausschliesslich nach entity. PK-Umstellung folgt in einer eigenen
-- Migration nach Task 0.2.2 (Anwendungscode-Anpassung).

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);
UPDATE users SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_company_id ON users(company_id);
CREATE INDEX IF NOT EXISTS idx_users_branch_id ON users(branch_id);

ALTER TABLE number_sequences
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id);
UPDATE number_sequences SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_number_sequences_company_id ON number_sequences(company_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE users DROP COLUMN IF EXISTS branch_id, DROP COLUMN IF EXISTS company_id;
--   ALTER TABLE number_sequences DROP COLUMN IF EXISTS company_id;
-- DATENVERLUSTRISIKO: Sobald ein zweiter Mandant real angelegt und
-- bestehende User/Nummernkreise diesem zugeordnet wurden, geht bei einem
-- Downgrade die Mandantenzuordnung unwiderruflich verloren.
