-- A storage connection becomes a thing in its own right, that can be shared.
--
-- Until now a bucket lived inside its owner's user payload: one connection per
-- account, reachable by nobody else. That is fine while a bucket is a private
-- setting and wrong as soon as it is something a coach lends to their athletes
-- - a club paying for one bucket, or a coach uploading demonstration videos to
-- the athlete's own storage rather than filling their own.
--
-- So it moves to its own table with an access list beside it. The connection
-- itself stays a JSONB payload, like everything else in this app: it is a
-- credential blob whose shape depends on the provider, and columns per field
-- would mean a migration per provider.
CREATE TABLE storages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The account the storage belongs to. Kept as a column beside the ACL's
    -- own "owner" row: it is what a cascade needs, and what makes "my
    -- storages" one index lookup rather than a join.
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_storages_owner_id ON storages(owner_id);

-- Who may do what with somebody else's storage.
--
--   owner  - it is theirs: edit the connection, share it, delete it
--   writer - may upload to it
--   reader - may play what is in it, even when the uploader's profile would
--            not otherwise be visible to them
--
-- The owner gets a row here too, rather than being implied by storages.owner_id
-- alone: every question this table answers ("what may this user do with this
-- storage") is then one lookup with one shape, instead of a special case that
-- has to be remembered at every call site.
CREATE TABLE storage_acl (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    storage_id UUID NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One role per person per storage: granting somebody write access twice is
    -- a change of role, not a second grant.
    UNIQUE (storage_id, user_id)
);

CREATE INDEX idx_storage_acl_user_id ON storage_acl(user_id);

-- Move every configured connection out of its owner's payload.
--
-- Configured, not merely present: an account carrying a half-filled form
-- (a type and nothing else) has no storage, and creating a row for it would
-- put an unusable destination in somebody's upload picker.
INSERT INTO storages (owner_id, data, created_at, updated_at)
SELECT
    id,
    data->'storage',
    coalesce(created_at, now()),
    now()
FROM users
WHERE data->'storage' IS NOT NULL
  AND (
    (data->'storage'->>'type' = 's3'
     AND coalesce(data->'storage'->>'endpoint', '') <> ''
     AND coalesce(data->'storage'->>'bucketName', '') <> ''
     AND coalesce(data->'storage'->>'accessKey', '') <> ''
     AND coalesce(data->'storage'->>'secretKey', '') <> '')
    OR
    (data->'storage'->>'type' = 'google_drive'
     AND coalesce(data->'storage'->>'serviceAccountBase64', '') <> ''
     AND coalesce(data->'storage'->>'folderId', '') <> '')
  );

INSERT INTO storage_acl (storage_id, user_id, role)
SELECT id, owner_id, 'owner' FROM storages;

-- And out of the payload for good. Left in place it would be a second copy of
-- live credentials that nothing reads and nobody would remember to rotate.
UPDATE users SET data = data - 'storage' WHERE data ? 'storage';
