-- Relax the position check constraint to allow temporary negative values during shuffle
-- The unique constraint on (card_id, position) still prevents duplicates
ALTER TABLE bingo_items DROP CONSTRAINT bingo_items_position_check;
