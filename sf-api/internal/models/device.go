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
	// SleepSource marks the one connected provider whose last night is shown.
	// The priority is the member's own: a watch's night (Google Health/Fitbit,
	// then Garmin) over a scale's (Withings), decided here so the clients and the
	// export agree.
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

// The chart window: twelve hours from 21:00 to 09:00, the night a sleep app
// draws.
const (
	sleepWindowStartHour = 21
	sleepWindowHours     = 12
)

// SleepSegment is one stretch of the night in a single state.
type SleepSegment struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	// Awake is true for a stretch spent awake in bed, false for one asleep.
	Awake bool `json:"awake"`
}

// SleepNight is last night's sleep as the clients render it: when it ran, how
// long was actually slept, and the stretches in between - with the good/fair/poor
// read a glance wants.
type SleepNight struct {
	Provider string `json:"provider"`
	// Date is the morning they woke, YYYY-MM-DD. Blank for "no recent night".
	Date      string         `json:"date,omitempty"`
	Start     time.Time      `json:"start"`
	End       time.Time      `json:"end"`
	AsleepMin int            `json:"asleepMin"`
	AwakeMin  int            `json:"awakeMin"`
	Segments  []SleepSegment `json:"segments,omitempty"`
}

// Recorded is whether there is a night to show at all.
func (n SleepNight) Recorded() bool { return n.Date != "" && n.AsleepMin > 0 }

// Status is the good/fair/poor read of the night, from how long was slept.
func (n SleepNight) Status() string {
	switch {
	case n.AsleepMin >= sleepGoodMinutes:
		return SleepStatusGood
	case n.AsleepMin >= sleepFairMinutes:
		return SleepStatusFair
	default:
		return SleepStatusPoor
	}
}

// SleepHour is one hour of the chart window: the clock hour, and how much of it
// was spent in bed and asleep.
type SleepHour struct {
	Hour      int
	InBedMin  int
	AsleepMin int
}

// Hours breaks the night into the twelve one-hour buckets from 21:00 to 09:00,
// in order, for the chart - each one's minutes in bed and minutes asleep. The
// window is anchored on the evening the night began, in the reading's own
// timezone, so a member who went to bed after midnight still reads under the
// right hours.
func (n SleepNight) Hours() []SleepHour {
	hours := make([]SleepHour, sleepWindowHours)
	loc := time.UTC
	if !n.Start.IsZero() && n.Start.Location() != nil {
		loc = n.Start.Location()
	}
	evening := sleepEveningAnchor(n.Start, loc)
	for h := 0; h < sleepWindowHours; h++ {
		bucketStart := evening.Add(time.Duration(h) * time.Hour)
		bucketEnd := bucketStart.Add(time.Hour)
		hours[h].Hour = (sleepWindowStartHour + h) % 24
		hours[h].InBedMin = overlapMinutes(n.Start, n.End, bucketStart, bucketEnd)
		for _, segment := range n.Segments {
			if !segment.Awake {
				hours[h].AsleepMin += overlapMinutes(segment.Start, segment.End, bucketStart, bucketEnd)
			}
		}
		if hours[h].AsleepMin > hours[h].InBedMin {
			hours[h].AsleepMin = hours[h].InBedMin
		}
	}
	return hours
}

// sleepEveningAnchor is the 21:00 the chart window hangs off: the start's own
// evening when the member was already up past nine, the evening before when they
// went to bed after midnight.
func sleepEveningAnchor(start time.Time, loc *time.Location) time.Time {
	if start.IsZero() {
		return start
	}
	start = start.In(loc)
	evening := time.Date(start.Year(), start.Month(), start.Day(), sleepWindowStartHour, 0, 0, 0, loc)
	if start.Hour() < sleepWindowStartHour {
		evening = evening.AddDate(0, 0, -1)
	}
	return evening
}

// overlapMinutes is how many minutes [aStart, aEnd) and [bStart, bEnd) share.
func overlapMinutes(aStart, aEnd, bStart, bEnd time.Time) int {
	start := aStart
	if bStart.After(start) {
		start = bStart
	}
	end := aEnd
	if bEnd.Before(end) {
		end = bEnd
	}
	if !end.After(start) {
		return 0
	}
	return int(end.Sub(start).Minutes())
}
