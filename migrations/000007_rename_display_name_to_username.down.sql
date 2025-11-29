-- Revert: rename username column back to display_name
ALTER TABLE users RENAME COLUMN username TO display_name;

-- Drop and recreate the unique index with old name
DROP INDEX users_username_unique;
CREATE UNIQUE INDEX users_display_name_unique ON users (LOWER(display_name));
