CREATE TABLE IF NOT EXISTS quote_item_approval_requests (
    id UUID PRIMARY KEY,
    quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    quote_item_id UUID NOT NULL REFERENCES quote_items(id) ON DELETE CASCADE,
    status text NOT NULL,
    reason_code text NOT NULL,
    reason_text text NOT NULL DEFAULT '',
    current_unit_price_snapshot numeric(18,4) NOT NULL,
    cost_basis_unit_price_snapshot numeric(18,4) NOT NULL,
    target_unit_price_snapshot numeric(18,4) NOT NULL,
    target_margin_percent_snapshot numeric(9,4) NOT NULL,
    target_difference_snapshot numeric(18,4) NOT NULL,
    margin_percent_snapshot numeric(9,4),
    price_decision_id UUID REFERENCES quote_item_price_decisions(id) ON DELETE SET NULL,
    requested_by text REFERENCES users(id) ON DELETE SET NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    cancelled_by text REFERENCES users(id) ON DELETE SET NULL,
    cancelled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_status'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            ADD CONSTRAINT chk_quote_item_approval_requests_status
            CHECK (status IN ('requested', 'cancelled'));
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_reason_code'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            ADD CONSTRAINT chk_quote_item_approval_requests_reason_code
            CHECK (reason_code IN ('negative_margin', 'below_target_margin'));
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_snapshots'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            ADD CONSTRAINT chk_quote_item_approval_requests_snapshots
            CHECK (
                current_unit_price_snapshot >= 0
                AND cost_basis_unit_price_snapshot >= 0
                AND target_unit_price_snapshot >= 0
                AND target_margin_percent_snapshot >= 0
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_cancelled_state'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            ADD CONSTRAINT chk_quote_item_approval_requests_cancelled_state
            CHECK (
                (status = 'requested' AND cancelled_at IS NULL)
                OR
                (status = 'cancelled' AND cancelled_at IS NOT NULL)
            );
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS ux_quote_item_approval_requests_active
    ON quote_item_approval_requests(quote_item_id)
    WHERE status = 'requested';

CREATE INDEX IF NOT EXISTS idx_quote_item_approval_requests_quote_created
    ON quote_item_approval_requests(quote_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_quote_item_approval_requests_item_created
    ON quote_item_approval_requests(quote_item_id, created_at DESC);
