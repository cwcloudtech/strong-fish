-- API keys: the same data model as ~/cwclock's, so a token minted here can be
-- consumed by the same kind of client. Only the sha256 of the plaintext is
-- stored - the token itself is shown once, at creation, and is unrecoverable
-- afterwards.
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_hash_expires ON api_keys(key_hash, expires_at);

-- Program visibility lives in the JSONB payload like every other program
-- field. A row with no "visibility" key is private, which is what every
-- program written before this migration is: sharing has to be opted into, it
-- is never inherited.
CREATE INDEX idx_programs_visibility ON programs((data->>'visibility'));
