package models

import "time"

// ApiKey lets a member authenticate a script - or the mobile app, enrolled by
// scanning a QR code - with the X-Api-Key header instead of a JWT. KeyHash is
// the sha256 of the plaintext token; the plaintext only ever exists in the
// response to the call that created it (see ApiKeyCreated).
type ApiKey struct {
	ID          string     `json:"id"`
	UserID      string     `json:"userId"`
	KeyHash     string     `json:"-"`
	Description string     `json:"description"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// ApiKeyCreated is returned only from the create endpoint: it is the one and
// only time Token is available, so a client that means to keep it has to keep
// it now.
type ApiKeyCreated struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	Token       string     `json:"token"`
}
