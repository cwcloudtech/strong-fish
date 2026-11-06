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

// CallbackStore keeps a member's recording callbacks: the triggers their phone
// fires when a bar-speed set starts - a webhook (HTTP), a WebSocket message or
// an MQTT publish.
//
// Shaped on the connection half of AIStore, without the sharing: a callback
// points at the member's own device and is nobody else's to spend, so there is
// no ACL and no catalogue - just a per-owner list.
type CallbackStore struct {
	pool *pgxpool.Pool
}

func NewCallbackStore(pool *pgxpool.Pool) *CallbackStore {
	return &CallbackStore{pool: pool}
}

// recordingCallbackData is the JSONB payload of the recording_callbacks table.
type recordingCallbackData struct {
	Name     string                  `json:"name,omitempty"`
	Type     string                  `json:"type,omitempty"`
	URL      string                  `json:"url"`
	Token    string                  `json:"token,omitempty"`
	Headers  []models.CallbackHeader `json:"headers,omitempty"`
	Username string                  `json:"username,omitempty"`
	Password string                  `json:"password,omitempty"`
	Topic    string                  `json:"topic,omitempty"`
	ClientID string                  `json:"clientId,omitempty"`
	QoS      int                     `json:"qos,omitempty"`
	// Enabled carries no omitempty on purpose: a false has to be stored, not
	// dropped, or a disabled callback would read back as enabled.
	Enabled bool `json:"enabled"`
}

const recordingCallbackSelect = `
	SELECT id, owner_id::text, data, created_at, updated_at
	FROM recording_callbacks`

func scanRecordingCallback(row pgx.Row) (models.RecordingCallback, error) {
	var callback models.RecordingCallback
	var raw []byte
	if err := row.Scan(&callback.ID, &callback.OwnerID, &raw,
		&callback.CreatedAt, &callback.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.RecordingCallback{}, ErrNotFound
		}
		return models.RecordingCallback{}, err
	}

	var data recordingCallbackData
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	callback.Name = data.Name
	callback.Type = data.Type
	callback.URL = data.URL
	callback.Token = data.Token
	callback.HasToken = strings.TrimSpace(data.Token) != ""
	callback.Headers = data.Headers
	callback.Username = data.Username
	callback.Password = data.Password
	callback.HasPassword = strings.TrimSpace(data.Password) != ""
	callback.Topic = data.Topic
	callback.ClientID = data.ClientID
	callback.QoS = data.QoS
	callback.Enabled = data.Enabled
	return callback, nil
}

// ListFor is every callback a member has, oldest first.
//
// The secrets are kept, not stripped: the phone that fires the callback needs
// them, and they are the member's own device credentials - theirs to read, and
// shared with no one. This is the one difference from the AI list, which hides
// its key because that key can be a coach's.
func (s *CallbackStore) ListFor(ctx context.Context, userID string) ([]models.RecordingCallback, error) {
	rows, err := s.pool.Query(ctx, recordingCallbackSelect+`
		WHERE owner_id::text = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	callbacks := []models.RecordingCallback{}
	for rows.Next() {
		callback, err := scanRecordingCallback(rows)
		if err != nil {
			return nil, err
		}
		callbacks = append(callbacks, callback)
	}
	return callbacks, rows.Err()
}

// FindByID reads one callback. The caller decides who may see it - the handler
// refuses anybody but the owner.
func (s *CallbackStore) FindByID(ctx context.Context, id string) (models.RecordingCallback, error) {
	return scanRecordingCallback(s.pool.QueryRow(ctx, recordingCallbackSelect+` WHERE id = $1`, id))
}

// Create stores a new callback for ownerID.
func (s *CallbackStore) Create(ctx context.Context, ownerID string,
	callback models.RecordingCallback) (models.RecordingCallback, error) {
	data, err := json.Marshal(dataOf(callback, callback.Token, callback.Password))
	if err != nil {
		return models.RecordingCallback{}, err
	}
	return scanRecordingCallback(s.pool.QueryRow(ctx, `
		INSERT INTO recording_callbacks (owner_id, data)
		VALUES ($1, $2)
		RETURNING id, owner_id::text, data, created_at, updated_at`, ownerID, data))
}

// Update replaces a callback's settings.
//
// A blank secret keeps the stored one, the same bargain the AI form strikes: a
// member editing the name or the URL submits empty token and password fields,
// and taking that literally would wipe the device's credentials on every
// rename. The non-secret fields (headers, username, mqtt settings) are replaced
// wholesale, so clearing a header is just leaving it out.
func (s *CallbackStore) Update(ctx context.Context, id string,
	callback models.RecordingCallback) (models.RecordingCallback, error) {
	// The merged document carries every field but the two secrets, which are
	// patched on top only when a new value was given.
	data, err := json.Marshal(dataOf(callback, "", ""))
	if err != nil {
		return models.RecordingCallback{}, err
	}

	token := strings.TrimSpace(callback.Token)
	password := strings.TrimSpace(callback.Password)
	return scanRecordingCallback(s.pool.QueryRow(ctx, `
		UPDATE recording_callbacks
		SET data = ((c.data || $2::jsonb)
		            || CASE WHEN $3 = '' THEN '{}'::jsonb
		                    ELSE jsonb_build_object('token', $3::text) END)
		            || CASE WHEN $4 = '' THEN '{}'::jsonb
		                    ELSE jsonb_build_object('password', $4::text) END,
		    updated_at = now()
		FROM recording_callbacks c
		WHERE recording_callbacks.id = $1 AND c.id = $1
		RETURNING recording_callbacks.id, recording_callbacks.owner_id::text,
		          recording_callbacks.data, recording_callbacks.created_at,
		          recording_callbacks.updated_at`, id, data, token, password))
}

// dataOf is the JSONB document for a callback, with the two secrets passed
// explicitly so Create can embed them and Update can leave them to its patch.
func dataOf(callback models.RecordingCallback, token, password string) recordingCallbackData {
	return recordingCallbackData{
		Name: callback.Name, Type: callback.Type, URL: callback.URL,
		Token: token, Headers: callback.Headers,
		Username: callback.Username, Password: password,
		Topic: callback.Topic, ClientID: callback.ClientID, QoS: callback.QoS,
		Enabled: callback.Enabled,
	}
}

// Delete removes a callback.
func (s *CallbackStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM recording_callbacks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
