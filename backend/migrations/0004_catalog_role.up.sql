-- Minimal role model closing .vibeflow/specs/catalog-write-authorization.md:
-- any authenticated user could write shared catalog data (artists/albums/
-- tracks) before this. An enum-shaped column even for two values today so
-- adding a third role later doesn't require another migration + rename.
--
-- No admin UI ships with this (see the spec's anti-scope) — grant the role
-- manually: UPDATE users SET role = 'catalog_curator' WHERE id = '<uuid>';
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user'
    CHECK (role IN ('user', 'catalog_curator'));
