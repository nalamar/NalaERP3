-- Zeiterfassung (ADR 0024, Backlog F.1.2). Eine neue Tabelle, eine
-- additive Spalte, drei neue Permissions:
--   - hr_employees.user_id: verknuepft einen Login-Benutzer mit seinem
--     Mitarbeiterdatensatz. Ohne diese Verknuepfung kann das System nicht
--     wissen, fuer wen ein angemeldeter Benutzer stempelt.
--   - hr_time_entries: Beginn/Ende als Zeitpunkte, Pause in Minuten.
--     Eintraege sind unveraenderlich; Korrektur nur ueber Storno plus
--     Ersatzeintrag (replaces_entry_id).
--
-- WICHTIG zur Semantik: gespeichert wird die TATSAECHLICH geleistete
-- Arbeitszeit. Es gibt bewusst KEINEN CHECK auf ArbZG-Grenzen (10 h,
-- Pausen, Ruhezeit) - ein Verstoss ist eine Tatsache und muss
-- aufgezeichnet werden koennen. Die Constraints unten lehnen nur ab, was
-- nicht stimmen KANN.

-- 1) Benutzer <-> Mitarbeiter.
--
-- Nullable: nicht jeder Mitarbeiter hat einen Login, und nicht jeder
-- Login ist ein Mitarbeiter (z. B. ein externer Steuerberater-Zugang).
-- UNIQUE: ein Benutzer stempelt fuer genau einen Mitarbeiter.
-- ON DELETE SET NULL: das Loeschen eines Logins darf den
-- Mitarbeiterdatensatz nicht mitreissen.
ALTER TABLE hr_employees
    ADD COLUMN IF NOT EXISTS user_id text REFERENCES users(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_hr_employees_user_id
    ON hr_employees(user_id) WHERE user_id IS NOT NULL;

-- 2) Zeiteintraege.
--
-- Kein eigenes company_id: der Scope wird ueber das verpflichtende
-- employee_id geerbt, wie bei hr_leave_requests/hr_absences (ADR 0002).
--
-- employee_id mit ON DELETE RESTRICT - ANDERS als die uebrigen
-- HR-Kindtabellen, die CASCADE verwenden. Das ist Absicht und nicht
-- anzugleichen: mit CASCADE wuerde das Loeschen eines Mitarbeiters
-- aufbewahrungspflichtige Aufzeichnungen still mitloeschen.
CREATE TABLE IF NOT EXISTS hr_time_entries (
    id uuid PRIMARY KEY,
    employee_id uuid NOT NULL REFERENCES hr_employees(id) ON DELETE RESTRICT,
    work_date date NOT NULL,
    started_at timestamptz NOT NULL,
    ended_at timestamptz,
    break_minutes integer NOT NULL DEFAULT 0,
    project_id uuid REFERENCES projects(id) ON DELETE SET NULL,
    note text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active',
    storno_reason text NOT NULL DEFAULT '',
    storniert_at timestamptz,
    storniert_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    replaces_entry_id uuid REFERENCES hr_time_entries(id) ON DELETE RESTRICT,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT hr_time_entries_status_check
        CHECK (status IN ('active', 'storniert')),
    CONSTRAINT hr_time_entries_break_nonnegative
        CHECK (break_minutes >= 0),
    CONSTRAINT hr_time_entries_end_after_start
        CHECK (ended_at IS NULL OR ended_at > started_at),
    -- Pause muss kuerzer sein als die Anwesenheit. Bei einem laufenden
    -- Eintrag (ended_at IS NULL) noch nicht pruefbar.
    CONSTRAINT hr_time_entries_break_within_presence
        CHECK (ended_at IS NULL OR break_minutes * interval '1 minute' < ended_at - started_at),
    -- Ein Storno ist nur vollstaendig gueltig: Zeitpunkt und Begruendung
    -- gehoeren zwingend dazu, und ein aktiver Eintrag traegt keines von
    -- beiden. storniert_by_user_id ist bewusst NICHT Teil des Checks -
    -- die Spalte faellt per ON DELETE SET NULL auf NULL, wenn der Benutzer
    -- spaeter geloescht wird, und das darf den Eintrag nicht ungueltig machen.
    CONSTRAINT hr_time_entries_storno_consistent
        CHECK (
            (status = 'active' AND storniert_at IS NULL AND storno_reason = '')
            OR (status = 'storniert' AND storniert_at IS NOT NULL AND btrim(storno_reason) <> '')
        )
);

-- Hoechstens ein laufender Eintrag je Mitarbeiter - von der Datenbank
-- garantiert, nicht nur vom Anwendungscode (zwei gleichzeitige
-- "Kommen"-Buchungen koennten eine reine Vorabpruefung beide passieren).
CREATE UNIQUE INDEX IF NOT EXISTS idx_hr_time_entries_one_open
    ON hr_time_entries(employee_id) WHERE ended_at IS NULL AND status = 'active';

CREATE INDEX IF NOT EXISTS idx_hr_time_entries_employee_date ON hr_time_entries(employee_id, work_date);
CREATE INDEX IF NOT EXISTS idx_hr_time_entries_project ON hr_time_entries(project_id);
CREATE INDEX IF NOT EXISTS idx_hr_time_entries_replaces ON hr_time_entries(replaces_entry_id);

COMMENT ON TABLE hr_time_entries IS
    'Arbeitszeiteintraege (ADR 0024). Unveraenderlich: Korrektur nur ueber Storno plus Ersatzeintrag, kein UPDATE der Zeiten, kein DELETE.';
COMMENT ON COLUMN hr_time_entries.work_date IS
    'Kalendertag des Beginns in Europe/Berlin. Eine Nachtschicht gehoert zum Tag, an dem sie beginnt.';
COMMENT ON COLUMN hr_time_entries.ended_at IS
    'NULL = Eintrag laeuft noch (gestempelt, noch nicht ausgestempelt).';
COMMENT ON COLUMN hr_time_entries.replaces_entry_id IS
    'Bei einer Korrektur: der stornierte Eintrag, den dieser ersetzt.';

-- 3) Permissions.
--
-- time_entries.self geht an ALLE Systemrollen: die eigene Arbeitszeit
-- erfasst jeder, unabhaengig von seiner fachlichen Rolle. read/write
-- (Zeiten ANDERER lesen bzw. fuer andere erfassen, stornieren,
-- korrigieren) gehen nur an HR und Admin.
INSERT INTO permissions (id, code, name, description, context)
VALUES
  ('perm-time-entries-self', 'time_entries.self', 'Eigene Arbeitszeit erfassen', 'Eigene Arbeitszeiten erfassen, korrigieren und anzeigen', 'hr'),
  ('perm-time-entries-read', 'time_entries.read', 'Arbeitszeiten lesen', 'Arbeitszeiten aller Mitarbeiter anzeigen', 'hr'),
  ('perm-time-entries-write', 'time_entries.write', 'Arbeitszeiten schreiben', 'Arbeitszeiten fuer andere erfassen, stornieren und korrigieren', 'hr')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-sales', 'role-procurement', 'role-inventory', 'role-finance', 'role-hr', 'role-production', 'role-fleet')
  AND p.code = 'time_entries.self'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.id IN ('role-admin', 'role-hr')
  AND p.code IN ('time_entries.read', 'time_entries.write')
ON CONFLICT DO NOTHING;

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('time_entries.self', 'time_entries.read', 'time_entries.write'));
--   DELETE FROM permissions WHERE code IN ('time_entries.self', 'time_entries.read', 'time_entries.write');
--   DROP TABLE IF EXISTS hr_time_entries;
--   DROP INDEX IF EXISTS idx_hr_employees_user_id;
--   ALTER TABLE hr_employees DROP COLUMN IF EXISTS user_id;
-- DATENVERLUSTRISIKO (HOCH): Mit der Tabelle gehen saemtliche erfassten
-- Arbeitszeiten samt Storno-/Korrekturhistorie unwiderruflich verloren.
-- Das sind Aufzeichnungen mit gesetzlicher Aufbewahrungspflicht
-- (ArbZG §16 Abs. 2, mindestens zwei Jahre) - ein Downgrade nach
-- produktiver Nutzung ist ohne vorherigen Export nicht zulaessig.
-- Zusaetzlich geht die Zuordnung Benutzer<->Mitarbeiter verloren.
-- Mitarbeiterstamm, Urlaubsantraege, Benutzer und Projekte bleiben
-- unberuehrt: hr_time_entries wird von keiner anderen Tabelle referenziert.
