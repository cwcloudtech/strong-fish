-- A member's link to an outside account that counts something for them: a
-- Google Health profile today, whatever else gets added next.
--
-- Its own table rather than a few keys in users.data, for two reasons. The
-- tokens are credentials: everything that reads a user row would otherwise be
-- one forgotten projection away from serving somebody's refresh token to the
-- browser. And users.data is merged on every profile write, so a token would
-- sit in the blob every client has to echo back in full - exactly the shape of
-- write that silently drops fields.
--
-- provider is what makes this table outlive its first integration. There is
-- one today, and the one before it - the Fitbit Web API - is switched off on
-- 30 October 2026, which is the whole argument for not naming the table after
-- whichever service is current.
--
-- data holds what is not indexed and what differs between providers: the
-- granted scope, and the account id the provider knows the member by.
CREATE TABLE device_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    access_token TEXT NOT NULL,
    -- Absent when the provider issues none: the link then lasts exactly as
    -- long as its access token, and the member reconnects.
    refresh_token TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One link per provider per member: connecting again replaces the tokens
-- rather than leaving a second row nobody will ever refresh. Linking a second
-- provider is a second row, which is the point.
CREATE UNIQUE INDEX idx_device_links_user_provider ON device_links(user_id, provider);

-- Reading a member's connections is the common query, and it is one per page
-- load of the profile.
CREATE INDEX idx_device_links_user ON device_links(user_id);
