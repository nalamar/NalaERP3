-- Entnahme fuer Fertigungsauftraege (ADR 0038 Frage 2/3, Backlog H.3.1).
--
-- 1. stock_reservations.status kennt zusaetzlich 'verbraucht': die
--    Reservierung ist durch Entnahmen aufgebraucht. Eine teilweise
--    Entnahme verringert qty der aktiven Reservierung (qty > 0 bleibt);
--    die letzte setzt 'verbraucht' und laesst qty stehen. Ein Storno der
--    Entnahme stellt den verbrauchten Teil wieder her, solange der Auftrag
--    offen ist (Regeln in production/withdrawal_rules.go).
-- 2. production_order_withdrawals: eine Entnahme ist ein Beleg - Lager-
--    abgang (stock_movements, Menge negativ) mit Bezug zum Auftrag und,
--    wenn vorhanden, zur Zeile der Fertigungsstueckliste. Ohne Zeile:
--    Material, das nicht in der Stueckliste steht (Kleinteile).
--    Unveraenderlich bis auf das einmalige Storno mit Begruendung und
--    Gegenbuchung (Muster asset_assignments, 089). Keine Charge
--    (ADR 0034, Praezisierung EN 1090).
--    reservierung_menge: wie viel dieser Entnahme die Reservierung der
--    Zeile verbraucht hat - damit stellt das Storno genau das wieder her.
--
-- Material und Lager sind RESTRICT: ein Material oder Lager mit Entnahmen
-- laesst sich nicht mehr loeschen (bisher loeschte das Material seine
-- Lagerbewegungen per CASCADE mit; der Beleg haelt jetzt die Bewegung
-- fest). Die Reservierung ist SET NULL wie an der Zeile (108).

ALTER TABLE stock_reservations DROP CONSTRAINT IF EXISTS chk_stock_reservations_status;
ALTER TABLE stock_reservations
    ADD CONSTRAINT chk_stock_reservations_status CHECK (status IN ('aktiv', 'freigegeben', 'verbraucht'));

CREATE TABLE IF NOT EXISTS production_order_withdrawals (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    order_id uuid NOT NULL REFERENCES production_orders(id) ON DELETE RESTRICT,
    item_id uuid REFERENCES production_order_items(id) ON DELETE RESTRICT,
    material_id text NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    warehouse_id text NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    location_id text REFERENCES locations(id) ON DELETE SET NULL,
    menge numeric(18,6) NOT NULL,
    einheit text NOT NULL DEFAULT '',
    stock_movement_id text NOT NULL REFERENCES stock_movements(id) ON DELETE RESTRICT,
    stock_reservation_id text REFERENCES stock_reservations(id) ON DELETE SET NULL,
    reservierung_menge numeric(18,6) NOT NULL DEFAULT 0,
    notiz text NOT NULL DEFAULT '',
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    storno_movement_id text REFERENCES stock_movements(id) ON DELETE RESTRICT,
    CONSTRAINT production_order_withdrawals_menge_check CHECK (menge > 0),
    CONSTRAINT production_order_withdrawals_reservierung_check
        CHECK (reservierung_menge >= 0 AND reservierung_menge <= menge),
    -- Storno nur vollstaendig: Zeitpunkt, Begruendung und Gegenbuchung.
    CONSTRAINT production_order_withdrawals_storno_check CHECK (
        (storniert_at IS NULL AND storno_reason = '' AND storno_movement_id IS NULL)
        OR (storniert_at IS NOT NULL AND btrim(storno_reason) <> '' AND storno_movement_id IS NOT NULL)
    ),
    CONSTRAINT production_order_withdrawals_movement_key UNIQUE (stock_movement_id),
    CONSTRAINT production_order_withdrawals_storno_movement_key UNIQUE (storno_movement_id),
    CONSTRAINT production_order_withdrawals_movements_differ CHECK (storno_movement_id IS NULL OR storno_movement_id <> stock_movement_id)
);

CREATE INDEX IF NOT EXISTS idx_production_order_withdrawals_order ON production_order_withdrawals(order_id);
CREATE INDEX IF NOT EXISTS idx_production_order_withdrawals_item ON production_order_withdrawals(item_id) WHERE item_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_production_order_withdrawals_material ON production_order_withdrawals(material_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS production_order_withdrawals;
--   UPDATE stock_reservations SET status='freigegeben', released_at=COALESCE(released_at, now()) WHERE status='verbraucht';
--   ALTER TABLE stock_reservations DROP CONSTRAINT IF EXISTS chk_stock_reservations_status;
--   ALTER TABLE stock_reservations ADD CONSTRAINT chk_stock_reservations_status CHECK (status IN ('aktiv', 'freigegeben'));
-- DATENVERLUSTRISIKO (MITTEL): verloren geht, welche Lagerabgaenge
-- Entnahmen fuer welchen Auftrag und welche Stuecklistenzeile waren, wer
-- sie gebucht und wer sie storniert hat. Die Lagerbewegungen selbst
-- (Abgang und Gegenbuchung) bleiben in stock_movements und damit der
-- Bestand richtig; die Referenz dort ist die Auftragsnummer. Verbrauchte
-- Reservierungen werden zu "freigegeben" (zaehlen weiter nicht) - der
-- Unterschied verbraucht/freigegeben geht verloren.
