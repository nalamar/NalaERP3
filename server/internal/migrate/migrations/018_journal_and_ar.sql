-- Journal und Ausgangsrechnungen
CREATE TABLE IF NOT EXISTS journal_entries (
    id UUID PRIMARY KEY,
    entry_date date NOT NULL,
    description text NOT NULL DEFAULT '',
    currency char(3) NOT NULL DEFAULT 'EUR',
    source text NOT NULL DEFAULT '',
    source_id text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS journal_lines (
    id UUID PRIMARY KEY,
    entry_id UUID NOT NULL REFERENCES journal_entries(id) ON DELETE CASCADE,
    account_code text NOT NULL REFERENCES accounts(code),
    debit numeric(18,4) NOT NULL DEFAULT 0,
    credit numeric(18,4) NOT NULL DEFAULT 0,
    memo text NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_journal_lines_entry ON journal_lines(entry_id);

CREATE TABLE IF NOT EXISTS invoices_out (
    id UUID PRIMARY KEY,
    nummer text UNIQUE,
    contact_id text NOT NULL REFERENCES contacts(id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'draft', -- draft | booked | paid
    invoice_date date NOT NULL,
    due_date date,
    currency char(3) NOT NULL DEFAULT 'EUR',
    net_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_amount numeric(18,4) NOT NULL DEFAULT 0,
    gross_amount numeric(18,4) NOT NULL DEFAULT 0,
    journal_entry_id UUID REFERENCES journal_entries(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_invoices_out_contact ON invoices_out(contact_id);
CREATE INDEX IF NOT EXISTS idx_invoices_out_status ON invoices_out(status);

CREATE TABLE IF NOT EXISTS invoice_out_items (
    id UUID PRIMARY KEY,
    invoice_id UUID NOT NULL REFERENCES invoices_out(id) ON DELETE CASCADE,
    position int NOT NULL,
    description text NOT NULL,
    qty numeric(18,4) NOT NULL DEFAULT 1,
    unit_price numeric(18,4) NOT NULL DEFAULT 0,
    net_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_amount numeric(18,4) NOT NULL DEFAULT 0,
    tax_code text REFERENCES tax_codes(code),
    account_code text NOT NULL REFERENCES accounts(code)
);
CREATE INDEX IF NOT EXISTS idx_invoice_out_items_invoice ON invoice_out_items(invoice_id);
