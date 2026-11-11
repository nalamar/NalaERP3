-- Rueckmeldung je Arbeitsgang (ADR 0038 Frage 5 (a), (b) vorbereitet,
-- Backlog H.3.5a).
--
-- production_operation_feedback: eine Rueckmeldung "erledigt" je
-- Arbeitsgang als eigener Datensatz (nicht als Spalten am Arbeitsgang),
-- damit Ruecknahmen nachvollziehbar bleiben und (b) spaeter nur einen
-- nullbaren Bezug der Zeiterfassung zum Arbeitsgang braucht.
--   - ist_minuten: optionale Ist-Zeit des Arbeitsgangs in Minuten, OHNE
--     Personenbezug (keine Mitarbeiter-Spalte; BetrVG/DSGVO, ADR 0038).
--   - gemeldet_by_user_id: wer im System gemeldet hat (Benutzer, nicht
--     ausfuehrender Mitarbeiter) - Protokoll, keine Leistungskontrolle.
--   - Ruecknahme nur vollstaendig: Zeitpunkt und Begruendung (Benutzer
--     optional, SET NULL wie ueberall). Danach kann neu gemeldet werden.
-- Je Arbeitsgang hoechstens eine gueltige (nicht zurueckgenommene)
-- Rueckmeldung (partieller Unique-Index).
--
-- Der Arbeitsgang ist RESTRICT: ein Arbeitsgang mit Rueckmeldungen laesst
-- sich nicht mehr entfernen (der Service lehnt das ab Backlog H.3.5b mit
-- einer Meldung ab). company_id wie bei den Entnahmen (113) fuer die
-- Mandantenpruefung ohne Umweg ueber den Auftrag.

CREATE TABLE IF NOT EXISTS production_operation_feedback (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    operation_id uuid NOT NULL REFERENCES production_order_operations(id) ON DELETE RESTRICT,
    ist_minuten numeric(10,2),
    notiz text NOT NULL DEFAULT '',
    gemeldet_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    gemeldet_at timestamptz NOT NULL DEFAULT now(),
    zurueckgenommen_at timestamptz,
    zurueckgenommen_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    ruecknahme_reason text NOT NULL DEFAULT '',
    CONSTRAINT production_operation_feedback_ist_check
        CHECK (ist_minuten IS NULL OR (ist_minuten > 0 AND ist_minuten <= 100000)),
    -- Ruecknahme nur vollstaendig: Zeitpunkt und Begruendung.
    CONSTRAINT production_operation_feedback_ruecknahme_check CHECK (
        (zurueckgenommen_at IS NULL AND ruecknahme_reason = '' AND zurueckgenommen_by_user_id IS NULL)
        OR (zurueckgenommen_at IS NOT NULL AND btrim(ruecknahme_reason) <> '' AND zurueckgenommen_at >= gemeldet_at)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_production_operation_feedback_active
    ON production_operation_feedback(operation_id) WHERE zurueckgenommen_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_production_operation_feedback_operation ON production_operation_feedback(operation_id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS production_operation_feedback;
-- DATENVERLUSTRISIKO (MITTEL): verloren geht, welche Arbeitsgaenge als
-- erledigt gemeldet waren, die Ist-Minuten, wer gemeldet und wer
-- zurueckgenommen hat (mit Begruendung). Arbeitsgaenge, Auftraege und
-- Planzeiten bleiben; erledigte Arbeitsgaenge zaehlen danach wieder zur
-- Auslastung.
