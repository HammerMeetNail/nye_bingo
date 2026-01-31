DROP TABLE IF EXISTS premium_codes;
DROP TABLE IF EXISTS stripe_webhook_events;

ALTER TABLE users
    DROP COLUMN IF EXISTS stripe_customer_id,
    DROP COLUMN IF EXISTS stripe_subscription_id,
    DROP COLUMN IF EXISTS billing_plan,
    DROP COLUMN IF EXISTS billing_source,
    DROP COLUMN IF EXISTS billing_status,
    DROP COLUMN IF EXISTS billing_current_period_end,
    DROP COLUMN IF EXISTS billing_cancel_at_period_end,
    DROP COLUMN IF EXISTS billing_updated_at;

