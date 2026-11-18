-- Connection sessions and IP bans.
--
-- Where V6 kept a per-account list of addresses inside users.data.ips, this
-- promotes it to a table of its own: every connection - a signed-in member's or
-- an anonymous visitor's - is one row, carrying the IP and whatever a geo
-- lookup made of it. A row that belongs to nobody (user_id NULL) is an
-- anonymous visit; a row with a user is that member connecting. The geo
-- enrichment lives here in `data` rather than in a tracing table, so the IP
-- administration screen reads an address and everything known about it from one
-- place.
--
-- data: { count, geo: {source, country, countryCode, region, city, zip,
--         timezone, isp, org, asn, lat, lon} }
-- created_at is the address's first-seen, updated_at its last-seen.
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    ip TEXT NOT NULL,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per (address, member), and one per address for anonymous visits.
-- Split into two partial unique indexes because a plain UNIQUE(ip, user_id)
-- treats every NULL as distinct, which would let the same anonymous address
-- insert a new row on every request instead of bumping its counter.
CREATE UNIQUE INDEX idx_sessions_ip_user ON sessions(ip, user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX idx_sessions_ip_anon ON sessions(ip) WHERE user_id IS NULL;
-- The two reads the feature makes: every address a member has used (the
-- per-member view), and the roster of addresses newest-first (the admin screen).
CREATE INDEX idx_sessions_user ON sessions(user_id, updated_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX idx_sessions_ip ON sessions(ip);

-- Banned addresses. Separate from `sessions` because a ban is about the address
-- itself, not about any one connection from it, and an administrator may ban an
-- address that has never been seen. The guard middleware reads this on every
-- request, so it is kept to one indexed row per address.
--
-- data: { reason, bannedBy }  (bannedBy is the superadmin's user id)
CREATE TABLE ip_bans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip TEXT NOT NULL UNIQUE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ip_bans_ip ON ip_bans(ip);

-- The GIN index on users.data->'ips' is retired with the column's use: the
-- addresses now live in `sessions`. Existing data.ips payloads are left in
-- place, harmless and no longer read.
DROP INDEX IF EXISTS idx_users_ips;
