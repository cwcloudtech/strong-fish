-- A post can be shared with several clubs at once.
--
-- Until now a post carried one club_id, or NULL for a public post. A join
-- table rather than an array column, for the cascade: deleting a club must
-- take the post out of *that* club and leave it in the others, which is what
-- ON DELETE CASCADE on this table does. The old column cascaded to the post
-- itself, so deleting a club deleted every post shared with it - acceptable
-- when a post belonged to one club, and destructive once it can belong to
-- three.
CREATE TABLE post_clubs (
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, club_id)
);

-- The club feed reads by club, so that direction needs its own index; the
-- primary key already covers lookups by post.
CREATE INDEX idx_post_clubs_club_id ON post_clubs(club_id);

INSERT INTO post_clubs (post_id, club_id)
SELECT id, club_id FROM posts WHERE club_id IS NOT NULL;

-- Every post that had a club was club-only, whatever its payload said; and
-- every post without one was public. Stated explicitly now, because from here
-- on the label is what decides who may read a post - a club-only post whose
-- last club was deleted has no clubs left, and must not become public by
-- having nothing to check.
UPDATE posts SET data = data || '{"visibility":"club"}'::jsonb WHERE club_id IS NOT NULL;
UPDATE posts SET data = data || '{"visibility":"public"}'::jsonb WHERE club_id IS NULL;

DROP INDEX IF EXISTS idx_posts_club_id;
ALTER TABLE posts DROP COLUMN club_id;
