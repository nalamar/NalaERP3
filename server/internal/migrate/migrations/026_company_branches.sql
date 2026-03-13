CREATE TABLE IF NOT EXISTS company_branches (
    id TEXT PRIMARY KEY,
    company_id TEXT NOT NULL REFERENCES company_profiles(id) ON DELETE CASCADE,
    code TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    street TEXT NOT NULL DEFAULT '',
    postal_code TEXT NOT NULL DEFAULT '',
    city TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT 'DE',
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_company_branches_company_code
    ON company_branches(company_id, lower(btrim(code)))
    WHERE btrim(code) <> '';

CREATE INDEX IF NOT EXISTS idx_company_branches_company
    ON company_branches(company_id, is_default DESC, name ASC);
