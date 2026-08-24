-- Several storage targets per member, in a priority order.
--
-- V12 gave a storage its own row so it could be shared; one per owner was
-- still assumed everywhere, and "the" storage was whichever row came first by
-- creation date. That is not an order anybody chose - it is the order they
-- happened to configure things in, and it decided which bucket a video ended
-- up in.
--
-- An upload now goes to *every* target a member has, and the link in the post
-- comes from the first one. So the order is a real setting, and it needs a
-- column rather than an accident of created_at.
ALTER TABLE storages ADD COLUMN IF NOT EXISTS position integer NOT NULL DEFAULT 0;

-- Existing rows keep the order they already had in practice - by creation -
-- so nobody's first target changes under them on the day this ships.
WITH ordered AS (
  SELECT id, row_number() OVER (PARTITION BY owner_id ORDER BY created_at, id) - 1 AS pos
  FROM storages
)
UPDATE storages s
SET position = ordered.pos
FROM ordered
WHERE ordered.id = s.id;

-- Every read of a member's targets is "theirs, in order".
CREATE INDEX IF NOT EXISTS storages_owner_position_idx ON storages (owner_id, position);
