-- Mail-Zugang je Mandant (ADR 0033, Backlog F.14.1).
--
-- Eine Zeile je Mandant; fehlt sie, ist kein Versand eingerichtet. Das
-- Passwort steht nur verschluesselt in password_enc (AES-256-GCM, Schluessel
-- aus SETTINGS_ENCRYPTION_KEY, Mandanten-ID als AAD) - der Schluessel liegt
-- nicht in der Datenbank. Die API liefert das Passwort nie aus.
--
-- Unverschluesselte Verbindungen sind nicht vorgesehen (ssl = implizites
-- TLS, starttls); das Passwort ginge sonst im Klartext ueber das Netz.
CREATE TABLE IF NOT EXISTS company_mail_settings (
    company_id text PRIMARY KEY REFERENCES company_profiles(id),
    aktiv boolean NOT NULL DEFAULT false,
    smtp_host text NOT NULL DEFAULT '',
    smtp_port integer NOT NULL DEFAULT 465,
    sicherheit text NOT NULL DEFAULT 'ssl',
    benutzer text NOT NULL DEFAULT '',
    password_enc bytea,
    absender_adresse text NOT NULL DEFAULT '',
    absender_name text NOT NULL DEFAULT '',
    updated_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT company_mail_settings_port_check
        CHECK (smtp_port BETWEEN 1 AND 65535),
    CONSTRAINT company_mail_settings_sicherheit_check
        CHECK (sicherheit IN ('ssl', 'starttls')),
    -- Version 1: 1 Byte Version + 12 Byte Nonce + mind. 16 Byte Tag.
    CONSTRAINT company_mail_settings_password_check
        CHECK (password_enc IS NULL OR (get_byte(password_enc, 0) = 1 AND length(password_enc) >= 29)),
    -- Aktiv nur vollstaendig eingerichtet.
    CONSTRAINT company_mail_settings_aktiv_check
        CHECK (NOT aktiv OR (btrim(smtp_host) <> '' AND btrim(benutzer) <> ''
                             AND password_enc IS NOT NULL AND btrim(absender_adresse) <> ''))
);

COMMENT ON TABLE company_mail_settings IS
    'Mail-Zugang je Mandant (ADR 0033): Passwort nur verschluesselt, nie ueber die API.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS company_mail_settings;
-- DATENVERLUSTRISIKO (NIEDRIG): verloren gehen die eingetragenen
-- Mail-Zugaenge; sie lassen sich neu eintragen (Passwort beim Betreiber des
-- Mailkontos). Keine Geschaeftsdaten betroffen.
