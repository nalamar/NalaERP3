-- Gesamtbewertung eines Importlaufs bei der Freigabe (ADR 0046 Frage 3,
-- Backlog I.8.3b).
--
-- Bei der Freigabe kann der Kalkulator den Lauf bewerten: Note 1 bis 5,
-- geschaetzte Nacharbeit in Minuten und Freitext, alles freiwillig. Wer und
-- wann wird nur festgehalten, wenn mindestens eine Angabe da ist.

ALTER TABLE quote_imports
    ADD COLUMN IF NOT EXISTS bewertung smallint,
    ADD COLUMN IF NOT EXISTS nacharbeit_min integer,
    ADD COLUMN IF NOT EXISTS bewertung_text text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bewertet_von_user_id text REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS bewertet_at timestamptz;

ALTER TABLE quote_imports
    DROP CONSTRAINT IF EXISTS quote_imports_bewertung_check,
    ADD CONSTRAINT quote_imports_bewertung_check CHECK (
        (bewertung IS NULL OR bewertung BETWEEN 1 AND 5)
        AND (nacharbeit_min IS NULL OR nacharbeit_min BETWEEN 0 AND 100000)
        AND char_length(bewertung_text) <= 2000
        AND (bewertet_at IS NULL) = (bewertung IS NULL AND nacharbeit_min IS NULL AND bewertung_text = '')
        AND (bewertet_von_user_id IS NULL OR bewertet_at IS NOT NULL));

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE quote_imports DROP CONSTRAINT IF EXISTS quote_imports_bewertung_check,
--     DROP COLUMN IF EXISTS bewertung, DROP COLUMN IF EXISTS nacharbeit_min, DROP COLUMN IF EXISTS bewertung_text,
--     DROP COLUMN IF EXISTS bewertet_von_user_id, DROP COLUMN IF EXISTS bewertet_at;
-- DATENVERLUSTRISIKO (NIEDRIG): verloren gehen nur die Bewertungen der
-- Importlaeufe; Laeufe, Positionen und Freigaben bleiben. Vor dem Rueckweg
-- sichern, wenn die Bewertungen fuer das Scoring gebraucht werden.
