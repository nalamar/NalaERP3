-- Quotes / Angebote
CREATE TABLE IF NOT EXISTS quotes (
    id UUID PRIMARY KEY,
    nummer text NOT NULL UNIQUE,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    contact_id text NOT NULL REFERENCES contacts(id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'draft', -- draft | sent | accepted | rejected
    quote_date date NOT NULL,
    valid_until date,
    currency char(3) NOT NULL DEFAULT 'EUR',
    note text NOT NULL DEFAULT '',
    net_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_amount numeric(18,4) NOT NULL DEFAULT 0,
    gross_amount numeric(18,4) NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_quotes_project ON quotes(project_id);
CREATE INDEX IF NOT EXISTS idx_quotes_contact ON quotes(contact_id);
CREATE INDEX IF NOT EXISTS idx_quotes_status ON quotes(status);

CREATE TABLE IF NOT EXISTS quote_items (
    id UUID PRIMARY KEY,
    quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    position int NOT NULL,
    description text NOT NULL,
    qty numeric(18,4) NOT NULL DEFAULT 1,
    unit text NOT NULL DEFAULT 'Stk',
    unit_price numeric(18,4) NOT NULL DEFAULT 0,
    net_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_code text REFERENCES tax_codes(code)
);

CREATE INDEX IF NOT EXISTS idx_quote_items_quote ON quote_items(quote_id);

INSERT INTO number_sequences(entity, pattern, next_value)
SELECT 'quote', 'ANG-{YYYY}-{NNNN}', 1
WHERE NOT EXISTS (SELECT 1 FROM number_sequences WHERE entity='quote');
