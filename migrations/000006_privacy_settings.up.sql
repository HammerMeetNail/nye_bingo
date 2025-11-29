-- Add case-insensitive unique constraint on display_name
CREATE UNIQUE INDEX users_display_name_unique ON users (LOWER(display_name));

-- Add searchable column for privacy control (default false = opt-in to search)
ALTER TABLE users ADD COLUMN searchable BOOLEAN NOT NULL DEFAULT false;
