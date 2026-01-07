CREATE TABLE bingo_card_shares (
    card_id UUID PRIMARY KEY REFERENCES bingo_cards(id) ON DELETE CASCADE,
    token VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    last_accessed_at TIMESTAMPTZ,
    access_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_bingo_card_shares_expires ON bingo_card_shares(expires_at);
