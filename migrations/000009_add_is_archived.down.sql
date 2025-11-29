-- Remove is_archived column from bingo_cards

DROP INDEX IF EXISTS idx_bingo_cards_is_archived;
ALTER TABLE bingo_cards DROP COLUMN IF EXISTS is_archived;
