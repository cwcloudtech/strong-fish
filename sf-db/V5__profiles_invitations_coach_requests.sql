-- Profile visibility replaces the publicProfile boolean with three levels.
--
-- The mapping is exact rather than a guess: publicProfile true meant "anyone
-- can read this", which is 'public'; false meant "only its owner and a
-- superadmin", which is 'private'. The new middle level ('clubs') is opt-in and
-- nobody is moved into it - widening an audience is not a migration's decision
-- to make.
UPDATE users SET data = data || jsonb_build_object(
    'profileVisibility',
    CASE WHEN coalesce((data->>'publicProfile')::boolean, false) THEN 'public' ELSE 'private' END
);

-- The search filters on it, and the profile endpoint reads it on every hit.
CREATE INDEX idx_users_profile_visibility ON users((data->>'profileVisibility'));

-- Search matches on email, name and surname. Postgres cannot use a plain
-- b-tree for a leading-wildcard LIKE, so these are trigram indexes; the
-- extension is created if the deployment doesn't already have it.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_users_email_trgm ON users USING gin (lower(email) gin_trgm_ops);
CREATE INDEX idx_users_name_trgm ON users USING gin (
    lower(coalesce(data->>'name', '') || ' ' || coalesce(data->>'surname', '')) gin_trgm_ops
);

-- Club invitations.
--
-- An invitation is keyed by *email*, not by a user id, because a coach invites
-- people who do not have an account yet - that is most of the point. Matching
-- at accept time means an account created a week later still finds the
-- invitation waiting, and it also means the invitation carries no id anybody
-- could guess their way into.
--
-- data: {role, status, message, clubName, inviterName}
CREATE TABLE club_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
    inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_club_invitations_club_id ON club_invitations(club_id);
CREATE INDEX idx_club_invitations_email ON club_invitations(lower(email));

-- One live invitation per club and address: inviting somebody twice should
-- update the invitation, not stack up a second one they have to decline as
-- well. Declined and accepted rows are excluded so a fresh invitation can
-- follow an old one.
CREATE UNIQUE INDEX idx_club_invitations_pending
    ON club_invitations(club_id, lower(email))
    WHERE data->>'status' = 'pending';
