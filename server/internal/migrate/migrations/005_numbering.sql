-- Nummernkreise / Numbering sequences

CREATE TABLE IF NOT EXISTS number_sequences (
  entity text PRIMARY KEY,
  pattern text NOT NULL DEFAULT 'PO-{YYYY}-{NNNN}',
  next_value integer NOT NULL DEFAULT 1,
  last_year integer,
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Seed purchase order numbering if not exists
INSERT INTO number_sequences(entity, pattern, next_value)
SELECT 'purchase_order', 'PO-{YYYY}-{NNNN}', 1
WHERE NOT EXISTS (SELECT 1 FROM number_sequences WHERE entity='purchase_order');

