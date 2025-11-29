-- Add visibility column to bingo_cards
-- Default to true (visible to friends) to maintain backward compatibility
ALTER TABLE bingo_cards ADD COLUMN visible_to_friends BOOLEAN NOT NULL DEFAULT true;

-- Index for efficient friend card queries
CREATE INDEX idx_bingo_cards_visibility ON bingo_cards(user_id, is_finalized, visible_to_friends);
