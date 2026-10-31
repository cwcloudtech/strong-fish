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
	// Capabilities is what the provider can be read for: "activity" (steps
	// and heart rate), "body-composition", or both.
	Capabilities []string `json:"capabilities"`
	// ActivitySource marks the one connected provider whose steps and heart
	// rate are shown. Decided here so the web app and the phone cannot pick
	// differently when a member has connected more than one.
	ActivitySource bool `json:"activitySource"`
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

// HeartDay is one day's heart rate, and the hardest zone reached that day.
type HeartDay struct {
	Date string `json:"date"`
	Avg  int    `json:"avg"`
	Max  int    `json:"max"`
	Min  int    `json:"min"`
	// Zone is one of light, moderate, vigorous, peak - the provider's own
	// four bands, which is what the colour on screen reads from.
	Zone string `json:"zone"`
	// Known separates "the watch was off" from "a resting day": both would
	// otherwise be a row of zeroes.
	Known bool `json:"known"`
}

// HeartSeries is a range of days of heart rate.
//
// The averages skip the days with no reading at all. A night without the watch
// is not a day at zero beats per minute, and letting it count would drag a
// week's average down by a quarter for no reason a member could see.
type HeartSeries struct {
	Provider string     `json:"provider"`
	Range    string     `json:"range"`
	From     string     `json:"from"`
	To       string     `json:"to"`
	Days     []HeartDay `json:"days"`
	// AverageBPM is the mean of the daily averages over the days with a
	// reading; PeakBPM the highest single reading in the range.
	AverageBPM int `json:"averageBpm"`
	PeakBPM    int `json:"peakBpm"`
	// Clamped says the provider would not reach as far back as was asked -
	// heart rate stops at a fortnight where steps go back three months. The
	// days returned are real, they just cover less than the question did,
	// which is why a client shows the latest reading from them but not a
	// chart that would be captioned with the wrong range.
	Clamped bool `json:"clamped"`
}

// BodyMeasure is the latest reading of one body composition measure.
type BodyMeasure struct {
	// Type is weight, bmi, fatRatio, fatMass, fatFreeMass, muscleMass,
	// hydration, boneMass, visceralFat or basalMetabolicRate.
	Type  string  `json:"type"`
	Value float64 `json:"value"`
	// Unit is kg, %, kcal, kg/m2, or blank for an index.
	Unit    string    `json:"unit"`
	TakenAt time.Time `json:"takenAt"`
	// Adjusted is the same measure corrected for the fat a scale's athlete
	// mode is known to miss, printed as "value (~ adjusted)". Absent for a
	// measure the correction leaves alone: weight, bone, BMI and the like.
	Adjusted *float64 `json:"adjusted,omitempty"`
}

// BodyComposition is the latest of every measure a member's scale has taken,
// in the order a panel lists them. A measure their scale does not take is
// absent, not zero.
type BodyComposition struct {
	Provider string        `json:"provider"`
	Measures []BodyMeasure `json:"measures"`
}
