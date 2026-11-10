package velocity

import "math"

// EffortEstimate is a velocity-based reading of how hard a set was, both ways
// the same number is written: an RPE on the 1-10 scale, and the reps in reserve
// it implies (RIR = 10 - RPE). A client shows whichever the member configured;
// the pair is carried so neither end has to do the subtraction.
type EffortEstimate struct {
	RPE float64 `json:"rpe"`
	RIR float64 `json:"rir"`
}

// effortModel is the per-movement velocity-to-effort tuning the estimate reads
// against: the minimum velocity threshold (MVT, the speed of the last possible
// rep, where RPE is 10) and k, the mean concentric velocity lost per rep of
// fatigue. Both are exercise-specific - a deadlift grinds out near failure far
// slower than it leaves the floor, a bench press sits lower again.
//
// The values are population defaults from the velocity-based-training
// literature, not one lifter's fitted profile: enough to read a sensible RPE
// off a single rep with no history, which the personal method (see PredictRPE)
// then refines once the member has rated enough sets. See the tutorial
// sf-wiki/docs/tutorials/velocity-based-rpe.md for the model and the maths.
type effortModel struct {
	mvt float64
	k   float64
}

// effortModels is the per-lift table, keyed by the movement the engine reads
// ("squat", "bench", "deadlift"). It is meant to be edited: tuning a lift is
// changing a line, and an unknown movement simply gets no estimate rather than
// a number read off the wrong scale.
var effortModels = map[string]effortModel{
	"squat":    {mvt: 0.30, k: 0.05},
	"bench":    {mvt: 0.17, k: 0.045},
	"deadlift": {mvt: 0.16, k: 0.05},
}

// EstimateEffort reads a velocity-based RPE/RIR from one rep's mean concentric
// velocity and the movement, with no personal history:
//
//	RIR = (v - MVT) / k        RPE = 10 - RIR
//
// using the population MVT and k for the lift. The rep passed is the effort
// signal - the set's last (slowest) rep - and v is its mean concentric speed in
// m/s.
//
// The RPE is clamped to 5..10 and rounded to the nearest half point: far from
// failure the load-velocity line is too flat to read an effort off, so below 5
// the honest answer is only "5 or under", not a precise low number. ok is false
// for an unknown movement or a non-positive velocity - a guess with no basis is
// worse than none, and the caller shows nothing.
func EstimateEffort(movement string, lastRepVelocity float64) (EffortEstimate, bool) {
	model, known := effortModels[movement]
	if !known || lastRepVelocity <= 0 || model.k <= 0 {
		return EffortEstimate{}, false
	}
	rir := (lastRepVelocity - model.mvt) / model.k
	rpe := 10 - rir
	if rpe < 5 {
		rpe = 5
	}
	if rpe > 10 {
		rpe = 10
	}
	rpe = math.Round(rpe*2) / 2
	return EffortEstimate{RPE: rpe, RIR: 10 - rpe}, true
}
