-- Mandanten-/Standort-Scoping fuer Angebote/Auftraege/Rechnungen/Bestellungen
-- (ADR 0002, Micro-Subtask 0.2.1.2.3). Nur die Kopf-Tabellen bekommen eigene
-- Spalten; Positions-/Kind-Tabellen (quote_items, quote_imports,
-- quote_import_items, sales_order_items, invoice_out_items,
-- invoice_out_payments, purchase_order_items) erben den Scope ueber ihren
-- Fremdschluessel zur jeweiligen Kopf-Tabelle (siehe Praezisierung in
-- docs/adr/0002-mandanten-standort-scoping.md).
--
-- Wie bei 054/055: company_id bleibt NULLABLE (Expand-Contract, kein
-- sofortiges SET NOT NULL), da der Anwendungscode company_id erst mit
-- Subtask 0.2.2.1 setzt.

ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);
UPDATE quotes SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_quotes_company_id ON quotes(company_id);
CREATE INDEX IF NOT EXISTS idx_quotes_branch_id ON quotes(branch_id);

ALTER TABLE sales_orders
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);
UPDATE sales_orders SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_sales_orders_company_id ON sales_orders(company_id);
CREATE INDEX IF NOT EXISTS idx_sales_orders_branch_id ON sales_orders(branch_id);

ALTER TABLE invoices_out
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);
UPDATE invoices_out SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_invoices_out_company_id ON invoices_out(company_id);
CREATE INDEX IF NOT EXISTS idx_invoices_out_branch_id ON invoices_out(branch_id);

ALTER TABLE purchase_orders
    ADD COLUMN IF NOT EXISTS company_id text REFERENCES company_profiles(id),
    ADD COLUMN IF NOT EXISTS branch_id text REFERENCES company_branches(id);
UPDATE purchase_orders SET company_id = 'default' WHERE company_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_purchase_orders_company_id ON purchase_orders(company_id);
CREATE INDEX IF NOT EXISTS idx_purchase_orders_branch_id ON purchase_orders(branch_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE quotes DROP COLUMN IF EXISTS branch_id, DROP COLUMN IF EXISTS company_id;
--   ALTER TABLE sales_orders DROP COLUMN IF EXISTS branch_id, DROP COLUMN IF EXISTS company_id;
--   ALTER TABLE invoices_out DROP COLUMN IF EXISTS branch_id, DROP COLUMN IF EXISTS company_id;
--   ALTER TABLE purchase_orders DROP COLUMN IF EXISTS branch_id, DROP COLUMN IF EXISTS company_id;
-- DATENVERLUSTRISIKO: Sobald ein zweiter Mandant real angelegt und
-- bestehende Angebote/Auftraege/Rechnungen/Bestellungen diesem zugeordnet
-- wurden, geht bei einem Downgrade die Mandantenzuordnung unwiderruflich
-- verloren.
