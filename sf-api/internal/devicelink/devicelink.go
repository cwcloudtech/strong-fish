// Package devicelink is the seam between the app and whatever counts a
// member's steps for them.
//
// There is one provider behind it today - Google Health, which is where Fitbit
// data arrives now that the Fitbit Web API is switched off on 30 October 2026.
// The seam exists because that sentence has been true of a different service
// before, and will be again: a provider here is four methods and a name, and
// adding one touches this file and nothing in the handlers.
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
}

// DayCount is one day's steps, as a provider reports them.
type DayCount struct {
	Date  string
	Steps int64
}

// Provider is everything the handlers need of an integration.
//
// Deliberately small: start a consent, finish one, renew a grant, and read
// days of steps. A provider that needs more than this is one whose difference
// belongs in its own package, not in a wider interface every other provider
// then has to implement emptily.
type Provider interface {
	// Name is the stable identifier: the {provider} path segment, and the
	// value stored in device_links.provider.
	Name() string
	// Enabled is whether the operator gave this integration the credentials
	// it needs. A provider is registered either way - a member is told it
	// exists and is switched off here, rather than being shown nothing and
	// left to wonder whether the feature is real.
	Enabled() bool
	// MaxRangeDays is the longest span of days it will return at once, which
	// is what bounds the views the app offers.
	MaxRangeDays() int

	AuthorizationURL(redirectURI, state string) string
	Exchange(ctx context.Context, code, redirectURI string) (Token, error)
	Refresh(ctx context.Context, refreshToken string) (Token, error)

	// CheckAccount confirms a fresh grant can actually read anything, so a
	// member learns at the moment of connecting rather than at the first empty
	// chart. It returns the id the provider knows them by.
	CheckAccount(ctx context.Context, accessToken string) (string, error)

	// DailySteps returns one entry per day from start to end inclusive.
	DailySteps(ctx context.Context, accessToken string, start, end time.Time) ([]DayCount, error)
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
