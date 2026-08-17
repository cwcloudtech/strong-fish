package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
)

// WebAuthnCredentialStore persists security keys (YubiKeys and platform
// authenticators) registered as an MFA factor.
type WebAuthnCredentialStore struct {
	pool *pgxpool.Pool
}

func NewWebAuthnCredentialStore(pool *pgxpool.Pool) *WebAuthnCredentialStore {
	return &WebAuthnCredentialStore{pool: pool}
}

const credentialColumns = `id, user_id, credential_id, public_key, sign_count, transports, name, created_at, updated_at`

func scanCredential(row pgx.Row) (models.WebAuthnCredential, error) {
	var c models.WebAuthnCredential
	var signCount int64
	if err := row.Scan(&c.ID, &c.UserID, &c.CredentialID, &c.PublicKey, &signCount,
		&c.Transports, &c.Name, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.WebAuthnCredential{}, ErrNotFound
		}
		return models.WebAuthnCredential{}, err
	}
	c.SignCount = uint32(signCount)
	return c, nil
}

// Create registers a new security key for userID.
func (s *WebAuthnCredentialStore) Create(ctx context.Context, userID string, credentialID, publicKey []byte, transports []string, name string) (models.WebAuthnCredential, error) {
	return scanCredential(s.pool.QueryRow(ctx, `
		INSERT INTO webauthn_credentials (user_id, credential_id, public_key, transports, name)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+credentialColumns, userID, credentialID, publicKey, transports, name))
}

// ListByUser returns userID's registered security keys, oldest first.
func (s *WebAuthnCredentialStore) ListByUser(ctx context.Context, userID string) ([]models.WebAuthnCredential, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+credentialColumns+` FROM webauthn_credentials WHERE user_id = $1 ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	credentials := []models.WebAuthnCredential{}
	for rows.Next() {
		c, err := scanCredential(rows)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, c)
	}
	return credentials, rows.Err()
}

// CountByUser reports how many security keys userID has registered, used to
// decide whether MFA should stay enabled after one is removed.
func (s *WebAuthnCredentialStore) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM webauthn_credentials WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

// UpdateSignCount persists the authenticator's signature counter after a
// successful login assertion, so a future login can detect a cloned
// authenticator.
func (s *WebAuthnCredentialStore) UpdateSignCount(ctx context.Context, credentialID []byte, signCount uint32) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE webauthn_credentials SET sign_count = $2, updated_at = now() WHERE credential_id = $1
	`, credentialID, signCount)
	return err
}

// Delete revokes one of userID's security keys.
func (s *WebAuthnCredentialStore) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM webauthn_credentials WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAllForUser revokes every security key userID has registered, used when
// a superadmin clears MFA for an account.
func (s *WebAuthnCredentialStore) DeleteAllForUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM webauthn_credentials WHERE user_id = $1`, userID)
	return err
}
