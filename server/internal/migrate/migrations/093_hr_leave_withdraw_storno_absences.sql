-- Urlaubsantraege zurueckziehen und stornieren, Abwesenheiten aus der
-- Genehmigung (ADR 0029, Backlog F.8.4.1).
--
-- Statuswerte eines Antrags:
--   pending   offen
--   approved  genehmigt (es gibt eine Abwesenheit dazu)
--   rejected  abgelehnt
--   withdrawn vom Antragsteller (oder der Personalabteilung) zurueckgezogen,
--             solange er offen war
--   storniert genehmigt und danach mit Begruendung storniert; die
--             Abwesenheit ist mitstorniert, die Tage stehen wieder im Konto
-- Bisher stand der Status nur im Kommentar von 020; jetzt als CHECK.

ALTER TABLE hr_leave_requests
    ADD COLUMN IF NOT EXISTS withdrawn_at timestamptz,
    ADD COLUMN IF NOT EXISTS storniert_at timestamptz,
    ADD COLUMN IF NOT EXISTS storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS storno_reason text;

ALTER TABLE hr_leave_requests DROP CONSTRAINT IF EXISTS hr_leave_requests_status_check;
ALTER TABLE hr_leave_requests ADD CONSTRAINT hr_leave_requests_status_check
    CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn', 'storniert'));
-- Ein stornierter Antrag traegt immer Zeitpunkt und Begruendung.
ALTER TABLE hr_leave_requests DROP CONSTRAINT IF EXISTS hr_leave_requests_storno_check;
ALTER TABLE hr_leave_requests ADD CONSTRAINT hr_leave_requests_storno_check
    CHECK (status <> 'storniert' OR (storniert_at IS NOT NULL AND btrim(COALESCE(storno_reason, '')) <> ''));

-- Abwesenheiten (020, bisher ungenutzt): Storno statt Loeschen, wie bei der
-- Zeiterfassung. status 'approved' = gilt, 'storniert' = gilt nicht mehr.
ALTER TABLE hr_absences
    ADD COLUMN IF NOT EXISTS storniert_at timestamptz,
    ADD COLUMN IF NOT EXISTS storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS storno_reason text;
ALTER TABLE hr_absences DROP CONSTRAINT IF EXISTS hr_absences_status_check;
ALTER TABLE hr_absences ADD CONSTRAINT hr_absences_status_check
    CHECK (status IN ('approved', 'storniert'));
-- Zu einem Antrag gibt es hoechstens eine geltende Abwesenheit.
CREATE UNIQUE INDEX IF NOT EXISTS idx_hr_absences_source_request_active
    ON hr_absences(source_request_id) WHERE source_request_id IS NOT NULL AND status = 'approved';

-- Bestand: bereits genehmigte Antraege bekommen ihre Abwesenheit.
INSERT INTO hr_absences (id, employee_id, typ, start_date, end_date, status, source_request_id)
SELECT gen_random_uuid(), lr.employee_id, lr.typ, lr.start_date, lr.end_date, 'approved', lr.id
  FROM hr_leave_requests lr
 WHERE lr.status = 'approved'
   AND NOT EXISTS (SELECT 1 FROM hr_absences a WHERE a.source_request_id = lr.id);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP INDEX IF EXISTS idx_hr_absences_source_request_active;
--   ALTER TABLE hr_absences DROP CONSTRAINT IF EXISTS hr_absences_status_check,
--       DROP COLUMN IF EXISTS storno_reason, DROP COLUMN IF EXISTS storniert_by_user_id, DROP COLUMN IF EXISTS storniert_at;
--   ALTER TABLE hr_leave_requests DROP CONSTRAINT IF EXISTS hr_leave_requests_storno_check,
--       DROP CONSTRAINT IF EXISTS hr_leave_requests_status_check,
--       DROP COLUMN IF EXISTS storno_reason, DROP COLUMN IF EXISTS storniert_by_user_id,
--       DROP COLUMN IF EXISTS storniert_at, DROP COLUMN IF EXISTS withdrawn_at;
--   (Vorher Antraege mit status 'withdrawn'/'storniert' auf einen alten Wert
--   setzen - sonst sind sie danach nicht mehr von offenen/genehmigten zu
--   unterscheiden.)
-- DATENVERLUSTRISIKO (MITTEL): Begruendung, Urheber und Zeitpunkt von
-- Stornos gehen verloren; die bei der Migration angelegten Abwesenheiten
-- bleiben stehen.
