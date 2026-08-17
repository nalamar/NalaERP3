-- Backlog 0.41: chk_quote_item_price_decisions_type (048_quote_item_price_decisions.sql)
-- erlaubte bisher nur decision_type='primary_source_applied'. Der Code
-- fuegt aber seit Einfuehrung von ApplyTargetUnitPriceForQuoteItem
-- (server/internal/quotes/service.go, insertTargetPriceAppliedDecisionTx)
-- auch decision_type='target_price_applied' ein - jeder Aufruf von
-- POST /quotes/{id}/items/{itemID}/apply-target-price scheiterte daher
-- IMMER mit einer Check-Constraint-Verletzung (500).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_quote_item_price_decisions_type'
    ) THEN
        ALTER TABLE quote_item_price_decisions
            DROP CONSTRAINT chk_quote_item_price_decisions_type;
    END IF;

    ALTER TABLE quote_item_price_decisions
        ADD CONSTRAINT chk_quote_item_price_decisions_type
        CHECK (decision_type IN ('primary_source_applied', 'target_price_applied'));
END $$;
