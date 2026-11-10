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

// ramp appends a linear segment from `from` to `to` lasting `sec` at `dt`.
func ramp(v []float64, from, to, sec, dt float64) []float64 {
	n := int(math.Round(sec / dt))
	for i := 0; i < n; i++ {
		v = append(v, from+(to-from)*(float64(i)/float64(n)))
	}
	return v
}

// A squat often grinds through a sticking point: mid-ascent the bar nearly
// stalls - velocity dips toward zero - but never reverses downward. That is one
// rep. Before the fix the sub-threshold stall cut the single concentric in two,
// so a 3-rep squat came back as 6.
func TestDetectRepsMergesStickingPoint(t *testing.T) {
	dt := 0.005
	var v []float64
	v = ramp(v, 0, -0.5, 0.25, dt)   // the rep's descent (squat is eccentric-first)
	v = ramp(v, -0.5, 0, 0.25, dt)
	v = ramp(v, 0, 0.6, 0.30, dt)    // drive up
	v = ramp(v, 0.6, 0.02, 0.20, dt) // slow into the sticking point
	v = ramp(v, 0.02, 0.02, 0.25, dt) // stall, still rising (never negative)
	v = ramp(v, 0.02, 0.5, 0.20, dt) // grind through
	v = ramp(v, 0.5, 0, 0.30, dt)    // finish
	reps := detectReps(v, v, dt, movementProfile("squat"))
	if len(reps) != 1 {
		t.Fatalf("a sticking-point ascent is one rep, got %d", len(reps))
	}
}

// Reps truly apart are separated by a real eccentric - the bar going clearly
// negative - and must stay two reps, so the merge above never glues real reps.
func TestDetectRepsSeparatesRepsAcrossEccentric(t *testing.T) {
	dt := 0.005
	var v []float64
	v = ramp(v, 0, -0.5, 0.25, dt) // rep 1 descent (eccentric-first)
	v = ramp(v, -0.5, 0, 0.25, dt)
	v = ramp(v, 0, 0.6, 0.30, dt)  // rep 1 concentric
	v = ramp(v, 0.6, 0, 0.30, dt)
	v = ramp(v, 0, -0.5, 0.25, dt) // the bar is lowered: a clear reversal
	v = ramp(v, -0.5, 0, 0.25, dt)
	v = ramp(v, 0, 0.6, 0.30, dt)  // rep 2 concentric
	v = ramp(v, 0.6, 0, 0.30, dt)
	reps := detectReps(v, v, dt, movementProfile("squat"))
	if len(reps) != 2 {
		t.Fatalf("two reps split by an eccentric, got %d", len(reps))
	}
}

// With the lift-off removal dropped, a racked squat's un-rack counts as a
// concentric phase like any other: the engine counts what it sees and leaves a
// stray un-rack to be corrected on the review screen, the way the tracker's own
// algorithm is trusted rather than second-guessed. Trying to read the lift-off
// out of a drifting integrated velocity is what merged a controlled bench to
// zero, so the guess is gone.
func TestDetectRepsCountsUnrackNoDerack(t *testing.T) {
	dt := 0.005
	var v []float64
	v = ramp(v, 0, 0, 0.30, dt)      // standing under the bar
	v = ramp(v, 0, 0.6, 0.30, dt)    // un-rack: ascent, no descent before it
	v = ramp(v, 0.6, 0, 0.30, dt)
	v = ramp(v, 0, 0, 0.30, dt)      // walk-out
	for i := 0; i < 2; i++ {         // two real reps: descend then ascend
		v = ramp(v, 0, -0.5, 0.25, dt)
		v = ramp(v, -0.5, 0, 0.25, dt)
		v = ramp(v, 0, 0.6, 0.30, dt)
		v = ramp(v, 0.6, 0, 0.30, dt)
	}
	reps := detectReps(v, v, dt, movementProfile("squat"))
	if len(reps) != 3 {
		t.Fatalf("un-rack now counts (no de-rack), want 3 phases, got %d", len(reps))
	}
}

// When a descent genuinely precedes the first ascent (no un-rack captured, or a
// member who started recording already standing and dropped straight in), the
// first ascent is a real rep and must be kept.
func TestDetectRepsKeepsFirstRepWhenDescentPrecedes(t *testing.T) {
	dt := 0.005
	var v []float64
	for i := 0; i < 2; i++ {
		v = ramp(v, 0, -0.5, 0.25, dt) // descent first
		v = ramp(v, -0.5, 0, 0.25, dt)
		v = ramp(v, 0, 0.6, 0.30, dt)
		v = ramp(v, 0.6, 0, 0.30, dt)
	}
	reps := detectReps(v, v, dt, movementProfile("squat"))
	if len(reps) != 2 {
		t.Fatalf("first ascent preceded by a descent is a rep, want 2, got %d", len(reps))
	}
}

// Deadlift is concentric-first: the pull off the floor is a real rep with no
// descent before it, so the un-rack rule must never touch it.
func TestDetectRepsKeepsDeadliftFirstPull(t *testing.T) {
	dt := 0.005
	var v []float64
	v = ramp(v, 0, 0, 0.30, dt) // bar on the floor
	for i := 0; i < 2; i++ {
		v = ramp(v, 0, 0.6, 0.30, dt) // pull up (concentric first)
		v = ramp(v, 0.6, 0, 0.30, dt)
		v = ramp(v, 0, -0.5, 0.25, dt) // lower
		v = ramp(v, -0.5, 0, 0.25, dt)
	}
	reps := detectReps(v, v, dt, movementProfile("deadlift"))
	if len(reps) != 2 {
		t.Fatalf("deadlift first pull must be kept, want 2, got %d", len(reps))
	}
}

// A controlled bench - a slow, nearly-paused descent whose velocity never
// reaches a fraction of the peak - must detect every rep and, above all, never
// collapse to zero. The old reversal/peak test merged the whole set into one
// over-long phase the duration filter then dropped: the "0 reps" regression.
// The gap-length split keeps the reps apart; the lift-off counts too (no
// de-rack), so the four presses read as five phases - five, never zero.
func TestDetectRepsBenchSlowDescentNotZero(t *testing.T) {
	dt := 0.005
	var v []float64
	v = ramp(v, 0, 0, 0.25, dt)    // on the hooks
	v = ramp(v, 0, 0.55, 0.25, dt) // lift-off
	v = ramp(v, 0.55, 0, 0.30, dt)
	for i := 0; i < 4; i++ {
		v = ramp(v, 0, -0.08, 0.5, dt) // slow, controlled descent
		v = ramp(v, -0.08, 0, 0.5, dt)
		v = ramp(v, 0, 0.55, 0.30, dt)
		v = ramp(v, 0.55, 0, 0.30, dt)
	}
	reps := detectReps(v, v, dt, movementProfile("bench"))
	if len(reps) != 5 {
		t.Fatalf("a controlled bench detects its reps and the lift-off, want 5, got %d", len(reps))
	}
}

// A clean odd-rep set (a triple, a five) has no lift-off and must not lose a
// rep - the trap a plain even-parity de-rack falls into, dropping one from every
// clean triple.
func TestDetectRepsCleanOddSetKept(t *testing.T) {
	dt := 0.005
	var v []float64
	for i := 0; i < 3; i++ {
		v = ramp(v, 0, -0.3, 0.3, dt) // descent first, no lift-off
		v = ramp(v, -0.3, 0, 0.3, dt)
		v = ramp(v, 0, 0.55, 0.3, dt)
		v = ramp(v, 0.55, 0, 0.3, dt)
	}
	reps := detectReps(v, v, dt, movementProfile("squat"))
	if len(reps) != 3 {
		t.Fatalf("a clean 3-rep squat stays 3 (no de-rack), got %d", len(reps))
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
