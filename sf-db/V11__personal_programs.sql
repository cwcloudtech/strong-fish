-- A program can belong to a person rather than to a club.
--
-- Until now every program was a club's: club_id was NOT NULL, and only a
-- club's coaches could write one. An athlete writing their own block had
-- nowhere to put it. A NULL club is that program - authored by one member,
-- readable according to its own visibility rather than by club membership.
ALTER TABLE programs ALTER COLUMN club_id DROP NOT NULL;

-- Reading a member's own programs is now a query in its own right - the
-- personal library screen - and club_id is no help for it.
CREATE INDEX idx_programs_author_id ON programs(author_id);

-- Programs written before this have no stored visibility and read as
-- club-only; stating it explicitly keeps the rule "no visibility means
-- club-only" out of the queries that decide who may read one.
UPDATE programs
SET data = data || '{"visibility":"club"}'::jsonb
WHERE data->>'visibility' IS NULL;
