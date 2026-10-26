-- Zwischenbestaetigung von Nachweisen (Backlog F.10.3, Entscheidung des
-- Nutzers vom 2026-10-26: Vorschlag aus docs/open-questions.md 2026-10-21).
--
-- Anlass ist die Schweisserpruefung nach DIN EN ISO 9606-1: alle sechs
-- Monate bestaetigt die Schweissaufsicht bzw. der Arbeitgeber, dass der
-- Schweisser im Geltungsbereich geschweisst hat; ohne Bestaetigung
-- verliert das Zertifikat seine Gueltigkeit. Allgemein gehalten, weil es
-- solche Zwischenbestaetigungen auch anderswo gibt.
--
-- 1) An der Art: bestaetigung_monate NULL = keine Zwischenbestaetigung.
--    Ist sie gesetzt, muss ein aktiver Nachweis dieser Art spaetestens
--    N Monate nach dem Erwerb bzw. nach der letzten Bestaetigung bestaetigt
--    werden; eine ueberfaellige Bestaetigung wirkt wie ein abgelaufener
--    Nachweis (berechnet beim Lesen, nicht gespeichert).
ALTER TABLE hr_qualification_types
    ADD COLUMN IF NOT EXISTS bestaetigung_monate integer;

ALTER TABLE hr_qualification_types
    DROP CONSTRAINT IF EXISTS hr_qualification_types_bestaetigung_positive;
ALTER TABLE hr_qualification_types
    ADD CONSTRAINT hr_qualification_types_bestaetigung_positive
        CHECK (bestaetigung_monate IS NULL OR bestaetigung_monate > 0);

COMMENT ON COLUMN hr_qualification_types.bestaetigung_monate IS
    'Zwischenbestaetigung alle N Monate (z. B. Schweisserpruefung: 6). NULL = keine. Ueberfaellig wirkt wie abgelaufen (F.10.3).';

-- 2) Die Bestaetigungen: eigene, unveraenderliche Eintraege am Nachweis
--    (Datum, wer bestaetigt hat). Fehler nur ueber Storno, wie die
--    Nachweise selbst (088). Kein company_id: Scope ueber den Nachweis und
--    dessen Mitarbeiter; die Zugehoerigkeit prueft der Service.
CREATE TABLE IF NOT EXISTS hr_qualification_confirmations (
    id uuid PRIMARY KEY,
    qualification_id uuid NOT NULL REFERENCES hr_employee_qualifications(id) ON DELETE RESTRICT,
    bestaetigt_am date NOT NULL,
    -- Wer bestaetigt hat (Name bzw. Funktion, z. B. "Schweissaufsicht").
    bestaetigt_durch text NOT NULL,
    note text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT hr_qualification_confirmations_durch_nonempty CHECK (btrim(bestaetigt_durch) <> ''),
    CONSTRAINT hr_qualification_confirmations_status_check CHECK (status IN ('active', 'storniert')),
    CONSTRAINT hr_qualification_confirmations_storno_consistent
        CHECK (
            (status = 'active' AND storniert_at IS NULL AND storno_reason = '')
            OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
        )
);
CREATE INDEX IF NOT EXISTS idx_hr_qualification_confirmations_qualification
    ON hr_qualification_confirmations(qualification_id, bestaetigt_am DESC);

COMMENT ON TABLE hr_qualification_confirmations IS
    'Zwischenbestaetigungen eines Nachweises (F.10.3), z. B. halbjaehrlich bei der Schweisserpruefung. Unveraenderlich, Fehler nur ueber Storno.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS hr_qualification_confirmations;
--   ALTER TABLE hr_qualification_types DROP CONSTRAINT IF EXISTS hr_qualification_types_bestaetigung_positive;
--   ALTER TABLE hr_qualification_types DROP COLUMN IF EXISTS bestaetigung_monate;
-- DATENVERLUSTRISIKO (HOCH): mit hr_qualification_confirmations gehen die
-- erfassten Zwischenbestaetigungen samt Storno-Historie verloren - ohne sie
-- ist nicht mehr belegbar, dass ein Schweisserzertifikat gueltig geblieben
-- ist. Vorher exportieren. Mit der Spalte geht die Einstellung "Bestaetigung
-- alle N Monate" je Art verloren (geringes Risiko, neu eintragbar).
-- Nachweise, Arten und alle uebrigen Tabellen bleiben unberuehrt.
