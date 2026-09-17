-- Unique partial indexes for login identifiers (email/phone).
--
-- Rationale: email/phone are login identifiers (username already has a unique
-- index). The service-level ExistsBy checks are advisory and racy: two users
-- concurrently binding the same email both pass the check and both succeed,
-- leaving the identifier ambiguous for password-reset/login resolution.
-- These partial indexes are the authoritative guard; service layers convert
-- violation (SQLSTATE 23505) into friendly business errors.
--
-- Soft-deleted rows (deleted_at <> 0) and empty strings are excluded so the
-- index never conflicts with "not set" values or deleted accounts.
--
-- NOTE: if this migration fails with "could not create unique index ... key is
-- duplicated", resolve existing duplicates first, then re-run:
--   SELECT email, COUNT(*) FROM users      WHERE deleted_at = 0 AND email <> '' GROUP BY email HAVING COUNT(*) > 1;
--   SELECT phone, COUNT(*) FROM users      WHERE deleted_at = 0 AND phone <> '' GROUP BY phone HAVING COUNT(*) > 1;
--   SELECT email, COUNT(*) FROM admin_user WHERE deleted_at = 0 AND email <> '' GROUP BY email HAVING COUNT(*) > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email ON users(email) WHERE deleted_at = 0 AND email <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_phone ON users(phone) WHERE deleted_at = 0 AND phone <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uq_admin_user_email ON admin_user(email) WHERE deleted_at = 0 AND email <> '';
