-- Add category column (optional, for categorization)
ALTER TABLE bingo_cards ADD COLUMN category VARCHAR(50);

-- Add title column (optional, custom name for the card)
ALTER TABLE bingo_cards ADD COLUMN title VARCHAR(100);

-- Drop old unique constraint (one card per user per year)
ALTER TABLE bingo_cards DROP CONSTRAINT bingo_cards_user_id_year_key;

-- Add new unique constraint: prevent duplicate titles within same user/year
-- Cards without titles (NULL) are always allowed (no uniqueness check on NULL)
CREATE UNIQUE INDEX idx_bingo_cards_user_year_title
    ON bingo_cards(user_id, year, title)
    WHERE title IS NOT NULL;

-- Also prevent multiple NULL-titled cards per user per year for clean UX
-- Users should name their cards if they want multiples
CREATE UNIQUE INDEX idx_bingo_cards_user_year_null_title
    ON bingo_cards(user_id, year)
    WHERE title IS NULL;
