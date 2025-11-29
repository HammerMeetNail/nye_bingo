-- Rename display_name column to username
ALTER TABLE users RENAME COLUMN display_name TO username;

-- Drop and recreate the unique index with new name
DROP INDEX users_display_name_unique;
CREATE UNIQUE INDEX users_username_unique ON users (LOWER(username));
