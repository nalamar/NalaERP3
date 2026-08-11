-- Mandanten-/Standort-Scoping fuer contacts (ADR 0002, Micro-Subtask 0.2.1.2.1)
--
-- company_id bleibt HIER bewusst NULLABLE (kein SET NOT NULL): bei einem
-- Testlauf gegen frische DB (2026-08-11) hat sich gezeigt, dass der
-- Anwendungscode (server/internal/contacts) company_id beim Anlegen noch
-- nicht setzt (das ist erst Subtask 0.2.2.1) - ein sofortiges NOT NULL
-- haette jeden bestehenden POST /api/v1/contacts/ mit
-- "null value in column company_id violates not-null constraint" brechen
-- lassen. Expand-Contract-Strategie: Spalte + Backfill jetzt, NOT NULL erst
-- in einer spaeteren Migration, sobald 0.2.2.1 company_id in allen
-- betroffenen Domaenen-Packages zuverlaessig setzt.

ALTER TABLE contacts
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);

UPDATE contacts SET company_id = 'default' WHERE company_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_contacts_company_id ON contacts(company_id);
CREATE INDEX IF NOT EXISTS idx_contacts_branch_id ON contacts(branch_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE contacts DROP COLUMN IF EXISTS branch_id;
--   ALTER TABLE contacts DROP COLUMN IF EXISTS company_id;
-- DATENVERLUSTRISIKO: Sobald ein zweiter Mandant real angelegt und
-- bestehende contacts diesem zugeordnet wurden, geht bei einem Downgrade die
-- Mandantenzuordnung unwiderruflich verloren (welcher Kontakt zu welchem
-- Mandanten/Standort gehoerte, ist nach dem Spalten-Drop nicht mehr
-- rekonstruierbar).
