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
	// (/profile/{handle}). Assigned from the email's local part at
	// registration and editable afterwards.
	Handle   string  `json:"handle,omitempty"`
	Bio      string  `json:"bio,omitempty"`
	Picture  string  `json:"picture,omitempty"`
	PictureX float64 `json:"pictureX"`
	PictureY float64 `json:"pictureY"`
	// Locale is the language the user picked, used to send transactional
	// emails in the right one.
	Locale string `json:"locale,omitempty"`
	// PublicProfile gates whether the profile and its public posts are
	// readable without logging in.
	PublicProfile bool `json:"publicProfile"`
	// Bodyweight, in kg, shown on the public profile next to the member's
	// bests. Zero means "not stated".
	Bodyweight float64 `json:"bodyweight,omitempty"`
	// MFAEnabled is true once at least one MFA factor (TOTP or a WebAuthn
	// credential) has been confirmed. It gates password login behind a second
	// factor.
	MFAEnabled bool `json:"-"`
	// MFATOTPSecret is the base32-encoded TOTP shared secret. It's written as
	// soon as enrollment starts but MFAEnabled stays false until the user
	// confirms a code generated from it.
	MFATOTPSecret string `json:"-"`
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
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	Name          string     `json:"name"`
	Surname       string     `json:"surname"`
	Handle        string     `json:"handle,omitempty"`
	Bio           string     `json:"bio,omitempty"`
	Role          GlobalRole `json:"role"`
	Picture       string     `json:"picture,omitempty"`
	PictureX      float64    `json:"pictureX"`
	PictureY      float64    `json:"pictureY"`
	Locale        string     `json:"locale,omitempty"`
	PublicProfile bool       `json:"publicProfile"`
	Bodyweight    float64    `json:"bodyweight,omitempty"`
	MFAEnabled    bool       `json:"mfaEnabled"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	I18nCode      string     `json:"i18nCode,omitempty"`
}

// PublicProfile is the outward-facing profile of a member or coach - never
// carries the email or any account setting, since it's readable by anyone
// (including logged-out visitors) when the owner opted in.
type PublicProfile struct {
	ID         string     `json:"id"`
	Handle     string     `json:"handle,omitempty"`
	Name       string     `json:"name"`
	Surname    string     `json:"surname"`
	Role       GlobalRole `json:"role"`
	Bio        string     `json:"bio,omitempty"`
	Picture    string     `json:"picture,omitempty"`
	PictureX   float64    `json:"pictureX"`
	PictureY   float64    `json:"pictureY"`
	Bodyweight float64    `json:"bodyweight,omitempty"`
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
