-- The member search matches on the username as well as the name and email, and
-- a leading-wildcard LIKE cannot use a plain b-tree - so it gets the same kind
-- of trigram index the other two searchable columns already have.
CREATE INDEX idx_users_username_trgm ON users USING gin (
    lower(coalesce(data->>'username', '')) gin_trgm_ops
);
