package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
)

// DeviceLinkStore keeps a member's grants to the outside accounts that count
// things for them - a Google Health profile today, another provider tomorrow.
//
// The tokens never leave this package for a client: the handlers read a link
// to call the provider with it, and what they return to the browser is the
// member's steps, never the credential that fetched them.
type DeviceLinkStore struct {
	pool *pgxpool.Pool
}

func NewDeviceLinkStore(pool *pgxpool.Pool) *DeviceLinkStore {
	return &DeviceLinkStore{pool: pool}
}

// deviceLinkData is the JSONB payload of the device_links table: what is not
// indexed, and what differs between providers.
type deviceLinkData struct {
	Scope          string `json:"scope,omitempty"`
	ExternalUserID string `json:"externalUserId,omitempty"`
}

const deviceLinkSelect = `
	SELECT id, user_id::text, provider, access_token, refresh_token, expires_at, data, created_at, updated_at
	FROM device_links`

func scanDeviceLink(row pgx.Row) (models.DeviceLink, error) {
	var link models.DeviceLink
	var expires *time.Time
	var raw []byte
	if err := row.Scan(&link.ID, &link.UserID, &link.Provider, &link.AccessToken,
		&link.RefreshToken, &expires, &raw, &link.CreatedAt, &link.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.DeviceLink{}, ErrNotFound
		}
		return models.DeviceLink{}, err
	}
	if expires != nil {
		link.ExpiresAt = *expires
	}
	var data deviceLinkData
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	link.Scope = data.Scope
	link.ExternalUserID = data.ExternalUserID
	return link, nil
}

// Find returns a member's link to one provider, or ErrNotFound.
func (s *DeviceLinkStore) Find(ctx context.Context, userID, provider string) (models.DeviceLink, error) {
	return scanDeviceLink(s.pool.QueryRow(ctx,
		deviceLinkSelect+` WHERE user_id = $1 AND provider = $2`, userID, provider))
}

// ListProviders returns the providers a member has connected. Only the names:
// the profile screen asks which switches are on, not for anybody's tokens.
func (s *DeviceLinkStore) ListProviders(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT provider FROM device_links WHERE user_id = $1 ORDER BY provider`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := []string{}
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

// Save writes a member's grant, replacing whatever they had for that provider.
//
// Connecting again is an update rather than a second row: two links to the
// same provider would mean two refresh tokens, one of which nobody renews and
// which therefore rots into a mysterious failure months later.
func (s *DeviceLinkStore) Save(ctx context.Context, link models.DeviceLink) (models.DeviceLink, error) {
	data, err := json.Marshal(deviceLinkData{Scope: link.Scope, ExternalUserID: link.ExternalUserID})
	if err != nil {
		return models.DeviceLink{}, err
	}

	var expires *time.Time
	if !link.ExpiresAt.IsZero() {
		expires = &link.ExpiresAt
	}

	return scanDeviceLink(s.pool.QueryRow(ctx, `
		INSERT INTO device_links (user_id, provider, access_token, refresh_token, expires_at, data)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			access_token = excluded.access_token,
			-- Kept when the provider sends none back, which is what a refresh
			-- does: losing it here would turn a working link into one that
			-- expires within the hour.
			refresh_token = CASE WHEN excluded.refresh_token = '' THEN device_links.refresh_token
			                     ELSE excluded.refresh_token END,
			expires_at = excluded.expires_at,
			data = device_links.data || excluded.data,
			updated_at = now()
		RETURNING id, user_id::text, provider, access_token, refresh_token, expires_at, data, created_at, updated_at
	`, link.UserID, link.Provider, link.AccessToken, link.RefreshToken, expires, data))
}

// Delete removes a member's link. Deleting one that is not there is not an
// error: disconnecting twice should be as quiet as disconnecting once.
func (s *DeviceLinkStore) Delete(ctx context.Context, userID, provider string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM device_links WHERE user_id = $1 AND provider = $2`, userID, provider)
	return err
}
