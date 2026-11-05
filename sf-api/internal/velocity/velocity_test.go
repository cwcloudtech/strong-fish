package velocity

import (
	"math"
	"testing"
)

// synth builds a set from a list of concentric peak velocities. Each rep is a
// full cycle - the bar goes down (eccentric, negative velocity) then up
// (concentric, positive velocity) - so the velocity averages to zero over the
// rep and the integration starts and ends at rest, the way a real set does. The
// concentric half is what the engine should detect, peaking at the given value.
func synth(rate float64, peaks []float64, repDur, gap, lead float64, withQuat bool) Capture {
	dt := 1.0 / rate
	var accel []float64
	add := func(sec float64, f func(u float64) float64) {
		steps := int(math.Round(sec / dt))
		for k := 0; k < steps; k++ {
			accel = append(accel, f(float64(k)/float64(steps)))
		}
	}

	add(lead, func(float64) float64 { return 0 })
	for _, vp := range peaks {
		// v(u) = -vp*sin(2πu): negative (down) then positive (up); a = dv/dt.
		add(repDur, func(u float64) float64 {
			return -vp * (2 * math.Pi / repDur) * math.Cos(2*math.Pi*u)
		})
		add(gap, func(float64) float64 { return 0 })
	}

	samples := make([]Sample, len(accel))
	for i, a := range accel {
		s := Sample{T: float64(i) * dt, AZ: a + gravity}
		if withQuat {
			s.QW = 1 // identity orientation: worldZ == AZ
		}
		samples[i] = s
	}
	return Capture{Movement: "deadlift", Samples: samples, SampleRateHz: rate}
}

func TestAnalyzeCountsRepsAndReadsVelocity(t *testing.T) {
	for _, quat := range []bool{true, false} {
		got := Analyze(synth(200, []float64{0.80}, 1.2, 0.8, 0.5, quat))
		if len(got.Reps) != 1 {
			t.Fatalf("withQuat=%v: want 1 rep, got %d (%+v)", quat, len(got.Reps), got.Reps)
		}
		rep := got.Reps[0]
		if rep.PeakMps < 0.65 || rep.PeakMps > 0.9 {
			t.Errorf("withQuat=%v: peak %.3f, want ~0.80", quat, rep.PeakMps)
		}
		// The concentric phase trims the slow tails, so the mean sits above the
		// half-sine mean (0.64*peak) and below the peak.
		if rep.MeanMps < 0.45 || rep.MeanMps > rep.PeakMps {
			t.Errorf("withQuat=%v: mean %.3f out of range for peak %.3f", quat, rep.MeanMps, rep.PeakMps)
		}
	}
}

func TestAnalyzeThreeRepsWithVelocityLoss(t *testing.T) {
	got := Analyze(synth(200, []float64{0.80, 0.70, 0.60}, 1.2, 0.8, 0.5, true))
	if len(got.Reps) != 3 {
		t.Fatalf("want 3 reps, got %d (%+v)", len(got.Reps), got.Reps)
	}
	// Velocities fall rep to rep.
	if !(got.Reps[0].MeanMps > got.Reps[1].MeanMps && got.Reps[1].MeanMps > got.Reps[2].MeanMps) {
		t.Errorf("reps not decreasing: %.3f %.3f %.3f",
			got.Reps[0].MeanMps, got.Reps[1].MeanMps, got.Reps[2].MeanMps)
	}
	// Peaks 0.8 -> 0.6 is a 25% drop; the mean-based loss lands near it.
	if got.VelocityLoss < 12 || got.VelocityLoss > 40 {
		t.Errorf("velocity loss %.1f%%, want ~25%%", got.VelocityLoss)
	}
	if got.PeakVelocity < 0.65 {
		t.Errorf("set peak %.3f, want ~0.80", got.PeakVelocity)
	}
}

func TestAnalyzeNoMotionIsNoReps(t *testing.T) {
	samples := make([]Sample, 400)
	for i := range samples {
		samples[i] = Sample{T: float64(i) * 0.005, AZ: gravity, QW: 1}
	}
	got := Analyze(Capture{Movement: "squat", Samples: samples, SampleRateHz: 200})
	if len(got.Reps) != 0 {
		t.Errorf("still bar, want 0 reps, got %d", len(got.Reps))
	}
}

func TestAnalyzeTooShortIsEmpty(t *testing.T) {
	got := Analyze(Capture{Movement: "bench", Samples: make([]Sample, 4), SampleRateHz: 200})
	if len(got.Reps) != 0 || got.MeanVelocity != 0 {
		t.Errorf("too few samples should be empty, got %+v", got)
	}
}

func TestAnalyzeConfidenceHigherForRegularReps(t *testing.T) {
	// Three even reps read as a clean, confident capture.
	even := Analyze(synth(200, []float64{0.8, 0.75, 0.7}, 1.2, 0.8, 0.5, true))
	if even.Confidence < 0.8 {
		t.Errorf("even reps: confidence %.2f, want high", even.Confidence)
	}
	if even.Confidence > 1 || even.Confidence < 0 {
		t.Errorf("confidence %.2f out of [0,1]", even.Confidence)
	}
}

func TestMovementProfileBounds(t *testing.T) {
	// The deadlift allows longer reps than the bench.
	if movementProfile("deadlift").maxRepDurationSec <= movementProfile("bench").maxRepDurationSec {
		t.Errorf("deadlift should allow longer reps than bench")
	}
	// An unknown movement still has a usable profile.
	if movementProfile("curl").minRepDurationSec <= 0 {
		t.Errorf("unknown movement should fall back to a usable profile")
	}
}
