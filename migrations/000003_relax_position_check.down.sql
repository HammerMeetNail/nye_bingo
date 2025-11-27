-- Restore the position check constraint
ALTER TABLE bingo_items ADD CONSTRAINT bingo_items_position_check CHECK (position >= 0 AND position <= 24);
