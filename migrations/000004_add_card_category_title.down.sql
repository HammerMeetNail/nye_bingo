-- Drop the new unique indexes
DROP INDEX IF EXISTS idx_bingo_cards_user_year_title;
DROP INDEX IF EXISTS idx_bingo_cards_user_year_null_title;

-- Restore the original unique constraint
-- Note: This will fail if there are multiple cards per user/year
ALTER TABLE bingo_cards ADD CONSTRAINT bingo_cards_user_id_year_key UNIQUE(user_id, year);

-- Drop the new columns
ALTER TABLE bingo_cards DROP COLUMN IF EXISTS title;
ALTER TABLE bingo_cards DROP COLUMN IF EXISTS category;
