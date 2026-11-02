-- The AI assistant: the connections a member holds to a language model, the
-- conversations they have through them, and what the model said about their
-- sessions.
--
-- One migration for the whole feature rather than one per table: these tables
-- are born together and are meaningless apart - a conversation without a
-- connection has nothing to answer it.

-- A member's connection to a language model, and who else may use it.
--
-- Deliberately the same shape as storages: an account holds the credential,
-- the owner decides who else may spend it, and the ACL is a row per person.
-- A coach paying for a model key and letting their athletes ask it questions
-- is the same arrangement as a club sharing one bucket, and giving it a second
-- shape would mean two permission systems to keep in step.
--
-- data holds the connection itself: provider, base URL, model, and the token.
-- The token is a credential sitting in a column, like a storage's secret key
-- beside it - the API never sends it back to a client, only what it is for.
CREATE TABLE ai_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_connections_owner_id ON ai_connections(owner_id);

-- Who may do what with somebody else's connection.
--
--   owner - it is theirs: edit it, share it, delete it
--   user  - may ask the model through it, and nothing else
--
-- Two roles rather than storage's three: there is no reading a connection
-- without spending it, so "may look at it" is not a thing to grant.
CREATE TABLE ai_connection_acl (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES ai_connections(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One role per person per connection: granting access twice is a change
    -- of role, not a second grant.
    UNIQUE (connection_id, user_id)
);

CREATE INDEX idx_ai_connection_acl_user_id ON ai_connection_acl(user_id);

-- What the model said about one session of somebody's block.
--
-- Kept rather than regenerated on every visit, for three reasons: the answer
-- costs money to produce, it is not deterministic - asking twice gives two
-- different readings of the same session, which is unsettling when nothing
-- changed - and a member should be able to come back to what it told them in
-- March. Deleting it is how you ask for a new one.
--
-- One per session per assignment: the same program assigned to two athletes
-- gets a reading each, which is the point.
CREATE TABLE session_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assignment_id UUID NOT NULL REFERENCES program_assignments(id) ON DELETE CASCADE,
    day_id UUID NOT NULL REFERENCES program_days(id) ON DELETE CASCADE,
    -- Who asked. The feedback is about the assignment's athlete, but a coach
    -- may be the one who generated it, and "who spent the tokens" is a
    -- question an owner of a shared connection will eventually ask.
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    -- data: {provider, model} - what produced it, so a reading from a small
    -- local model is not mistaken later for one from a large hosted one.
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (assignment_id, day_id)
);

CREATE INDEX idx_session_feedback_assignment ON session_feedback(assignment_id);


-- The questions a member put to the model, and what it answered.
--
-- Kept because a conversation is worth coming back to: the answer to "why is
-- my squat stalling" is still the answer three weeks later, and a chat that
-- forgets itself on reload is one nobody uses twice.
--
-- Shaped on cwcloud's prompt_list/prompt pair, including the part that looks
-- unusual: a row is one *exchange*, question and answer together, rather than
-- one turn each. It is the honest unit - an answer without its question is
-- meaningless, they are written at the same moment, and the model that
-- produced the answer belongs to the pair rather than to either half.
CREATE TABLE ai_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- data: {title, shareToken}. The title is taken from the first question
    -- and renameable; the share token is absent until somebody shares it.
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A member's own list, newest first, is the query every chat screen opens with.
CREATE INDEX idx_ai_conversations_user ON ai_conversations(user_id, updated_at DESC);

-- A shared conversation is addressed by its token alone, so that lookup has to
-- be an index rather than a scan of everybody's.
--
-- Partial, because only a shared conversation has one: an index over a column
-- that is null for almost every row is an index mostly of nothing.
CREATE UNIQUE INDEX idx_ai_conversations_share_token
    ON ai_conversations ((data->>'shareToken'))
    WHERE data->>'shareToken' IS NOT NULL;

-- One exchange.
--
-- data: {provider, model, usage} - what produced the answer, so a reply from a
-- small local model is not later mistaken for one from a large hosted one, and
-- so somebody paying for a shared key can see what their tokens went on.
--
-- The system prompt is deliberately not stored: it is not part of the
-- conversation, it lives in a file, and it is meant to change underneath old
-- conversations when the coaching skill is improved.
CREATE TABLE ai_prompts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Reading a conversation is reading its exchanges in order.
CREATE INDEX idx_ai_prompts_conversation ON ai_prompts(conversation_id, created_at);

-- What the AI said about a member's health data.
--
-- One row per member per source, replaced when a new reading is asked for:
-- unlike a session, which is a fixed thing that happened, health data is a
-- moving ninety-day window, and last month's reading of it is not a record
-- worth keeping beside this month's - it is simply out of date.
--
-- Regeneration is rate-limited to a day by the handler, from updated_at. The
-- underlying numbers move once a day at most, and an answer that costs tokens
-- should not be re-bought for a window that has not changed.
CREATE TABLE health_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Which device account the reading was made from - google_health,
    -- withings. Not the AI vendor, which is in data alongside the model.
    source TEXT NOT NULL,
    content TEXT NOT NULL,
    -- data: {provider, model, days} - what produced it and over how long a
    -- window, so a reading of a fortnight is not mistaken for one of a
    -- quarter.
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, source)
);
