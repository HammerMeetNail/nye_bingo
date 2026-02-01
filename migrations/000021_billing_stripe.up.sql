-- Stripe Billing + Premium entitlements (additive)

ALTER TABLE users
    ADD COLUMN stripe_customer_id VARCHAR(64) UNIQUE NULL,
    ADD COLUMN stripe_subscription_id VARCHAR(64) UNIQUE NULL,
    ADD COLUMN billing_plan VARCHAR(20) NOT NULL DEFAULT 'free',
    ADD COLUMN billing_source VARCHAR(20) NOT NULL DEFAULT 'none',
    ADD COLUMN billing_status VARCHAR(20) NOT NULL DEFAULT 'inactive',
    ADD COLUMN billing_current_period_end TIMESTAMPTZ NULL,
    ADD COLUMN billing_cancel_at_period_end BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN billing_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE TABLE stripe_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stripe_event_id VARCHAR(128) NOT NULL UNIQUE,
    event_type VARCHAR(128) NOT NULL,
    livemode BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NULL,
    processing_error TEXT NULL
);

CREATE TABLE premium_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash VARCHAR(64) NOT NULL UNIQUE,
    duration_days INT NULL,
    expires_at TIMESTAMPTZ NULL,
    redeemed_by_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    redeemed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

