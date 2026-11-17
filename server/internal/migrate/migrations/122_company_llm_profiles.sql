-- Modellprofile je Mandant (ADR 0046 Frage 1, Backlog I.8.1a1).
--
-- Ein Mandant kann mehrere Sprachmodelle hinterlegen; der Kalkulator waehlt
-- beim Lauf eines (ADR 0046 Frage 2). Alles Modellbezogene steht am Profil:
-- Anbieter, Endpunkt, Modell, API-Schluessel (nur verschluesselt, AAD wie
-- bisher "llm-api-key:<Mandant>", damit der uebernommene Schluessel lesbar
-- bleibt), Zeitlimit und die Schwelle fuer bestaetigungsreife Vorschlaege
-- (ADR 0045 Frage 4, angepasst durch ADR 0046; Standard 0,9). aktiv = im
-- Lauf waehlbar und nur vollstaendig eingerichtet (wie bisher an
-- company_llm_settings). Hoechstens ein Standardprofil je Mandant.
--
-- company_llm_settings behaelt Hauptschalter (aktiv) und Tageslimit des
-- Mandanten. Die Spalten anbieter, endpunkt, modell, api_key_enc und
-- timeout_s sind ab hier veraltet: sie behalten den Stand bei der Migration
-- und werden nicht mehr geschrieben (kein Loeschen ohne ADR; Entfernen in
-- einer spaeteren Migration). Die Pruefung "aktiv nur vollstaendig" an
-- company_llm_settings entfaellt, weil sie veraltete Spalten prueft.
--
-- Uebernahme: jede bestehende Einstellung wird Profil "Standard"
-- (Standardprofil, aktiv wie bisher). Wiederholbar: nur fuer Mandanten ohne
-- Profil.

CREATE TABLE IF NOT EXISTS company_llm_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id text NOT NULL REFERENCES company_profiles(id),
    name text NOT NULL,
    aktiv boolean NOT NULL DEFAULT false,
    standard boolean NOT NULL DEFAULT false,
    anbieter text NOT NULL DEFAULT 'openai_compatible',
    endpunkt text NOT NULL DEFAULT '',
    modell text NOT NULL DEFAULT '',
    api_key_enc bytea,
    timeout_s integer NOT NULL DEFAULT 300,
    schwelle numeric(4,3) NOT NULL DEFAULT 0.9,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT company_llm_profiles_name_check
        CHECK (btrim(name) <> '' AND char_length(name) <= 100),
    CONSTRAINT company_llm_profiles_anbieter_check
        CHECK (anbieter IN ('openai_compatible', 'anthropic', 'mistral')),
    CONSTRAINT company_llm_profiles_timeout_check
        CHECK (timeout_s BETWEEN 1 AND 1800),
    CONSTRAINT company_llm_profiles_schwelle_check
        CHECK (schwelle BETWEEN 0 AND 1),
    -- Version 1: 1 Byte Version + 12 Byte Nonce + mind. 16 Byte Tag.
    CONSTRAINT company_llm_profiles_api_key_check
        CHECK (api_key_enc IS NULL OR (get_byte(api_key_enc, 0) = 1 AND length(api_key_enc) >= 29)),
    CONSTRAINT company_llm_profiles_aktiv_check
        CHECK (NOT aktiv OR (
            (anbieter <> 'openai_compatible' OR (btrim(endpunkt) <> '' AND btrim(modell) <> ''))
            AND (anbieter = 'openai_compatible' OR api_key_enc IS NOT NULL)))
);

CREATE UNIQUE INDEX IF NOT EXISTS company_llm_profiles_name_unique
    ON company_llm_profiles (company_id, lower(name));
CREATE UNIQUE INDEX IF NOT EXISTS company_llm_profiles_standard_unique
    ON company_llm_profiles (company_id) WHERE standard;

COMMENT ON TABLE company_llm_profiles IS
    'Modellprofile je Mandant (ADR 0046): API-Schluessel nur verschluesselt, nie ueber die API.';

INSERT INTO company_llm_profiles (company_id, name, aktiv, standard, anbieter, endpunkt, modell, api_key_enc, timeout_s,
                                  updated_by_user_id, updated_at)
SELECT s.company_id, 'Standard', s.aktiv, true, s.anbieter, s.endpunkt, s.modell, s.api_key_enc, s.timeout_s,
       s.updated_by_user_id, s.updated_at
FROM company_llm_settings s
WHERE NOT EXISTS (SELECT 1 FROM company_llm_profiles p WHERE p.company_id = s.company_id);

ALTER TABLE company_llm_settings DROP CONSTRAINT IF EXISTS company_llm_settings_aktiv_check;

COMMENT ON COLUMN company_llm_settings.anbieter IS 'veraltet seit Migration 122, massgeblich company_llm_profiles';
COMMENT ON COLUMN company_llm_settings.endpunkt IS 'veraltet seit Migration 122, massgeblich company_llm_profiles';
COMMENT ON COLUMN company_llm_settings.modell IS 'veraltet seit Migration 122, massgeblich company_llm_profiles';
COMMENT ON COLUMN company_llm_settings.api_key_enc IS 'veraltet seit Migration 122, massgeblich company_llm_profiles';
COMMENT ON COLUMN company_llm_settings.timeout_s IS 'veraltet seit Migration 122, massgeblich company_llm_profiles';

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13). Zuerst den Stand
-- des Standardprofils zurueckschreiben, dann die alte Pruefung wieder
-- anlegen (unvollstaendige Einstellungen vorher ausschalten):
--   UPDATE company_llm_settings s SET anbieter=p.anbieter, endpunkt=p.endpunkt, modell=p.modell,
--          api_key_enc=p.api_key_enc, timeout_s=p.timeout_s
--     FROM company_llm_profiles p WHERE p.company_id = s.company_id AND p.standard;
--   UPDATE company_llm_settings SET aktiv=false
--     WHERE aktiv AND NOT ((anbieter <> 'openai_compatible' OR (btrim(endpunkt) <> '' AND btrim(modell) <> ''))
--                          AND (anbieter = 'openai_compatible' OR api_key_enc IS NOT NULL));
--   ALTER TABLE company_llm_settings ADD CONSTRAINT company_llm_settings_aktiv_check
--       CHECK (NOT aktiv OR (
--           (anbieter <> 'openai_compatible' OR (btrim(endpunkt) <> '' AND btrim(modell) <> ''))
--           AND (anbieter = 'openai_compatible' OR api_key_enc IS NOT NULL)));
--   COMMENT ON COLUMN company_llm_settings.anbieter IS NULL;  (ebenso endpunkt, modell, api_key_enc, timeout_s)
--   DROP TABLE IF EXISTS company_llm_profiles;
-- DATENVERLUSTRISIKO (NIEDRIG): verloren gehen alle Profile ausser dem
-- Standardprofil (dessen Stand wird zurueckgeschrieben), die Schwellen und
-- die Namen der Profile; Schluessel weiterer Profile muessen beim Anbieter
-- neu geholt werden. Keine Geschaeftsdaten betroffen.
