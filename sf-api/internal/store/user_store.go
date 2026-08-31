package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
	Surname  string `json:"surname,omitempty"`
	Role     string `json:"role,omitempty"`
	Handle   string `json:"handle,omitempty"`
	// Username is the name the member picked; Anonymous makes it the only one
	// anybody else sees (see display_name.go).
	Username  string   `json:"username,omitempty"`
	Anonymous bool     `json:"anonymous,omitempty"`
	Bio       string   `json:"bio,omitempty"`
	Picture   string   `json:"picture,omitempty"`
	PictureX  *float64 `json:"pictureX,omitempty"`
	PictureY  *float64 `json:"pictureY,omitempty"`
	Locale    string   `json:"locale,omitempty"`
	// PublicProfile is the boolean ProfileVisibility replaced. It is still read
	// so an account the V5 migration hasn't touched still resolves to something
	// sensible, and never written.
	PublicProfile     bool                  `json:"publicProfile,omitempty"`
	ProfileVisibility string                `json:"profileVisibility,omitempty"`
	Birthdate         string                `json:"birthdate,omitempty"`
	Gender            string                `json:"gender,omitempty"`
	CoachRequest      *models.CoachRequest  `json:"coachRequest,omitempty"`
	IPs               []models.ConnectionIP `json:"ips,omitempty"`
	Bodyweight        float64               `json:"bodyweight,omitempty"`
	Socials           *models.Socials       `json:"socials,omitempty"`
	MFAEnabled        bool                  `json:"mfaEnabled,omitempty"`
	MFATOTPSecret     string                `json:"mfaTotpSecret,omitempty"`
	// CalendarFeed* back the ICS subscription: they live in the payload like
	// everything else here, and are never sent out with the user. (The
	// storage connection used to sit beside them; V12 moved it to its own
	// table, where it can be shared.)
	CalendarFeedEnabled bool   `json:"calendarFeedEnabled,omitempty"`
	CalendarFeedToken   string `json:"calendarFeedToken,omitempty"`
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
	u.Username = d.Username
	u.Anonymous = d.Anonymous
	u.Bio = d.Bio
	u.Picture = d.Picture
	u.PictureX = resolveImagePosition(d.PictureX)
	u.PictureY = resolveImagePosition(d.PictureY)
	u.Locale = d.Locale
	u.ProfileVisibility = d.ProfileVisibility
	if utils.IsBlank(u.ProfileVisibility) {
		// Pre-V5 rows: the old boolean is the only statement of intent there
		// is, and it maps exactly - true meant "anybody", false meant "only me
		// and a superadmin".
		u.ProfileVisibility = models.ProfileVisibilityPrivate
		if d.PublicProfile {
			u.ProfileVisibility = models.ProfileVisibilityPublic
		}
	}
	u.ProfileVisibility = models.NormalizeProfileVisibility(u.ProfileVisibility)
	u.Birthdate = d.Birthdate
	if d.CoachRequest != nil {
		u.CoachRequest = *d.CoachRequest
	}
	u.IPs = d.IPs
	u.Bodyweight = d.Bodyweight
	u.Gender = d.Gender
	if d.Socials != nil {
		u.Socials = models.NormalizeSocials(*d.Socials)
	}
	u.MFAEnabled = d.MFAEnabled
	u.MFATOTPSecret = d.MFATOTPSecret
	u.CalendarFeedEnabled = d.CalendarFeedEnabled
	u.CalendarFeedToken = d.CalendarFeedToken
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
func (s *UserStore) insertUser(ctx context.Context, account NewAccount, role models.GlobalRole) (models.User, error) {
	// The same precedence the handle follows for the rest of the account's
	// life (see deriveHandle): the username when there is one, the name
	// otherwise. Both halves of the name, so the first profile save does not
	// rename a handle seeded from the first name alone.
	seed := account.Username
	if utils.IsBlank(seed) {
		seed = strings.TrimSpace(account.Name + " " + account.Surname)
	}
	handle, err := s.availableHandle(ctx, handleSeed(account.Email, seed))
	if err != nil {
		return models.User{}, err
	}

	// Somebody who registered under a username alone has no other name to be
	// shown by, which is what anonymous means here.
	anonymous := utils.IsNotBlank(account.Username) &&
		(utils.IsBlank(account.Name) || utils.IsBlank(account.Surname))

	data, err := json.Marshal(userData{
		Password: account.PasswordHash, Name: account.Name, Surname: account.Surname,
		Username: account.Username, Anonymous: anonymous,
		Role: string(role), Handle: handle, Locale: account.Locale,
	})
	if err != nil {
		return models.User{}, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, data)
		VALUES ($1, $2)
		RETURNING `+userColumns, strings.TrimSpace(account.Email), data)
	return scanUser(row)
}

// NewAccount is what registering needs: an address to reach somebody at, a way
// for them to sign in, and something to call them - a name, or the username
// they picked instead of one.
type NewAccount struct {
	Email        string
	PasswordHash string
	Name         string
	Surname      string
	Username     string
	Locale       string
}

// Create registers a user. The very first account ever created becomes the
// superadmin; every other account starts disabled until it's confirmed (by
// email link or by an administrator, depending on the activation mode).
func (s *UserStore) Create(ctx context.Context, account NewAccount) (models.User, error) {
	count, err := s.Count(ctx)
	if err != nil {
		return models.User{}, err
	}
	role := utils.If(count == 0, models.GlobalRoleSuperadmin, models.GlobalRoleDisabled)
	return s.insertUser(ctx, account, role)
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
	// An identity provider gives a name, never a username of ours.
	return s.insertUser(ctx, NewAccount{Email: email, Name: name, Surname: surname}, role)
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

// SearchByEmail powers the autocomplete used when adding a member to a club:
// it matches the email, the handle and the shown name, so a coach can find
// somebody without knowing their exact address.
//
// An anonymized account is findable by its username alone, for the same reason
// as in SearchMembers: matching a real name or an address is how you learn who
// a username belongs to. Such a member can still be invited, which is
// addressed to the email directly rather than searched for.
func (s *UserStore) SearchByEmail(ctx context.Context, query string, limit int) ([]models.User, error) {
	pattern := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := s.pool.Query(ctx, `
		SELECT `+userColumns+` FROM users u
		WHERE lower(coalesce(u.data->>'handle', '')) LIKE $1
		   OR lower(`+displayFullName("u")+`) LIKE $1
		   OR (coalesce(u.data->>'anonymous', 'false') <> 'true' AND lower(u.email) LIKE $1)
		ORDER BY u.email
		LIMIT $2
	`, pattern, limit)
	if err != nil {
		return nil, err
	}
	return scanUsers(rows)
}

// MemberSearch is what a caller is looking for. Every criterion is optional and
// they are combined with AND, the way uprodit's own search composes its query
// parameters - Terms is the free-text box that matches any of the three.
type MemberSearch struct {
	Terms    string
	Name     string
	Surname  string
	Username string
	Email    string
	Page     int
	Size     int
}

// blank reports whether the search has nothing to go on. An empty search must
// not enumerate the whole membership.
// SearchMembers finds accounts by email, name or surname, returning only the
// profiles callerID is allowed to see.
//
// The visibility predicate is in the query rather than applied to the results,
// for two reasons: a caller-side filter would make the page counts wrong (a
// page of 20 could come back with 3), and it would put the enforcement in
// whichever handler remembered to do it rather than in the one place every
// search goes through.
//
// An account that cannot sign in - disabled or banned - never appears: a
// pending registration is not a member yet, and a banned one is not one any
// more.
func (s *UserStore) SearchMembers(ctx context.Context, m MemberSearch, callerID string, superadmin bool) ([]models.User, int, error) {
	// No criteria is a browse, not an error: the screen opens on everybody the
	// caller may see, ordered by name, and typing narrows it. The visibility
	// predicate below is what makes that safe - an empty search still cannot
	// return a profile its owner hid.
	size := m.Size
	if size <= 0 || size > 100 {
		size = 20
	}
	page := m.Page
	if page < 0 {
		page = 0
	}

	// $1 caller, $2 superadmin, then one parameter per supplied criterion.
	args := []any{callerID, superadmin}
	where := []string{`data->>'role' NOT IN ('disabled', 'ban')`}

	like := func(expression, value string) {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(value))+"%")
		where = append(where, fmt.Sprintf("%s LIKE $%d", expression, len(args)))
	}

	// Anonymity has to hold here too, and it takes more than hiding the name in
	// the results: a search that *matches* on somebody's real name tells you
	// who a username belongs to as surely as printing it would. So an
	// anonymized account is findable only by the name it chose to show, and
	// its email is off the table as well - an address identifies a person just
	// as precisely.
	//
	// The consequence is deliberate: a coach cannot look an anonymized member
	// up by email to add them to a club. Inviting them still works, because an
	// invitation is addressed to the email without searching for it first.
	const notAnonymous = `coalesce(data->>'anonymous', 'false') <> 'true'`
	shown := `lower(` + displayFullName("users") + `)`

	// The username is matched for everybody, anonymized or not - unlike the
	// real name and the email, which are hidden for an anonymized account. It
	// is the name they chose to be known by and the handle is derived from it,
	// so matching on it reveals nothing the handle does not already.
	//
	// It is worth its own clause rather than relying on the handle: the handle
	// is a slug of the username, so "Marie.D" is searchable as "marie-d" and
	// not as what its owner actually typed.
	const username = `lower(coalesce(data->>'username', ''))`

	if utils.IsNotBlank(m.Terms) {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(m.Terms))+"%")
		where = append(where, fmt.Sprintf(
			"(%s LIKE $%d OR %s LIKE $%d OR lower(coalesce(data->>'handle', '')) LIKE $%d"+
				" OR (%s AND lower(email) LIKE $%d))",
			shown, len(args), username, len(args), len(args), notAnonymous, len(args)))
	}
	if utils.IsNotBlank(m.Username) {
		like(username, m.Username)
	}
	if utils.IsNotBlank(m.Name) {
		like(`lower(`+displayName("users")+`)`, m.Name)
	}
	if utils.IsNotBlank(m.Surname) {
		// An anonymized member has no surname to match, so this criterion
		// simply never selects one - which is the correct answer, not a gap.
		like(`lower(`+displaySurname("users")+`)`, m.Surname)
	}
	if utils.IsNotBlank(m.Email) {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(m.Email))+"%")
		where = append(where, fmt.Sprintf("(%s AND lower(email) LIKE $%d)", notAnonymous, len(args)))
	}

	// The visibility rules, as SQL. "clubs" needs a shared club; "private"
	// needs the caller to manage one. A row with no stored visibility is
	// treated as private, matching NormalizeProfileVisibility.
	where = append(where, `(
		$2
		OR users.id::text = $1
		OR data->>'profileVisibility' = 'public'
		OR (
			data->>'profileVisibility' = 'clubs'
			AND EXISTS (
				SELECT 1 FROM club_members target
				JOIN club_members caller ON caller.club_id = target.club_id AND caller.user_id::text = $1
				WHERE target.user_id = users.id
			)
		)
		OR (
			coalesce(data->>'profileVisibility', 'private') NOT IN ('public', 'clubs')
			AND EXISTS (
				SELECT 1 FROM club_members target
				JOIN club_members caller ON caller.club_id = target.club_id AND caller.user_id::text = $1
				WHERE target.user_id = users.id AND caller.role IN ('owner', 'admin')
			)
		)
	)`)

	predicate := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE `+predicate, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []models.User{}, 0, nil
	}

	args = append(args, size, page*size)
	rows, err := s.pool.Query(ctx, `
		SELECT `+userColumns+` FROM users
		WHERE `+predicate+`
		ORDER BY `+displayName("users")+`, `+displaySurname("users")+`, email
		LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	users, err := scanUsers(rows)
	return users, total, err
}

// ListByIDs loads several accounts at once, for the places that already know
// which people they need - the calendar's birthday entries, for one.
func (s *UserStore) ListByIDs(ctx context.Context, ids []string) ([]models.User, error) {
	if len(ids) == 0 {
		return []models.User{}, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT `+userColumns+` FROM users WHERE id = ANY($1)`, ids)
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
	Name    string
	Surname string
	// Username is what the member is known by when set, and what the handle is
	// derived from. Blank clears it, and the handle falls back to the name.
	Username          string
	Anonymous         bool
	Bio               string
	Locale            string
	ProfileVisibility string
	Birthdate         string
	Bodyweight        float64
	// Gender is "male" or "female", and drives the strength coefficients.
	Gender string
	// Socials replaces the stored accounts wholesale, so a form that sends a
	// field blank clears it.
	Socials      models.Socials
	PasswordHash *string
}

// UpdateProfile sets the connected user's own profile fields.
//
// The handle is derived here rather than accepted from the caller: it is the
// member's username when they picked one, and their name otherwise. That is
// what makes "@..." follow the username - there is no second field to keep in
// step with it, and no way for the two to disagree.
func (s *UserStore) UpdateProfile(ctx context.Context, id string, f ProfileFields) (models.User, error) {
	handle, err := s.deriveHandle(ctx, id, f)
	if err != nil {
		return models.User{}, err
	}

	patch := map[string]any{
		"name":              f.Name,
		"surname":           f.Surname,
		"username":          strings.TrimSpace(f.Username),
		"anonymous":         f.Anonymous,
		"handle":            handle,
		"bio":               f.Bio,
		"profileVisibility": models.NormalizeProfileVisibility(f.ProfileVisibility),
		"birthdate":         f.Birthdate,
		"bodyweight":        f.Bodyweight,
		"gender":            f.Gender,
		// Written as a whole object rather than field by field: the payload
		// merge is shallow, so this key replaces what was there, which is what
		// makes clearing one account work.
		"socials": models.NormalizeSocials(f.Socials),
	}
	if utils.IsNotBlank(f.Locale) {
		patch["locale"] = f.Locale
	}
	if f.PasswordHash != nil {
		patch["password"] = *f.PasswordHash
	}
	return s.merge(ctx, id, patch)
}

// deriveHandle works out the handle a profile should have after this update:
// the slugified username when one is set, the name and surname otherwise.
//
// The account's own current handle is never treated as taken - re-saving a
// profile without changing anything must not walk the handle to "marie-2".
func (s *UserStore) deriveHandle(ctx context.Context, id string, f ProfileFields) (string, error) {
	seed := utils.Slugify(f.Username)
	if utils.IsBlank(seed) {
		seed = utils.Slugify(strings.TrimSpace(f.Name + " " + f.Surname))
	}
	if utils.IsBlank(seed) {
		// Nothing usable in either: keep whatever the account already has
		// rather than renaming it to a placeholder.
		user, err := s.FindByID(ctx, id)
		if err != nil {
			return utils.EMPTY, err
		}
		return user.Handle, nil
	}
	return s.availableHandleFor(ctx, seed, id)
}

// UsernameTaken reports whether another account already uses this username.
// Case-insensitive, matching the unique index: two names that read the same to
// a person must not both exist.
func (s *UserStore) UsernameTaken(ctx context.Context, username, exceptID string) (bool, error) {
	if utils.IsBlank(username) {
		return false, nil
	}
	// Registration has no account to exclude yet, and an empty string is not a
	// uuid: passed as NULL rather than as "", which the cast used to choke on.
	var except any
	if utils.IsNotBlank(exceptID) {
		except = exceptID
	}

	var taken bool
	err := s.pool.QueryRow(ctx, `
		SELECT exists(SELECT 1 FROM users
		              WHERE lower(data->>'username') = lower($1)
		                AND ($2::uuid IS NULL OR id <> $2::uuid))
	`, strings.TrimSpace(username), except).Scan(&taken)
	return taken, err
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
// handleSeed picks the starting handle for a new account.
//
// The name comes first, because that is the rule the handle follows for the
// rest of the account's life: the username when one is set, the name
// otherwise (see UpdateProfile). Seeding from the email instead would give a
// new member a handle that changes the first time they save their profile.
//
// The email's local part is the fallback for a name with no usable letters.
func handleSeed(email, name string) string {
	if seed := utils.Slugify(name); utils.IsNotBlank(seed) {
		return seed
	}

	local := email
	if at := strings.IndexByte(email, '@'); at > 0 {
		local = email[:at]
	}
	if seed := utils.Slugify(local); utils.IsNotBlank(seed) {
		return seed
	}
	return "athlete"
}

// availableHandle returns seed, or seed with a numeric suffix when it's taken.
// The unique index on the handle is still the authority - this only avoids
// losing the race in the common case.
func (s *UserStore) availableHandle(ctx context.Context, seed string) (string, error) {
	return s.availableHandleFor(ctx, seed, utils.EMPTY)
}

// availableHandleFor is availableHandle for an account that already exists:
// exceptID's own handle doesn't count as taken, so re-saving a profile leaves
// the handle where it was instead of walking it to "marie-2".
func (s *UserStore) availableHandleFor(ctx context.Context, seed, exceptID string) (string, error) {
	var taken bool
	for suffix := 0; suffix < 50; suffix++ {
		candidate := seed
		if suffix > 0 {
			candidate = seed + "-" + strconv.Itoa(suffix+1)
		}
		err := s.pool.QueryRow(ctx, `
			SELECT exists(SELECT 1 FROM users
			              WHERE data->>'handle' = $1 AND ($2 = '' OR id <> $2::uuid))
		`, candidate, exceptID).Scan(&taken)
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

// --- storage connection and calendar feed ---

// SetCalendarFeedEnabled turns the ICS subscription on or off, minting a token
// the first time it's enabled. Disabling deliberately keeps the token, so
// somebody who turns the feed back on doesn't have to re-subscribe in Outlook;
// RegenerateCalendarFeedToken is how a leaked URL is actually revoked.
func (s *UserStore) SetCalendarFeedEnabled(ctx context.Context, id string, enabled bool) (models.User, error) {
	patch := map[string]any{"calendarFeedEnabled": enabled}
	if enabled {
		user, err := s.FindByID(ctx, id)
		if err != nil {
			return models.User{}, err
		}
		if utils.IsBlank(user.CalendarFeedToken) {
			token, err := generateToken()
			if err != nil {
				return models.User{}, err
			}
			patch["calendarFeedToken"] = token
		}
	}
	return s.merge(ctx, id, patch)
}

// RegenerateCalendarFeedToken mints a new token, which breaks every calendar
// already subscribed to the old URL. That is the point: it is the only way to
// take back a feed URL that got out.
func (s *UserStore) RegenerateCalendarFeedToken(ctx context.Context, id string) (models.User, error) {
	token, err := generateToken()
	if err != nil {
		return models.User{}, err
	}
	return s.merge(ctx, id, map[string]any{"calendarFeedToken": token, "calendarFeedEnabled": true})
}

// FindByCalendarFeedToken resolves the owner of a feed URL. A blank token is
// rejected outright rather than matching every account that never enabled the
// feed.
func (s *UserStore) FindByCalendarFeedToken(ctx context.Context, token string) (models.User, error) {
	if utils.IsBlank(token) {
		return models.User{}, ErrNotFound
	}
	return scanUser(s.pool.QueryRow(ctx, `
		SELECT `+userColumns+` FROM users
		WHERE data->>'calendarFeedToken' = $1 AND data->>'calendarFeedEnabled' = 'true'
	`, token))
}

// CountByRole counts accounts per global role, for the users gauge. One query
// rather than one per role, so a scrape stays cheap as roles are added.
func (s *UserStore) CountByRole(ctx context.Context) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT coalesce(data->>'role', 'unknown'), count(*) FROM users GROUP BY 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int64{}
	for rows.Next() {
		var role string
		var count int64
		if err := rows.Scan(&role, &count); err != nil {
			return nil, err
		}
		counts[role] = count
	}
	return counts, rows.Err()
}

// RecordConnection notes that userID connected from ip: a new address is
// appended, a known one has its counter bumped and its lastSeen refreshed.
//
// It reads the list, edits it in Go and writes it back, rather than doing the
// same in a jsonb_set expression. That is a deliberate trade: the SQL would be
// one statement but genuinely unreadable, and the race it avoids - two
// simultaneous logins from the same account - costs at worst one lost tick of a
// counter that exists to be looked at, not to be reconciled.
//
// The list is bounded (models.MaxConnectionIPs): an account connecting from a
// new address every time would otherwise grow its own row without limit, so
// past the cap the least recently seen address is dropped.
func (s *UserStore) RecordConnection(ctx context.Context, userID, ip string, at time.Time) error {
	if utils.IsBlank(userID) || utils.IsBlank(ip) {
		return nil
	}

	user, err := s.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	ips := user.IPs
	found := false
	for i := range ips {
		if ips[i].IP == ip {
			ips[i].Count++
			ips[i].LastSeen = at
			found = true
			break
		}
	}
	if !found {
		ips = append(ips, models.ConnectionIP{IP: ip, Count: 1, FirstSeen: at, LastSeen: at})
	}

	sort.Slice(ips, func(i, j int) bool { return ips[i].LastSeen.After(ips[j].LastSeen) })
	if len(ips) > models.MaxConnectionIPs {
		ips = ips[:models.MaxConnectionIPs]
	}

	_, err = s.merge(ctx, userID, map[string]any{"ips": ips})
	return err
}
