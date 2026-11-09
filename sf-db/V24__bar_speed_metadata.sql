-- Descriptive, extensible metadata for a bar-speed recording, as JSONB so a new
-- tag needs a key, not a column and a migration. The fixed, queried figures
-- (lift, load, velocities, RPE) stay as columns; this holds what rides along for
-- the panel and the AI reading - today the set type (warmup/topset/backoff) and
-- whether it was a tempo variation, tomorrow whatever else we learn to capture.
ALTER TABLE bar_speed_recordings
    ADD COLUMN data JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Migrate existing recordings to the current defaults, so every row carries the
-- same shape: a capture that predates the tags reads as a warm-up set with no
-- tempo variation rather than as missing data.
UPDATE bar_speed_recordings
SET data = jsonb_build_object('setType', 'warmup', 'tempoVariation', false)
WHERE NOT (data ? 'setType');
