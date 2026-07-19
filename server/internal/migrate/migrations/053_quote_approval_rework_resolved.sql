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
        CHECK (status IN ('requested', 'approved', 'rejected', 'cancelled', 'rework_resolved'));

    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_approval_requests_lifecycle_state'
    ) THEN
        ALTER TABLE quote_item_approval_requests
            DROP CONSTRAINT chk_quote_item_approval_requests_lifecycle_state;
    END IF;

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
            OR
            (status = 'rework_resolved' AND cancelled_at IS NULL AND decided_at IS NOT NULL AND approved_unit_price_snapshot IS NOT NULL AND approved_target_margin_percent_snapshot IS NOT NULL)
        );
END $$;
