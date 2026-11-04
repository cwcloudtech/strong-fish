// Package stance reads a member's barbell-velocity sets from Stance Fitness -
// the bar-speed tracker clipped to the barbell - and the velocity-derived
// "real" RPE Stance computes from how fast the bar actually moved.
//
// Stance's accounts live in Firebase Auth, and the finished sets in a Supabase
// (PostgREST) database keyed by the Firebase user id. A member is identified
// one of two ways, both ending in a Firebase session whose uid keys the reads:
// an email and password, or Google. The sets are then read from Supabase with
// the project's anon key, filtered to that uid. The endpoints and keys are
// provided by Stance; email hello@stancefitness.co to obtain them.
//
// Everything specific to Stance sits behind the Client interface here, so a
// change on their side touches only the implementation and nothing downstream
// of it. The capture itself happens on the phone over Bluetooth - this API
// only ever reads a finished set.
package stance

import (
	"context"
	"strings"
	"time"
)

// The three lifts Stance reports a velocity-derived RPE for, and the only ones
// StrongFish charts. Stance stores a free-text exercise on each set; these are
// what it is mapped onto (see matchLift), because a chart keyed on "Back Squat
// (comp)" is a chart nobody's second set lands in.
const (
	LiftSquat    = "squat"
	LiftBench    = "bench"
	LiftDeadlift = "deadlift"
)

// Lifts is the three, in the order a screen shows them.
var Lifts = []string{LiftSquat, LiftBench, LiftDeadlift}

// Session is a paired account's Firebase grant, stored per member like any
// other device link. The ID token is short (~1h) and renewed from the refresh
// token; FirebaseUID is the key every set is filtered by, kept in the link's
// data so a read needs no round-trip to find out whose sets to ask for.
type Session struct {
	IDToken      string
	RefreshToken string
	ExpiresAt    time.Time
	FirebaseUID  string
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
	// PredictedRPE is Stance's velocity-derived "real" RPE; 0 when absent.
	PredictedRPE float64
	// MeanVelocity and PeakVelocity are the set's concentric bar speed in m/s,
	// averaged and at its fastest rep.
	MeanVelocity float64
	PeakVelocity float64
	// RepVelocities is the mean concentric velocity of each rep in order -
	// what the per-set chart plots, one bar a rep. RepPeakVelocities is each
	// rep's peak concentric velocity, the faster line the app draws over the
	// mean one.
	RepVelocities     []float64
	RepPeakVelocities []float64
	PerformedAt       time.Time
}

// Client is everything StrongFish asks of Stance. Narrow on purpose: pair an
// account (by password or by Google), keep its grant fresh, and read the last
// set of each lift. A later public API is a second implementation of this.
type Client interface {
	// PairPassword exchanges an email and password for a Firebase session via
	// Firebase Auth. The password is used once, here, and never stored.
	PairPassword(ctx context.Context, email, password string) (Session, error)
	// PairGoogle signs the member in with a Google ID token the client
	// obtained with the public Google client id, exactly as the app does.
	PairGoogle(ctx context.Context, googleIDToken string) (Session, error)
	// GoogleClientID is the public client id the client needs to obtain that
	// Google ID token. Blank when Google sign-in is not configured.
	GoogleClientID() string
	// Refresh renews a session from its Firebase refresh token.
	Refresh(ctx context.Context, refreshToken string) (Session, error)
	// LastSets is the most recent set of each of the three lifts, newest
	// first, for the member Firebase knows as firebaseUID. A lift the member
	// has never tracked is simply absent.
	LastSets(ctx context.Context, firebaseUID string) ([]Set, error)
	// Enabled reports whether the operator configured the integration (both
	// the Firebase identity side and the Supabase read side). When false the
	// whole feature is off: the provider lists as unavailable.
	Enabled() bool
	// GoogleEnabled reports whether the Google sign-in option is configured on
	// top of the password one.
	GoogleEnabled() bool
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
