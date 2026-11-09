-- Produktion: Arbeitsplaetze, Arbeitsgangvorlagen und Arbeitsgaenge am
-- Fertigungsauftrag (ADR 0037, Backlog H.2.1). Fragen 2026-11-09 mit
-- "fortsetzen" beantwortet = alle Empfehlungen (a), ANNAHME.
--
-- Grobplanung: je Arbeitsgang ein Arbeitsplatz, eine Planzeit
-- (Ruestzeit + Minuten je Stueck x Stueckzahl des Auftrags) und eine
-- geplante Woche (gespeichert als Montag der ISO-Woche). Die Auslastung je
-- Arbeitsplatz und Woche rechnet der Service; Ueberlast ist ein Hinweis,
-- keine Sperre.
--
-- Arbeitsgaenge sind Planung, nicht Teil der eingefrorenen Kopie des
-- Auftrags (ADR 0034 §2): sie bleiben bis "fertig"/"storniert" aenderbar
-- (Service), Aenderungen ins entity_change_log. Dass Arbeitsplatz und
-- Vorlage zum Mandanten des Auftrags gehoeren, prueft der Service (wie bei
-- Lager und Material, Migration 108).
--
-- Arbeitsplaetze und Vorlagen werden nicht geloescht, sondern
-- deaktiviert (aktiv = false); Verweise darauf mit RESTRICT.

CREATE TABLE IF NOT EXISTS production_work_centers (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    name text NOT NULL,
    -- verfuegbare Stunden je Arbeitstag (Mo-Fr ohne Feiertage und
    -- Betriebsruhe, ADR 0037 §4)
    stunden_je_tag numeric(5,2) NOT NULL,
    aktiv boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT production_work_centers_name_nonempty CHECK (btrim(name) <> ''),
    CONSTRAINT production_work_centers_stunden_check CHECK (stunden_je_tag > 0 AND stunden_je_tag <= 24)
);
CREATE UNIQUE INDEX IF NOT EXISTS production_work_centers_company_name_key
    ON production_work_centers (company_id, lower(btrim(name)));

COMMENT ON TABLE production_work_centers IS
    'Arbeitsplaetze der Fertigung mit Stunden je Arbeitstag (ADR 0037), deaktivieren statt loeschen.';

CREATE TABLE IF NOT EXISTS production_operation_templates (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    name text NOT NULL,
    work_center_id uuid NOT NULL REFERENCES production_work_centers(id) ON DELETE RESTRICT,
    -- Reihenfolge beim Vorschlagen am Auftrag (aufsteigend)
    reihenfolge integer NOT NULL,
    ruestzeit_min numeric(10,2) NOT NULL DEFAULT 0,
    minuten_je_stueck numeric(10,2) NOT NULL DEFAULT 0,
    aktiv boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT production_operation_templates_name_nonempty CHECK (btrim(name) <> ''),
    CONSTRAINT production_operation_templates_reihenfolge_check CHECK (reihenfolge > 0),
    CONSTRAINT production_operation_templates_zeit_check CHECK (ruestzeit_min >= 0 AND minuten_je_stueck >= 0)
);
CREATE UNIQUE INDEX IF NOT EXISTS production_operation_templates_company_name_key
    ON production_operation_templates (company_id, lower(btrim(name)));

COMMENT ON TABLE production_operation_templates IS
    'Arbeitsgangvorlagen je Mandant mit Arbeitsplatz und Planzeit, am Auftrag vorgeschlagen (ADR 0037 §3).';

CREATE TABLE IF NOT EXISTS production_order_operations (
    id uuid PRIMARY KEY,
    order_id uuid NOT NULL REFERENCES production_orders(id) ON DELETE RESTRICT,
    pos integer NOT NULL,
    name text NOT NULL,
    work_center_id uuid NOT NULL REFERENCES production_work_centers(id) ON DELETE RESTRICT,
    -- nur Herkunft; die Werte sind beim Anlegen kopiert
    template_id uuid REFERENCES production_operation_templates(id) ON DELETE SET NULL,
    ruestzeit_min numeric(10,2) NOT NULL DEFAULT 0,
    minuten_je_stueck numeric(10,2) NOT NULL DEFAULT 0,
    -- Montag der geplanten ISO-Woche; NULL = noch nicht geplant
    geplante_woche date,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT production_order_operations_order_pos_key UNIQUE (order_id, pos),
    CONSTRAINT production_order_operations_pos_check CHECK (pos > 0),
    CONSTRAINT production_order_operations_name_nonempty CHECK (btrim(name) <> ''),
    CONSTRAINT production_order_operations_zeit_check CHECK (ruestzeit_min >= 0 AND minuten_je_stueck >= 0),
    CONSTRAINT production_order_operations_woche_montag CHECK (geplante_woche IS NULL OR extract(isodow FROM geplante_woche) = 1)
);
CREATE INDEX IF NOT EXISTS idx_production_order_operations_load
    ON production_order_operations (work_center_id, geplante_woche);

COMMENT ON TABLE production_order_operations IS
    'Arbeitsgaenge am Fertigungsauftrag: Arbeitsplatz, Planzeit, geplante Woche (ADR 0037), aenderbar bis fertig/storniert.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS production_order_operations;
--   DROP TABLE IF EXISTS production_operation_templates;
--   DROP TABLE IF EXISTS production_work_centers;
-- DATENVERLUSTRISIKO (MITTEL): verloren gehen Arbeitsplaetze, Vorlagen und
-- die Arbeitsgaenge samt Planung aller Fertigungsauftraege; Auftraege,
-- Stuecklisten und Reservierungen bleiben unberuehrt.
