// Package stance reads a member's barbell-velocity sets from Stance Fitness -
// the bar-speed tracker clipped to the barbell - and the "real" RPE Stance
// computes from how fast the bar actually moved.
//
// Stance's API is a Supabase backend for the account and the stored sets, plus
// a recompute service that turns a finished set into per-rep velocities and a
// velocity-derived RPE. Its endpoints and keys are provided by Stance; email
// hello@stancefitness.co to obtain them. Everything specific to Stance sits
// behind the Client interface here, so a change on their side touches only the
// one implementation below and nothing downstream of it.
//
// Three things are kept out of the handlers because they are Stance's shape,
// not ours: the account is paired with an email and password rather than an
// OAuth redirect (a Supabase sign-in), the velocity the bar moved at is the
// unit everything is expressed in, and the capture itself happens on the phone
// over Bluetooth - the API only ever receives a finished set.
package stance

import (
	"context"
	"strings"
	"time"
)

// The three lifts Stance recomputes RPE for, and the only ones StrongFish
// charts. Stance stores a free-text exercise on each set; these are what it is
// mapped onto (see matchLift), because a chart keyed on "Back Squat (comp)"
// is a chart nobody's second set lands in.
const (
	LiftSquat    = "squat"
	LiftBench    = "bench"
	LiftDeadlift = "deadlift"
)

// Lifts is the three, in the order a screen shows them.
var Lifts = []string{LiftSquat, LiftBench, LiftDeadlift}

// Session is a paired account's Supabase grant, stored per member like any
// other device link. Access tokens are short (~1h); RefreshToken renews them.
type Session struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	// UserID is who Supabase knows the member as - the key every set is
	// filtered by. Kept in the link's data so a read needs no second call to
	// find out whose sets to ask for.
	UserID string
}

// Set is one completed set, as StrongFish needs it: what was lifted, what the
// member said it felt like, and what the bar's speed said it really was.
type Set struct {
	// Lift is one of the constants above, or "" for a set on something that
	// is not one of the three (it is then left off the charts).
	Lift     string
	WeightKg float64
	Reps     int
	// GivenRPE is what the member rated the set; 0 when they did not.
	GivenRPE float64
	// PredictedRPE is Stance's recomputation from the bar speed - the "real"
	// RPE. 0 when it has not been computed.
	PredictedRPE float64
	// MeanVelocity and PeakVelocity are the set's concentric bar speed in m/s,
	// averaged and at its fastest rep.
	MeanVelocity float64
	PeakVelocity float64
	// RepVelocities is the mean concentric velocity of each rep in order -
	// what the per-set chart plots, one bar a rep.
	RepVelocities []float64
	PerformedAt   time.Time
}

// Submission is a finished set handed to Stance for the conclusion: the reps
// the phone segmented, what was on the bar, and the member's own RPE. The raw
// samples travel as an opaque blob in Stance's own encoding - this API carries
// it, it does not read it.
type Submission struct {
	Lift     string
	WeightKg float64
	Reps     int
	GivenRPE float64
	// Samples is the encoded recording the phone captured over Bluetooth, in
	// Stance's own format. Opaque here.
	Samples []byte
}

// Client is everything StrongFish asks of Stance. Narrow on purpose: pair an
// account, keep its grant fresh, read the last set of each lift, and submit a
// finished set to get its conclusion. A later public API is a second
// implementation of exactly this.
type Client interface {
	// Pair exchanges an email and password for a Supabase session. The
	// password is used once, here, and never returned or stored.
	Pair(ctx context.Context, email, password string) (Session, error)
	// Refresh renews a session from its refresh token.
	Refresh(ctx context.Context, refreshToken string) (Session, error)
	// LastSets is the most recent set of each of the three lifts, newest
	// first, for the member Supabase knows as userID. A lift the member has
	// never tracked is simply absent.
	LastSets(ctx context.Context, accessToken, userID string) ([]Set, error)
	// Submit sends a finished set and returns it with Stance's conclusion
	// filled in - the recomputed RPE and the per-rep velocities.
	Submit(ctx context.Context, accessToken, userID string, set Submission) (Set, error)
	// Enabled reports whether the operator configured the integration. When
	// false the whole feature is off: the provider lists as unavailable and
	// none of the above is called.
	Enabled() bool
	// RecordURL is what the "record a set" button opens - the Stance app,
	// which does the capture StrongFish cannot.
	RecordURL() string
}

// matchLift maps Stance's free-text exercise onto one of the three lifts, or
// "" for anything else. Substring and case-insensitive, because the app lets a
// member name a set "Comp Squat" or "Paused Bench" and all of those are still
// the lift whose last set the chart wants.
func matchLift(exercise string) string {
	e := strings.ToLower(exercise)
	switch {
	case strings.Contains(e, "squat"):
		return LiftSquat
	case strings.Contains(e, "bench"):
		return LiftBench
	case strings.Contains(e, "deadlift"), strings.Contains(e, "dead lift"):
		return LiftDeadlift
	}
	return ""
}
