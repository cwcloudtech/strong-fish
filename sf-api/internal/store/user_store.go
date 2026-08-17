package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

// userData is the JSONB payload of the users table.
type userData struct {
	Password      string   `json:"password"`
	Name          string   `json:"name,omitempty"`
	Surname       string   `json:"surname,omitempty"`
	Role          string   `json:"role,omitempty"`
	Handle        string   `json:"handle,omitempty"`
	Bio           string   `json:"bio,omitempty"`
	Picture       string   `json:"picture,omitempty"`
	PictureX      *float64 `json:"pictureX,omitempty"`
	PictureY      *float64 `json:"pictureY,omitempty"`
	Locale        string   `json:"locale,omitempty"`
	PublicProfile bool     `json:"publicProfile,omitempty"`
	Bodyweight    float64  `json:"bodyweight,omitempty"`
	MFAEnabled    bool     `json:"mfaEnabled,omitempty"`
	MFATOTPSecret string   `json:"mfaTotpSecret,omitempty"`
}

const userColumns = `id, email, data, created_at, updated_at`

func scanUser(row pgx.Row) (models.User, error) {
	var u models.User
	var raw []byte
	if err := row.Scan(&u.ID, &u.Email, &raw, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrNotFound
		}
		return models.User{}, err
	}
	var d userData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.User{}, err
	}
	u.PasswordHash = d.Password
	u.Name = d.Name
	u.Surname = d.Surname
	u.Role = models.GlobalRole(d.Role)
	u.Handle = d.Handle
	u.Bio = d.Bio
	u.Picture = d.Picture
	u.PictureX = resolveImagePosition(d.PictureX)
	u.PictureY = resolveImagePosition(d.PictureY)
	u.Locale = d.Locale
	u.PublicProfile = d.PublicProfile
	u.Bodyweight = d.Bodyweight
	u.MFAEnabled = d.MFAEnabled
	u.MFATOTPSecret = d.MFATOTPSecret
	return u, nil
}

func scanUsers(rows pgx.Rows) ([]models.User, error) {
	defer rows.Close()
	users := []models.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// Count returns the total number of registered users, used to decide whether a
// newly registering account is the very first (and thus superadmin).
func (s *UserStore) Count(ctx context.Context) (int, error) {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// insertUser inserts a brand-new account with an already-decided role, giving
// it a unique profile handle derived from its email.
func (s *UserStore) insertUser(ctx context.Context, email, passwordHash, name, surname, locale string, role models.GlobalRole) (models.User, error) {
	handle, err := s.availableHandle(ctx, handleSeed(email, name))
	if err != nil {
		return models.User{}, err
	}

	data, err := json.Marshal(userData{
		Password: passwordHash, Name: name, Surname: surname,
		Role: string(role), Handle: handle, Locale: locale,
	})
	if err != nil {
		return models.User{}, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, data)
		VALUES ($1, $2)
		RETURNING `+userColumns, strings.TrimSpace(email), data)
	return scanUser(row)
}

// Create registers a user. The very first account ever created becomes the
// superadmin; every other account starts disabled until it's confirmed (by
// email link or by an administrator, depending on the activation mode).
func (s *UserStore) Create(ctx context.Context, email, passwordHash, name, surname, locale string) (models.User, error) {
	count, err := s.Count(ctx)
	if err != nil {
		return models.User{}, err
	}
	role := utils.If(count == 0, models.GlobalRoleSuperadmin, models.GlobalRoleDisabled)
	return s.insertUser(ctx, email, passwordHash, name, surname, locale, role)
}

// FindOrCreateOIDC logs in a user authenticated via an OIDC provider: an
// existing account is matched by email (linking it regardless of how it was
// originally created), otherwise a new one is registered with no password hash.
// The very first account still becomes superadmin; a later one is confirmed
// immediately when activationMode is "email" (the identity provider already
// verified the address, so there's nothing left to confirm), otherwise it
// starts disabled like every other mode.
func (s *UserStore) FindOrCreateOIDC(ctx context.Context, email, name, surname, activationMode string) (models.User, error) {
	user, err := s.FindByEmail(ctx, email)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return models.User{}, err
	}

	count, err := s.Count(ctx)
	if err != nil {
		return models.User{}, err
	}
	role := models.GlobalRoleDisabled
	switch {
	case count == 0:
		role = models.GlobalRoleSuperadmin
	case activationMode == models.ActivationModeEmail:
		role = models.GlobalRoleConfirmed
	}
	return s.insertUser(ctx, email, utils.EMPTY, name, surname, utils.EMPTY, role)
}

func (s *UserStore) FindByEmail(ctx context.Context, email string) (models.User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)
	`, strings.TrimSpace(email)))
}

func (s *UserStore) FindByID(ctx context.Context, id string) (models.User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// FindByHandle resolves a public profile URL's handle back to its account.
func (s *UserStore) FindByHandle(ctx context.Context, handle string) (models.User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users WHERE data->>'handle' = $1
	`, handle))
}

// FindByIDOrHandle accepts either form, so a profile link works with a handle
// and an API client can still address an account by id.
func (s *UserStore) FindByIDOrHandle(ctx context.Context, idOrHandle string) (models.User, error) {
	user, err := s.FindByHandle(ctx, idOrHandle)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return models.User{}, err
	}
	// A handle is free-form, so an id-shaped lookup would otherwise fail the
	// query with a uuid parse error rather than a not-found.
	if !isUUID(idOrHandle) {
		return models.User{}, ErrNotFound
	}
	return s.FindByID(ctx, idOrHandle)
}

// List returns every registered user, for the superadmin's management screen.
func (s *UserStore) List(ctx context.Context) ([]models.User, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	return scanUsers(rows)
}

// SearchByEmail powers the autocomplete used when adding a member to a club: it
// matches on the email, the handle and the name, so a coach can find someone
// without knowing their exact address.
func (s *UserStore) SearchByEmail(ctx context.Context, query string, limit int) ([]models.User, error) {
	pattern := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := s.pool.Query(ctx, `
		SELECT `+userColumns+` FROM users
		WHERE lower(email) LIKE $1
		   OR lower(coalesce(data->>'handle', '')) LIKE $1
		   OR lower(coalesce(data->>'name', '') || ' ' || coalesce(data->>'surname', '')) LIKE $1
		ORDER BY email
		LIMIT $2
	`, pattern, limit)
	if err != nil {
		return nil, err
	}
	return scanUsers(rows)
}

// merge applies a shallow JSONB patch to one account, so a write can never
// clobber the fields it didn't mean to touch (the password hash lives in the
// same column as the avatar).
func (s *UserStore) merge(ctx context.Context, id string, patch map[string]any) (models.User, error) {
	data, err := json.Marshal(patch)
	if err != nil {
		return models.User{}, err
	}
	row := s.pool.QueryRow(ctx, `
		UPDATE users SET data = data || $2::jsonb, updated_at = now()
		WHERE id = $1
		RETURNING `+userColumns, id, data)
	user, err := scanUser(row)
	if isUniqueViolation(err) {
		return models.User{}, ErrDuplicateHandle
	}
	return user, err
}

// ProfileFields are the profile settings a user may change about themselves.
// PasswordHash is a pointer so nil means "leave the current password alone".
type ProfileFields struct {
	Name          string
	Surname       string
	Handle        string
	Bio           string
	Locale        string
	PublicProfile bool
	Bodyweight    float64
	PasswordHash  *string
}

// UpdateProfile sets the connected user's own profile fields.
func (s *UserStore) UpdateProfile(ctx context.Context, id string, f ProfileFields) (models.User, error) {
	patch := map[string]any{
		"name":          f.Name,
		"surname":       f.Surname,
		"bio":           f.Bio,
		"publicProfile": f.PublicProfile,
		"bodyweight":    f.Bodyweight,
	}
	if utils.IsNotBlank(f.Handle) {
		patch["handle"] = f.Handle
	}
	if utils.IsNotBlank(f.Locale) {
		patch["locale"] = f.Locale
	}
	if f.PasswordHash != nil {
		patch["password"] = *f.PasswordHash
	}
	return s.merge(ctx, id, patch)
}

// UpdatePassword sets a new password hash, leaving everything else alone.
func (s *UserStore) UpdatePassword(ctx context.Context, id, passwordHash string) (models.User, error) {
	return s.merge(ctx, id, map[string]any{"password": passwordHash})
}

// UpdatePicture sets the user's avatar (base64) and its x/y display position.
func (s *UserStore) UpdatePicture(ctx context.Context, id, picture string, x, y float64) (models.User, error) {
	return s.merge(ctx, id, map[string]any{"picture": picture, "pictureX": x, "pictureY": y})
}

// SetLocale records the language the user is browsing in, so transactional
// emails go out in the same one.
func (s *UserStore) SetLocale(ctx context.Context, id, locale string) error {
	_, err := s.merge(ctx, id, map[string]any{"locale": locale})
	return err
}

// Confirm moves a disabled account to confirmed, used when a user follows their
// emailed confirmation link.
func (s *UserStore) Confirm(ctx context.Context, id string) (models.User, error) {
	return s.merge(ctx, id, map[string]any{"role": string(models.GlobalRoleConfirmed)})
}

// SetRole is the superadmin's account-role change (including promoting a member
// to coach).
func (s *UserStore) SetRole(ctx context.Context, id string, role models.GlobalRole) (models.User, error) {
	return s.merge(ctx, id, map[string]any{"role": string(role)})
}

// AdminUserFields holds the fields a superadmin may set on any account.
type AdminUserFields struct {
	Email        string
	Name         string
	Surname      string
	Role         string
	PasswordHash *string
}

// AdminUpdate lets a superadmin edit any account. It merges only the provided
// keys into the stored JSON, so omitted fields (like an unset password) are left
// untouched.
func (s *UserStore) AdminUpdate(ctx context.Context, id string, f AdminUserFields) (models.User, error) {
	patch := map[string]any{"name": f.Name, "surname": f.Surname, "role": f.Role}
	if f.PasswordHash != nil {
		patch["password"] = *f.PasswordHash
	}
	data, err := json.Marshal(patch)
	if err != nil {
		return models.User{}, err
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE users SET email = $2, data = data || $3::jsonb, updated_at = now()
		WHERE id = $1
		RETURNING `+userColumns, id, strings.TrimSpace(f.Email), data)
	user, err := scanUser(row)
	if isUniqueViolation(err) {
		return models.User{}, ErrDuplicateEmail
	}
	return user, err
}

// Delete removes an account. Everything it owns (clubs, programs, posts,
// follows) cascades with it.
func (s *UserStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- MFA ---

// SetPendingTOTPSecret stores a freshly generated TOTP secret without enabling
// MFA yet: it only takes effect once ConfirmTOTP verifies the user actually
// enrolled it, so an abandoned setup never locks anyone out.
func (s *UserStore) SetPendingTOTPSecret(ctx context.Context, id, secret string) (models.User, error) {
	return s.merge(ctx, id, map[string]any{"mfaTotpSecret": secret})
}

// ConfirmTOTP turns on MFA once the pending TOTP secret has been verified.
func (s *UserStore) ConfirmTOTP(ctx context.Context, id string) (models.User, error) {
	return s.SetMFAEnabled(ctx, id, true)
}

// DisableTOTP removes the TOTP secret and, when keepEnabled is false (no other
// factor is left), turns MFA back off entirely.
func (s *UserStore) DisableTOTP(ctx context.Context, id string, keepEnabled bool) (models.User, error) {
	return s.merge(ctx, id, map[string]any{"mfaTotpSecret": utils.EMPTY, "mfaEnabled": keepEnabled})
}

// SetMFAEnabled sets the aggregate MFA flag, used by WebAuthn
// enrollment/removal and by the superadmin's disable-MFA action.
func (s *UserStore) SetMFAEnabled(ctx context.Context, id string, enabled bool) (models.User, error) {
	return s.merge(ctx, id, map[string]any{"mfaEnabled": enabled})
}

// ClearMFA strips every MFA factor from an account - the superadmin's recovery
// path for a user who lost their phone and their security key.
func (s *UserStore) ClearMFA(ctx context.Context, id string) (models.User, error) {
	return s.merge(ctx, id, map[string]any{"mfaEnabled": false, "mfaTotpSecret": utils.EMPTY})
}

// --- handles ---

// handleSeed derives a starting profile handle from an email's local part,
// falling back to the display name and finally to a generic one, so an address
// made entirely of punctuation still yields something addressable.
func handleSeed(email, name string) string {
	local := email
	if at := strings.IndexByte(email, '@'); at > 0 {
		local = email[:at]
	}
	if seed := utils.Slugify(local); utils.IsNotBlank(seed) {
		return seed
	}
	if seed := utils.Slugify(name); utils.IsNotBlank(seed) {
		return seed
	}
	return "athlete"
}

// availableHandle returns seed, or seed with a numeric suffix when it's taken.
// The unique index on the handle is still the authority - this only avoids
// losing the race in the common case.
func (s *UserStore) availableHandle(ctx context.Context, seed string) (string, error) {
	var taken bool
	for suffix := 0; suffix < 50; suffix++ {
		candidate := seed
		if suffix > 0 {
			candidate = seed + "-" + strconv.Itoa(suffix+1)
		}
		err := s.pool.QueryRow(ctx, `
			SELECT exists(SELECT 1 FROM users WHERE data->>'handle' = $1)
		`, candidate).Scan(&taken)
		if err != nil {
			return utils.EMPTY, err
		}
		if !taken {
			return candidate, nil
		}
	}
	// Fifty collisions on the same seed: fall back to a random suffix rather
	// than looping forever.
	random, err := generateToken()
	if err != nil {
		return utils.EMPTY, err
	}
	return seed + "-" + random[:8], nil
}

// IsHandleTaken reports whether handle already belongs to another account.
func (s *UserStore) IsHandleTaken(ctx context.Context, handle, exceptUserID string) (bool, error) {
	var taken bool
	err := s.pool.QueryRow(ctx, `
		SELECT exists(SELECT 1 FROM users WHERE data->>'handle' = $1 AND id <> $2)
	`, handle, exceptUserID).Scan(&taken)
	return taken, err
}

// isUniqueViolation reports whether err is Postgres' unique-constraint error
// (23505), which is how a duplicate email or handle surfaces.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isUUID reports whether s has the shape of a uuid, so a lookup that accepts
// "an id or a handle" doesn't hand a free-form handle to a uuid column.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
