-- Bar-speed sets StrongFish recorded itself, from any accelerometer: a phone
-- over Bluetooth, or a cheap board over HTTP. Its own store so the bar-speed
-- panel can read our sets beside the ones it reads from a connected tracker
-- (Stance), and show the most recent of each lift whichever produced it.
--
-- The computation is ours (sf-api/internal/velocity); this keeps the result.
-- source names what captured the signal, so a merged view can tell a
-- StrongFish-recorded set from one read off another provider.
CREATE TABLE bar_speed_recordings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- "squat", "bench" or "deadlift".
    lift TEXT NOT NULL,
    weight_kg DOUBLE PRECISION NOT NULL DEFAULT 0,
    -- What the member rated it, and what the bar's speed said; 0 when absent.
    given_rpe DOUBLE PRECISION NOT NULL DEFAULT 0,
    predicted_rpe DOUBLE PRECISION NOT NULL DEFAULT 0,
    mean_velocity DOUBLE PRECISION NOT NULL DEFAULT 0,
    peak_velocity DOUBLE PRECISION NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'strongfish',
    performed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The last set of each lift is the panel's query, one per page load: the newest
-- row per (member, lift).
CREATE INDEX idx_bar_speed_recordings_user_lift
    ON bar_speed_recordings(user_id, lift, performed_at DESC);

-- One row per rep, in order - the per-rep velocity chart the panel draws.
CREATE TABLE bar_speed_reps (
    recording_id UUID NOT NULL REFERENCES bar_speed_recordings(id) ON DELETE CASCADE,
    rep_order INT NOT NULL,
    mean_mps DOUBLE PRECISION NOT NULL DEFAULT 0,
    peak_mps DOUBLE PRECISION NOT NULL DEFAULT 0,
    PRIMARY KEY (recording_id, rep_order)
);
