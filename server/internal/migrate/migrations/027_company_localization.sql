CREATE TABLE IF NOT EXISTS company_localization_settings (
    id TEXT PRIMARY KEY,
    default_currency TEXT NOT NULL DEFAULT 'EUR',
    tax_country TEXT NOT NULL DEFAULT 'DE',
    standard_vat_rate NUMERIC(5,2) NOT NULL DEFAULT 19.00,
    locale TEXT NOT NULL DEFAULT 'de-DE',
    timezone TEXT NOT NULL DEFAULT 'Europe/Berlin',
    date_format TEXT NOT NULL DEFAULT 'dd.MM.yyyy',
    number_format TEXT NOT NULL DEFAULT 'de-DE',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO company_localization_settings (
    id,
    default_currency,
    tax_country,
    standard_vat_rate,
    locale,
    timezone,
    date_format,
    number_format
) VALUES (
    'default',
    'EUR',
    'DE',
    19.00,
    'de-DE',
    'Europe/Berlin',
    'dd.MM.yyyy',
    'de-DE'
)
ON CONFLICT (id) DO NOTHING;
