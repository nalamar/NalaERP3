-- Belegnummern und Materialnummern je Mandant eindeutig statt ueber alle
-- Mandanten (Backlog H.5.5a; Fragen H.5.5 vom 2026-11-07, beantwortet mit
-- "fortsetzen" = Empfehlungen 1a und 3, ANNAHME).
--
-- Die Nummernkreise sind seit Migration 061 je Mandant, die Nummern aber
-- waren global eindeutig: zwei Mandanten mit gleichem Muster erzeugen
-- dieselbe Nummer, der zweite Beleg scheiterte mit 500. Materialnummern
-- vergibt der Nutzer; global eindeutig verriet die Nummer eines fremden
-- Mandanten. Kein Code nutzt die alten Constraints (kein ON CONFLICT,
-- keine Fehlerauswertung nach Namen, keine Fremdschluessel auf die
-- Nummern). company_id ist in diesen Tabellen nullable; NULL zaehlt wie
-- bei hr_employees als ein eigener Bereich ('').
--
-- invoices_out.nummer ist im Entwurf leer (NULL): mehrere NULL bleiben
-- erlaubt wie bisher. production_orders ist schon je Mandant eindeutig
-- (Migration 107), quotes und projects haben keinen eindeutigen Index.

ALTER TABLE invoices_out DROP CONSTRAINT IF EXISTS invoices_out_nummer_key;
CREATE UNIQUE INDEX IF NOT EXISTS invoices_out_company_nummer_key
    ON invoices_out ((COALESCE(company_id, '')), nummer);

ALTER TABLE purchase_orders DROP CONSTRAINT IF EXISTS purchase_orders_number_key;
CREATE UNIQUE INDEX IF NOT EXISTS purchase_orders_company_number_key
    ON purchase_orders ((COALESCE(company_id, '')), number);

ALTER TABLE rfqs DROP CONSTRAINT IF EXISTS rfqs_nummer_key;
CREATE UNIQUE INDEX IF NOT EXISTS rfqs_company_nummer_key
    ON rfqs (company_id, nummer);

ALTER TABLE sales_orders DROP CONSTRAINT IF EXISTS sales_orders_nummer_key;
CREATE UNIQUE INDEX IF NOT EXISTS sales_orders_company_nummer_key
    ON sales_orders ((COALESCE(company_id, '')), nummer);

ALTER TABLE materials DROP CONSTRAINT IF EXISTS materials_nummer_key;
CREATE UNIQUE INDEX IF NOT EXISTS materials_company_nummer_key
    ON materials ((COALESCE(company_id, '')), nummer);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP INDEX IF EXISTS materials_company_nummer_key;
--   ALTER TABLE materials ADD CONSTRAINT materials_nummer_key UNIQUE (nummer);
--   DROP INDEX IF EXISTS sales_orders_company_nummer_key;
--   ALTER TABLE sales_orders ADD CONSTRAINT sales_orders_nummer_key UNIQUE (nummer);
--   DROP INDEX IF EXISTS rfqs_company_nummer_key;
--   ALTER TABLE rfqs ADD CONSTRAINT rfqs_nummer_key UNIQUE (nummer);
--   DROP INDEX IF EXISTS purchase_orders_company_number_key;
--   ALTER TABLE purchase_orders ADD CONSTRAINT purchase_orders_number_key UNIQUE (number);
--   DROP INDEX IF EXISTS invoices_out_company_nummer_key;
--   ALTER TABLE invoices_out ADD CONSTRAINT invoices_out_nummer_key UNIQUE (nummer);
-- DATENVERLUSTRISIKO: keins durch die Migration selbst (nur Constraints).
-- Der Rueckweg scheitert, sobald zwei Mandanten dieselbe Nummer haben;
-- dann vorher die Nummern umbenennen (Rechnungsnummern gebuchter
-- Rechnungen nicht aendern - GoBD).
