-- Backlog 0.32 (Mandanten-Onboarding, Subtask 0.32.2): accounts.code war
-- bisher GLOBAL eindeutiger Primaerschluessel (nicht (company_id, code)).
-- Das macht es unmoeglich, einem NEUEN Mandanten denselben Standard-
-- Kontenrahmen (1000, 1200, 8000, ...) zuzuweisen wie 'default', da jede
-- Kontonummer nur EINMAL im gesamten System existieren durfte - das
-- eigentliche Ziel von 0.32 ("Kontenrahmen-Vorlage kopieren") war damit
-- strukturell unerreichbar, nicht nur ungeschrieben. Migration 058 hatte
-- bereits eine company_id-Spalte ergaenzt, aber den Primaerschluessel nie
-- angepasst.
--
-- journal_lines/invoice_out_items tragen company_id bisher nur indirekt
-- (ueber journal_entries bzw. invoices_out) - fuer eine zusammengesetzte
-- FK auf accounts(company_id, code) brauchen beide Tabellen jetzt eine
-- eigene company_id-Spalte, per JOIN aus der jeweiligen Kopftabelle
-- rueckwirkend befuellt (deterministisch, kein Datenverlustrisiko).

ALTER TABLE journal_lines ADD COLUMN IF NOT EXISTS company_id text;
UPDATE journal_lines jl
    SET company_id = je.company_id
    FROM journal_entries je
    WHERE je.id = jl.entry_id AND jl.company_id IS NULL;
ALTER TABLE journal_lines ALTER COLUMN company_id SET NOT NULL;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'journal_lines_company_id_fkey'
    ) THEN
        ALTER TABLE journal_lines
            ADD CONSTRAINT journal_lines_company_id_fkey FOREIGN KEY (company_id) REFERENCES company_profiles(id);
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_journal_lines_company_id ON journal_lines(company_id);

ALTER TABLE invoice_out_items ADD COLUMN IF NOT EXISTS company_id text;
UPDATE invoice_out_items ii
    SET company_id = io.company_id
    FROM invoices_out io
    WHERE io.id = ii.invoice_id AND ii.company_id IS NULL;
ALTER TABLE invoice_out_items ALTER COLUMN company_id SET NOT NULL;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'invoice_out_items_company_id_fkey'
    ) THEN
        ALTER TABLE invoice_out_items
            ADD CONSTRAINT invoice_out_items_company_id_fkey FOREIGN KEY (company_id) REFERENCES company_profiles(id);
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_invoice_out_items_company_id ON invoice_out_items(company_id);

-- accounts.company_id ist seit Migration 058 fuer ALLE Zeilen auf 'default'
-- befuellt (Backfill dort) - jetzt sicher NOT NULL erzwingbar, Voraussetzung
-- fuer die Verwendung als Teil des neuen zusammengesetzten Primaerschluessels.
ALTER TABLE accounts ALTER COLUMN company_id SET NOT NULL;

-- Alte, auf accounts(code) alleine referenzierende Fremdschluessel muessen
-- VOR dem Umbau des Primaerschluessels entfernt werden (Postgres verbietet
-- das Droppen einer PK, solange abhaengige FKs bestehen).
ALTER TABLE journal_lines DROP CONSTRAINT IF EXISTS journal_lines_account_code_fkey;
ALTER TABLE invoice_out_items DROP CONSTRAINT IF EXISTS invoice_out_items_account_code_fkey;
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_parent_code_fkey;

ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_pkey;
ALTER TABLE accounts ADD CONSTRAINT accounts_pkey PRIMARY KEY (company_id, code);

-- parent_code ist aktuell in JEDER Zeile NULL (kein Anwendungscode setzt es
-- je), eine zusammengesetzte Selbstreferenz ist daher gefahrlos moeglich.
-- NULL in irgendeiner FK-Spalte gilt bei MATCH SIMPLE (Postgres-Standard)
-- automatisch als erfuellt.
ALTER TABLE accounts
    ADD CONSTRAINT accounts_parent_code_fkey
    FOREIGN KEY (company_id, parent_code) REFERENCES accounts(company_id, code);

ALTER TABLE journal_lines
    ADD CONSTRAINT journal_lines_account_code_fkey
    FOREIGN KEY (company_id, account_code) REFERENCES accounts(company_id, code);

ALTER TABLE invoice_out_items
    ADD CONSTRAINT invoice_out_items_account_code_fkey
    FOREIGN KEY (company_id, account_code) REFERENCES accounts(company_id, code);
