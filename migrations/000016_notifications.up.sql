CREATE TABLE notification_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    in_app_enabled BOOLEAN NOT NULL DEFAULT true,
    in_app_friend_request_received BOOLEAN NOT NULL DEFAULT true,
    in_app_friend_request_accepted BOOLEAN NOT NULL DEFAULT true,
    in_app_friend_bingo BOOLEAN NOT NULL DEFAULT true,
    in_app_friend_new_card BOOLEAN NOT NULL DEFAULT true,
    email_enabled BOOLEAN NOT NULL DEFAULT false,
    email_friend_request_received BOOLEAN NOT NULL DEFAULT false,
    email_friend_request_accepted BOOLEAN NOT NULL DEFAULT false,
    email_friend_bingo BOOLEAN NOT NULL DEFAULT false,
    email_friend_new_card BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    friendship_id UUID REFERENCES friendships(id) ON DELETE SET NULL,
    card_id UUID REFERENCES bingo_cards(id) ON DELETE SET NULL,
    bingo_count INT,
    in_app_delivered BOOLEAN NOT NULL DEFAULT true,
    email_delivered BOOLEAN NOT NULL DEFAULT false,
    email_sent_at TIMESTAMPTZ,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (type IN ('friend_request_received', 'friend_request_accepted', 'friend_bingo', 'friend_new_card'))
);

CREATE INDEX idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_user_unread ON notifications(user_id) WHERE read_at IS NULL;
CREATE INDEX idx_notifications_type ON notifications(type);

CREATE UNIQUE INDEX idx_notifications_friend_request_received ON notifications(user_id, friendship_id)
    WHERE type = 'friend_request_received';
CREATE UNIQUE INDEX idx_notifications_friend_request_accepted ON notifications(user_id, friendship_id)
    WHERE type = 'friend_request_accepted';
CREATE UNIQUE INDEX idx_notifications_friend_bingo ON notifications(user_id, card_id)
    WHERE type = 'friend_bingo';
CREATE UNIQUE INDEX idx_notifications_friend_new_card ON notifications(user_id, card_id)
    WHERE type = 'friend_new_card';

INSERT INTO notification_settings (user_id)
SELECT id FROM users
ON CONFLICT DO NOTHING;
