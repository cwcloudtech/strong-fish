-- Recording callbacks: a member's own triggers that the phone fires when they
-- start a bar-speed set, so a device of their own - an ESP32 on the gym network
-- - records the same set the phone is about to. The payload carries the lift
-- and the load, the way a brand's app signals its sensor.
--
-- Named after comwork-cloud's "callback": a webhook (an HTTP POST) is one
-- transport of it, alongside a WebSocket message and an MQTT publish. Whichever
-- the transport, the body is the same; only how it is carried differs, so the
-- transport, the endpoint, the credentials each kind needs and whether it is
-- switched on all live in the JSONB data - a new field is a code change rather
-- than a migration.
--
-- Its own store, shaped on ai_connections without the sharing: a callback
-- points at the member's own hardware and is nobody else's to spend, so there
-- is no ACL - just a per-owner list.
CREATE TABLE recording_callbacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The list is read per owner, oldest first - one small query when the settings
-- screen or the record screen asks for a member's devices.
CREATE INDEX idx_recording_callbacks_owner ON recording_callbacks(owner_id, created_at);
