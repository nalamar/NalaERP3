-- Offene Posten und Zahlungen / Bankimport
ALTER TABLE invoices_out
    ADD COLUMN IF NOT EXISTS paid_amount numeric(18,4) NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS invoice_out_payments (
    id UUID PRIMARY KEY,
    invoice_id UUID NOT NULL REFERENCES invoices_out(id) ON DELETE CASCADE,
    amount numeric(18,4) NOT NULL,
    currency char(3) NOT NULL DEFAULT 'EUR',
    method text NOT NULL DEFAULT 'bank', -- bank | cash | other
    reference text NOT NULL DEFAULT '',
    paid_at date NOT NULL,
    journal_entry_id UUID REFERENCES journal_entries(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_invoice_out_payments_invoice ON invoice_out_payments(invoice_id);

CREATE TABLE IF NOT EXISTS bank_statements (
    id UUID PRIMARY KEY,
    booking_date date NOT NULL,
    value_date date,
    amount numeric(18,4) NOT NULL,
    currency char(3) NOT NULL DEFAULT 'EUR',
    counterparty text NOT NULL DEFAULT '',
    reference text NOT NULL DEFAULT '',
    raw jsonb NOT NULL DEFAULT '{}'::jsonb,
    matched_payment_id UUID REFERENCES invoice_out_payments(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_bank_statements_amount ON bank_statements(amount);
CREATE INDEX IF NOT EXISTS idx_bank_statements_ref ON bank_statements(reference);
