ALTER TABLE bingo_cards
  DROP CONSTRAINT IF EXISTS bingo_cards_free_pos_in_range,
  DROP CONSTRAINT IF EXISTS bingo_cards_free_pos_required_when_enabled,
  DROP CONSTRAINT IF EXISTS bingo_cards_header_len,
  DROP CONSTRAINT IF EXISTS bingo_cards_valid_grid_size;

ALTER TABLE bingo_cards
  DROP COLUMN IF EXISTS free_space_position,
  DROP COLUMN IF EXISTS has_free_space,
  DROP COLUMN IF EXISTS header_text,
  DROP COLUMN IF EXISTS grid_size;

