-- Administrative audit trail.
--
-- One row per auditable event: an AI-assistant call (what it cost, on whose
-- key), a sign-in or registration, a superadmin role change, an IP ban or
-- unban, a deletion. The superadmin console reads it newest-first; nothing else
-- does. It is written fire-and-forget off the request that caused it, so an
-- audit write can never slow or fail the action it records.
--
-- `data` carries a snapshot, not a live reference: the actor's email is copied
-- in at write time (data->>'actorEmail') so a deleted account's events still
-- say who did them, and data->'detail' is the event's own payload (an AI call's
-- provider/model/tokens, a ban's reason, a role change's before/after).
CREATE TABLE audit_log (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  subject_type TEXT NOT NULL DEFAULT '',
  subject_id TEXT NOT NULL DEFAULT '',
  data JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The one read the console makes: the whole trail, newest-first, paged.
CREATE INDEX idx_audit_log_created ON audit_log(created_at DESC);
-- And the filter by what an event was about (a given connection, address, user).
CREATE INDEX idx_audit_log_subject ON audit_log(subject_type, subject_id);
