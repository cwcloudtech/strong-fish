package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

// StorageStore is the object stores members bring themselves, and who may use
// them.
//
// A storage used to live inside its owner's user payload (see V12): one per
// account, reachable by nobody else. It is a row now because it is shared -
// a coach lends their bucket to an athlete, a club pays for one everybody
// uploads to - and sharing needs something to point at.
type StorageStore struct {
	pool *pgxpool.Pool
}

func NewStorageStore(pool *pgxpool.Pool) *StorageStore {
	return &StorageStore{pool: pool}
}

const storageSelect = `SELECT s.id, s.owner_id, s.data, s.created_at, s.updated_at FROM storages s`

func scanStorage(row pgx.Row) (models.Storage, error) {
	var storage models.Storage
	var raw []byte
	if err := row.Scan(&storage.ID, &storage.OwnerID, &raw, &storage.CreatedAt, &storage.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Storage{}, ErrNotFound
		}
		return models.Storage{}, err
	}
	if err := json.Unmarshal(raw, &storage.Conn); err != nil {
		return models.Storage{}, err
	}
	return storage, nil
}

// FindByID reads one storage, credentials and all. The caller is responsible
// for having established that whoever is asking may use it - see RoleFor.
func (s *StorageStore) FindByID(ctx context.Context, id string) (models.Storage, error) {
	return scanStorage(s.pool.QueryRow(ctx, storageSelect+` WHERE s.id = $1`, id))
}

// FindOwn returns the storage an account owns, or ErrNotFound.
//
// One per owner is a rule this app keeps rather than one the schema enforces:
// the settings screen edits "my storage", and a second one would have nowhere
// to be shown. The schema stays open because a club-owned storage is the
// obvious next thing, and a unique index would have to be dropped to get there.
func (s *StorageStore) FindOwn(ctx context.Context, ownerID string) (models.Storage, error) {
	return scanStorage(s.pool.QueryRow(ctx,
		storageSelect+` WHERE s.owner_id = $1 ORDER BY s.created_at LIMIT 1`, ownerID))
}

// ListFor returns every storage the caller may use - their own first, then the
// ones shared with them - each carrying their role and the owner's name.
func (s *StorageStore) ListFor(ctx context.Context, userID string) ([]models.Storage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.owner_id, s.data, s.created_at, s.updated_at,
		       acl.role, `+displayFullName("u")+`
		FROM storages s
		JOIN storage_acl acl ON acl.storage_id = s.id AND acl.user_id = $1
		JOIN users u ON u.id = s.owner_id
		ORDER BY (s.owner_id = $1) DESC, s.created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	storages := []models.Storage{}
	for rows.Next() {
		var storage models.Storage
		var raw []byte
		if err := rows.Scan(&storage.ID, &storage.OwnerID, &raw, &storage.CreatedAt,
			&storage.UpdatedAt, &storage.Role, &storage.OwnerName); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &storage.Conn); err != nil {
			return nil, err
		}
		storages = append(storages, storage)
	}
	return storages, rows.Err()
}

// RoleFor is what userID may do with storageID: owner, writer, reader, or the
// empty string for no access at all.
//
// Every access question goes through here rather than comparing owner ids at
// the call site, which is how one of them ends up forgetting that a storage can
// be shared.
func (s *StorageStore) RoleFor(ctx context.Context, storageID, userID string) (string, error) {
	if utils.IsBlank(storageID) || utils.IsBlank(userID) {
		return utils.EMPTY, nil
	}

	var role string
	err := s.pool.QueryRow(ctx, `
		SELECT role FROM storage_acl WHERE storage_id = $1 AND user_id = $2
	`, storageID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.EMPTY, nil
	}
	if err != nil {
		return utils.EMPTY, err
	}
	return role, nil
}

// Upsert writes the caller's own connection, creating the storage the first
// time and its owner grant with it.
func (s *StorageStore) Upsert(ctx context.Context, ownerID string, conn models.StorageConnection) (models.Storage, error) {
	data, err := json.Marshal(conn)
	if err != nil {
		return models.Storage{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Storage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `SELECT id FROM storages WHERE owner_id = $1 ORDER BY created_at LIMIT 1`, ownerID).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if err := tx.QueryRow(ctx, `
			INSERT INTO storages (owner_id, data) VALUES ($1, $2) RETURNING id
		`, ownerID, data).Scan(&id); err != nil {
			return models.Storage{}, err
		}
		// The owner's own grant, written with the storage: every other query
		// asks the access list, so a storage without one would be invisible to
		// the person who just configured it.
		if _, err := tx.Exec(ctx, `
			INSERT INTO storage_acl (storage_id, user_id, role) VALUES ($1, $2, $3)
			ON CONFLICT (storage_id, user_id) DO UPDATE SET role = $3, updated_at = now()
		`, id, ownerID, models.StorageRoleOwner); err != nil {
			return models.Storage{}, err
		}
	case err != nil:
		return models.Storage{}, err
	default:
		if _, err := tx.Exec(ctx, `
			UPDATE storages SET data = $2, updated_at = now() WHERE id = $1
		`, id, data); err != nil {
			return models.Storage{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Storage{}, err
	}
	return s.FindByID(ctx, id)
}

// Delete removes a storage and, by cascade, every grant on it.
func (s *StorageStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM storages WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Grant gives somebody a role on a storage, or changes the one they have.
func (s *StorageStore) Grant(ctx context.Context, storageID, userID, role string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO storage_acl (storage_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (storage_id, user_id) DO UPDATE SET role = $3, updated_at = now()
	`, storageID, userID, role)
	return err
}

// Revoke takes an access away. The owner's own grant is refused: it is what
// makes the storage theirs, and a storage nobody owns could not be edited or
// deleted by anyone.
func (s *StorageStore) Revoke(ctx context.Context, storageID, userID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM storage_acl
		WHERE storage_id = $1 AND user_id = $2 AND role <> $3
	`, storageID, userID, models.StorageRoleOwner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListGrants is a storage's access list as its owner reads it, with each
// person's name - the owner's own row included, so the screen shows who it
// belongs to rather than implying it.
func (s *StorageStore) ListGrants(ctx context.Context, storageID string) ([]models.StorageGrant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT acl.user_id, `+displayFullName("u")+`,
		       coalesce(u.data->>'handle', ''), coalesce(u.data->>'picture', ''),
		       acl.role, acl.created_at
		FROM storage_acl acl
		JOIN users u ON u.id = acl.user_id
		WHERE acl.storage_id = $1
		ORDER BY (acl.role = 'owner') DESC, acl.created_at
	`, storageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grants := []models.StorageGrant{}
	for rows.Next() {
		var grant models.StorageGrant
		if err := rows.Scan(&grant.UserID, &grant.Name, &grant.Handle,
			&grant.Picture, &grant.Role, &grant.CreatedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}
