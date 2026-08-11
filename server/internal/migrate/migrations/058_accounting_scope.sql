-- Mandanten-Scoping fuer Buchhaltung (ADR 0002, Micro-Subtask 0.2.1.2.5).
-- Nur company_id, kein branch_id: Buchhaltung wird laut ADR 0002 auf
-- Mandantenebene konsolidiert (ein Kontenrahmen je Mandant, nicht je
-- Standort). Betroffen: accounts (Kontenrahmen), journal_entries
-- (Buchungssaetze), bank_statements (Kontoauszugszeilen) - alle drei haben
-- keinen natuerlichen, bereits vorhandenen Fremdschluessel, ueber den sie
-- den Scope erben koennten (anders als journal_lines, das ueber entry_id
-- von journal_entries erbt und daher KEINE eigene Spalte bekommt).
--
-- Wie bei 054-057: company_id bleibt NULLABLE (Expand-Contract, kein
-- sofortiges SET NOT NULL), da der Anwendungscode company_id erst mit
-- Subtask 0.2.2.1 setzt.

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id);
UPDATE accounts SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_accounts_company_id ON accounts(company_id);

ALTER TABLE journal_entries
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id);
UPDATE journal_entries SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_journal_entries_company_id ON journal_entries(company_id);

ALTER TABLE bank_statements
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id);
UPDATE bank_statements SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_bank_statements_company_id ON bank_statements(company_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE accounts DROP COLUMN IF EXISTS company_id;
--   ALTER TABLE journal_entries DROP COLUMN IF EXISTS company_id;
--   ALTER TABLE bank_statements DROP COLUMN IF EXISTS company_id;
-- DATENVERLUSTRISIKO: Sobald ein zweiter Mandant real angelegt und
-- bestehende Buchungen/Kontoauszuege diesem zugeordnet wurden, geht bei
-- einem Downgrade die Mandantenzuordnung unwiderruflich verloren.
