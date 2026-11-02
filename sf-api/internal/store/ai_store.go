package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
)

// AIStore keeps members' model connections, who may spend them, and what the
// model has already said about a session.
//
// Shaped on StorageStore on purpose: an owner holds a credential and decides
// who else may use it, which is the same arrangement a shared bucket is, and
// two permission systems to keep in step would be one too many.
type AIStore struct {
	pool *pgxpool.Pool
}

func NewAIStore(pool *pgxpool.Pool) *AIStore {
	return &AIStore{pool: pool}
}

// aiConnectionData is the JSONB payload of the ai_connections table.
type aiConnectionData struct {
	Name     string `json:"name,omitempty"`
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model,omitempty"`
	Token    string `json:"token,omitempty"`
}

const aiConnectionSelect = `
	SELECT c.id, c.owner_id::text, c.data, c.created_at, c.updated_at`

func scanAIConnection(row pgx.Row, withRole, withOwner bool) (models.AIConnection, error) {
	var connection models.AIConnection
	var raw []byte
	targets := []any{&connection.ID, &connection.OwnerID, &raw, &connection.CreatedAt, &connection.UpdatedAt}
	if withRole {
		targets = append(targets, &connection.Role)
	}
	if withOwner {
		targets = append(targets, &connection.OwnerName)
	}
	if err := row.Scan(targets...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.AIConnection{}, ErrNotFound
		}
		return models.AIConnection{}, err
	}

	var data aiConnectionData
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	connection.Name = data.Name
	connection.Provider = data.Provider
	connection.BaseURL = data.BaseURL
	connection.Model = data.Model
	connection.Token = data.Token
	connection.HasToken = strings.TrimSpace(data.Token) != ""
	return connection, nil
}

// FindByID reads one connection, token included. Callers decide who may see
// what: nothing here checks, because the uses differ - spending a connection
// needs the token, listing one must never carry it.
func (s *AIStore) FindByID(ctx context.Context, id string) (models.AIConnection, error) {
	return scanAIConnection(s.pool.QueryRow(ctx,
		aiConnectionSelect+` FROM ai_connections c WHERE c.id = $1`, id), false, false)
}

// ListFor is every connection a member may spend: their own, and the ones
// shared with them, newest first within each.
func (s *AIStore) ListFor(ctx context.Context, userID string) ([]models.AIConnection, error) {
	rows, err := s.pool.Query(ctx, aiConnectionSelect+`,
		       CASE WHEN c.owner_id::text = $1 THEN 'owner' ELSE coalesce(a.role, '') END,
		       `+displayFullName("u")+`
		FROM ai_connections c
		JOIN users u ON u.id = c.owner_id
		LEFT JOIN ai_connection_acl a ON a.connection_id = c.id AND a.user_id::text = $1
		WHERE c.owner_id::text = $1 OR a.user_id::text = $1
		ORDER BY (c.owner_id::text = $1) DESC, c.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	connections := []models.AIConnection{}
	for rows.Next() {
		connection, err := scanAIConnection(rows, true, true)
		if err != nil {
			return nil, err
		}
		// Never out of this function with a credential on it: this is the
		// list every client renders, and a token that leaves here once is a
		// token in somebody's browser cache forever.
		connection.Token = ""
		connections = append(connections, connection)
	}
	return connections, rows.Err()
}

// RoleFor is what userID may do with this connection, or "" for nothing.
func (s *AIStore) RoleFor(ctx context.Context, connectionID, userID string) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx, `
		SELECT CASE WHEN c.owner_id::text = $2 THEN 'owner' ELSE coalesce(a.role, '') END
		FROM ai_connections c
		LEFT JOIN ai_connection_acl a ON a.connection_id = c.id AND a.user_id::text = $2
		WHERE c.id = $1`, connectionID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return role, nil
}

// Usable picks the connection a member's questions go through: their own
// first, then one shared with them.
//
// Chosen here rather than asked of the member on every question. Somebody with
// one key should never have to pick it, and somebody using their coach's
// should not have to understand whose it is to ask something.
func (s *AIStore) Usable(ctx context.Context, userID string) (models.AIConnection, error) {
	return scanAIConnection(s.pool.QueryRow(ctx, aiConnectionSelect+`
		FROM ai_connections c
		LEFT JOIN ai_connection_acl a ON a.connection_id = c.id AND a.user_id::text = $1
		WHERE c.owner_id::text = $1 OR a.user_id::text = $1
		ORDER BY (c.owner_id::text = $1) DESC, c.created_at
		LIMIT 1`, userID), false, false)
}

// Create stores a new connection for ownerID.
func (s *AIStore) Create(ctx context.Context, ownerID string, connection models.AIConnection) (models.AIConnection, error) {
	data, err := json.Marshal(aiConnectionData{
		Name: connection.Name, Provider: connection.Provider,
		BaseURL: connection.BaseURL, Model: connection.Model, Token: connection.Token,
	})
	if err != nil {
		return models.AIConnection{}, err
	}
	return scanAIConnection(s.pool.QueryRow(ctx, `
		INSERT INTO ai_connections (owner_id, data) VALUES ($1, $2)
		RETURNING id, owner_id::text, data, created_at, updated_at`, ownerID, data), false, false)
}

// Update replaces a connection's settings.
//
// A blank token keeps the stored one. The form cannot show a secret it is not
// allowed to read, so it submits an empty field whenever the member did not
// retype one - and taking that literally would wipe the key every time
// somebody renamed their connection.
func (s *AIStore) Update(ctx context.Context, id string, connection models.AIConnection) (models.AIConnection, error) {
	data, err := json.Marshal(aiConnectionData{
		Name: connection.Name, Provider: connection.Provider,
		BaseURL: connection.BaseURL, Model: connection.Model,
	})
	if err != nil {
		return models.AIConnection{}, err
	}

	token := strings.TrimSpace(connection.Token)
	return scanAIConnection(s.pool.QueryRow(ctx, `
		UPDATE ai_connections
		SET data = (c.data || $2::jsonb) ||
		           CASE WHEN $3 = '' THEN '{}'::jsonb
		                ELSE jsonb_build_object('token', $3::text) END,
		    updated_at = now()
		FROM ai_connections c
		WHERE ai_connections.id = $1 AND c.id = $1
		RETURNING ai_connections.id, ai_connections.owner_id::text, ai_connections.data,
		          ai_connections.created_at, ai_connections.updated_at`, id, data, token), false, false)
}

func (s *AIStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM ai_connections WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Grant gives somebody else the right to spend this connection.
func (s *AIStore) Grant(ctx context.Context, connectionID, userID, role string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai_connection_acl (connection_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (connection_id, user_id) DO UPDATE SET role = excluded.role, updated_at = now()
	`, connectionID, userID, role)
	return err
}

// Revoke takes it back. Revoking what was never granted is not an error.
func (s *AIStore) Revoke(ctx context.Context, connectionID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM ai_connection_acl WHERE connection_id = $1 AND user_id = $2`, connectionID, userID)
	return err
}

// ListGrants is everybody an owner has shared this connection with.
func (s *AIStore) ListGrants(ctx context.Context, connectionID string) ([]models.AIGrant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.user_id::text, `+displayFullName("u")+`,
		       coalesce(u.data->>'handle', ''), coalesce(u.data->>'picture', ''), a.role, a.created_at
		FROM ai_connection_acl a JOIN users u ON u.id = a.user_id
		WHERE a.connection_id = $1 ORDER BY a.created_at`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grants := []models.AIGrant{}
	for rows.Next() {
		var grant models.AIGrant
		if err := rows.Scan(&grant.UserID, &grant.Name, &grant.Handle,
			&grant.Picture, &grant.Role, &grant.CreatedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

// --- session feedback ---

type sessionFeedbackData struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

// FindFeedback reads what the model already said about one session.
func (s *AIStore) FindFeedback(ctx context.Context, assignmentID, dayID string) (models.SessionFeedback, error) {
	var feedback models.SessionFeedback
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT f.id, f.assignment_id::text, f.day_id::text, f.author_id::text, f.content, f.data,
		       `+displayFullName("u")+`, f.created_at, f.updated_at
		FROM session_feedback f JOIN users u ON u.id = f.author_id
		WHERE f.assignment_id = $1 AND f.day_id = $2
	`, assignmentID, dayID).Scan(&feedback.ID, &feedback.AssignmentID, &feedback.DayID,
		&feedback.AuthorID, &feedback.Content, &raw, &feedback.AuthorName,
		&feedback.CreatedAt, &feedback.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.SessionFeedback{}, ErrNotFound
		}
		return models.SessionFeedback{}, err
	}

	var data sessionFeedbackData
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	feedback.Provider = data.Provider
	feedback.Model = data.Model
	return feedback, nil
}

// SaveFeedback stores a reading, replacing whatever was there.
//
// Replacing rather than appending: regenerating is asking for a better answer
// to the same question, not keeping both.
func (s *AIStore) SaveFeedback(ctx context.Context, feedback models.SessionFeedback) (models.SessionFeedback, error) {
	data, err := json.Marshal(sessionFeedbackData{Provider: feedback.Provider, Model: feedback.Model})
	if err != nil {
		return models.SessionFeedback{}, err
	}

	var id string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO session_feedback (assignment_id, day_id, author_id, content, data)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (assignment_id, day_id) DO UPDATE SET
			author_id = excluded.author_id, content = excluded.content,
			data = excluded.data, updated_at = now()
		RETURNING id
	`, feedback.AssignmentID, feedback.DayID, feedback.AuthorID, feedback.Content, data).Scan(&id); err != nil {
		return models.SessionFeedback{}, err
	}
	return s.FindFeedback(ctx, feedback.AssignmentID, feedback.DayID)
}

// DeleteFeedback removes a reading. Deleting one that is not there is not an
// error: it is how a client asks for a regeneration without checking first.
func (s *AIStore) DeleteFeedback(ctx context.Context, assignmentID, dayID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM session_feedback WHERE assignment_id = $1 AND day_id = $2`, assignmentID, dayID)
	return err
}

// FeedbackForAssignment is every reading on a block, by day.
//
// One query rather than one per session: a block has forty of them, and a
// screen that draws a week should not ask forty times to find out which three
// have been read.
func (s *AIStore) FeedbackForAssignment(ctx context.Context, assignmentID string) (
	map[string]models.SessionFeedback, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT f.id, f.assignment_id::text, f.day_id::text, f.author_id::text, f.content, f.data,
		       `+displayFullName("u")+`, f.created_at, f.updated_at
		FROM session_feedback f JOIN users u ON u.id = f.author_id
		WHERE f.assignment_id = $1`, assignmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byDay := map[string]models.SessionFeedback{}
	for rows.Next() {
		var feedback models.SessionFeedback
		var raw []byte
		if err := rows.Scan(&feedback.ID, &feedback.AssignmentID, &feedback.DayID,
			&feedback.AuthorID, &feedback.Content, &raw, &feedback.AuthorName,
			&feedback.CreatedAt, &feedback.UpdatedAt); err != nil {
			return nil, err
		}
		var data sessionFeedbackData
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &data)
		}
		feedback.Provider, feedback.Model = data.Provider, data.Model
		byDay[feedback.DayID] = feedback
	}
	return byDay, rows.Err()
}

// ReadingsForAssignment is the same, as text by day - what an export prints
// under each session.
func (s *AIStore) ReadingsForAssignment(ctx context.Context, assignmentID string) (map[string]string, error) {
	stored, err := s.FeedbackForAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	byDay := make(map[string]string, len(stored))
	for dayID, feedback := range stored {
		byDay[dayID] = feedback.Content
	}
	return byDay, nil
}

// --- conversations ---

type aiConversationData struct {
	Title      string `json:"title,omitempty"`
	ShareToken string `json:"shareToken,omitempty"`
}

// ListConversations is a member's threads, newest activity first, without the
// exchanges - a list shows titles, and loading everybody's questions to draw
// one would be loading a year of conversation to render a sidebar.
func (s *AIStore) ListConversations(ctx context.Context, userID string) ([]models.AIConversation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.user_id::text, c.data, c.created_at, c.updated_at,
		       (SELECT count(*) FROM ai_prompts p WHERE p.conversation_id = c.id)
		FROM ai_conversations c
		WHERE c.user_id = $1
		ORDER BY c.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := []models.AIConversation{}
	for rows.Next() {
		var conversation models.AIConversation
		var raw []byte
		if err := rows.Scan(&conversation.ID, &conversation.OwnerID, &raw,
			&conversation.CreatedAt, &conversation.UpdatedAt, &conversation.Count); err != nil {
			return nil, err
		}
		var data aiConversationData
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &data)
		}
		conversation.Title = data.Title
		// Whether it is shared, never the token: a list carrying everybody's
		// share tokens is a list of readable links.
		conversation.Shared = data.ShareToken != ""
		conversations = append(conversations, conversation)
	}
	return conversations, rows.Err()
}

// FindConversation reads one thread with its exchanges, for its owner.
func (s *AIStore) FindConversation(ctx context.Context, id, userID string) (models.AIConversation, error) {
	return s.conversationBy(ctx, `WHERE c.id = $1 AND c.user_id = $2`, true, id, userID)
}

// FindSharedConversation reads a thread by its share token, for anybody
// holding the link. The token is the whole credential, which is why it is
// long and why revoking it is immediate.
func (s *AIStore) FindSharedConversation(ctx context.Context, token string) (models.AIConversation, error) {
	return s.conversationBy(ctx, `WHERE c.data->>'shareToken' = $1`, false, token)
}

func (s *AIStore) conversationBy(ctx context.Context, where string, withToken bool, args ...any) (models.AIConversation, error) {
	var conversation models.AIConversation
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.user_id::text, c.data, c.created_at, c.updated_at
		FROM ai_conversations c `+where,
		args...).Scan(&conversation.ID, &conversation.OwnerID, &raw,
		&conversation.CreatedAt, &conversation.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.AIConversation{}, ErrNotFound
		}
		return models.AIConversation{}, err
	}

	var data aiConversationData
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	conversation.Title = data.Title
	conversation.Shared = data.ShareToken != ""
	if withToken {
		conversation.ShareToken = data.ShareToken
	} else {
		// A link's reader is told what was asked and answered, not who by.
		conversation.OwnerID = ""
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, question, answer, data, created_at FROM ai_prompts
		WHERE conversation_id = $1 ORDER BY created_at`, conversation.ID)
	if err != nil {
		return models.AIConversation{}, err
	}
	defer rows.Close()

	conversation.Exchanges = []models.AIExchange{}
	for rows.Next() {
		var exchange models.AIExchange
		var exchangeRaw []byte
		if err := rows.Scan(&exchange.ID, &exchange.Question, &exchange.Answer,
			&exchangeRaw, &exchange.CreatedAt); err != nil {
			return models.AIConversation{}, err
		}
		var meta struct {
			Provider string         `json:"provider,omitempty"`
			Model    string         `json:"model,omitempty"`
			Usage    map[string]any `json:"usage,omitempty"`
		}
		if len(exchangeRaw) > 0 {
			_ = json.Unmarshal(exchangeRaw, &meta)
		}
		exchange.Provider, exchange.Model, exchange.Usage = meta.Provider, meta.Model, meta.Usage
		conversation.Exchanges = append(conversation.Exchanges, exchange)
	}
	conversation.Count = len(conversation.Exchanges)
	return conversation, rows.Err()
}

// CreateConversation starts a thread, titled from its first question.
func (s *AIStore) CreateConversation(ctx context.Context, userID, title string) (string, error) {
	data, err := json.Marshal(aiConversationData{Title: title})
	if err != nil {
		return "", err
	}
	var id string
	err = s.pool.QueryRow(ctx,
		`INSERT INTO ai_conversations (user_id, data) VALUES ($1, $2) RETURNING id`,
		userID, data).Scan(&id)
	return id, err
}

// AppendExchange stores a question and its answer, and marks the thread as
// just used so the list orders by real activity.
func (s *AIStore) AppendExchange(ctx context.Context, conversationID string, exchange models.AIExchange) error {
	data, err := json.Marshal(map[string]any{
		"provider": exchange.Provider, "model": exchange.Model, "usage": exchange.Usage,
	})
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_prompts (conversation_id, question, answer, data) VALUES ($1, $2, $3, $4)
	`, conversationID, exchange.Question, exchange.Answer, data); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE ai_conversations SET updated_at = now() WHERE id = $1`, conversationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RenameConversation sets a thread's title.
func (s *AIStore) RenameConversation(ctx context.Context, id, userID, title string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE ai_conversations SET data = data || jsonb_build_object('title', $3::text),
		    updated_at = now()
		WHERE id = $1 AND user_id = $2`, id, userID, title)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ShareConversation mints a link token, or returns the one already minted -
// sharing twice should hand back the same link, not invalidate the one a
// member has already sent somebody.
func (s *AIStore) ShareConversation(ctx context.Context, id, userID string) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}

	var stored string
	err = s.pool.QueryRow(ctx, `
		UPDATE ai_conversations
		SET data = data || jsonb_build_object('shareToken',
		        coalesce(nullif(data->>'shareToken', ''), $3::text)),
		    updated_at = now()
		WHERE id = $1 AND user_id = $2
		RETURNING data->>'shareToken'`, id, userID, token).Scan(&stored)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return stored, nil
}

// UnshareConversation revokes the link. Anybody holding it stops being able
// to read the thread immediately, which is the whole point of revoking.
func (s *AIStore) UnshareConversation(ctx context.Context, id, userID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE ai_conversations SET data = data - 'shareToken', updated_at = now()
		WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteConversations removes threads, by id or all of them.
//
// Scoped to the owner in the statement rather than checked first: a delete
// that takes the ids it was given and filters by user in the same breath
// cannot be talked into removing somebody else's by passing their id.
func (s *AIStore) DeleteConversations(ctx context.Context, userID string, ids []string, all bool) (int64, error) {
	if all {
		tag, err := s.pool.Exec(ctx, `DELETE FROM ai_conversations WHERE user_id = $1`, userID)
		if err != nil {
			return 0, err
		}
		return tag.RowsAffected(), nil
	}
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM ai_conversations WHERE user_id = $1 AND id::text = ANY($2)`, userID, ids)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// --- health readings ---

type healthFeedbackData struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Days     int    `json:"days,omitempty"`
}

// FindHealthFeedback is the member's stored reading of their health data.
func (s *AIStore) FindHealthFeedback(ctx context.Context, userID, source string) (models.HealthFeedback, error) {
	var feedback models.HealthFeedback
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, source, content, data, created_at, updated_at
		FROM health_feedback WHERE user_id = $1 AND source = $2`, userID, source).
		Scan(&feedback.ID, &feedback.Source, &feedback.Content, &raw,
			&feedback.CreatedAt, &feedback.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.HealthFeedback{}, ErrNotFound
		}
		return models.HealthFeedback{}, err
	}

	var data healthFeedbackData
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	feedback.Provider, feedback.Model, feedback.Days = data.Provider, data.Model, data.Days
	return feedback, nil
}

// SaveHealthFeedback replaces the member's reading for that source.
func (s *AIStore) SaveHealthFeedback(ctx context.Context, userID string,
	feedback models.HealthFeedback) (models.HealthFeedback, error) {
	data, err := json.Marshal(healthFeedbackData{
		Provider: feedback.Provider, Model: feedback.Model, Days: feedback.Days,
	})
	if err != nil {
		return models.HealthFeedback{}, err
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO health_feedback (user_id, source, content, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, source) DO UPDATE
		SET content = EXCLUDED.content, data = EXCLUDED.data, updated_at = now()
		RETURNING id, created_at, updated_at`,
		userID, feedback.Source, feedback.Content, data).
		Scan(&feedback.ID, &feedback.CreatedAt, &feedback.UpdatedAt)
	return feedback, err
}

// DeleteHealthFeedback removes it, which is how a member asks for another one
// - subject to the same wait as regenerating.
func (s *AIStore) DeleteHealthFeedback(ctx context.Context, userID, source string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM health_feedback WHERE user_id = $1 AND source = $2`, userID, source)
	return err
}
