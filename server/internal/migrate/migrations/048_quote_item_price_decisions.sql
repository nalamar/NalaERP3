CREATE TABLE IF NOT EXISTS quote_item_price_decisions (
    id UUID PRIMARY KEY,
    quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    quote_item_id UUID NOT NULL REFERENCES quote_items(id) ON DELETE CASCADE,
    material_id text REFERENCES materials(id) ON DELETE SET NULL,
    decision_type text NOT NULL,
    source_label text NOT NULL,
    source_unit_price numeric(18,4) NOT NULL,
    applied_unit_price numeric(18,4) NOT NULL,
    currency char(3) NOT NULL DEFAULT 'EUR',
    source_reference text NOT NULL DEFAULT '',
    source_date timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_price_decisions_type'
    ) THEN
        ALTER TABLE quote_item_price_decisions
            ADD CONSTRAINT chk_quote_item_price_decisions_type
            CHECK (decision_type IN ('primary_source_applied'));
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_price_decisions_prices'
    ) THEN
        ALTER TABLE quote_item_price_decisions
            ADD CONSTRAINT chk_quote_item_price_decisions_prices
            CHECK (source_unit_price >= 0 AND applied_unit_price >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_price_decisions_currency'
    ) THEN
        ALTER TABLE quote_item_price_decisions
            ADD CONSTRAINT chk_quote_item_price_decisions_currency
            CHECK (BTRIM(currency) <> '');
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_quote_item_price_decisions_quote_item_created
    ON quote_item_price_decisions(quote_item_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_quote_item_price_decisions_quote_created
    ON quote_item_price_decisions(quote_id, created_at DESC);
