-- Vorschlaege am GAEB-Import (ADR 0045 Fragen 1, 2 und 5; ADR 0046 Frage 2;
-- Backlog I.5.2a).
--
-- quote_import_item_matches: hoechstens ein Vorschlag je Importposition.
-- code = Abschrift des Leistungscodes ('' = keine Zuordnung, z. B. keine
-- Kandidaten); leistung_id verweist auf die Leistung, solange es sie gibt.
-- Profil (ID und Abschrift von Name, Anbieter, Modell, Schwelle) bleibt am
-- Vorschlag, auch wenn das Profil spaeter geaendert oder entfernt wird
-- (Scoring je Profil, ADR 0046 Frage 4). Quelle 'wortabgleich' = ohne
-- Sprachmodell (ADR 0045 Frage 3), dann ohne Profil.
--
-- Entscheidung eines Menschen (ADR 0045 Frage 5): offen, bestaetigt (nur mit
-- Vorschlag), geaendert (andere oder erstmals eine Leistung) oder verworfen.
-- Ein neuer Lauf ersetzt nur offene Vorschlaege. Wird die Importposition beim
-- erneuten Verarbeiten neu angelegt, verschwinden ihre Vorschlaege mit ihr.
--
-- Der Mandant kommt wie bei den Importpositionen ueber Import -> Projekt.
--
-- Status des Laufs am Importlauf (ADR 0045 Frage 2): '' = nie gestartet,
-- laeuft, fertig, fehler, abgebrochen (Prozess beendet waehrend des Laufs);
-- Fortschritt in Positionen; Abschrift des gewaehlten Profils.

CREATE TABLE IF NOT EXISTS quote_import_item_matches (
    quote_import_item_id uuid PRIMARY KEY REFERENCES quote_import_items(id) ON DELETE CASCADE,
    import_id uuid NOT NULL REFERENCES quote_imports(id) ON DELETE CASCADE,
    leistung_id text REFERENCES leistungen(id) ON DELETE SET NULL,
    code text NOT NULL DEFAULT '',
    score numeric(4,3) NOT NULL DEFAULT 0,
    begruendung text NOT NULL DEFAULT '',
    systemvorgaben jsonb NOT NULL DEFAULT '{}',
    quelle text NOT NULL,
    profil_id uuid REFERENCES company_llm_profiles(id) ON DELETE SET NULL,
    profil_name text NOT NULL DEFAULT '',
    anbieter text NOT NULL DEFAULT '',
    modell text NOT NULL DEFAULT '',
    schwelle numeric(4,3) NOT NULL,
    vorgeschlagen_at timestamptz NOT NULL DEFAULT now(),
    entscheidung text NOT NULL DEFAULT 'offen',
    entschieden_leistung_id text REFERENCES leistungen(id) ON DELETE SET NULL,
    entschieden_code text NOT NULL DEFAULT '',
    entschieden_von_user_id text REFERENCES users(id) ON DELETE SET NULL,
    entschieden_at timestamptz,
    CONSTRAINT quote_import_item_matches_code_check CHECK (char_length(code) <= 50 AND char_length(entschieden_code) <= 50),
    CONSTRAINT quote_import_item_matches_score_check CHECK (score BETWEEN 0 AND 1 AND schwelle BETWEEN 0 AND 1),
    CONSTRAINT quote_import_item_matches_begruendung_check CHECK (char_length(begruendung) <= 2000),
    CONSTRAINT quote_import_item_matches_systemvorgaben_check CHECK (jsonb_typeof(systemvorgaben) = 'object'),
    CONSTRAINT quote_import_item_matches_quelle_check CHECK (
        (quelle = 'modell' AND profil_name <> '') OR (quelle = 'wortabgleich' AND profil_id IS NULL AND profil_name = '')),
    CONSTRAINT quote_import_item_matches_entscheidung_check CHECK (
        entscheidung IN ('offen', 'bestaetigt', 'geaendert', 'verworfen')
        AND (entscheidung = 'offen') = (entschieden_at IS NULL)
        AND (entscheidung <> 'bestaetigt' OR code <> '')
        AND ((entscheidung = 'geaendert') = (entschieden_code <> '')))
);

CREATE INDEX IF NOT EXISTS idx_quote_import_item_matches_import
    ON quote_import_item_matches (import_id, entscheidung);
CREATE INDEX IF NOT EXISTS idx_quote_import_item_matches_profil
    ON quote_import_item_matches (profil_id) WHERE profil_id IS NOT NULL;

COMMENT ON TABLE quote_import_item_matches IS
    'Vorschlaege des Matchings je Importposition (ADR 0045): gelten erst nach Entscheidung eines Menschen, ohne Preise.';

ALTER TABLE quote_imports
    ADD COLUMN IF NOT EXISTS match_status text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS match_fehler text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS match_quelle text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS match_profil_id uuid REFERENCES company_llm_profiles(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS match_profil_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS match_anbieter text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS match_modell text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS match_schwelle numeric(4,3),
    ADD COLUMN IF NOT EXISTS match_fortschritt integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS match_gesamt integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS match_gestartet_von_user_id text REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS match_gestartet_at timestamptz,
    ADD COLUMN IF NOT EXISTS match_beendet_at timestamptz;

ALTER TABLE quote_imports
    DROP CONSTRAINT IF EXISTS quote_imports_match_check,
    ADD CONSTRAINT quote_imports_match_check CHECK (
        match_status IN ('', 'laeuft', 'fertig', 'fehler', 'abgebrochen')
        AND match_quelle IN ('', 'modell', 'wortabgleich')
        AND (match_status = '') = (match_gestartet_at IS NULL)
        AND (match_status = '' OR match_quelle <> '')
        AND (match_schwelle IS NULL OR match_schwelle BETWEEN 0 AND 1)
        AND match_fortschritt BETWEEN 0 AND match_gesamt
        AND char_length(match_fehler) <= 2000);

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE quote_imports DROP CONSTRAINT IF EXISTS quote_imports_match_check,
--     DROP COLUMN IF EXISTS match_status, DROP COLUMN IF EXISTS match_fehler, DROP COLUMN IF EXISTS match_quelle,
--     DROP COLUMN IF EXISTS match_profil_id, DROP COLUMN IF EXISTS match_profil_name, DROP COLUMN IF EXISTS match_anbieter,
--     DROP COLUMN IF EXISTS match_modell, DROP COLUMN IF EXISTS match_schwelle, DROP COLUMN IF EXISTS match_fortschritt,
--     DROP COLUMN IF EXISTS match_gesamt, DROP COLUMN IF EXISTS match_gestartet_von_user_id,
--     DROP COLUMN IF EXISTS match_gestartet_at, DROP COLUMN IF EXISTS match_beendet_at;
--   DROP TABLE IF EXISTS quote_import_item_matches;
-- DATENVERLUSTRISIKO (MITTEL, sobald genutzt): verloren gehen alle
-- Vorschlaege mit den Entscheidungen der Kalkulatoren (Grundlage fuer
-- Feedback und Scoring nach ADR 0046) und der Status der Laeufe. Importe,
-- Positionen, bereits uebernommene Angebote und Leistungen bleiben
-- unberuehrt. Vor dem Rueckweg die Tabelle sichern, wenn Entscheidungen
-- erhalten bleiben sollen.
