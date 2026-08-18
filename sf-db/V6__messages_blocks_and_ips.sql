-- Private messages.
--
-- A conversation is addressed by the *pair* of people in it, not by an id the
-- sender has to look up first: there is exactly one thread between any two
-- members, so it is derived rather than created. member_a/member_b hold that
-- pair with the lower id always first, which is what makes the pair a unique
-- key instead of two rows for the same two people.
--
-- data: {lastMessage, lastSenderId}
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_a UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    member_b UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT conversation_members_ordered CHECK (member_a < member_b)
);

CREATE UNIQUE INDEX idx_conversations_pair ON conversations(member_a, member_b);
CREATE INDEX idx_conversations_member_a ON conversations(member_a, updated_at DESC);
CREATE INDEX idx_conversations_member_b ON conversations(member_b, updated_at DESC);

-- data: {content, readAt}
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_conversation ON messages(conversation_id, created_at DESC);
-- The unread badge counts messages somebody else sent and this member hasn't
-- opened, so it filters on the sender and the read marker together.
CREATE INDEX idx_messages_unread ON messages(conversation_id, sender_id)
    WHERE data->>'readAt' IS NULL;

-- Blocks.
--
-- Directional on purpose: A blocking B is A's decision about their own feed and
-- inbox, and it is not B's. Both directions are checked before a message is
-- delivered or a post is shown, so the *effect* is mutual - what isn't mutual
-- is who can undo it.
CREATE TABLE blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT block_is_not_self CHECK (blocker_id <> blocked_id)
);

CREATE UNIQUE INDEX idx_blocks_pair ON blocks(blocker_id, blocked_id);
CREATE INDEX idx_blocks_blocked ON blocks(blocked_id);

-- Connection IPs live in the user's own payload rather than in a table of their
-- own: they are a bounded, per-account list read only on that account's admin
-- screen, and a row per connection would be a write on every single login for
-- data nobody queries across users.
--
-- data.ips: [{ip, count, firstSeen, lastSeen}]
CREATE INDEX idx_users_ips ON users USING gin ((data->'ips'));
