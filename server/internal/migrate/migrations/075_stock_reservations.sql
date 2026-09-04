-- Projektbezogene Lagerreservierung (ADR 0013, Backlog C.1.2). Neue,
-- eigenstaendige Tabelle, keine bestehende Tabelle veraendert:
--   - stock_reservations: manuell angelegte Reservierung von Material in
--     einem Lager fuer ein Projekt (project_id bewusst NOT NULL, siehe
--     ADR 0013 - "projektbezogen" ist fuer C.1 konstitutiv). Granularitaet
--     Material+Lager, bewusst OHNE location_id/batch_id (Batches
--     existieren erst nach Wareneingang, eine Reservierung muss aber auch
--     vorher moeglich sein).
--
-- Kein eigenes company_id/branch_id - Scope wird ueber warehouse_id ->
-- warehouses.company_id geerbt, exakt das etablierte Muster fuer
-- stock_movements (ADR 0002, 057_materials_warehouses_scope.sql).
--
-- Die Verfuegbarkeitspruefung (physischer Bestand minus aktive
-- Reservierungen) ist reine Anwendungslogik (folgt in C.1.3) - hier nur
-- die Datenstruktur inkl. CHECK-Constraints fuer die stabilen Wertesaetze
-- (qty > 0, status-Enum).

CREATE TABLE IF NOT EXISTS stock_reservations (
    id text PRIMARY KEY,
    material_id text NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    warehouse_id text NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    qty numeric(18,6) NOT NULL,
    status text NOT NULL DEFAULT 'aktiv',
    grund text NOT NULL DEFAULT '',
    referenz text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    released_at timestamptz,
    CONSTRAINT chk_stock_reservations_qty_positive CHECK (qty > 0),
    CONSTRAINT chk_stock_reservations_status CHECK (status IN ('aktiv', 'freigegeben'))
);

CREATE INDEX IF NOT EXISTS idx_stock_reservations_material_warehouse_status
    ON stock_reservations(material_id, warehouse_id, status);
CREATE INDEX IF NOT EXISTS idx_stock_reservations_project ON stock_reservations(project_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS stock_reservations;
-- DATENVERLUSTRISIKO: Sobald echte Reservierungen erfasst wurden, gehen
-- diese bei einem Downgrade unwiderruflich verloren. Kein Risiko fuer
-- bestehende materials/warehouses/projects/stock_movements - alle vier
-- bleiben unangetastet.
