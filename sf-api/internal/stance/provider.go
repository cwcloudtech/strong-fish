package stance

import (
	"context"
	"errors"
	"strong-fish-api/internal/devicelink"
)

// Provider is the name this link is stored under (see device_links.provider).
const Provider = "stance"

// provider adapts the bar-speed Client to the devicelink.Provider seam so the
// device handler treats Stance like any other integration - listed, paired,
// read, disconnected - without knowing it is a Supabase account rather than an
// OAuth one.
//
// It is an AccountPairer (email/password, not a consent redirect) and a
// BarSpeedReader (the last set of each lift), and nothing else: the OAuth
// methods of Provider are present only to satisfy the interface and refuse,
// because there is no authorization URL to send a browser to.
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

// errNotOAuth is what the consent methods return: Stance is paired with a
// password, so a caller that reached for an authorization URL is on the wrong
// path and should be told so plainly rather than handed an empty string.
var errNotOAuth = errors.New("stance: paired with an account, not an authorization redirect")

func (p *provider) AuthorizationURL(string, string) string { return "" }

func (p *provider) Exchange(context.Context, string, string) (devicelink.Token, error) {
	return devicelink.Token{}, errNotOAuth
}

func (p *provider) Refresh(ctx context.Context, refreshToken string) (devicelink.Token, error) {
	session, err := p.client.Refresh(ctx, refreshToken)
	if err != nil {
		return devicelink.Token{}, mapError(err)
	}
	return toToken(session), nil
}

// CheckAccount has nothing to do once paired: pairing already returned the
// user id, and a bar-speed account has no "is there a profile behind it"
// question the way Google Health does. It reports the id it is given.
func (p *provider) CheckAccount(context.Context, string) (string, error) {
	return "", nil
}

// PairAccount exchanges the member's Stance email and password for a grant.
func (p *provider) PairAccount(ctx context.Context, email, password string) (devicelink.Token, error) {
	session, err := p.client.Pair(ctx, email, password)
	if err != nil {
		return devicelink.Token{}, mapError(err)
	}
	return toToken(session), nil
}

// LastSets reads the last set of each lift, mapping the package's own Set onto
// the device seam's BarSet.
func (p *provider) LastSets(ctx context.Context, accessToken, externalUserID string) ([]devicelink.BarSet, error) {
	sets, err := p.client.LastSets(ctx, accessToken, externalUserID)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]devicelink.BarSet, 0, len(sets))
	for _, set := range sets {
		out = append(out, devicelink.BarSet{
			Lift: set.Lift, WeightKg: set.WeightKg, Reps: set.Reps,
			GivenRPE: set.GivenRPE, PredictedRPE: set.PredictedRPE,
			MeanVelocity: set.MeanVelocity, PeakVelocity: set.PeakVelocity,
			RepVelocities: set.RepVelocities, PerformedAt: set.PerformedAt,
		})
	}
	return out, nil
}

func toToken(s Session) devicelink.Token {
	return devicelink.Token{
		AccessToken: s.AccessToken, RefreshToken: s.RefreshToken,
		ExpiresAt: s.ExpiresAt, UserID: s.UserID,
	}
}

// mapError translates the package's own errors into the seam's, so the
// handler answers "pair again" the same way whatever provider raised it.
func mapError(err error) error {
	switch {
	case errors.Is(err, ErrRefused):
		return err
	case errors.Is(err, ErrReauthorize):
		return devicelink.ErrReauthorize
	}
	return err
}
