-- Add flexible (predefined) grid configuration to bingo_cards
ALTER TABLE bingo_cards
  ADD COLUMN grid_size SMALLINT NOT NULL DEFAULT 5,
  ADD COLUMN header_text VARCHAR(5) NOT NULL DEFAULT 'BINGO',
  ADD COLUMN has_free_space BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN free_space_position INTEGER;

-- Backfill existing cards to current behavior (5x5 with center FREE)
UPDATE bingo_cards
SET free_space_position = 12
WHERE has_free_space = true AND free_space_position IS NULL;

ALTER TABLE bingo_cards
  ADD CONSTRAINT bingo_cards_valid_grid_size
    CHECK (grid_size IN (2,3,4,5)),
  ADD CONSTRAINT bingo_cards_header_len
    CHECK (char_length(header_text) >= 1 AND char_length(header_text) <= grid_size),
  ADD CONSTRAINT bingo_cards_free_pos_required_when_enabled
    CHECK (
      (has_free_space = false AND free_space_position IS NULL) OR
      (has_free_space = true AND free_space_position IS NOT NULL)
    ),
  ADD CONSTRAINT bingo_cards_free_pos_in_range
    CHECK (
      free_space_position IS NULL OR
      (free_space_position >= 0 AND free_space_position < (grid_size * grid_size))
    );

