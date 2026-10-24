-- Stellen am Mitarbeiter (ADR 0031, Backlog F.11.3.1). Ein Mitarbeiter
-- kann mehrere Stellen haben ("Monteur" und "Staplerfahrer"). Erforderlich
-- ist fuer ihn, was seine AKTIVEN Stellen verlangen, dazu seine eigenen
-- Anforderungen und was fuer alle Pflicht ist - dauerhaft: eine Aenderung
-- an der Stelle wirkt sofort auf alle Inhaber (Entscheidung B der ADR).
--
-- Die Stelle muss zum Mandanten des Mitarbeiters gehoeren (geprueft im
-- Service, wie bei den Anforderungen). ON DELETE CASCADE vom Mitarbeiter
-- wie bei hr_employee_qualification_requirements; RESTRICT zur Stelle -
-- Stellen werden ohnehin nicht geloescht, nur stillgelegt.
CREATE TABLE IF NOT EXISTS hr_employee_positions (
    employee_id uuid NOT NULL REFERENCES hr_employees(id) ON DELETE CASCADE,
    position_id uuid NOT NULL REFERENCES hr_positions(id) ON DELETE RESTRICT,
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (employee_id, position_id)
);
CREATE INDEX IF NOT EXISTS idx_hr_employee_positions_position ON hr_employee_positions(position_id);

COMMENT ON TABLE hr_employee_positions IS
    'Stellen eines Mitarbeiters (ADR 0031). Ihre Qualifikationsarten gelten als Anforderung, solange die Stelle aktiv ist. Planung, aenderbar - kein Beleg.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS hr_employee_positions;
-- DATENVERLUSTRISIKO (MITTEL): welche Stellen welche Mitarbeiter haben,
-- geht verloren; damit fehlen im Stand die Anforderungen aus den Stellen.
-- Stellen, Nachweise und eigene Anforderungen bleiben unberuehrt.
