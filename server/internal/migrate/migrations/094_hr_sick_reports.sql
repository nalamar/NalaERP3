-- Krankmeldungen als Abwesenheit (ADR 0029, Backlog F.8.4.2).
--
-- Eine Krankmeldung wird nicht beantragt und nicht genehmigt: sie ist eine
-- Abwesenheit vom Typ 'sick' ohne Antrag (source_request_id NULL).
-- Gespeichert werden NUR Zeitraum und ob ein Attest vorliegt - keine
-- Diagnose, kein Grund, kein Freitext (Gesundheitsdaten, Art. 9 DSGVO).
--
-- gemeldet_von:
--   antrag    aus einem genehmigten Urlaubsantrag (F.8.4.1)
--   personal  von der Personalabteilung erfasst (gilt als bestaetigt)
--   selbst    vom Mitarbeiter selbst gemeldet - erscheint in der Liste
--             "neue Krankmeldungen", bis die Personalabteilung bestaetigt
--
-- Ein Attest bei Krankheit im genehmigten Urlaub fuehrt dazu, dass die
-- kranken Arbeitstage nicht als genommener Urlaub zaehlen (§9 BUrlG). Das
-- rechnet das Urlaubskonto beim Lesen; gespeichert wird dafuer nichts.

ALTER TABLE hr_absences
    ADD COLUMN IF NOT EXISTS attest boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS gemeldet_von text NOT NULL DEFAULT 'antrag',
    ADD COLUMN IF NOT EXISTS gemeldet_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS bestaetigt_at timestamptz,
    ADD COLUMN IF NOT EXISTS bestaetigt_by_user_id text REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE hr_absences DROP CONSTRAINT IF EXISTS hr_absences_gemeldet_von_check;
ALTER TABLE hr_absences ADD CONSTRAINT hr_absences_gemeldet_von_check
    CHECK (gemeldet_von IN ('antrag', 'personal', 'selbst'));
ALTER TABLE hr_absences DROP CONSTRAINT IF EXISTS hr_absences_typ_check;
ALTER TABLE hr_absences ADD CONSTRAINT hr_absences_typ_check
    CHECK (typ IN ('vacation', 'sick', 'unpaid', 'other'));
ALTER TABLE hr_absences DROP CONSTRAINT IF EXISTS hr_absences_end_check;
ALTER TABLE hr_absences ADD CONSTRAINT hr_absences_end_check CHECK (end_date >= start_date);
-- Eine Abwesenheit ohne Antrag ist eine Krankmeldung - und umgekehrt.
ALTER TABLE hr_absences DROP CONSTRAINT IF EXISTS hr_absences_source_check;
ALTER TABLE hr_absences ADD CONSTRAINT hr_absences_source_check
    CHECK ((gemeldet_von = 'antrag') = (source_request_id IS NOT NULL)
       AND (gemeldet_von = 'antrag' OR typ = 'sick'));

CREATE INDEX IF NOT EXISTS idx_hr_absences_new_sick
    ON hr_absences(employee_id) WHERE typ = 'sick' AND gemeldet_von = 'selbst' AND bestaetigt_at IS NULL AND status = 'approved';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM hr_absences WHERE source_request_id IS NULL;  -- Krankmeldungen
--   DROP INDEX IF EXISTS idx_hr_absences_new_sick;
--   ALTER TABLE hr_absences DROP CONSTRAINT IF EXISTS hr_absences_source_check,
--       DROP CONSTRAINT IF EXISTS hr_absences_end_check, DROP CONSTRAINT IF EXISTS hr_absences_typ_check,
--       DROP CONSTRAINT IF EXISTS hr_absences_gemeldet_von_check,
--       DROP COLUMN IF EXISTS bestaetigt_by_user_id, DROP COLUMN IF EXISTS bestaetigt_at,
--       DROP COLUMN IF EXISTS gemeldet_by_user_id, DROP COLUMN IF EXISTS gemeldet_von, DROP COLUMN IF EXISTS attest;
-- DATENVERLUSTRISIKO (HOCH): alle Krankmeldungen gehen verloren - sie sind
-- Grundlage der Entgeltfortzahlung und der Rueckbuchung von Urlaub nach
-- §9 BUrlG. Vor einem Downgrade exportieren.
