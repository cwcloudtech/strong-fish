-- An author can no longer like their own post, so the ones recorded before that
-- rule existed are removed.
--
-- Left in place they would be unremovable: the button that created them is gone
-- from the client and the endpoint now refuses, so the author could neither
-- keep them honestly nor take them back.
DELETE FROM post_likes
USING posts
WHERE post_likes.post_id = posts.id
  AND post_likes.user_id = posts.author_id;
