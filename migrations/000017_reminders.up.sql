CREATE TABLE reminder_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email_enabled BOOLEAN NOT NULL DEFAULT false,
    daily_email_cap INT NOT NULL DEFAULT 3,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE card_checkin_reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_id UUID NOT NULL REFERENCES bingo_cards(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    frequency TEXT NOT NULL,
    schedule JSONB NOT NULL DEFAULT '{}'::jsonb,
    include_image BOOLEAN NOT NULL DEFAULT true,
    include_recommendations BOOLEAN NOT NULL DEFAULT true,
    next_send_at TIMESTAMPTZ,
    last_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, card_id)
);

CREATE INDEX idx_card_checkin_due ON card_checkin_reminders(next_send_at) WHERE enabled = true;

CREATE TABLE goal_reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_id UUID NOT NULL REFERENCES bingo_cards(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES bingo_items(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    kind TEXT NOT NULL,
    schedule JSONB NOT NULL DEFAULT '{}'::jsonb,
    next_send_at TIMESTAMPTZ,
    last_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, item_id)
);

CREATE INDEX idx_goal_reminders_due ON goal_reminders(next_send_at) WHERE enabled = true;

CREATE TABLE reminder_image_tokens (
    token VARCHAR(64) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_id UUID NOT NULL REFERENCES bingo_cards(id) ON DELETE CASCADE,
    show_completions BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_accessed_at TIMESTAMPTZ,
    access_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_reminder_image_tokens_expires ON reminder_image_tokens(expires_at);

CREATE TABLE reminder_email_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    source_id UUID NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_on DATE NOT NULL DEFAULT CURRENT_DATE,
    provider_message_id TEXT,
    status TEXT NOT NULL,
    CHECK (source_type IN ('card_checkin', 'goal_reminder')),
    CHECK (status IN ('sent', 'failed'))
);

CREATE INDEX idx_reminder_email_log_user_sent ON reminder_email_log(user_id, sent_at DESC);
CREATE UNIQUE INDEX idx_reminder_email_log_checkin_day ON reminder_email_log(source_type, source_id, sent_on)
    WHERE source_type = 'card_checkin';

CREATE TABLE reminder_unsubscribe_tokens (
    token VARCHAR(64) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    used_at TIMESTAMPTZ
);

CREATE INDEX idx_reminder_unsubscribe_tokens_expires ON reminder_unsubscribe_tokens(expires_at);

INSERT INTO reminder_settings (user_id)
SELECT id FROM users
ON CONFLICT DO NOTHING;
