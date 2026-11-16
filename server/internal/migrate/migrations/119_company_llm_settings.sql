-- Sprachmodell je Mandant (ADR 0043 Fragen 3 und 6, Backlog I.3.5a1).
--
-- Eine Zeile je Mandant; fehlt sie, ist kein Sprachmodell eingerichtet und es
-- geht nichts nach aussen. Der API-Schluessel steht nur verschluesselt in
-- api_key_enc (secretbox wie der Mail-Zugang, ADR 0033: Schluessel aus
-- SETTINGS_ENCRYPTION_KEY, AAD "llm-api-key:<Mandant>"); die API liefert ihn
-- nie aus. Zeitlimit Standard 300 s (vom Nutzer so festgelegt), einstellbar
-- 1 bis 1800 s. Tageslimit begrenzt die Aufrufe je Kalendertag.
CREATE TABLE IF NOT EXISTS company_llm_settings (
    company_id text PRIMARY KEY REFERENCES company_profiles(id),
    aktiv boolean NOT NULL DEFAULT false,
    anbieter text NOT NULL DEFAULT 'openai_compatible',
    endpunkt text NOT NULL DEFAULT '',
    modell text NOT NULL DEFAULT '',
    api_key_enc bytea,
    timeout_s integer NOT NULL DEFAULT 300,
    tageslimit integer NOT NULL DEFAULT 500,
    updated_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT company_llm_settings_anbieter_check
        CHECK (anbieter IN ('openai_compatible', 'anthropic', 'mistral')),
    CONSTRAINT company_llm_settings_timeout_check
        CHECK (timeout_s BETWEEN 1 AND 1800),
    CONSTRAINT company_llm_settings_tageslimit_check
        CHECK (tageslimit BETWEEN 1 AND 100000),
    -- Version 1: 1 Byte Version + 12 Byte Nonce + mind. 16 Byte Tag.
    CONSTRAINT company_llm_settings_api_key_check
        CHECK (api_key_enc IS NULL OR (get_byte(api_key_enc, 0) = 1 AND length(api_key_enc) >= 29)),
    -- Aktiv nur vollstaendig eingerichtet: OpenAI-kompatibel braucht Endpunkt
    -- und Modell, Anthropic und Mistral einen Schluessel.
    CONSTRAINT company_llm_settings_aktiv_check
        CHECK (NOT aktiv OR (
            (anbieter <> 'openai_compatible' OR (btrim(endpunkt) <> '' AND btrim(modell) <> ''))
            AND (anbieter = 'openai_compatible' OR api_key_enc IS NOT NULL)))
);

COMMENT ON TABLE company_llm_settings IS
    'Sprachmodell je Mandant (ADR 0043): API-Schluessel nur verschluesselt, nie ueber die API.';

-- Aufrufe je Mandant und Tag fuer das Tageslimit.
CREATE TABLE IF NOT EXISTS company_llm_usage (
    company_id text NOT NULL REFERENCES company_profiles(id),
    tag date NOT NULL,
    aufrufe integer NOT NULL DEFAULT 0,
    PRIMARY KEY (company_id, tag),
    CONSTRAINT company_llm_usage_aufrufe_check CHECK (aufrufe >= 0)
);

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS company_llm_usage;
--   DROP TABLE IF EXISTS company_llm_settings;
-- DATENVERLUSTRISIKO (NIEDRIG): verloren gehen die eingetragenen Zugaenge
-- zum Sprachmodell (Schluessel beim Anbieter neu holen) und die Zaehler fuer
-- das Tageslimit. Keine Geschaeftsdaten betroffen.
