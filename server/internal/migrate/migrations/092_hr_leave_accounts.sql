-- Urlaubskonto (ADR 0029, Backlog F.8.3.1).
--
-- 1) Jahresanspruch am Mitarbeiter. NULL = gesetzliche Vorgabe: 20 Tage
-- bei der 5-Tage-Woche, anteilig 4 Tage je woechentlichem Arbeitstag
-- (Entscheidung des Nutzers 2026-10-19: "Jeder Mitarbeiter bekommt
-- mindestens 20 Tage Urlaub, wenn anderes ausgehandelt wird, hinterlegt
-- die Personalabteilung das im Mitarbeiter"; die anteilige Rechnung bei
-- Teilzeit ist eine Annahme, siehe ADR 0029). Ein eingetragener Anspruch
-- darf diese Untergrenze nicht unterschreiten - der CHECK sichert das auch
-- gegen gleichzeitige Aenderung der Arbeitstage ab. Halbe Tage zulaessig.
ALTER TABLE hr_employees
    ADD COLUMN IF NOT EXISTS urlaubstage_jahr numeric(5,1)
        CHECK (urlaubstage_jahr IS NULL OR (urlaubstage_jahr <= 366 AND urlaubstage_jahr >= 4 * cardinality(arbeitstage)));

-- 2) Konto je Mitarbeiter und Jahr. Keine Zeile = das Jahr laeuft mit dem
-- Jahresanspruch des Mitarbeiters und ohne Uebertrag. Die Zeile legt die
-- Personalabteilung an: abweichender Anspruch (Teilurlaub im Ein-/
-- Austrittsjahr), Uebertrag aus dem Vorjahr und der Nachweis, wann auf
-- Resturlaub und Verfall hingewiesen wurde (BAG 9 AZR 541/15). Das System
-- laesst nichts verfallen - der Uebertrag wird ausdruecklich gesetzt.
-- Genommene und verplante Tage werden nicht gespeichert, sondern aus den
-- Urlaubsantraegen berechnet.
CREATE TABLE IF NOT EXISTS hr_leave_accounts (
    employee_id uuid NOT NULL REFERENCES hr_employees(id) ON DELETE CASCADE,
    jahr integer NOT NULL CHECK (jahr BETWEEN 1900 AND 2200),
    anspruch_tage numeric(5,1) NOT NULL CHECK (anspruch_tage >= 0 AND anspruch_tage <= 366),
    uebertrag_tage numeric(5,1) NOT NULL DEFAULT 0 CHECK (uebertrag_tage >= 0 AND uebertrag_tage <= 366),
    hinweis_erteilt_am date,
    notiz text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (employee_id, jahr)
);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS hr_leave_accounts;
--   ALTER TABLE hr_employees DROP COLUMN IF EXISTS urlaubstage_jahr;
-- DATENVERLUSTRISIKO (MITTEL): verloren gehen abweichende Jahresansprueche,
-- Uebertraege und die Nachweise "Hinweis erteilt am" - letztere braucht der
-- Arbeitgeber, um den Verfall von Resturlaub belegen zu koennen. Vor einem
-- Downgrade exportieren. Die Urlaubsantraege bleiben unberuehrt.
