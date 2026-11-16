-- Herkunft von Auftrags- und Nachtragspositionen (ADR 0042 Frage 4,
-- Backlog I.2.6a).
--
-- outline_no: OZ aus dem LV, leer bei von Hand erfassten Positionen.
-- source_quote_item_id: Angebotsposition, aus der die Position stammt
-- (gewaehlte Alternative, Grundposition, abgerufene Bedarfsposition).
-- SET NULL: angenommene Angebote sind schreibgeschuetzt, ihre Positionen
-- werden praktisch nicht geloescht; falls doch, bleibt die Auftragsposition
-- mit OZ stehen. Bestehende Zeilen: keine OZ, keine Herkunft.

ALTER TABLE sales_order_items
    ADD COLUMN IF NOT EXISTS outline_no text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_quote_item_id uuid REFERENCES quote_items(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_sales_order_items_source_quote_item ON sales_order_items(source_quote_item_id);

ALTER TABLE sales_order_addendum_items
    ADD COLUMN IF NOT EXISTS outline_no text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_quote_item_id uuid REFERENCES quote_items(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_sales_order_addendum_items_source_quote_item ON sales_order_addendum_items(source_quote_item_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP INDEX IF EXISTS idx_sales_order_addendum_items_source_quote_item;
--   ALTER TABLE sales_order_addendum_items
--       DROP COLUMN IF EXISTS source_quote_item_id,
--       DROP COLUMN IF EXISTS outline_no;
--   DROP INDEX IF EXISTS idx_sales_order_items_source_quote_item;
--   ALTER TABLE sales_order_items
--       DROP COLUMN IF EXISTS source_quote_item_id,
--       DROP COLUMN IF EXISTS outline_no;
-- DATENVERLUSTRISIKO: OZ und Herkunft von Auftrags- und Nachtragspositionen
-- gehen beim Downgrade verloren - danach ist nicht mehr nachvollziehbar,
-- welche Alternative oder Bedarfsposition beauftragt wurde. Positionen,
-- Mengen und Preise bleiben erhalten.
