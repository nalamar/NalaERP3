-- Verschnitt-/Reststueckverwaltung fuer Profile (ADR 0015, Backlog C.3.2).
-- Eine neue Tabelle, keine bestehende Tabelle veraendert:
--   - profile_offcuts: einzelne, konkrete Reststuecke eines Profil-
--     Materials (length_mm ist die Laenge DIESES Stuecks, nicht die
--     Artikel-Standardlaenge aus materials.length_mm). Registrierung
--     durch den Nutzer ist die fachliche Entscheidung "dieser Rest ist
--     wiederverwendbar" - nicht registrierte Reste bleiben implizit
--     Verschnitt (siehe ADR 0015, keine erfundene Mindestlaenge).
--
-- Kein eigenes company_id/branch_id - Scope wird ueber warehouse_id ->
-- warehouses.company_id geerbt, exakt das etablierte Muster fuer
-- stock_movements/stock_reservations/inventories (ADR 0002).
--
-- source_offcut_id verkettet ein beim Verbrauch eines Reststuecks neu
-- entstandenes, kleineres Reststueck mit seinem Ursprung (Storno-statt-
-- Mutation-Muster: die verbrauchte Zeile wird abgeschlossen, nie in der
-- Laenge veraendert).

CREATE TABLE IF NOT EXISTS profile_offcuts (
    id text PRIMARY KEY,
    material_id text NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    warehouse_id text NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    location_id text REFERENCES locations(id) ON DELETE SET NULL,
    length_mm numeric(18,6) NOT NULL,
    status text NOT NULL DEFAULT 'verfügbar',
    source_offcut_id text REFERENCES profile_offcuts(id) ON DELETE SET NULL,
    note text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    consumed_at timestamptz,
    used_length_mm numeric(18,6),
    CONSTRAINT chk_profile_offcuts_length_positive CHECK (length_mm > 0),
    CONSTRAINT chk_profile_offcuts_status CHECK (status IN ('verfügbar', 'verbraucht'))
);

CREATE INDEX IF NOT EXISTS idx_profile_offcuts_material_warehouse_status
    ON profile_offcuts(material_id, warehouse_id, status);
CREATE INDEX IF NOT EXISTS idx_profile_offcuts_source ON profile_offcuts(source_offcut_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS profile_offcuts;
-- DATENVERLUSTRISIKO: Sobald echte Reststuecke registriert wurden, gehen
-- diese bei einem Downgrade unwiderruflich verloren. Kein Risiko fuer
-- bestehende materials/warehouses/locations/stock_movements/
-- stock_reservations/inventories - alle bleiben unangetastet.
