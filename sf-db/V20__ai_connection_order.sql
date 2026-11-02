-- The order a member's AI connections sit in.
--
-- Same reason a storage has one: the first is the default - it is the
-- subscription a question goes through when nobody picks, and the first row
-- of every "ask with" menu - so which one leads is a real setting rather than
-- a display preference. Until now it was whichever was created first, which
-- nobody could change without deleting a key and pasting it back.
--
-- Backfilled by creation date, so the order a member already saw is the order
-- they keep.
ALTER TABLE ai_connections ADD COLUMN position INT NOT NULL DEFAULT 0;

UPDATE ai_connections SET position = ranked.position
FROM (
    SELECT id, row_number() OVER (PARTITION BY owner_id ORDER BY created_at) - 1 AS position
    FROM ai_connections
) AS ranked
WHERE ranked.id = ai_connections.id;

-- A member's own list, in order, is the query every picker opens with.
CREATE INDEX idx_ai_connections_owner_position ON ai_connections(owner_id, position);
