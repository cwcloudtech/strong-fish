package models

import (
	"math"
	"strings"
)

// How a member wants effort shown wherever StrongFish prints it - a prescribed
// intensity, a logged set, a velocity-based estimate.
//
// The two scales carry the same information: RPE is the rate of perceived
// exertion (10 is a maximal grind), RIR is the reps left in reserve, and
//
//	RIR = 10 - RPE
//
// Effort is always stored and computed as RPE; the metric is a display choice
// only, applied at the last moment (see RIRFromRPE). RPE is the default an
// account that never chose reads as.
const (
	EffortMetricRPE = "rpe"
	EffortMetricRIR = "rir"
)

// NormalizeEffortMetric maps whatever was stored or sent to one of the two
// known metrics, defaulting to RPE - the scale the whole app computes in, and
// the safe reading for a row no migration has touched.
func NormalizeEffortMetric(metric string) string {
	if strings.EqualFold(strings.TrimSpace(metric), EffortMetricRIR) {
		return EffortMetricRIR
	}
	return EffortMetricRPE
}

// RIRFromRPE is the reps-in-reserve an RPE implies: 10 - RPE, never negative.
// It is the one place the conversion lives, so every surface that honours the
// RIR preference reads the same number.
func RIRFromRPE(rpe float64) float64 {
	return math.Max(0, 10-rpe)
}
