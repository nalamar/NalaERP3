-- Fuhrpark: Fahrerzuordnung (ADR 0032 §4, Backlog G.3a).
--
-- Ein Fahrzeug wird einem Mitarbeiter zugeordnet (Dienstwagen mit festem
-- Fahrer) - Muster der Ausgabe von Betriebsmitteln (asset_assignments,
-- 089): Beleg, unveraenderlich bis auf das einmalige Ende und das Storno.
-- Pool-Fahrzeuge haben keine Zuordnung; ihre Nutzung zeigt spaeter das
-- Fahrtenbuch (G.3c).
--
-- Kein company_id: der Mandant haengt am Fahrzeug; dass der Mitarbeiter
-- zum selben Mandanten gehoert, prueft der Service beim Zuordnen.
--
-- Eine HU oder UVV mit "maengel"/"nicht_bestanden" sperrt die Zuordnung;
-- ein ueberfaelliger Termin und eine nicht gueltige Fuehrerscheinkontrolle
-- sind nur Hinweise (Entscheidung des Nutzers vom 2026-10-25). Beides
-- prueft der Service, nicht die Datenbank.
CREATE TABLE IF NOT EXISTS fleet_vehicle_assignments (
    id uuid PRIMARY KEY,
    vehicle_id uuid NOT NULL REFERENCES fleet_vehicles(id) ON DELETE RESTRICT,
    employee_id uuid NOT NULL REFERENCES hr_employees(id) ON DELETE RESTRICT,
    beginn date NOT NULL,
    -- NULL, solange der Mitarbeiter das Fahrzeug hat.
    ende date,
    notiz text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    zugeordnet_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    beendet_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fleet_vehicle_assignments_status_check
        CHECK (status IN ('active', 'storniert')),
    -- Ende am Tag des Beginns ist zulaessig (Tageswagen).
    CONSTRAINT fleet_vehicle_assignments_ende_after_beginn
        CHECK (ende IS NULL OR ende >= beginn),
    CONSTRAINT fleet_vehicle_assignments_storno_consistent
        CHECK (
            (status = 'active' AND storniert_at IS NULL AND storno_reason = '')
            OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
        )
);

-- Hoechstens eine offene Zuordnung je Fahrzeug - von der Datenbank
-- garantiert, nicht nur vom Code (Muster idx_asset_assignments_one_open).
-- Ein Mitarbeiter darf mehrere Fahrzeuge haben (z. B. Transporter und
-- Anhaenger).
CREATE UNIQUE INDEX IF NOT EXISTS idx_fleet_vehicle_assignments_one_open
    ON fleet_vehicle_assignments(vehicle_id) WHERE ende IS NULL AND status = 'active';

CREATE INDEX IF NOT EXISTS idx_fleet_vehicle_assignments_vehicle_date
    ON fleet_vehicle_assignments(vehicle_id, beginn DESC);
CREATE INDEX IF NOT EXISTS idx_fleet_vehicle_assignments_employee
    ON fleet_vehicle_assignments(employee_id);

COMMENT ON TABLE fleet_vehicle_assignments IS
    'Fahrerzuordnung Fahrzeug -> Mitarbeiter mit Beginn und Ende (ADR 0032 §4). Unveraenderlich bis auf Ende und Storno.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS fleet_vehicle_assignments;
-- DATENVERLUSTRISIKO (MITTEL): verloren geht, wer wann welches Fahrzeug
-- hatte - gebraucht fuer Halterfragen (Bussgeld, Unfall) und die
-- Zuordnung von Kosten. Vorher exportieren. Fahrzeuge, Termine und
-- Mitarbeiter bleiben unberuehrt.
