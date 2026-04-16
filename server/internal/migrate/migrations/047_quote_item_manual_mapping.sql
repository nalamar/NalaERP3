ALTER TABLE quote_items
    ADD COLUMN IF NOT EXISTS material_id text REFERENCES materials(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS price_mapping_status text NOT NULL DEFAULT 'open';

UPDATE quote_items
SET price_mapping_status = 'open'
WHERE COALESCE(TRIM(price_mapping_status), '') = '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_items_price_mapping_status'
    ) THEN
        ALTER TABLE quote_items
            ADD CONSTRAINT chk_quote_items_price_mapping_status
            CHECK (price_mapping_status IN ('open', 'manual'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_quote_items_material ON quote_items(material_id);
