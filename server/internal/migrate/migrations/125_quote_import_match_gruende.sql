-- Gruende zu Entscheidungen der Zuordnung (ADR 0046 Frage 3, Backlog I.8.3a).
--
-- Beim Aendern oder Verwerfen eines Vorschlags kann der Kalkulator einen
-- Grund aus einer festen Auswahl angeben (zaehlbar fuer das Scoring je
-- Profil, I.8.4), optional mit Freitext. Der Grund ist freiwillig ('' =
-- ohne Angabe); Bestaetigen loescht ihn. Freitext nur mit Grund.

ALTER TABLE quote_import_item_matches
    ADD COLUMN IF NOT EXISTS grund text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS grund_text text NOT NULL DEFAULT '';

ALTER TABLE quote_import_item_matches
    DROP CONSTRAINT IF EXISTS quote_import_item_matches_grund_check,
    ADD CONSTRAINT quote_import_item_matches_grund_check CHECK (
        grund IN ('', 'falsche_leistung', 'systemvorgabe_falsch', 'einheit_menge', 'keine_passende_leistung', 'sonstiges')
        AND (grund = '' OR entscheidung IN ('geaendert', 'verworfen'))
        AND (grund_text = '' OR grund <> '')
        AND char_length(grund_text) <= 1000);

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE quote_import_item_matches DROP CONSTRAINT IF EXISTS quote_import_item_matches_grund_check,
--     DROP COLUMN IF EXISTS grund, DROP COLUMN IF EXISTS grund_text;
-- DATENVERLUSTRISIKO (NIEDRIG): verloren gehen nur die angegebenen Gruende
-- und Freitexte; Vorschlaege und Entscheidungen bleiben. Vor dem Rueckweg
-- sichern, wenn die Gruende fuer das Scoring gebraucht werden.
