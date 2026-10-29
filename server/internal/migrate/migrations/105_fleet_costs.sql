-- Fuhrpark: Kosten je Fahrzeug (ADR 0032 §7, Backlog G.3d).
--
-- Ein Eintrag ist ein Beleg: Datum, Kostenart, Nettobetrag, optional
-- Lieferant, Belegnummer, km-Stand, Liter (nur Kraftstoff) und
-- Kostenstelle. Wie Fahrtenbuch und Erledigungen in GESCHLOSSENER FORM:
-- nie geaendert, nie geloescht; ein Fehler wird mit Begruendung storniert
-- und neu erfasst. Die Kostenstelle wird beim Erfassen festgehalten (ohne
-- Angabe die des Fahrzeugs) - eine spaetere Aenderung am Fahrzeug
-- verschiebt keine gebuchten Kosten. Die Anbindung an Eingangsrechnungen
-- (Epic E) folgt spaeter.
--
-- Kein company_id: der Mandant haengt am Fahrzeug.
CREATE TABLE IF NOT EXISTS fleet_costs (
    id uuid PRIMARY KEY,
    vehicle_id uuid NOT NULL REFERENCES fleet_vehicles(id) ON DELETE RESTRICT,
    datum date NOT NULL,
    kostenart text NOT NULL,
    -- Pflicht bei "sonstiges", sonst freie Ergaenzung.
    bezeichnung text NOT NULL DEFAULT '',
    betrag_netto numeric(18,2) NOT NULL,
    lieferant text NOT NULL DEFAULT '',
    belegnummer text NOT NULL DEFAULT '',
    km_stand integer,
    liter numeric(10,2),
    kostenstelle_id text REFERENCES cost_centers(id) ON DELETE RESTRICT,
    notiz text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fleet_costs_kostenart_check
        CHECK (kostenart IN ('kraftstoff', 'wartung', 'reparatur', 'versicherung', 'steuer', 'leasing', 'sonstiges')),
    CONSTRAINT fleet_costs_sonstiges_check
        CHECK (kostenart <> 'sonstiges' OR btrim(bezeichnung) <> ''),
    CONSTRAINT fleet_costs_betrag_check
        CHECK (betrag_netto > 0),
    CONSTRAINT fleet_costs_km_check
        CHECK (km_stand IS NULL OR km_stand >= 0),
    CONSTRAINT fleet_costs_liter_check
        CHECK (liter IS NULL OR (liter > 0 AND kostenart = 'kraftstoff')),
    CONSTRAINT fleet_costs_status_check
        CHECK (status IN ('active', 'storniert')),
    CONSTRAINT fleet_costs_storno_consistent
        CHECK (
            (status = 'active' AND storniert_at IS NULL AND storno_reason = '')
            OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
        )
);

CREATE INDEX IF NOT EXISTS idx_fleet_costs_vehicle_date ON fleet_costs(vehicle_id, datum DESC);
CREATE INDEX IF NOT EXISTS idx_fleet_costs_kostenstelle ON fleet_costs(kostenstelle_id) WHERE kostenstelle_id IS NOT NULL;

COMMENT ON TABLE fleet_costs IS
    'Kosten je Fahrzeug (ADR 0032 §7): Belege in geschlossener Form, Fehler nur ueber Storno; Kostenstelle beim Erfassen festgehalten.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS fleet_costs;
-- DATENVERLUSTRISIKO (HOCH): verloren gehen alle erfassten Fahrzeugkosten
-- samt Storno-Historie und Kostenstellen-Zuordnung. Die Belege selbst
-- liegen in der Buchhaltung, die Auswertung je Fahrzeug aber nur hier -
-- vorher vollstaendig exportieren. Fahrzeuge und Kostenstellen bleiben
-- unberuehrt.
