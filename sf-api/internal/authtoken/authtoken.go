// Package authtoken mints the session JWT shared by password login,
// registration and OIDC login, so all three issue tokens middleware.Auth can
// verify the same way.
package authtoken

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"strong-fish-api/internal/utils"
)

// sessionTTL is how long a session token stays valid.
const sessionTTL = 20 * 24 * time.Hour

// Generate signs a session token for userID.
func Generate(secret, userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(sessionTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// Purpose-scoped token kinds minted by GeneratePurpose - each carries a
// "purpose" claim so a confirmation or password-reset link can never be
// replayed as a session token (or vice versa) even though they share the same
// signing secret.
const (
	PurposeConfirmAccount = "confirm_account"
	PurposeResetPassword  = "reset_password"
	// PurposeMFALogin scopes the short-lived token handed back by Login when
	// the password was correct but the account has MFA enabled - it authorizes
	// finishing the login via /v1/users/login/mfa/*, nothing else.
	PurposeMFALogin = "mfa_login"
	// PurposeWebAuthnCeremony scopes a token carrying a JSON-encoded
	// webauthn.SessionData between a WebAuthn "begin" and "finish" call. Using
	// a signed token instead of server-side session storage keeps every MFA
	// endpoint stateless.
	PurposeWebAuthnCeremony = "webauthn_ceremony"
)

// ErrInvalidPurposeToken is returned by ParsePurpose for a token that is
// missing, expired, tampered with, or minted for a different purpose.
var ErrInvalidPurposeToken = errors.New("invalid or expired token")

// GeneratePurpose signs a purpose-scoped token for userID, valid for ttl.
func GeneratePurpose(secret, userID, purpose string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":     userID,
		"purpose": purpose,
		"exp":     time.Now().Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParsePurpose verifies a purpose-scoped token minted by GeneratePurpose and
// returns the user id it was issued for.
func ParsePurpose(secret, tokenString, purpose string) (string, error) {
	return ParsePurposeWithData(secret, tokenString, purpose, nil)
}

// GeneratePurposeWithData is GeneratePurpose plus an arbitrary JSON-able
// payload carried in a "data" claim - used for PurposeWebAuthnCeremony, which
// needs to round-trip a whole webauthn.SessionData between "begin" and
// "finish".
func GeneratePurposeWithData(secret, userID, purpose string, ttl time.Duration, data any) (string, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return utils.EMPTY, err
	}
	claims := jwt.MapClaims{
		"sub":     userID,
		"purpose": purpose,
		"data":    string(raw),
		"exp":     time.Now().Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParsePurposeWithData is ParsePurpose plus decoding the "data" claim set by
// GeneratePurposeWithData into dataOut (nil to ignore it).
func ParsePurposeWithData(secret, tokenString, purpose string, dataOut any) (string, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return utils.EMPTY, ErrInvalidPurposeToken
	}
	if p, _ := claims["purpose"].(string); p != purpose {
		return utils.EMPTY, ErrInvalidPurposeToken
	}
	sub, ok := claims["sub"].(string)
	if !ok || utils.IsBlank(sub) {
		return utils.EMPTY, ErrInvalidPurposeToken
	}
	if raw, _ := claims["data"].(string); dataOut != nil && utils.IsNotBlank(raw) {
		if err := json.Unmarshal([]byte(raw), dataOut); err != nil {
			return utils.EMPTY, ErrInvalidPurposeToken
		}
	}
	return sub, nil
}
