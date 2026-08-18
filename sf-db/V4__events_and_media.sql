-- Events: competitions and club dates, subscribable from Outlook or Google
-- Calendar through the ICS feed (see sf-api's calendar handler).
--
-- club_id is nullable on purpose: an event without a club is the federation
-- calendar - a meet anybody can see - while one with a club belongs to that
-- club's own season. Visibility inside data decides which, so a club event can
-- still be published to everyone.
--
-- data: {title, description, location, url, kind, startsAt, endsAt, allDay, visibility}
CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id UUID REFERENCES clubs(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_events_club_id ON events(club_id);
-- Every listing is ordered by start date, and the calendar feed reads a window
-- of it, so the sort key is worth its own index.
CREATE INDEX idx_events_starts_at ON events((data->>'startsAt'));

-- The calendar feed is polled by Outlook and Google Calendar, which cannot
-- send an Authorization header - the token in the URL is the whole credential,
-- so it has to be looked up as fast as a session would be.
CREATE INDEX idx_users_calendar_feed_token ON users((data->>'calendarFeedToken'));
