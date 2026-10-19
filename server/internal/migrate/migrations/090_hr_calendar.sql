-- Feiertagskalender je Mandant (ADR 0029, Backlog F.8.2.2).
--
-- Gesetzliche Feiertage werden im Code berechnet (internal/hr/holidays.go),
-- nicht gespeichert. Hier steht nur, WELCHES Land gilt, plus die
-- betrieblichen Zusatztage (Betriebsruhe, 24./31.12.).
--
-- hr_holidays (020) bleibt unberuehrt und ungenutzt: ADR 0002 hat sie als
-- mandantenfreien Kalender je Land festgelegt; die betrieblichen Tage sind
-- dagegen Sache des Mandanten. Eine eigene Tabelle vermeidet, dass eine
-- Tabelle zwei Bedeutungen bekommt (Abweichung von ADR 0029 Abschnitt 2,
-- dort vermerkt).

-- 1) Einstellung je Mandant. Keine Zeile oder bundesland NULL = noch nicht
-- eingestellt; dann gelten nur die neun bundesweiten Feiertage.
-- Die Schalter wirken nur in Bayern (Mariae Himmelfahrt nur in
-- ueberwiegend katholischen Gemeinden, Augsburger Friedensfest nur in
-- Augsburg).
CREATE TABLE IF NOT EXISTS hr_calendar_settings (
    company_id text PRIMARY KEY REFERENCES company_profiles(id),
    bundesland text CHECK (bundesland IS NULL OR bundesland IN (
        'BW', 'BY', 'BE', 'BB', 'HB', 'HH', 'HE', 'MV',
        'NI', 'NW', 'RP', 'SL', 'SN', 'ST', 'SH', 'TH')),
    mariae_himmelfahrt boolean NOT NULL DEFAULT false,
    augsburger_friedensfest boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by_user_id text REFERENCES users(id) ON DELETE SET NULL
);

-- 2) Betriebliche Zusatztage: ganze freie Tage fuer alle Mitarbeiter des
-- Mandanten. Ein Tag je Datum. Loeschen ist zulaessig - es ist eine
-- Planungsangabe, kein Nachweis.
CREATE TABLE IF NOT EXISTS hr_company_days (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    datum date NOT NULL,
    name text NOT NULL CHECK (btrim(name) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by_user_id text REFERENCES users(id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_hr_company_days_company_datum ON hr_company_days(company_id, datum);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS hr_company_days;
--   DROP TABLE IF EXISTS hr_calendar_settings;
-- DATENVERLUSTRISIKO (GERING): verloren gehen das eingestellte Bundesland
-- und die gepflegten betrieblichen Zusatztage. Keine andere Tabelle
-- verweist auf die beiden.
