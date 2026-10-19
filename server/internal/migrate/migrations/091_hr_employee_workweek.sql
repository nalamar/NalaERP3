-- Bundesland und Arbeitstage am Mitarbeiter (ADR 0029, Backlog F.8.2.3).
--
-- bundesland NULL = es gilt das Bundesland des Mandanten
-- (hr_calendar_settings, 090). Gesetzt fuer Mitarbeiter an einem Standort
-- in einem anderen Land (z. B. Monteure einer Niederlassung).
--
-- arbeitstage: Wochentage nach ISO 8601 (1 = Montag ... 7 = Sonntag), an
-- denen der Mitarbeiter regelmaessig arbeitet. Standard Mo-Fr. Daraus
-- zaehlen Urlaubsantraege ihre Tage (Teilzeit an drei Tagen: ein
-- Urlaubstag je freiem Arbeitstag, Wochenenden und Feiertage zaehlen nicht).

ALTER TABLE hr_employees
    ADD COLUMN IF NOT EXISTS bundesland text CHECK (bundesland IS NULL OR bundesland IN (
        'BW', 'BY', 'BE', 'BB', 'HB', 'HH', 'HE', 'MV',
        'NI', 'NW', 'RP', 'SL', 'SN', 'ST', 'SH', 'TH')),
    ADD COLUMN IF NOT EXISTS arbeitstage smallint[] NOT NULL DEFAULT '{1,2,3,4,5}'
        CHECK (cardinality(arbeitstage) BETWEEN 1 AND 7 AND arbeitstage <@ '{1,2,3,4,5,6,7}'::smallint[]);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE hr_employees DROP COLUMN IF EXISTS arbeitstage, DROP COLUMN IF EXISTS bundesland;
-- DATENVERLUSTRISIKO (GERING): verloren gehen abweichende Bundeslaender und
-- Arbeitstage der Mitarbeiter; Urlaubsantraege behalten ihre gespeicherte
-- Tageszahl.
