package models

import "time"

// GlobalRole is the account-wide role, separate from a user's per-club Role
// (owner/admin/member).
type GlobalRole string

const (
	// GlobalRoleSuperadmin can moderate every post and comment, manage every
	// account and reach into any club.
	GlobalRoleSuperadmin GlobalRole = "superadmin"
	// GlobalRoleCoach can create clubs, upload programs and add exercises to
	// the shared catalog. A coach owns the clubs they create.
	GlobalRoleCoach GlobalRole = "coach"
	// GlobalRoleConfirmed is a plain athlete: they join clubs, run the
	// programs assigned to them and post.
	GlobalRoleConfirmed GlobalRole = "confirmed"
	// GlobalRoleDisabled is a freshly registered account still waiting on its
	// confirmation link (or on an administrator, depending on the activation
	// mode).
	GlobalRoleDisabled GlobalRole = "disabled"
	// GlobalRoleBan is like GlobalRoleDisabled except it's a deliberate
	// administrator action rather than a pending-approval state, so it carries
	// its own i18n code and can never be lifted by a confirmation link or a
	// password renewal.
	GlobalRoleBan GlobalRole = "ban"
)

// IsValidGlobalRole reports whether role is one a superadmin may assign.
func IsValidGlobalRole(role GlobalRole) bool {
	switch role {
	case GlobalRoleSuperadmin, GlobalRoleCoach, GlobalRoleConfirmed, GlobalRoleDisabled, GlobalRoleBan:
		return true
	}
	return false
}

// CanCoach reports whether role may create clubs, upload programs and extend
// the exercise catalog. A superadmin can do everything a coach can.
func CanCoach(role GlobalRole) bool {
	return role == GlobalRoleCoach || role == GlobalRoleSuperadmin
}

// IsActive reports whether role may do anything beyond logging in and reading
// its own status.
func IsActive(role GlobalRole) bool {
	return role != GlobalRoleDisabled && role != GlobalRoleBan
}

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Surname      string     `json:"surname"`
	Role         GlobalRole `json:"role"`
	PasswordHash string     `json:"-"`
	// Handle is the short, unique name the public profile is addressed by
	// (/profile/{handle}). It is derived, never set directly: the member's
	// Username when they picked one, otherwise their name and surname. See
	// UserStore.UpdateProfile.
	Handle string `json:"handle,omitempty"`
	// Username is the name the member chose for themselves, unique across the
	// app and optional. It is what an anonymized profile is known by.
	Username string `json:"username,omitempty"`
	// Anonymous hides the real name behind the username, everywhere the
	// profile surfaces - a post's author, a message's sender, a club's member
	// list (see the store's display_name.go).
	Anonymous bool    `json:"anonymous"`
	Bio       string  `json:"bio,omitempty"`
	Picture   string  `json:"picture,omitempty"`
	PictureX  float64 `json:"pictureX"`
	PictureY  float64 `json:"pictureY"`
	// Locale is the language the user picked, used to send transactional
	// emails in the right one.
	Locale string `json:"locale,omitempty"`
	// ProfileVisibility gates who may read the profile and its posts: everybody,
	// the members of the owner's clubs, or only their coaches (see
	// CanSeeProfile).
	ProfileVisibility string `json:"profileVisibility"`
	// Birthdate is optional, stored as YYYY-MM-DD. When set it becomes a
	// calendar entry for the people who may see the profile.
	Birthdate string `json:"birthdate,omitempty"`
	// Bodyweight, in kg, shown on the public profile next to the member's
	// bests. Zero means "not stated".
	Bodyweight float64 `json:"bodyweight,omitempty"`
	// Specialty is the lift the member says is theirs - or that none is (see
	// specialty.go). Empty means they have not picked one.
	Specialty string `json:"specialty,omitempty"`
	// Socials are the accounts the member chose to show, stored as names on
	// their service rather than as URLs (see socials.go).
	Socials Socials `json:"socials,omitempty"`
	// MFAEnabled is true once at least one MFA factor (TOTP or a WebAuthn
	// credential) has been confirmed. It gates password login behind a second
	// factor.
	MFAEnabled bool `json:"-"`
	// MFATOTPSecret is the base32-encoded TOTP shared secret. It's written as
	// soon as enrollment starts but MFAEnabled stays false until the user
	// confirms a code generated from it.
	MFATOTPSecret string `json:"-"`
	// IPs are the addresses this account has connected from, with a hit count
	// each. Administrative data, never serialized with the user.
	IPs []ConnectionIP `json:"-"`
	// CoachRequest is set when the account asked to be a coach at signup. The
	// role is not granted by asking: it waits on a superadmin's decision.
	CoachRequest CoachRequest `json:"-"`
	// Storage is where this member's uploaded videos go. It holds live
	// credentials, so it is never serialized with the user - the account
	// screen reads it back through its own endpoint, redacted.
	Storage StorageConnection `json:"-"`
	// CalendarFeedEnabled and CalendarFeedToken back the ICS subscription.
	// The token is the whole credential for an unauthenticated poll by Outlook
	// or Google Calendar, so it is never serialized either.
	CalendarFeedEnabled bool      `json:"-"`
	CalendarFeedToken   string    `json:"-"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// UserResponse is what a successful login/registration returns.
type UserResponse struct {
	ID       string     `json:"id"`
	Email    string     `json:"email"`
	Name     string     `json:"name"`
	Surname  string     `json:"surname"`
	Handle   string     `json:"handle,omitempty"`
	Role     GlobalRole `json:"role"`
	Token    string     `json:"token"`
	Picture  string     `json:"picture,omitempty"`
	PictureX float64    `json:"pictureX"`
	PictureY float64    `json:"pictureY"`
	// I18nCode is set when Role is disabled or ban, so the frontend can
	// display the right explanation without hardcoding the server's activation
	// mode (see I18nCodeForRole).
	I18nCode string `json:"i18nCode,omitempty"`
}

// MFAChallengeResponse replaces UserResponse when password login succeeds but
// the account has MFA enabled: no session Token is issued yet, only a
// short-lived ChallengeToken the client exchanges for one via
// /v1/users/login/mfa/* once the second factor is verified.
type MFAChallengeResponse struct {
	MFARequired    bool   `json:"mfaRequired"`
	ChallengeToken string `json:"challengeToken"`
	HasTOTP        bool   `json:"hasTotp"`
	HasWebAuthn    bool   `json:"hasWebAuthn"`
}

// UserMeResponse is the connected user's own account, including the settings
// only they can see.
type UserMeResponse struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
	// Handle is derived from Username, or from the name when there is none.
	Handle string `json:"handle,omitempty"`
	// Username and Anonymous are the account's own view of its choices, so
	// they are the real stored values rather than the anonymized projection -
	// this is the screen where they are edited.
	Username          string     `json:"username,omitempty"`
	Anonymous         bool       `json:"anonymous"`
	Bio               string     `json:"bio,omitempty"`
	Role              GlobalRole `json:"role"`
	Picture           string     `json:"picture,omitempty"`
	PictureX          float64    `json:"pictureX"`
	PictureY          float64    `json:"pictureY"`
	Locale            string     `json:"locale,omitempty"`
	ProfileVisibility string     `json:"profileVisibility"`
	Birthdate         string     `json:"birthdate,omitempty"`
	// CoachRequest is the account's own view of its coach application: whether
	// it is still waiting, and - if it was turned down - why.
	CoachRequest CoachRequest `json:"coachRequest,omitempty"`
	Bodyweight   float64      `json:"bodyweight,omitempty"`
	// Specialty is sent even when empty: this is the account's own view, and
	// the form needs to be able to tell "none picked" from "not in the
	// response at all".
	Specialty string `json:"specialty"`
	// Socials, likewise, is the form's own view of what it will edit.
	Socials    Socials   `json:"socials"`
	MFAEnabled bool      `json:"mfaEnabled"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	I18nCode   string    `json:"i18nCode,omitempty"`
}

// PublicProfile is the outward-facing profile of a member or coach - never
// carries the email or any account setting, since it's readable by anyone
// (including logged-out visitors) when the owner opted in.
type PublicProfile struct {
	ID      string     `json:"id"`
	Handle  string     `json:"handle,omitempty"`
	Name    string     `json:"name"`
	Surname string     `json:"surname"`
	Role    GlobalRole `json:"role"`
	// Anonymous says the name above is a username, not a person's name.
	Anonymous  bool    `json:"anonymous,omitempty"`
	Bio        string  `json:"bio,omitempty"`
	Picture    string  `json:"picture,omitempty"`
	PictureX   float64 `json:"pictureX"`
	PictureY   float64 `json:"pictureY"`
	Bodyweight float64 `json:"bodyweight,omitempty"`
	// Specialty is the badge the member picked for themselves, absent when
	// they picked none.
	Specialty string `json:"specialty,omitempty"`
	// Socials are the accounts to link to from the profile; absent when the
	// member filled none in.
	Socials Socials `json:"socials,omitempty"`
	// Birthdate is only ever on a profile the caller was already allowed to
	// read, which is the same gate that decides whether they see the birthday
	// in their calendar.
	Birthdate string `json:"birthdate,omitempty"`
	// Bests are the member's current 1RMs on the three competition lifts, plus
	// the total, shown as the headline of a powerlifting profile.
	Bests     []ProfileBest `json:"bests"`
	Total     float64       `json:"total"`
	Followers int           `json:"followers"`
	Following int           `json:"following"`
	// Followed reports whether the *caller* follows this profile; false for an
	// anonymous visitor.
	Followed  bool          `json:"followed"`
	Clubs     []ProfileClub `json:"clubs"`
	CreatedAt time.Time     `json:"createdAt"`
}

// ProfileBest is one lift's current 1RM on a public profile.
type ProfileBest struct {
	ExerciseID string            `json:"exerciseId"`
	Slug       string            `json:"slug"`
	Labels     map[string]string `json:"labels"`
	Value      float64           `json:"value"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

// ProfileClub is a club membership as shown on a public profile.
type ProfileClub struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role Role   `json:"role"`
}

// DisplayName is the pair of names shown for a user: their own, or their
// username alone when they chose to be anonymous.
//
// It mirrors the SQL in the store's display_name.go, which is what most of the
// app goes through - this is for the handful of projections built from a
// loaded User instead of from a query. The two must agree; changing one means
// changing the other.
func (u User) DisplayName() (name, surname string) {
	if !u.Anonymous {
		return u.Name, u.Surname
	}
	if u.Username != "" {
		return u.Username, ""
	}
	// Anonymous with no username: the handle is all that is left, and showing
	// it beats showing nothing.
	return u.Handle, ""
}

// UserSummary is the author/member shape embedded in posts, comments, club
// member lists and search results.
type UserSummary struct {
	ID       string     `json:"id"`
	Handle   string     `json:"handle,omitempty"`
	Name     string     `json:"name"`
	Surname  string     `json:"surname"`
	Email    string     `json:"email,omitempty"`
	Role     GlobalRole `json:"role"`
	Picture  string     `json:"picture,omitempty"`
	PictureX float64    `json:"pictureX"`
	PictureY float64    `json:"pictureY"`
	// Anonymous tells a client this is a username rather than a real name, so
	// it can say so rather than rendering it as somebody's first name.
	Anonymous bool `json:"anonymous,omitempty"`
}

// WebAuthnCredential is a hardware or platform security key (e.g. a YubiKey)
// registered as an MFA factor. CredentialID, PublicKey and SignCount are never
// serialized to the client - they're only used server-side to verify a login
// assertion.
type WebAuthnCredential struct {
	ID           string    `json:"id"`
	UserID       string    `json:"-"`
	CredentialID []byte    `json:"-"`
	PublicKey    []byte    `json:"-"`
	SignCount    uint32    `json:"-"`
	Transports   []string  `json:"transports,omitempty"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
