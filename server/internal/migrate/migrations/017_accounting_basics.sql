-- Basis Finanzen: Kontenrahmen (SKR04-Auszug) und Steuerkennzeichen
CREATE TABLE IF NOT EXISTS tax_codes (
    code text PRIMARY KEY,
    name text NOT NULL,
    rate numeric(6,4) NOT NULL DEFAULT 0,
    country char(2) NOT NULL DEFAULT 'DE',
    kind text NOT NULL DEFAULT 'both', -- sales | purchase | both
    reverse_charge boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tax_codes_active ON tax_codes (is_active);

CREATE TABLE IF NOT EXISTS accounts (
    code text PRIMARY KEY,
    name text NOT NULL,
    type text NOT NULL CHECK (type IN ('asset','liability','equity','revenue','expense')),
    parent_code text REFERENCES accounts(code),
    tax_code text REFERENCES tax_codes(code),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_accounts_type ON accounts (type);

-- Seed Steuerkennzeichen (DE)
INSERT INTO tax_codes (code, name, rate, country, kind, reverse_charge)
VALUES
    ('DE19', 'USt 19 %', 0.1900, 'DE', 'both', false),
    ('DE7',  'USt 7 %',  0.0700, 'DE', 'both', false),
    ('DE0',  'Steuerfrei 0 %', 0.0000, 'DE', 'both', false),
    ('EU-RC', 'Innergemeinschaftlicher Erwerb (RC)', 0.0000, 'EU', 'purchase', true),
    ('RC',   'Reverse Charge (Drittland/Dienstleistung)', 0.0000, 'DE', 'purchase', true)
ON CONFLICT (code) DO NOTHING;

-- Seed SKR04-Auszug
INSERT INTO accounts (code, name, type, tax_code)
VALUES
    ('1000', 'Kasse', 'asset', NULL),
    ('1200', 'Bank', 'asset', NULL),
    ('1400', 'Forderungen aus Lieferungen und Leistungen', 'asset', NULL),
    ('1571', 'Vorsteuer 7 %', 'asset', 'DE7'),
    ('1576', 'Vorsteuer 19 %', 'asset', 'DE19'),
    ('1600', 'Roh-, Hilfs- und Betriebsstoffe', 'asset', NULL),
    ('1771', 'Umsatzsteuer 7 %', 'liability', 'DE7'),
    ('1776', 'Umsatzsteuer 19 %', 'liability', 'DE19'),
    ('2000', 'Verbindlichkeiten aus Lieferungen und Leistungen', 'liability', NULL),
    ('3000', 'Eigenkapital', 'equity', NULL),
    ('3400', 'Wareneingang 19 %', 'expense', 'DE19'),
    ('3420', 'Wareneingang 7 %', 'expense', 'DE7'),
    ('8000', 'Umsatzerlöse 19 %', 'revenue', 'DE19'),
    ('8100', 'Umsatzerlöse 7 %', 'revenue', 'DE7')
ON CONFLICT (code) DO NOTHING;

-- Nummernkreise für Rechnungen (AR/AP)
INSERT INTO number_sequences(entity, pattern, next_value)
SELECT 'invoice_out', 'RE-{YYYY}-{NNNN}', 1
WHERE NOT EXISTS (SELECT 1 FROM number_sequences WHERE entity='invoice_out');

INSERT INTO number_sequences(entity, pattern, next_value)
SELECT 'invoice_in', 'ER-{YYYY}-{NNNN}', 1
WHERE NOT EXISTS (SELECT 1 FROM number_sequences WHERE entity='invoice_in');
