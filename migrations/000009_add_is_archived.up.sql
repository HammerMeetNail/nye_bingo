-- Add is_archived column to bingo_cards
-- This allows users to manually archive/unarchive cards

ALTER TABLE bingo_cards ADD COLUMN is_archived BOOLEAN NOT NULL DEFAULT false;

-- Index for filtering archived cards
CREATE INDEX idx_bingo_cards_is_archived ON bingo_cards(is_archived);
