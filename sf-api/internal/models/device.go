package models

import "time"

// DeviceLink is a member's grant to an outside account that counts something
// for them - the steps their watch already recorded, today through Google
// Health.
//
// The tokens are on this struct because the store and the handler need them to
// call the provider. They are never serialized: a link reaches a client as
// DeviceConnection, which carries no credential at all.
type DeviceLink struct {
	ID       string
	UserID   string
	Provider string

	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time

	// Scope is what the member actually granted, which can be less than what
	// was asked for - Google's consent screen lets them untick.
	Scope string
	// ExternalUserID is who the provider knows the member as, kept so a
	// support question about somebody's data has an answer.
	ExternalUserID string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Expired reports whether the access token needs renewing before use. A link
// with no recorded expiry is treated as current: a provider that does not say
// when a token dies is one that will say so with a 401.
func (l DeviceLink) Expired() bool {
	return !l.ExpiresAt.IsZero() && time.Now().After(l.ExpiresAt)
}

// DeviceConnection is what a client is told about one provider: whether the
// deployment offers it, and whether this member has connected it. Deliberately
// without tokens - this is the shape the profile screen renders.
type DeviceConnection struct {
	Provider string `json:"provider"`
	// Available is whether the operator configured this integration at all.
	// An unconfigured provider is shown as off rather than hidden, so the
	// screen does not silently differ between deployments.
	Available bool `json:"available"`
	Connected bool `json:"connected"`
	// ConnectedAt is when the member linked it, for the "connected since"
	// line. Zero when they have not.
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
}

// StepDay is one day's step count.
type StepDay struct {
	Date  string `json:"date"`
	Steps int64  `json:"steps"`
}

// StepSeries is a range of days and what to say about it.
//
// The average is computed here rather than in each client: the web app and the
// phone would otherwise each decide what to do about a day the provider has
// nothing for, and disagree about the same week.
type StepSeries struct {
	Provider string    `json:"provider"`
	Range    string    `json:"range"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	Days     []StepDay `json:"days"`
	Total    int64     `json:"total"`
	// DailyAverage is the mean over the days in the range, rounded to the
	// nearest step. Zero-step days count: they are days the member did not
	// walk, not days that did not happen.
	DailyAverage int64 `json:"dailyAverage"`
}
