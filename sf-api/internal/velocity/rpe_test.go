package velocity

import (
	"math"
	"testing"
)

func TestPredictRPEMatchesTheBand(t *testing.T) {
	// A member whose slower last reps are higher RPE, faster are lower.
	history := []RPEPoint{
		{Rpe: 10, Velocity: 0.15}, {Rpe: 10, Velocity: 0.17},
		{Rpe: 8, Velocity: 0.30}, {Rpe: 8, Velocity: 0.32}, {Rpe: 8, Velocity: 0.34},
		{Rpe: 6, Velocity: 0.45}, {Rpe: 6, Velocity: 0.47},
	}

	// A last rep around the RPE-8 band reads as 8.
	if got := PredictRPE(history, 0.31); math.Abs(got-8) > 0.6 {
		t.Errorf("v=0.31 -> RPE %.1f, want ~8", got)
	}
	// A fast last rep reads as the lower RPE.
	if got := PredictRPE(history, 0.46); math.Abs(got-6) > 0.6 {
		t.Errorf("v=0.46 -> RPE %.1f, want ~6", got)
	}
	// A slow grind reads as the top band.
	if got := PredictRPE(history, 0.16); math.Abs(got-10) > 0.6 {
		t.Errorf("v=0.16 -> RPE %.1f, want ~10", got)
	}
}

func TestPredictRPEHalfPoints(t *testing.T) {
	// A velocity sitting between two bands gives a half point.
	history := []RPEPoint{
		{Rpe: 8, Velocity: 0.30},
		{Rpe: 7, Velocity: 0.36},
	}
	got := PredictRPE(history, 0.33) // window [0.264, 0.396] overlaps both
	if got != 7.5 {
		t.Errorf("between bands -> RPE %.2f, want 7.5", got)
	}
}

func TestPredictRPENoHistoryOrOutOfRange(t *testing.T) {
	if got := PredictRPE(nil, 0.3); got != 0 {
		t.Errorf("no history -> %.1f, want 0", got)
	}
	history := []RPEPoint{{Rpe: 8, Velocity: 0.30}}
	// A velocity nowhere near anything the member has done: no confident guess.
	if got := PredictRPE(history, 2.0); got != 0 {
		t.Errorf("out of range -> %.1f, want 0", got)
	}
	if got := PredictRPE(history, 0); got != 0 {
		t.Errorf("no velocity -> %.1f, want 0", got)
	}
}
