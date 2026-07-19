CREATE TABLE IF NOT EXISTS quote_calculation_settings (
    id TEXT PRIMARY KEY,
    target_margin_percent NUMERIC(6,2) NOT NULL DEFAULT 20.00,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO quote_calculation_settings (
    id,
    target_margin_percent
) VALUES (
    'default',
    20.00
)
ON CONFLICT (id) DO NOTHING;
