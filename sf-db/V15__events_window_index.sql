-- The calendar asks for events that have not finished yet, and had no index
-- for it.
--
-- The filter is on when an event *ends* - falling back to its start for the
-- ones with no end - and the only index was on the start alone, so every
-- calendar load was a sequential scan of the whole table. A club with five
-- years behind it reads fifty thousand rows to show the four thousand ahead of
-- it, and that grows for as long as the club exists.
--
-- The expression has to match the query's exactly (see EventStore.ListVisible)
-- or the planner cannot use it.
CREATE INDEX IF NOT EXISTS idx_events_window
    ON events ((coalesce(nullif(data->>'endsAt', ''), data->>'startsAt')));
