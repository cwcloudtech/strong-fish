// Package devicelink is the seam between the app and whatever measures a
// member for them: the steps a watch counted, what a scale weighed.
//
// Two providers sit behind it today - Google Health, which is where Fitbit
// data arrives now that the Fitbit Web API is switched off on 30 October 2026,
// and Withings for its scales. The seam exists because the first of those
// sentences has been true of a different service before, and will be again: a
// provider here is a handful of methods and a name, plus a reader for each
// kind of data it has.
package devicelink

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrNotLinked is a provider saying the account consented but has nothing
// behind it - a Google account with no Google Health profile, say. It is the
// common first-run failure, and the member can fix it, so it travels all the
// way to the screen rather than being folded into a generic error.
var ErrNotLinked = errors.New("devicelink: account not linked at the provider")

// ErrReauthorize is a provider refusing the stored grant: revoked by the
// member, or lapsed beyond refreshing. The answer is to connect again, not to
// retry.
var ErrReauthorize = errors.New("devicelink: authorization no longer valid")

// Token is one grant as a provider hands it over.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scope        string
	// UserID is who the provider knows the member as, for a provider that
	// says so in the token response itself rather than behind an API call.
	UserID string
}

// DayCount is one day's steps, as a provider reports them.
type DayCount struct {
	Date  string
	Steps int64
}

// Heart rate zones, lowest effort first. These are the provider's own four
// bands rather than thresholds invented here: a zone depends on a person's
// maximum heart rate, which is theirs to know, not ours to guess from a
// birthday that may not even be filled in.
const (
	ZoneLight    = "light"
	ZoneModerate = "moderate"
	ZoneVigorous = "vigorous"
	ZonePeak     = "peak"
)

// DayHeart is one day's heart rate.
//
// Zone is the hardest band the member actually reached that day, which is what
// makes a number worth colouring: an average of 62 says little, an average of
// 62 on a day that peaked into the vigorous band says what kind of day it was.
// A day spent entirely at rest reports ZoneLight.
type DayHeart struct {
	Date  string
	Avg   float64
	Max   float64
	Min   float64
	Zone  string
	Known bool
}

// Provider is everything the handlers need of an integration to connect it.
//
// Deliberately small: start a consent, finish one, renew a grant. What can be
// read once connected is a separate interface per kind of data, because the
// integrations do not overlap - a watch counts steps, a scale weighs - and a
// provider made to implement readers for data it never has would only answer
// every one of them with an error.
type Provider interface {
	// Name is the stable identifier: the {provider} path segment, and the
	// value stored in device_links.provider.
	Name() string
	// Enabled is whether the operator gave this integration the credentials
	// it needs. A provider is registered either way - a member is told it
	// exists and is switched off here, rather than being shown nothing and
	// left to wonder whether the feature is real.
	Enabled() bool

	AuthorizationURL(redirectURI, state string) string
	Exchange(ctx context.Context, code, redirectURI string) (Token, error)
	Refresh(ctx context.Context, refreshToken string) (Token, error)

	// CheckAccount confirms a fresh grant can actually read anything, so a
	// member learns at the moment of connecting rather than at the first empty
	// chart. It returns the id the provider knows them by, or blank when the
	// token exchange already said (see Token.UserID).
	CheckAccount(ctx context.Context, accessToken string) (string, error)
}

// ActivityReader is a provider that counts steps and heart rate.
type ActivityReader interface {
	// MaxRangeDays is the longest span of days it will return at once, which
	// is what bounds the views the app offers.
	MaxRangeDays() int
	// MaxHeartRateDays is the same for heart rate, which providers cap much
	// shorter than steps - a separate number because a view that is fine for
	// one is refused for the other.
	MaxHeartRateDays() int

	// DailySteps returns one entry per day from start to end inclusive.
	DailySteps(ctx context.Context, accessToken string, start, end time.Time) ([]DayCount, error)

	// DailyHeartRate returns one entry per day over the same span.
	DailyHeartRate(ctx context.Context, accessToken string, start, end time.Time) ([]DayHeart, error)
}

// ActivityProvider is a provider that can be read for activity.
type ActivityProvider interface {
	Provider
	ActivityReader
}

// BodyCompositionReader is a provider with a scale behind it.
type BodyCompositionReader interface {
	// LatestBodyComposition returns the most recent reading of each measure
	// the member's scale has ever taken, one entry per measure. A measure the
	// scale does not take is absent rather than zero.
	LatestBodyComposition(ctx context.Context, accessToken string) ([]BodyMeasure, error)
}

// What a provider can be read for, as the clients are told.
const (
	CapabilityActivity        = "activity"
	CapabilityBodyComposition = "body-composition"
)

// Capabilities lists what this provider can be read for, so a client draws
// the panels that have something behind them and no others.
func Capabilities(provider Provider) []string {
	capabilities := []string{}
	if _, ok := provider.(ActivityReader); ok {
		capabilities = append(capabilities, CapabilityActivity)
	}
	if _, ok := provider.(BodyCompositionReader); ok {
		capabilities = append(capabilities, CapabilityBodyComposition)
	}
	return capabilities
}

// ActivitySource picks, among the providers a member connected, the one whose
// steps and heart rate are shown - the first in registration order that can be
// read for activity. The order is the priority: a member with both a watch on
// Google Health and a Withings scale that counts steps from the phone in their
// pocket sees the watch's numbers, and the scale's only when it is all they
// have connected.
func ActivitySource(providers []Provider, connected map[string]bool) string {
	for _, provider := range providers {
		if _, ok := provider.(ActivityReader); ok && connected[provider.Name()] && provider.Enabled() {
			return provider.Name()
		}
	}
	return ""
}

// Body composition measures. Names rather than the provider's numeric codes,
// so the clients translate a word and never learn a vendor's numbering.
const (
	MeasureWeight             = "weight"
	MeasureBMI                = "bmi"
	MeasureFatRatio           = "fatRatio"
	MeasureFatMass            = "fatMass"
	MeasureFatFreeMass        = "fatFreeMass"
	MeasureMuscleMass         = "muscleMass"
	MeasureHydration          = "hydration"
	MeasureBoneMass           = "boneMass"
	MeasureVisceralFat        = "visceralFat"
	MeasureBasalMetabolicRate = "basalMetabolicRate"
)

// BodyMeasure is one measure as last taken.
type BodyMeasure struct {
	Type  string
	Value float64
	// Unit is "kg", "%", "kcal", "kg/m2", or blank for an index with none.
	Unit    string
	TakenAt time.Time
}

// Find returns the configured provider with this name.
func Find(providers []Provider, name string) (Provider, bool) {
	for _, provider := range providers {
		if provider.Name() == name {
			return provider, true
		}
	}
	return nil, false
}

// Names lists the configured providers, in registration order.
func Names(providers []Provider) []string {
	names := make([]string, len(providers))
	for i, provider := range providers {
		names[i] = provider.Name()
	}
	return names
}

// StateLabel binds a consent redirect to both the provider and the member who
// started it.
//
// The callback arrives as a plain browser redirect with no session on it, so
// who was connecting has to travel in the state parameter - and the state
// parameter has to be signed, or anybody could post a callback claiming to be
// somebody else connecting. Reusing the OIDC package's signed state keeps one
// implementation of that signature in the codebase.
func StateLabel(provider, userID string) string {
	return provider + ":" + userID
}

// SplitStateLabel reads back what StateLabel bound together. The label still
// has to be verified afterwards: reading it proves nothing on its own.
func SplitStateLabel(label string) (provider, userID string, ok bool) {
	provider, userID, found := strings.Cut(label, ":")
	if !found || provider == "" || userID == "" {
		return "", "", false
	}
	return provider, userID, true
}
