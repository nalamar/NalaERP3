-- Versand der Hinweise per E-Mail (ADR 0033, Backlog F.14.4).
--
-- Werktags um 7:00 erhaelt jeder Benutzer EINE Sammelmail mit seinen
-- Hinweisen (dieselben wie auf dem Dashboard: nur Arten und Anzahlen, nie
-- Namen) - und nur, wenn etwas faellig ist. Vorschlag des Nutzers
-- angenommen am 2026-10-29.
--
-- notice_mail_optouts: wer die Mails fuer sich abbestellt hat.
-- notice_mail_log: das Versandprotokoll - wann, an welchen Benutzer,
-- Ergebnis und bei Fehler die Stufe (Verbindung, Anmeldung ...). OHNE
-- Inhalt, ohne Adresse, ohne Antworttext des Mail-Servers. Hoechstens ein
-- Eintrag je Benutzer und Versandtag: er wird VOR dem Versand angelegt
-- ("versand") und sichert so gegen doppelten Versand, auch wenn zwei
-- Prozesse gleichzeitig laufen. Ein Fehler wird am selben Tag nicht
-- wiederholt.
CREATE TABLE IF NOT EXISTS notice_mail_optouts (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    abbestellt_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notice_mail_log (
    id uuid PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    -- SET NULL: das Protokoll bleibt, wenn ein Benutzer entfernt wird.
    user_id text REFERENCES users(id) ON DELETE SET NULL,
    versand_tag date NOT NULL,
    ergebnis text NOT NULL,
    fehler_stufe text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    CONSTRAINT notice_mail_log_ergebnis_check
        CHECK (ergebnis IN ('versand', 'gesendet', 'fehler')),
    CONSTRAINT notice_mail_log_stufe_check
        CHECK ((ergebnis = 'fehler') = (fehler_stufe <> ''))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notice_mail_log_once
    ON notice_mail_log(company_id, user_id, versand_tag);
CREATE INDEX IF NOT EXISTS idx_notice_mail_log_company_created
    ON notice_mail_log(company_id, created_at DESC);

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS notice_mail_log;
--   DROP TABLE IF EXISTS notice_mail_optouts;
-- DATENVERLUSTRISIKO (MITTEL): verloren gehen das Versandprotokoll und
-- die Abbestellungen. Ohne die Abbestellungen bekaemen Benutzer, die keine
-- Mails wollen, wieder welche - vorher die Liste sichern. Ohne Protokoll
-- koennte am selben Tag ein zweites Mal versandt werden.
