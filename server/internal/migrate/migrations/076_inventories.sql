-- Inventurprozess (ADR 0014, Backlog C.2.2). Zwei neue Tabellen, keine
-- bestehende Tabelle veraendert:
--   - inventories: Inventur-Header je Lager, Status laufend/abgeschlossen.
--   - inventory_lines: Zaehlpositionen (soll_qty wird beim Hinzufuegen aus
--     dem aktuellen Buchbestand fixiert, ist_qty ist die Zaehlung). Die
--     Differenz wird bewusst NICHT gespeichert (siehe ADR 0014), sondern
--     beim Lesen/Abschliessen berechnet.
--
-- Kein eigenes company_id/branch_id auf inventories - Scope wird ueber
-- warehouse_id -> warehouses.company_id geerbt, exakt das etablierte
-- Muster fuer stock_movements/stock_reservations (ADR 0002).
--
-- Bewusst KEINE UNIQUE-Constraint auf inventory_lines (Inventur+Material
-- +Ort) - siehe ADR 0014, Abschnitt "Granularitaet/Eindeutigkeit je
-- Zaehlposition": eine Doppelzaehlung ist ein Anwenderproblem, keine
-- Datenintegritaetsverletzung.

CREATE TABLE IF NOT EXISTS inventories (
    id text PRIMARY KEY,
    warehouse_id text NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'laufend',
    note text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    closed_at timestamptz,
    CONSTRAINT chk_inventories_status CHECK (status IN ('laufend', 'abgeschlossen'))
);

CREATE INDEX IF NOT EXISTS idx_inventories_warehouse ON inventories(warehouse_id);
CREATE INDEX IF NOT EXISTS idx_inventories_status ON inventories(status);

CREATE TABLE IF NOT EXISTS inventory_lines (
    id text PRIMARY KEY,
    inventory_id text NOT NULL REFERENCES inventories(id) ON DELETE CASCADE,
    material_id text NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    location_id text REFERENCES locations(id) ON DELETE SET NULL,
    soll_qty numeric(18,6) NOT NULL,
    ist_qty numeric(18,6) NOT NULL,
    counted_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_inventory_lines_ist_qty_non_negative CHECK (ist_qty >= 0)
);

CREATE INDEX IF NOT EXISTS idx_inventory_lines_inventory ON inventory_lines(inventory_id);
CREATE INDEX IF NOT EXISTS idx_inventory_lines_material ON inventory_lines(material_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS inventory_lines;
--   DROP TABLE IF EXISTS inventories;
-- DATENVERLUSTRISIKO: Sobald echte Inventuren (Header + Zaehlpositionen)
-- erfasst wurden, gehen diese bei einem Downgrade unwiderruflich
-- verloren. Kein Risiko fuer bestehende materials/warehouses/locations/
-- stock_movements/stock_reservations - alle bleiben unangetastet.
