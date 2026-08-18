package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

type ApiKeyStore struct {
	pool *pgxpool.Pool
}

func NewApiKeyStore(pool *pgxpool.Pool) *ApiKeyStore {
	return &ApiKeyStore{pool: pool}
}

func scanApiKey(row pgx.Row) (models.ApiKey, error) {
	var k models.ApiKey
	if err := row.Scan(&k.ID, &k.UserID, &k.Description, &k.ExpiresAt, &k.CreatedAt, &k.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ApiKey{}, ErrNotFound
		}
		return models.ApiKey{}, err
	}
	return k, nil
}

// Create mints a key for userID and returns both the stored record and the
// plaintext token - the only moment the plaintext exists outside the caller.
func (s *ApiKeyStore) Create(ctx context.Context, userID, description string, expiresAt *time.Time) (models.ApiKey, string, error) {
	// The same 32 random bytes every other secret in this app is minted from
	// (see store.generateToken); only its sha256 is ever written down.
	token, err := generateToken()
	if err != nil {
		return models.ApiKey{}, utils.EMPTY, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO api_keys (user_id, key_hash, description, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, description, expires_at, created_at, updated_at
	`, userID, utils.HashToken(token), description, expiresAt)
	key, err := scanApiKey(row)
	if err != nil {
		return models.ApiKey{}, utils.EMPTY, err
	}
	return key, token, nil
}

// ListByUser returns userID's keys, most recently created first.
func (s *ApiKeyStore) ListByUser(ctx context.Context, userID string) ([]models.ApiKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, description, expires_at, created_at, updated_at
		FROM api_keys WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []models.ApiKey{}
	for rows.Next() {
		key, err := scanApiKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// Delete revokes a key, scoped to its owner so nobody can revoke someone
// else's by guessing an id.
func (s *ApiKeyStore) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// VerifyHash resolves the user a still-valid (non-expired) key belongs to,
// from the sha256 of its plaintext token.
func (s *ApiKeyStore) VerifyHash(ctx context.Context, hash string) (string, error) {
	var userID string
	err := s.pool.QueryRow(ctx, `
		SELECT user_id FROM api_keys
		WHERE key_hash = $1 AND (expires_at IS NULL OR expires_at > now())
	`, hash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.EMPTY, ErrNotFound
		}
		return utils.EMPTY, err
	}
	return userID, nil
}
