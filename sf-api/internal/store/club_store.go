package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
)

type ClubStore struct {
	pool *pgxpool.Pool
}

func NewClubStore(pool *pgxpool.Pool) *ClubStore {
	return &ClubStore{pool: pool}
}

// clubData is the JSONB payload of the clubs table.
type clubData struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	City        string   `json:"city,omitempty"`
	Country     string   `json:"country,omitempty"`
	Picture     string   `json:"picture,omitempty"`
	PictureX    *float64 `json:"pictureX,omitempty"`
	PictureY    *float64 `json:"pictureY,omitempty"`
}

// clubSelect carries the caller's own role and the member count alongside every
// club, since the frontend needs both on every listing and neither is worth a
// second round trip. $1 is the calling user's id.
const clubSelect = `
	SELECT c.id, c.owner_id, c.data, c.created_at, c.updated_at,
	       coalesce(cm.role, ''),
	       (SELECT count(*) FROM club_members WHERE club_id = c.id),
	       coalesce(u.data->>'name', '') || ' ' || coalesce(u.data->>'surname', '')
	FROM clubs c
	LEFT JOIN club_members cm ON cm.club_id = c.id AND cm.user_id = $1
	JOIN users u ON u.id = c.owner_id`

func scanClub(row pgx.Row) (models.Club, error) {
	var c models.Club
	var raw []byte
	if err := row.Scan(&c.ID, &c.OwnerID, &raw, &c.CreatedAt, &c.UpdatedAt, &c.Role, &c.MemberCount, &c.OwnerName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Club{}, ErrNotFound
		}
		return models.Club{}, err
	}
	var d clubData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.Club{}, err
	}
	c.Name = d.Name
	c.Description = d.Description
	c.City = d.City
	c.Country = d.Country
	c.Picture = d.Picture
	c.PictureX = resolveImagePosition(d.PictureX)
	c.PictureY = resolveImagePosition(d.PictureY)
	return c, nil
}

func scanClubs(rows pgx.Rows) ([]models.Club, error) {
	defer rows.Close()
	clubs := []models.Club{}
	for rows.Next() {
		c, err := scanClub(rows)
		if err != nil {
			return nil, err
		}
		clubs = append(clubs, c)
	}
	return clubs, rows.Err()
}

// ClubFields are the club settings an owner or admin may set.
type ClubFields struct {
	Name        string
	Description string
	City        string
	Country     string
	Picture     *string
	PictureX    *float64
	PictureY    *float64
}

func (f ClubFields) patch() map[string]any {
	patch := map[string]any{
		"name":        f.Name,
		"description": f.Description,
		"city":        f.City,
		"country":     f.Country,
	}
	if f.Picture != nil {
		patch["picture"] = *f.Picture
	}
	if f.PictureX != nil {
		patch["pictureX"] = *f.PictureX
	}
	if f.PictureY != nil {
		patch["pictureY"] = *f.PictureY
	}
	return patch
}

// Create opens a new club and enrolls its creator as owner, in one transaction:
// a club with no owner row would be invisible to its own creator.
func (s *ClubStore) Create(ctx context.Context, ownerID string, f ClubFields) (models.Club, error) {
	data, err := json.Marshal(f.patch())
	if err != nil {
		return models.Club{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Club{}, err
	}
	defer tx.Rollback(ctx)

	var clubID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO clubs (owner_id, data) VALUES ($1, $2) RETURNING id
	`, ownerID, data).Scan(&clubID); err != nil {
		return models.Club{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO club_members (club_id, user_id, role) VALUES ($1, $2, $3)
	`, clubID, ownerID, models.RoleOwner); err != nil {
		return models.Club{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Club{}, err
	}
	return s.FindByID(ctx, clubID, ownerID)
}

// FindByID returns one club as seen by callerID (whose own role it carries).
func (s *ClubStore) FindByID(ctx context.Context, id, callerID string) (models.Club, error) {
	return scanClub(s.pool.QueryRow(ctx, clubSelect+` WHERE c.id = $2`, callerID, id))
}

// ListForUser returns the clubs userID belongs to.
func (s *ClubStore) ListForUser(ctx context.Context, userID string) ([]models.Club, error) {
	rows, err := s.pool.Query(ctx, clubSelect+`
		WHERE cm.user_id IS NOT NULL
		ORDER BY c.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	return scanClubs(rows)
}

// ListAll returns every club, for the superadmin's management screen. The
// caller's own role still comes through, so the UI can tell "I own this" from
// "I'm only administering it".
func (s *ClubStore) ListAll(ctx context.Context, callerID string) ([]models.Club, error) {
	rows, err := s.pool.Query(ctx, clubSelect+` ORDER BY c.created_at DESC`, callerID)
	if err != nil {
		return nil, err
	}
	return scanClubs(rows)
}

// Update changes a club's settings.
func (s *ClubStore) Update(ctx context.Context, id, callerID string, f ClubFields) (models.Club, error) {
	data, err := json.Marshal(f.patch())
	if err != nil {
		return models.Club{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE clubs SET data = data || $2::jsonb, updated_at = now() WHERE id = $1
	`, id, data)
	if err != nil {
		return models.Club{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Club{}, ErrNotFound
	}
	return s.FindByID(ctx, id, callerID)
}

// Delete removes a club and everything inside it (members, programs, club
// posts) by cascade.
func (s *ClubStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM clubs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- members ---

const memberSelect = `
	SELECT cm.id, cm.club_id, cm.user_id, u.email,
	       coalesce(u.data->>'handle', ''), coalesce(u.data->>'name', ''),
	       coalesce(u.data->>'surname', ''), coalesce(u.data->>'picture', ''),
	       coalesce((u.data->>'pictureX')::float, 50), coalesce((u.data->>'pictureY')::float, 50),
	       cm.role, cm.created_at, cm.updated_at
	FROM club_members cm
	JOIN users u ON u.id = cm.user_id`

func scanMember(row pgx.Row) (models.ClubMember, error) {
	var m models.ClubMember
	if err := row.Scan(&m.ID, &m.ClubID, &m.UserID, &m.Email, &m.Handle, &m.Name, &m.Surname,
		&m.Picture, &m.PictureX, &m.PictureY, &m.Role, &m.CreatedAt, &m.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ClubMember{}, ErrNotFound
		}
		return models.ClubMember{}, err
	}
	return m, nil
}

// ListMembers returns a club's roster, owner first then admins then members.
func (s *ClubStore) ListMembers(ctx context.Context, clubID string) ([]models.ClubMember, error) {
	rows, err := s.pool.Query(ctx, memberSelect+`
		WHERE cm.club_id = $1
		ORDER BY CASE cm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, u.email
	`, clubID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []models.ClubMember{}
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// FindMembership returns userID's role in clubID, or ErrNotFound when they
// aren't a member.
func (s *ClubStore) FindMembership(ctx context.Context, clubID, userID string) (models.ClubMember, error) {
	return scanMember(s.pool.QueryRow(ctx, memberSelect+` WHERE cm.club_id = $1 AND cm.user_id = $2`, clubID, userID))
}

// AddMember enrolls a user in a club.
func (s *ClubStore) AddMember(ctx context.Context, clubID, userID string, role models.Role) (models.ClubMember, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO club_members (club_id, user_id, role) VALUES ($1, $2, $3) RETURNING id
	`, clubID, userID, role).Scan(&id)
	if isUniqueViolation(err) {
		return models.ClubMember{}, ErrAlreadyMember
	}
	if err != nil {
		return models.ClubMember{}, err
	}
	return s.FindMembership(ctx, clubID, userID)
}

// SetMemberRole promotes a member to admin or demotes them back. The owner is
// refused outright: their role is the club's anchor, transferred by
// TransferOwnership rather than edited.
func (s *ClubStore) SetMemberRole(ctx context.Context, clubID, userID string, role models.Role) (models.ClubMember, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE club_members SET role = $3, updated_at = now()
		WHERE club_id = $1 AND user_id = $2 AND role <> 'owner'
	`, clubID, userID, role)
	if err != nil {
		return models.ClubMember{}, err
	}
	if tag.RowsAffected() == 0 {
		// Either there's no such membership, or it's the owner's. Distinguish
		// the two so the caller can report the right thing.
		if _, err := s.FindMembership(ctx, clubID, userID); err == nil {
			return models.ClubMember{}, ErrCannotRemoveOwner
		}
		return models.ClubMember{}, ErrNotFound
	}
	return s.FindMembership(ctx, clubID, userID)
}

// RemoveMember takes a user out of a club. The owner can't be removed - the
// club would be left without one.
func (s *ClubStore) RemoveMember(ctx context.Context, clubID, userID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM club_members WHERE club_id = $1 AND user_id = $2 AND role <> 'owner'
	`, clubID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		if _, err := s.FindMembership(ctx, clubID, userID); err == nil {
			return ErrCannotRemoveOwner
		}
		return ErrNotFound
	}
	return nil
}

// TransferOwnership hands a club to another of its members, demoting the
// previous owner to admin. Both writes happen together, so the club is never
// left with two owners or none.
func (s *ClubStore) TransferOwnership(ctx context.Context, clubID, newOwnerID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var previousOwnerID string
	if err := tx.QueryRow(ctx, `SELECT owner_id FROM clubs WHERE id = $1`, clubID).Scan(&previousOwnerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if previousOwnerID == newOwnerID {
		return nil
	}

	// The new owner must already be a member: ownership is transferred within
	// the club, not granted to an outsider.
	tag, err := tx.Exec(ctx, `
		UPDATE club_members SET role = 'owner', updated_at = now()
		WHERE club_id = $1 AND user_id = $2
	`, clubID, newOwnerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE club_members SET role = 'admin', updated_at = now()
		WHERE club_id = $1 AND user_id = $2
	`, clubID, previousOwnerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE clubs SET owner_id = $2, updated_at = now() WHERE id = $1
	`, clubID, newOwnerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ListClubIDsForUser returns just the ids of the clubs a user belongs to, for
// the feed's visibility filter.
func (s *ClubStore) ListClubIDsForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT club_id FROM club_members WHERE user_id = $1`, userID)
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

// ListProfileClubs returns the clubs shown on a user's public profile.
func (s *ClubStore) ListProfileClubs(ctx context.Context, userID string) ([]models.ProfileClub, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, coalesce(c.data->>'name', ''), cm.role
		FROM club_members cm
		JOIN clubs c ON c.id = cm.club_id
		WHERE cm.user_id = $1
		ORDER BY c.created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clubs := []models.ProfileClub{}
	for rows.Next() {
		var c models.ProfileClub
		if err := rows.Scan(&c.ID, &c.Name, &c.Role); err != nil {
			return nil, err
		}
		clubs = append(clubs, c)
	}
	return clubs, rows.Err()
}
