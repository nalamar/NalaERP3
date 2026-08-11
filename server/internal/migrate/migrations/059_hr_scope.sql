-- Mandanten-/Standort-Scoping fuer HR (ADR 0002, Micro-Subtask 0.2.1.2.6,
-- letzte Subtask von Task 0.2.1.2).
--
-- hr_employees und hr_teams bekommen jeweils EIGENE Spalten: hr_teams ist
-- kein Kind von hr_employees (hr_employees.team_id ist nullable, ein
-- Mitarbeiter kann teamlos sein, und die Standortzugehoerigkeit einer
-- Person ist keine zwingende Ableitung aus ihrem Team). hr_leave_requests
-- und hr_absences bekommen bewusst KEINE eigene Spalte - sie erben ueber
-- ihr verpflichtendes employee_id (NOT NULL) von hr_employees (siehe
-- Praezisierung in docs/adr/0002-mandanten-standort-scoping.md).
-- hr_holidays ist nicht Teil dieser Subtask (Feiertagskalender je
-- Land/Region, kein Mandanten-Scoping-Bedarf erkennbar, siehe ADR 0002).
--
-- Wie bei 054-058: company_id bleibt NULLABLE (Expand-Contract, kein
-- sofortiges SET NOT NULL), da der Anwendungscode company_id erst mit
-- Subtask 0.2.2.1 setzt.

ALTER TABLE hr_employees
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);
UPDATE hr_employees SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_hr_employees_company_id ON hr_employees(company_id);
CREATE INDEX IF NOT EXISTS idx_hr_employees_branch_id ON hr_employees(branch_id);

ALTER TABLE hr_teams
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);
UPDATE hr_teams SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_hr_teams_company_id ON hr_teams(company_id);
CREATE INDEX IF NOT EXISTS idx_hr_teams_branch_id ON hr_teams(branch_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE hr_employees DROP COLUMN IF EXISTS branch_id, DROP COLUMN IF EXISTS company_id;
--   ALTER TABLE hr_teams DROP COLUMN IF EXISTS branch_id, DROP COLUMN IF EXISTS company_id;
-- DATENVERLUSTRISIKO: Sobald ein zweiter Mandant real angelegt und
-- bestehende Mitarbeiter/Teams diesem zugeordnet wurden, geht bei einem
-- Downgrade die Mandantenzuordnung unwiderruflich verloren. Zusaetzlich
-- enthaelt hr_employees personenbezogene Daten (Name, E-Mail) - beim
-- Rueckbau ist DSGVO-Loeschfristenlogik zu beachten, falls zwischenzeitlich
-- mandantenspezifische Zugriffsbeschraenkungen darauf aufgebaut wurden.
