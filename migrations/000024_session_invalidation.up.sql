-- Track a per-user cutoff for session invalidation. Sessions created before this
-- timestamp are rejected during validation, allowing password change/reset to
-- revoke all existing sessions (including Redis-only sessions with no PG row).
ALTER TABLE users ADD COLUMN sessions_invalidated_at TIMESTAMPTZ;
