package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
)

type MessageStore struct {
	pool *pgxpool.Pool
}

func NewMessageStore(pool *pgxpool.Pool) *MessageStore {
	return &MessageStore{pool: pool}
}

// pair orders two user ids the way the conversations table stores them.
//
// A thread between two people has to be one row whichever of them writes
// first, so the pair is normalized rather than trusted: the table's CHECK and
// its unique index both rely on this ordering, which is what makes "one
// conversation per pair" a database guarantee instead of a convention.
func pair(a, b string) (string, string) {
	if a < b {
		return a, b
	}
	return b, a
}

// FindOrCreateConversation returns the thread between two members, opening it
// on first use. ON CONFLICT rather than a select-then-insert: two people
// sending their first message at the same moment would otherwise race into two
// rows for the same pair.
func (s *MessageStore) FindOrCreateConversation(ctx context.Context, callerID, otherID string) (string, error) {
	memberA, memberB := pair(callerID, otherID)

	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO conversations (member_a, member_b) VALUES ($1, $2)
		ON CONFLICT (member_a, member_b) DO UPDATE SET updated_at = conversations.updated_at
		RETURNING id
	`, memberA, memberB).Scan(&id)
	return id, err
}

// Every reference to the caller is written $1::uuid rather than left to
// inference. pgx sends a Go string as untyped text, and Postgres picks one type
// for a parameter across the whole statement: mixing `member_a = $1` with
// `member_a::text = $1` made it choose text, and the uuid comparison then failed
// outright with "operator does not exist: uuid = text". Casting the parameter
// instead of the columns also keeps idx_conversations_pair and idx_blocks_pair
// usable - a cast on the column side is what stops an index being used.
//
// ListConversations returns one member's threads, most recently active first,
// each projected for them: "the other person" is whichever of the pair is not
// the caller, and the unread count is messages the caller did not send and has
// not opened.
//
// Blocked members are excluded in the query rather than filtered afterwards, so
// a blocked thread cannot appear on a page and then be dropped from it.
func (s *MessageStore) ListConversations(ctx context.Context, callerID string) ([]models.Conversation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.created_at, c.updated_at,
		       coalesce(c.data->>'lastMessage', ''), coalesce(c.data->>'lastSenderId', ''),
		       other.id, coalesce(other.data->>'handle', ''),
		       coalesce(other.data->>'name', ''), coalesce(other.data->>'surname', ''),
		       coalesce(other.data->>'picture', ''),
		       (SELECT count(*) FROM messages m
		        WHERE m.conversation_id = c.id AND m.sender_id <> $1::uuid AND m.data->>'readAt' IS NULL)
		FROM conversations c
		JOIN users other ON other.id = CASE WHEN c.member_a = $1::uuid THEN c.member_b ELSE c.member_a END
		WHERE $1::uuid IN (c.member_a, c.member_b)
		  AND NOT EXISTS (
		      SELECT 1 FROM blocks b
		      WHERE (b.blocker_id = $1::uuid AND b.blocked_id = other.id)
		         OR (b.blocked_id = $1::uuid AND b.blocker_id = other.id)
		  )
		  AND EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = c.id)
		ORDER BY c.updated_at DESC
	`, callerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := []models.Conversation{}
	for rows.Next() {
		var c models.Conversation
		if err := rows.Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt, &c.LastMessage, &c.LastSenderID,
			&c.Other.ID, &c.Other.Handle, &c.Other.Name, &c.Other.Surname, &c.Other.Picture,
			&c.Unread); err != nil {
			return nil, err
		}
		conversations = append(conversations, c)
	}
	return conversations, rows.Err()
}

// CountUnread backs the badge on the messages nav entry.
func (s *MessageStore) CountUnread(ctx context.Context, callerID string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE $1::uuid IN (c.member_a, c.member_b)
		  AND m.sender_id <> $1::uuid
		  AND m.data->>'readAt' IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM blocks b
		      WHERE (b.blocker_id = $1::uuid AND b.blocked_id = m.sender_id)
		         OR (b.blocked_id = $1::uuid AND b.blocker_id = m.sender_id)
		  )
	`, callerID).Scan(&count)
	return count, err
}

// ConversationMembers returns the two people in a thread, so a caller can be
// checked against it before anything is read or written.
func (s *MessageStore) ConversationMembers(ctx context.Context, id string) (string, string, error) {
	var memberA, memberB string
	err := s.pool.QueryRow(ctx,
		`SELECT member_a, member_b FROM conversations WHERE id = $1`, id).Scan(&memberA, &memberB)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return memberA, memberB, err
}

// ListMessages returns a thread's messages, oldest first - the order they are
// read in.
func (s *MessageStore) ListMessages(ctx context.Context, conversationID string, limit int) ([]models.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id, m.data, m.created_at,
		       coalesce(u.data->>'handle', ''), coalesce(u.data->>'name', ''),
		       coalesce(u.data->>'surname', ''), coalesce(u.data->>'picture', '')
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.conversation_id = $1
		ORDER BY m.created_at DESC
		LIMIT $2
	`, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []models.Message{}
	for rows.Next() {
		var m models.Message
		var raw []byte
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &raw, &m.CreatedAt,
			&m.Sender.Handle, &m.Sender.Name, &m.Sender.Surname, &m.Sender.Picture); err != nil {
			return nil, err
		}
		var d messageData
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, err
		}
		m.Content = d.Content
		m.ReadAt = d.ReadAt
		m.Sender.ID = m.SenderID
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Queried newest-first so LIMIT keeps the *latest* page, then reversed:
	// a thread is read oldest to newest, but the interesting end is the new one.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

type messageData struct {
	Content string     `json:"content"`
	ReadAt  *time.Time `json:"readAt,omitempty"`
}

// Send writes a message and updates its conversation's preview in one
// transaction: a message whose thread still shows the previous line would sort
// to the wrong place in everybody's list.
func (s *MessageStore) Send(ctx context.Context, conversationID, senderID, content string) (models.Message, error) {
	data, err := json.Marshal(messageData{Content: content})
	if err != nil {
		return models.Message{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Message{}, err
	}
	defer tx.Rollback(ctx)

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, data) VALUES ($1, $2, $3) RETURNING id
	`, conversationID, senderID, data).Scan(&id); err != nil {
		return models.Message{}, err
	}

	preview, err := json.Marshal(map[string]any{
		"lastMessage": truncatePreview(content), "lastSenderId": senderID,
	})
	if err != nil {
		return models.Message{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE conversations SET data = data || $2::jsonb, updated_at = now() WHERE id = $1
	`, conversationID, preview); err != nil {
		return models.Message{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Message{}, err
	}
	return s.FindMessage(ctx, id)
}

func (s *MessageStore) FindMessage(ctx context.Context, id string) (models.Message, error) {
	var m models.Message
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id, m.data, m.created_at,
		       coalesce(u.data->>'handle', ''), coalesce(u.data->>'name', ''),
		       coalesce(u.data->>'surname', ''), coalesce(u.data->>'picture', '')
		FROM messages m JOIN users u ON u.id = m.sender_id
		WHERE m.id = $1
	`, id).Scan(&m.ID, &m.ConversationID, &m.SenderID, &raw, &m.CreatedAt,
		&m.Sender.Handle, &m.Sender.Name, &m.Sender.Surname, &m.Sender.Picture)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Message{}, ErrNotFound
		}
		return models.Message{}, err
	}

	var d messageData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.Message{}, err
	}
	m.Content = d.Content
	m.ReadAt = d.ReadAt
	m.Sender.ID = m.SenderID
	return m, nil
}

// MarkRead stamps everything the caller hasn't opened in one thread. Scoped to
// messages somebody else sent: marking your own as read is meaningless, and
// would clear the other side's "delivered" state.
func (s *MessageStore) MarkRead(ctx context.Context, conversationID, callerID string, at time.Time) error {
	stamp, err := json.Marshal(map[string]any{"readAt": at})
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE messages SET data = data || $3::jsonb, updated_at = now()
		WHERE conversation_id = $1 AND sender_id <> $2 AND data->>'readAt' IS NULL
	`, conversationID, callerID, stamp)
	return err
}

// truncatePreview keeps the conversation list's preview line short. The whole
// message is in the thread; this is the one line under a name.
func truncatePreview(content string) string {
	const max = 140
	content = strings.Join(strings.Fields(content), " ")
	if len(content) <= max {
		return content
	}
	return strings.TrimSpace(content[:max]) + "…"
}

// --- blocks ---

// Block records that blockerID no longer wants to hear from blockedID. Blocking
// twice is not an error - it is the state the user asked for either way.
func (s *MessageStore) Block(ctx context.Context, blockerID, blockedID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING
	`, blockerID, blockedID)
	return err
}

// Unblock lifts one block. Only the blocker's own row is touched: a block is
// theirs to undo, and being blocked is not.
func (s *MessageStore) Unblock(ctx context.Context, blockerID, blockedID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`, blockerID, blockedID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListBlocks returns who the caller has blocked, so they can lift one.
func (s *MessageStore) ListBlocks(ctx context.Context, blockerID string) ([]models.Block, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.blocked_id, b.created_at,
		       coalesce(u.data->>'handle', ''), coalesce(u.data->>'name', ''),
		       coalesce(u.data->>'surname', ''), coalesce(u.data->>'picture', '')
		FROM blocks b JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC
	`, blockerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := []models.Block{}
	for rows.Next() {
		var b models.Block
		if err := rows.Scan(&b.ID, &b.BlockedID, &b.CreatedAt,
			&b.Blocked.Handle, &b.Blocked.Name, &b.Blocked.Surname, &b.Blocked.Picture); err != nil {
			return nil, err
		}
		b.Blocked.ID = b.BlockedID
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

// IsBlockedEither reports whether either of two members has blocked the other.
//
// Both directions, because a block is one person's decision with a mutual
// effect: the blocker should not have to see the person they blocked, and the
// blocked person must not be able to keep writing to somebody who stopped
// listening.
func (s *MessageStore) IsBlockedEither(ctx context.Context, a, b string) (bool, error) {
	var blocked bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM blocks
			WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)
		)
	`, a, b).Scan(&blocked)
	return blocked, err
}

// ListBlockedIDs returns every id the caller cannot see, in either direction -
// the set the feed queries filter their authors against.
func (s *MessageStore) ListBlockedIDs(ctx context.Context, callerID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT blocked_id FROM blocks WHERE blocker_id = $1
		UNION
		SELECT blocker_id FROM blocks WHERE blocked_id = $1
	`, callerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
