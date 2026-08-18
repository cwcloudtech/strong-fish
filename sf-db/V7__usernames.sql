-- A username the member picks for themselves, and the option to be known by it
-- alone.
--
-- Unique, case-insensitively: "Marie" and "marie" are the same name to a
-- reader, so letting both exist would make the handle they resolve to a
-- coin toss. Partial, because it is optional - accounts without one must not
-- collide with each other on NULL.
CREATE UNIQUE INDEX idx_users_username ON users(lower(data->>'username'))
    WHERE data->>'username' IS NOT NULL;

-- Anonymity is read on nearly every query that names somebody (a post's
-- author, a message's sender, a club's members), so it is worth an index of
-- its own rather than a sequential check per row.
CREATE INDEX idx_users_anonymous ON users((data->>'anonymous'))
    WHERE data->>'anonymous' = 'true';
