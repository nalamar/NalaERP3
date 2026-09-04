-- Vollstaendiges Kalkulationsschema (Material/Lohn/Fremdleistung/
-- Zuschlaege getrennt) - ADR 0011, Backlog B.3.2. Neue, separate
-- 1:1-Tabelle statt Umbau von quote_items.unit_price (siehe ADR 0011,
-- verworfene Option A): unit_price wird an sehr vielen Stellen in
-- quotes/service.go verwendet, ein Spaltenersatz waere ein riskanter
-- Umbau des gesamten bestehenden Preispfads.
--
-- Klassische deutsche Zuschlagskalkulation mit Kostenartentrennung:
-- Material/Lohn/Fremdleistung bekommen JEWEILS einen eigenen
-- Zuschlagssatz statt einer gemeinsamen Marge auf eine Gesamtsumme.
-- Die Berechnung (material_total/lohn_total/fremdleistung_total/
-- calculated_unit_price) erfolgt bewusst NICHT in der DB, sondern im
-- Anwendungscode (Backlog B.3.3) - reine Ableitung aus den hier
-- gespeicherten Rohwerten, keine gespeicherte, potenziell inkonsistente
-- Ergebnisspalte.
--
-- Kein eigenes company_id - Scope wird ueber quote_item_id -> quote_id ->
-- quotes.company_id geerbt (Muster wie quote_item_groups, ADR 0009).

CREATE TABLE IF NOT EXISTS quote_item_calculations (
    id UUID PRIMARY KEY,
    quote_item_id UUID NOT NULL UNIQUE REFERENCES quote_items(id) ON DELETE CASCADE,
    material_cost numeric(18,4) NOT NULL DEFAULT 0,
    material_zuschlag_percent numeric(6,2) NOT NULL DEFAULT 0,
    lohn_stunden numeric(18,4) NOT NULL DEFAULT 0,
    lohn_stundensatz numeric(18,4) NOT NULL DEFAULT 0,
    lohn_zuschlag_percent numeric(6,2) NOT NULL DEFAULT 0,
    fremdleistung_cost numeric(18,4) NOT NULL DEFAULT 0,
    fremdleistung_zuschlag_percent numeric(6,2) NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_quote_item_calculations_material_cost CHECK (material_cost >= 0),
    CONSTRAINT chk_quote_item_calculations_material_zuschlag CHECK (material_zuschlag_percent >= 0),
    CONSTRAINT chk_quote_item_calculations_lohn_stunden CHECK (lohn_stunden >= 0),
    CONSTRAINT chk_quote_item_calculations_lohn_stundensatz CHECK (lohn_stundensatz >= 0),
    CONSTRAINT chk_quote_item_calculations_lohn_zuschlag CHECK (lohn_zuschlag_percent >= 0),
    CONSTRAINT chk_quote_item_calculations_fremdleistung_cost CHECK (fremdleistung_cost >= 0),
    CONSTRAINT chk_quote_item_calculations_fremdleistung_zuschlag CHECK (fremdleistung_zuschlag_percent >= 0)
);

-- Standard-Zuschlagssaetze/-Stundensatz additiv auf der bestehenden
-- Singleton-Settings-Tabelle statt einer neuen Tabelle. Bewusst DEFAULT 0
-- statt eines erfundenen Praxiswerts (ADR 0011: keine unbelegte
-- fachliche Vermutung, aufgabe.md SS0/SS7.10) - Mandanten muessen ihre
-- tatsaechlichen Saetze selbst hinterlegen. Das bestehende
-- target_margin_percent bleibt unveraendert.
ALTER TABLE quote_calculation_settings
    ADD COLUMN IF NOT EXISTS default_material_zuschlag_percent numeric(6,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS default_lohn_zuschlag_percent numeric(6,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS default_fremdleistung_zuschlag_percent numeric(6,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS default_lohn_stundensatz numeric(18,4) NOT NULL DEFAULT 0;

-- decision_type-CHECK um den neuen Wert 'calculation_scheme_applied'
-- erweitern (identisches Muster wie Migration 066 fuer
-- 'target_price_applied') - der "Anwenden"-Schritt aus B.3.3 schreibt das
-- Kalkulationsergebnis in dieselbe, bereits bestehende
-- quote_item_price_decisions-Tabelle statt ein zweites Protokoll
-- aufzubauen.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_price_decisions_type'
    ) THEN
        ALTER TABLE quote_item_price_decisions
            DROP CONSTRAINT chk_quote_item_price_decisions_type;
    END IF;

    ALTER TABLE quote_item_price_decisions
        ADD CONSTRAINT chk_quote_item_price_decisions_type
        CHECK (decision_type IN ('primary_source_applied', 'target_price_applied', 'calculation_scheme_applied'));
END $$;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DO $$
--   BEGIN
--       IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_quote_item_price_decisions_type') THEN
--           ALTER TABLE quote_item_price_decisions DROP CONSTRAINT chk_quote_item_price_decisions_type;
--       END IF;
--       ALTER TABLE quote_item_price_decisions
--           ADD CONSTRAINT chk_quote_item_price_decisions_type
--           CHECK (decision_type IN ('primary_source_applied', 'target_price_applied'));
--   END $$;
--   ALTER TABLE quote_calculation_settings
--       DROP COLUMN IF EXISTS default_material_zuschlag_percent,
--       DROP COLUMN IF EXISTS default_lohn_zuschlag_percent,
--       DROP COLUMN IF EXISTS default_fremdleistung_zuschlag_percent,
--       DROP COLUMN IF EXISTS default_lohn_stundensatz;
--   DROP TABLE IF EXISTS quote_item_calculations;
-- DATENVERLUSTRISIKO: Sobald echte Kalkulationsdetails erfasst wurden,
-- gehen diese bei einem Downgrade unwiderruflich verloren. Bereits
-- bestehende 'calculation_scheme_applied'-Eintraege in
-- quote_item_price_decisions wuerden das erneut verschaerfte CHECK
-- verletzen und muessten vor dem Downgrade manuell bereinigt werden.
