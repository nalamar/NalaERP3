-- Fuhrpark: Termine (ADR 0032 §3 mit Praezisierung G.2, Backlog G.2.1).
-- Zwei Tabellen:
--   - fleet_vehicle_schedules: ein TERMIN je Fahrzeug und Art (HU, UVV,
--     Wartung, Versicherung, Leasingende, Sonstiges) - Planung, aenderbar.
--     Der Starttermin gilt, bis die erste Erledigung erfasst ist: wer ein
--     Fahrzeug neu anlegt, kennt meist nur den naechsten Termin von der
--     Plakette, nicht die letzte HU.
--   - fleet_vehicle_completions: die ERLEDIGUNG eines Termins mit Ergebnis
--     und gespeichertem Folgetermin - Beleg, unveraenderlich, Fehler ueber
--     Storno (wie asset_inspections, 089).
--
-- Faellig ist der Folgetermin der juengsten nicht stornierten Erledigung,
-- sonst der Starttermin. Eine HU oder UVV mit "maengel" oder
-- "nicht_bestanden" sperrt das Fahrzeug (Entscheidung des Nutzers vom
-- 2026-10-25); die Sperre wirkt ab der Fahrerzuordnung (G.3a).
--
-- HU: faellig ist der Monat der Plakette (§29 StVZO) - gespeichert wird
-- deshalb immer der LETZTE TAG des Monats. Fuer den Starttermin prueft
-- das die Datenbank selbst; fuer den Folgetermin der Service (die Art
-- steht am Termin, nicht an der Erledigung).

CREATE TABLE IF NOT EXISTS fleet_vehicle_schedules (
    id uuid PRIMARY KEY,
    vehicle_id uuid NOT NULL REFERENCES fleet_vehicles(id) ON DELETE RESTRICT,
    art text NOT NULL,
    -- Nur bei "sonstige" Pflicht (z. B. "Tachographenpruefung"); sonst leer.
    bezeichnung text NOT NULL DEFAULT '',
    starttermin date,
    -- Wartung nach Kilometern (z. B. alle 30.000 km): Ziel vor der ersten
    -- Erledigung. Eine Faelligkeit nach km kommt mit dem Fahrtenbuch (G.3c).
    start_km integer,
    -- Vorschlag fuer den Folgetermin beim Erledigen; nil = keiner.
    intervall_monate integer,
    intervall_km integer,
    notiz text NOT NULL DEFAULT '',
    aktiv boolean NOT NULL DEFAULT true,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fleet_vehicle_schedules_art_check
        CHECK (art IN ('hu', 'uvv', 'wartung', 'versicherung', 'leasing', 'sonstige')),
    CONSTRAINT fleet_vehicle_schedules_bezeichnung_check
        CHECK ((art = 'sonstige') = (btrim(bezeichnung) <> '')),
    CONSTRAINT fleet_vehicle_schedules_hu_monatsende
        CHECK (art <> 'hu' OR starttermin IS NULL
               OR starttermin = (date_trunc('month', starttermin) + interval '1 month - 1 day')::date),
    CONSTRAINT fleet_vehicle_schedules_km_check CHECK (start_km IS NULL OR start_km > 0),
    CONSTRAINT fleet_vehicle_schedules_intervall_check
        CHECK ((intervall_monate IS NULL OR intervall_monate > 0) AND (intervall_km IS NULL OR intervall_km > 0))
);
-- Je Fahrzeug ein Termin je Art; bei "sonstige" je Bezeichnung.
CREATE UNIQUE INDEX IF NOT EXISTS idx_fleet_vehicle_schedules_unique
    ON fleet_vehicle_schedules(vehicle_id, art, lower(bezeichnung));

CREATE TABLE IF NOT EXISTS fleet_vehicle_completions (
    id uuid PRIMARY KEY,
    schedule_id uuid NOT NULL REFERENCES fleet_vehicle_schedules(id) ON DELETE RESTRICT,
    erledigt_am date NOT NULL,
    -- HU/UVV: bestanden, maengel, nicht_bestanden; alle anderen: erledigt.
    ergebnis text NOT NULL,
    km_stand integer,
    naechster_termin date,
    naechster_km integer,
    -- Pruefstelle oder Werkstatt (Freitext).
    pruefer text NOT NULL DEFAULT '',
    notiz text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fleet_vehicle_completions_ergebnis_check
        CHECK (ergebnis IN ('bestanden', 'maengel', 'nicht_bestanden', 'erledigt')),
    CONSTRAINT fleet_vehicle_completions_naechster_check
        CHECK (naechster_termin IS NULL OR naechster_termin > erledigt_am),
    CONSTRAINT fleet_vehicle_completions_km_check
        CHECK ((km_stand IS NULL OR km_stand >= 0) AND (naechster_km IS NULL OR naechster_km > 0)
               AND (km_stand IS NULL OR naechster_km IS NULL OR naechster_km > km_stand)),
    CONSTRAINT fleet_vehicle_completions_status_check CHECK (status IN ('active', 'storniert')),
    CONSTRAINT fleet_vehicle_completions_storno_consistent
        CHECK (
            (status = 'active' AND storniert_at IS NULL AND storno_reason = '')
            OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
        )
);
CREATE INDEX IF NOT EXISTS idx_fleet_vehicle_completions_schedule ON fleet_vehicle_completions(schedule_id, erledigt_am DESC);

COMMENT ON TABLE fleet_vehicle_schedules IS
    'Termine je Fahrzeug (HU, UVV, Wartung, Fristen) - Planung, aenderbar; Starttermin bis zur ersten Erledigung (ADR 0032).';
COMMENT ON TABLE fleet_vehicle_completions IS
    'Erledigte Termine mit Ergebnis und gespeichertem Folgetermin (ADR 0032). Unveraenderlich, Fehler nur ueber Storno.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS fleet_vehicle_completions;
--   DROP TABLE IF EXISTS fleet_vehicle_schedules;
-- DATENVERLUSTRISIKO (HOCH): mit fleet_vehicle_completions gehen die
-- Nachweise ueber HU, UVV-Pruefungen und Wartungen samt Storno-Historie
-- verloren - die UVV-Pruefung ist bis zur naechsten Pruefung schriftlich
-- aufzubewahren (DGUV Vorschrift 70). Vorher exportieren. Fahrzeuge
-- bleiben unberuehrt.
