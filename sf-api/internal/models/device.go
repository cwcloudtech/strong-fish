package models

import (
	"strings"
	"time"
)

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
	// BodyCompositionSource marks the one connected scale whose measures are
	// shown, the same way: a member with two providers that both report weight
	// sees one panel, not one per provider. Decided here so the clients and the
	// export agree on which.
	BodyCompositionSource bool `json:"bodyCompositionSource"`
	// NutritionSource marks the one connected food diary whose figures are
	// shown - a member with two nutrition apps sees one histogram, not one per
	// app. Decided here, like the other two sources, so the clients and the
	// export never pick differently.
	NutritionSource bool `json:"nutritionSource"`
	// SleepSource marks the one connected provider whose week of sleep is shown.
	// The priority is the member's own: a watch's nights (Google Health/Fitbit)
	// over a scale's (Withings), decided here so the clients and the export agree.
	SleepSource bool `json:"sleepSource"`
	// AccountPaired is true for a provider that can be connected with an email
	// and password (Stance), so a client offers a sign-in form.
	AccountPaired bool `json:"accountPaired,omitempty"`
	// OAuth is true when the provider offers a consent redirect to connect
	// (Google Health, Withings). A client shows a "connect" button.
	OAuth bool `json:"oauth,omitempty"`
	// GoogleClientID is set for a provider that can be connected with a Google
	// ID token the client obtains itself (Stance): the public client id the
	// client needs to run "Sign in with Google". Blank when unavailable, and a
	// provider can offer this and AccountPaired together.
	GoogleClientID string `json:"googleClientId,omitempty"`
	// KeyPaired is true for a provider connected with a single personal API
	// key the member enters (Garmin), so a client shows a one-field key form.
	KeyPaired bool `json:"keyPaired,omitempty"`
}

// BarSet is one completed barbell-velocity set, as the clients render it: what
// was lifted, what the member rated it, and the RPE the tracker computed from
// how fast the bar moved.
type BarSet struct {
	// Lift is "squat", "bench" or "deadlift".
	Lift string `json:"lift"`
	// Source names what produced the set - "strongfish" for one we captured and
	// computed ourselves, or the provider it was read from (e.g. "stance") - so
	// a merged panel can tell them apart and label each.
	Source       string  `json:"source,omitempty"`
	WeightKg     float64 `json:"weightKg,omitempty"`
	Reps         int     `json:"reps,omitempty"`
	GivenRpe     float64 `json:"givenRpe,omitempty"`
	PredictedRpe float64 `json:"predictedRpe,omitempty"`
	// MeanVelocity and PeakVelocity are the set's concentric bar speed, m/s.
	MeanVelocity float64 `json:"meanVelocity,omitempty"`
	PeakVelocity float64 `json:"peakVelocity,omitempty"`
	// Confidence (0..1) is how clean a StrongFish-computed capture looked, so a
	// client can flag a shaky reading. Set on a fresh capture only; not stored.
	Confidence float64 `json:"confidence,omitempty"`
	// RepVelocities is each rep's mean concentric velocity in order - the
	// per-set chart, one bar a rep. RepPeakVelocities is each rep's peak
	// concentric velocity, the faster series the app overlays on the mean one.
	RepVelocities     []float64 `json:"repVelocities,omitempty"`
	RepPeakVelocities []float64 `json:"repPeakVelocities,omitempty"`
	PerformedAt       time.Time `json:"performedAt,omitempty"`
	// SetType is "warmup", "topset" or "backoff" - what the set was for. A top
	// set is the effort signal; a warm-up is not, and the AI reading weighs them
	// differently. Defaults to "warmup". Stored in the recording's JSONB data.
	SetType string `json:"setType,omitempty"`
	// TempoVariation marks a set done to a tempo or with a pause (a "2CT" or
	// "paused" cue, a "3:3:0" prescription, a spotto press...). The bar moves
	// slower for a prescribed reason, so the AI is told not to read that slower
	// speed as fatigue or a grind. Stored in the recording's JSONB data.
	TempoVariation bool `json:"tempoVariation,omitempty"`
}

// BarSetMetadata is the JSONB-stored, extensible side of a bar-speed recording:
// the descriptive tags that ride along for the panel and the AI, kept out of the
// fixed columns so a new one needs a key, not a migration.
type BarSetMetadata struct {
	SetType        string `json:"setType,omitempty"`
	TempoVariation bool   `json:"tempoVariation,omitempty"`
}

// Bar-speed set types. Warmup is the default a recording takes when none is
// given, and what older rows were migrated to.
const (
	SetTypeWarmup  = "warmup"
	SetTypeTopset  = "topset"
	SetTypeBackoff = "backoff"
)

// NormalizeSetType lowercases and validates a set type, falling back to warmup
// for anything unknown or empty - the clients default to warmup too.
func NormalizeSetType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case SetTypeTopset:
		return SetTypeTopset
	case SetTypeBackoff:
		return SetTypeBackoff
	default:
		return SetTypeWarmup
	}
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
}

// BodyComposition is the latest of every measure a member's scale has taken,
// in the order a panel lists them. A measure their scale does not take is
// absent, not zero.
type BodyComposition struct {
	Provider string        `json:"provider"`
	Measures []BodyMeasure `json:"measures"`
}

// Sleep statuses - the three-way read a consumer sleep app gives, named rather
// than scored.
const (
	SleepStatusGood = "good"
	SleepStatusFair = "fair"
	SleepStatusPoor = "poor"
)

// How long asleep makes a night good or fair, in minutes. Below the fair mark it
// is poor. Seven to nine hours is the usual recommendation; this reads seven as
// a good night and six as a short one.
const (
	sleepGoodMinutes = 7 * 60
	sleepFairMinutes = 6 * 60
)

// SleepDay is one night in a week of sleep as the clients render it: the morning
// woken, how long was actually slept, and the good/fair/poor read a glance wants.
// A night the tracker has nothing for is present with AsleepMin 0 and no status,
// so the week still shows seven bars.
type SleepDay struct {
	Date      string `json:"date"`
	AsleepMin int    `json:"asleepMin"`
	Status    string `json:"status,omitempty"`
}

// SleepWeek is the last seven nights from one provider - a bar a night read
// good/fair/poor, the way the step and nutrition charts show a week rather than a
// single reading.
type SleepWeek struct {
	Provider string `json:"provider"`
	// From and To are the first and last mornings covered, YYYY-MM-DD.
	From string     `json:"from,omitempty"`
	To   string     `json:"to,omitempty"`
	Days []SleepDay `json:"days"`
}

// SleepStatusFor is the good/fair/poor read of one night, from how long was
// slept. A night with nothing recorded (0 minutes) has no status.
func SleepStatusFor(asleepMin int) string {
	switch {
	case asleepMin >= sleepGoodMinutes:
		return SleepStatusGood
	case asleepMin >= sleepFairMinutes:
		return SleepStatusFair
	case asleepMin > 0:
		return SleepStatusPoor
	default:
		return ""
	}
}

// Recorded is whether any night in the week has sleep to show.
func (w SleepWeek) Recorded() bool {
	for _, day := range w.Days {
		if day.AsleepMin > 0 {
			return true
		}
	}
	return false
}
