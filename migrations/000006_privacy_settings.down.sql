-- Remove searchable column
ALTER TABLE users DROP COLUMN searchable;

-- Remove unique constraint on display_name
DROP INDEX users_display_name_unique;
