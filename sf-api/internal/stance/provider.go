package stance

import (
	"context"
	"errors"

	"strong-fish-api/internal/devicelink"
)

// Provider is the name this link is stored under (see device_links.provider).
const Provider = "stance"

// provider adapts the bar-speed Client to the devicelink.Provider seam so the
// device handler treats Stance like any other integration - listed, connected,
// read, disconnected. It is paired without an OAuth consent redirect: an email
// and a password (AccountPairer), or a Google ID token the client obtained
// itself (GoogleAccountPairer). Both end in the same Firebase session, and
// BarSpeedReader reads the last set of each lift.
type provider struct {
	client Client
}

// NewProvider wraps a Client as a device-link provider.
func NewProvider(client Client) devicelink.Provider {
	return &provider{client: client}
}

func (p *provider) Name() string      { return Provider }
func (p *provider) Enabled() bool     { return p.client.Enabled() }
func (p *provider) RecordURL() string { return p.client.RecordURL() }

// errNotOAuth is what the consent methods return: Stance is paired with an
// account, not an authorization redirect, so a caller that reached for an
// authorization URL is on the wrong path.
var errNotOAuth = errors.New("stance: paired with an account, not an authorization redirect")

func (p *provider) AuthorizationURL(string, string) string { return "" }

func (p *provider) Exchange(context.Context, string, string, string) (devicelink.Token, error) {
	return devicelink.Token{}, errNotOAuth
}

func (p *provider) Refresh(ctx context.Context, refreshToken string) (devicelink.Token, error) {
	session, err := p.client.Refresh(ctx, refreshToken)
	if err != nil {
		return devicelink.Token{}, mapError(err)
	}
	return toToken(session), nil
}

// CheckAccount has nothing to do once signed in: the session already carries
// the uid, and a bar-speed account has no "is there a profile behind it"
// question the way Google Health does.
func (p *provider) CheckAccount(context.Context, string) (string, error) {
	return "", nil
}

// PairAccount signs in with the member's Stance email and password.
func (p *provider) PairAccount(ctx context.Context, email, password string) (devicelink.Token, error) {
	session, err := p.client.PairPassword(ctx, email, password)
	if err != nil {
		return devicelink.Token{}, mapError(err)
	}
	return toToken(session), nil
}

// PairGoogleIDToken signs in with a Google ID token the client obtained with
// the public Google client id.
func (p *provider) PairGoogleIDToken(ctx context.Context, idToken string) (devicelink.Token, error) {
	session, err := p.client.PairGoogle(ctx, idToken)
	if err != nil {
		return devicelink.Token{}, mapError(err)
	}
	return toToken(session), nil
}

// GoogleClientID is the public client id the client needs to obtain a Google
// ID token, or blank when Google sign-in is not configured.
func (p *provider) GoogleClientID() string {
	if !p.client.GoogleEnabled() {
		return ""
	}
	return p.client.GoogleClientID()
}

// LastSets reads the last set of each lift for the signed-in member, mapping
// the package's own Set onto the device seam's BarSet. The Firebase uid is
// what keys the reads, so externalUserID is used and accessToken is not.
func (p *provider) LastSets(ctx context.Context, _, externalUserID string) ([]devicelink.BarSet, error) {
	sets, err := p.client.LastSets(ctx, externalUserID)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]devicelink.BarSet, 0, len(sets))
	for _, set := range sets {
		out = append(out, devicelink.BarSet{
			Lift: set.Lift, WeightKg: set.WeightKg, Reps: set.Reps,
			GivenRPE: set.GivenRPE, PredictedRPE: set.PredictedRPE,
			MeanVelocity: set.MeanVelocity, PeakVelocity: set.PeakVelocity,
			RepVelocities: set.RepVelocities, RepPeakVelocities: set.RepPeakVelocities,
			PerformedAt: set.PerformedAt,
		})
	}
	return out, nil
}

func toToken(s Session) devicelink.Token {
	return devicelink.Token{
		AccessToken: s.IDToken, RefreshToken: s.RefreshToken,
		ExpiresAt: s.ExpiresAt, UserID: s.FirebaseUID,
	}
}

// mapError translates the package's own errors into the seam's, so the handler
// answers "sign in again" the same way whatever provider raised it.
func mapError(err error) error {
	switch {
	case errors.Is(err, ErrRefused):
		return err
	case errors.Is(err, ErrReauthorize):
		return devicelink.ErrReauthorize
	}
	return err
}
