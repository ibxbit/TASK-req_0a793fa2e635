# Business Gaps & Questions

## Q: How to handle expired matches?
- Hypothesis: Auto-cancel after 3 mins per prompt.
- Solution: Implemented background cleanup logic.

## Q: How to ensure offline access for critical datasets?
- Hypothesis: Use client-side cache and persistent retry queue.
- Solution: Implemented IndexedDB cache and local write queue.

## Q: How to prevent duplicate submissions in offline mode?
- Hypothesis: Use idempotency keys for queued actions.
- Solution: All write actions include a unique idempotency token.

## Q: How to enforce discount stacking rules?
- Hypothesis: Limit to one coupon + one campaign, max 40% off.
- Solution: Pricing engine validates and explains stacking in UI.

## Q: How to handle approval workflow for bulk edits?
- Hypothesis: Require admin sign-off within 48 hours.
- Solution: Bulk edits are pending until admin approval or auto-revert.
