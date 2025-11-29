DROP INDEX IF EXISTS idx_bingo_cards_visibility;
ALTER TABLE bingo_cards DROP COLUMN visible_to_friends;
