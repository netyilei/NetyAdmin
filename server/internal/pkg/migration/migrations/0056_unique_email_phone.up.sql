-- Unique partial index for admin email.
--
-- users.email/phone already carry unique partial indexes (0018:
-- users_email_key / users_phone_key, excluding soft-deleted and empty
-- values), so only admin_user.email lacks the DB-level guard — the
-- service-level ExistsBy checks are advisory and racy under concurrent
-- creates. Service layers convert the violation (SQLSTATE 23505) into
-- friendly business errors (pkg/database.IsUniqueViolation).
--
-- NOTE: if this migration fails with "could not create unique index
-- ... key is duplicated", resolve existing duplicates first, then re-run:
--   SELECT email, COUNT(*) FROM admin_user WHERE deleted_at = 0 AND email <> '' GROUP BY email HAVING COUNT(*) > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_admin_user_email ON admin_user(email) WHERE deleted_at = 0 AND email <> '';
