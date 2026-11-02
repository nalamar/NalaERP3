-- Reservierungen bei der Freigabe eines Fertigungsauftrags (ADR 0034
-- Abschnitt 4, Backlog H.1.5). Nur neue, nullbare Spalten an den Tabellen
-- aus Migration 107; stock_reservations (075) bleibt unveraendert:
--   - production_orders.reservierung_lager_id: das Lager, aus dem bei der
--     Freigabe reserviert wurde. Leer heisst: ohne Reservierung
--     freigegeben (die Reservierung ist optional). Erst ab der Freigabe
--     gesetzt; ein Lager mit Reservierungen laesst sich nicht loeschen
--     (RESTRICT, wie stock_reservations.warehouse_id).
--   - production_order_items.material_id: das Lagermaterial der
--     Projektzeile, bei der Freigabe mitkopiert. SET NULL wie an den
--     Projektzeilen (013): ein geloeschtes Material laesst die
--     Fertigungsstueckliste stehen.
--   - production_order_items.stock_reservation_id: die Reservierung dieser
--     Zeile. Hoechstens eine je Zeile und eine Reservierung hoechstens an
--     einer Zeile. SET NULL, weil stock_reservations beim Loeschen des
--     Materials mitgeloescht wird (CASCADE, 075).
-- Bewusst keine Pruefung "Reservierung nur mit Material": beim Loeschen
-- eines Materials setzen die beiden SET-NULL-Aktionen die Spalten
-- nacheinander, die Pruefung koennte dazwischen fehlschlagen.

ALTER TABLE production_orders
    ADD COLUMN IF NOT EXISTS reservierung_lager_id text NULL REFERENCES warehouses(id) ON DELETE RESTRICT;

ALTER TABLE production_orders DROP CONSTRAINT IF EXISTS production_orders_reservierung_check;
ALTER TABLE production_orders
    ADD CONSTRAINT production_orders_reservierung_check CHECK (reservierung_lager_id IS NULL OR status <> 'entwurf');

ALTER TABLE production_order_items
    ADD COLUMN IF NOT EXISTS material_id text NULL REFERENCES materials(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS stock_reservation_id text NULL REFERENCES stock_reservations(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS production_order_items_reservation_key
    ON production_order_items(stock_reservation_id) WHERE stock_reservation_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_production_order_items_material ON production_order_items(material_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP INDEX IF EXISTS idx_production_order_items_material;
--   DROP INDEX IF EXISTS production_order_items_reservation_key;
--   ALTER TABLE production_order_items DROP COLUMN IF EXISTS stock_reservation_id, DROP COLUMN IF EXISTS material_id;
--   ALTER TABLE production_orders DROP CONSTRAINT IF EXISTS production_orders_reservierung_check;
--   ALTER TABLE production_orders DROP COLUMN IF EXISTS reservierung_lager_id;
-- DATENVERLUSTRISIKO (MITTEL): verloren geht, welche Reservierung zu
-- welcher Zeile eines Fertigungsauftrags gehoert und aus welchem Lager
-- reserviert wurde. Die Reservierungen selbst bleiben in
-- stock_reservations (Referenz = Auftragsnummer), werden dann aber beim
-- Storno nicht mehr automatisch freigegeben - vorher aktive
-- Reservierungen der Auftraege pruefen. Auftraege, Stuecklisten, Lager
-- und Material bleiben unberuehrt.
