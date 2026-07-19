ALTER TABLE quote_item_approval_requests
    ADD COLUMN IF NOT EXISTS decided_by text REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS decided_at timestamptz,
    ADD COLUMN IF NOT EXISTS decision_comment text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS approved_unit_price_snapshot numeric(18,4),
    ADD COLUMN IF NOT EXISTS approved_target_margin_percent_snapshot numeric(9,4);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_status'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            DROP CONSTRAINT chk_quote_item_approval_requests_status;
    END IF;

    ALTER TABLE quote_item_approval_requests
        ADD CONSTRAINT chk_quote_item_approval_requests_status
        CHECK (status IN ('requested', 'approved', 'rejected', 'cancelled'));

    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_cancelled_state'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            DROP CONSTRAINT chk_quote_item_approval_requests_cancelled_state;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_lifecycle_state'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            ADD CONSTRAINT chk_quote_item_approval_requests_lifecycle_state
            CHECK (
                (status = 'requested' AND cancelled_at IS NULL AND decided_at IS NULL)
                OR
                (status = 'cancelled' AND cancelled_at IS NOT NULL AND decided_at IS NULL)
                OR
                (status = 'approved' AND cancelled_at IS NULL AND decided_at IS NOT NULL AND approved_unit_price_snapshot IS NOT NULL AND approved_target_margin_percent_snapshot IS NOT NULL)
                OR
                (status = 'rejected' AND cancelled_at IS NULL AND decided_at IS NOT NULL)
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_decision_comment'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            ADD CONSTRAINT chk_quote_item_approval_requests_decision_comment
            CHECK (char_length(decision_comment) <= 500);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_decision_snapshots'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            ADD CONSTRAINT chk_quote_item_approval_requests_decision_snapshots
            CHECK (
                (approved_unit_price_snapshot IS NULL OR approved_unit_price_snapshot >= 0)
                AND
                (approved_target_margin_percent_snapshot IS NULL OR approved_target_margin_percent_snapshot >= 0)
            );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_quote_item_approval_requests_status_created
    ON quote_item_approval_requests(status, created_at DESC);
