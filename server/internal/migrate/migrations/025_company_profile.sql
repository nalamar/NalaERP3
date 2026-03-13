CREATE TABLE IF NOT EXISTS company_profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    legal_form TEXT NOT NULL DEFAULT '',
    branch_name TEXT NOT NULL DEFAULT '',
    street TEXT NOT NULL DEFAULT '',
    postal_code TEXT NOT NULL DEFAULT '',
    city TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT 'DE',
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    website TEXT NOT NULL DEFAULT '',
    invoice_email TEXT NOT NULL DEFAULT '',
    tax_no TEXT NOT NULL DEFAULT '',
    vat_id TEXT NOT NULL DEFAULT '',
    bank_name TEXT NOT NULL DEFAULT '',
    account_holder TEXT NOT NULL DEFAULT '',
    iban TEXT NOT NULL DEFAULT '',
    bic TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO company_profiles (
    id, name, country
) VALUES (
    'default', 'Nala ERP', 'DE'
)
ON CONFLICT (id) DO NOTHING;
