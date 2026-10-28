-- Fuhrpark: Fahrtenbuch (ADR 0032 §6, Backlog G.3c).
--
-- Optional je Fahrzeug (fleet_vehicles.fahrtenbuch, 099). Steuerlich
-- anerkannt wird nur die GESCHLOSSENE FORM (BFH VI R 64/04, VI B 37/23):
-- ein Eintrag wird nie geaendert und nie geloescht; ein Fehler wird
-- sichtbar storniert (mit Begruendung) und neu erfasst. Der
-- Erfassungszeitpunkt (created_at) wird gespeichert und ist nicht
-- aenderbar - so bleibt erkennbar, ob zeitnah eingetragen wurde.
--
-- Der Kilometerstand ist LUECKENLOS: eine Fahrt beginnt dort, wo die
-- vorige (nicht stornierte) endet. Das prueft der Service unter der
-- Sperre der Fahrzeugzeile; die Datenbank verhindert zusaetzlich zwei
-- nicht stornierte Fahrten mit gleichem Beginn bzw. gleichem Ende.
--
-- Fahrtart: dienstlich (Reiseziel und Reisezweck Pflicht, aufgesuchter
-- Geschaeftspartner soweit vorhanden), privat (nur Kilometer) oder
-- arbeitsweg (Fahrt Wohnung - erste Taetigkeitsstaette).
--
-- Bewegungsdaten sind personenbezogen (DSGVO): lesen nur Fuhrpark
-- (fleet.read) und der Fahrer selbst (fleet.self) - das regeln die Routen.
--
-- Kein company_id: der Mandant haengt am Fahrzeug.
CREATE TABLE IF NOT EXISTS fleet_trips (
    id uuid PRIMARY KEY,
    vehicle_id uuid NOT NULL REFERENCES fleet_vehicles(id) ON DELETE RESTRICT,
    -- Der Fahrer.
    employee_id uuid NOT NULL REFERENCES hr_employees(id) ON DELETE RESTRICT,
    datum date NOT NULL,
    km_beginn integer NOT NULL,
    km_ende integer NOT NULL,
    fahrtart text NOT NULL,
    ziel text NOT NULL DEFAULT '',
    zweck text NOT NULL DEFAULT '',
    geschaeftspartner text NOT NULL DEFAULT '',
    notiz text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fleet_trips_fahrtart_check
        CHECK (fahrtart IN ('dienstlich', 'privat', 'arbeitsweg')),
    CONSTRAINT fleet_trips_km_check
        CHECK (km_beginn >= 0 AND km_ende > km_beginn),
    CONSTRAINT fleet_trips_dienstlich_check
        CHECK (fahrtart <> 'dienstlich' OR (btrim(ziel) <> '' AND btrim(zweck) <> '')),
    CONSTRAINT fleet_trips_status_check
        CHECK (status IN ('active', 'storniert')),
    CONSTRAINT fleet_trips_storno_consistent
        CHECK (
            (status = 'active' AND storniert_at IS NULL AND storno_reason = '')
            OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
        )
);

-- Zweite Absicherung der Kette gegen gleichzeitiges Erfassen.
CREATE UNIQUE INDEX IF NOT EXISTS idx_fleet_trips_km_beginn
    ON fleet_trips(vehicle_id, km_beginn) WHERE status = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS idx_fleet_trips_km_ende
    ON fleet_trips(vehicle_id, km_ende) WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_fleet_trips_vehicle_date ON fleet_trips(vehicle_id, datum DESC);
CREATE INDEX IF NOT EXISTS idx_fleet_trips_employee ON fleet_trips(employee_id);

COMMENT ON TABLE fleet_trips IS
    'Fahrtenbuch in geschlossener Form (ADR 0032 §6): unveraenderlich, Fehler nur ueber Storno; lueckenloser Kilometerstand.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS fleet_trips;
-- DATENVERLUSTRISIKO (HOCH): verloren gehen alle Fahrtenbuecher samt
-- Storno-Historie. Ein Fahrtenbuch ist steuerlicher Nachweis (Privatanteil
-- der Dienstwagennutzung) und aufbewahrungspflichtig - vorher vollstaendig
-- exportieren. Fahrzeuge und Mitarbeiter bleiben unberuehrt.
