package loadcalc

import (
	"math"
	"testing"
)

func f(v float64) *float64 { return &v }

// TestLoadRoundTripsThroughE1RM is the property the reference spreadsheet fails
// and this package exists to restore: the load prescribed for reps at an RPE
// against a 1RM must estimate back to exactly that 1RM. The sheet's hand-typed
// integer percentages drift instead - a 225kg deadlift at 3 @ RPE 5 is written
// as 79% -> 177.75kg, whose e1RM comes back 225.2kg.
func TestLoadRoundTripsThroughE1RM(t *testing.T) {
	cases := []struct {
		oneRM float64
		reps  int
		rpe   float64
	}{
		{225, 3, 5}, {225, 3, 7}, {225, 1, 7}, {225, 5, 8},
		{120, 1, 5}, {120, 5, 8}, {120, 5, 7}, {120, 3, 6},
		{95, 3, 6}, {95, 3, 8}, {95, 7, 6}, {95, 4, 8}, {95, 8, 8},
		{140, 10, 8}, {140, 2, 9.5}, {180, 6, 6.5},
	}
	for _, c := range cases {
		got := Load(Prescription{Mode: ModeRPE, Reps: c.reps, RPE: f(c.rpe)}, f(c.oneRM), 0)
		if !got.Known {
			t.Fatalf("%v: load unknown", c)
		}
		back := E1RM(got.Load, c.reps, f(c.rpe))
		// The load itself is rounded to a tenth of a kilo, so the round trip
		// lands within a tenth rather than exactly - well inside the
		// spreadsheet's multi-kilo drift.
		if math.Abs(back-c.oneRM) > 0.15 {
			t.Errorf("1RM %.1f, %d reps @ RPE %.1f: load %.1f estimates back to %.1f, want %.1f",
				c.oneRM, c.reps, c.rpe, got.Load, back, c.oneRM)
		}
	}
}

// TestChartMatchesSpreadsheet checks the chart this package reads lands on the
// integers the spreadsheet's author hand-copied, confirming it's the same table
// they were working from. The tolerance is what the file's own rounding costs:
// its values sit a fraction of a point above the chart.
func TestChartMatchesSpreadsheet(t *testing.T) {
	// Every (reps, RPE) -> percentage the reference file states unambiguously
	// (the pairs it contradicts itself on are covered by
	// TestSpreadsheetContradictionsResolve).
	cases := []struct {
		reps     int
		rpe      float64
		authored float64
	}{
		{1, 10, 100}, {1, 9, 96}, {1, 8, 93}, {1, 7, 90}, {1, 6, 87}, {1, 5, 84},
		{2, 7, 86}, {2, 5, 81},
		{3, 9, 90}, {3, 5, 79},
		{4, 6, 79}, {4, 5, 77},
		{5, 6, 76}, {5, 5, 74},
		{6, 6, 74}, {6, 7, 76}, {6, 5, 71},
		{7, 6, 71}, {7, 7, 74},
		{8, 8, 74}, {8, 7, 71}, {8, 6, 69}, {8, 5, 65},
		{10, 8, 69},
	}
	for _, c := range cases {
		got := Intensity(c.reps, f(c.rpe)) * 100
		if math.Abs(got-c.authored) > 1.75 {
			t.Errorf("%d reps @ RPE %.0f: chart says %.1f%%, spreadsheet authored %.0f%%",
				c.reps, c.rpe, got, c.authored)
		}
	}
}

// TestSpreadsheetContradictionsResolve covers the (reps, RPE) pairs the
// reference file gives two different percentages for. Whichever the author
// meant, both can't be right - and the chart resolves each to the one value
// they were both approximating.
func TestSpreadsheetContradictionsResolve(t *testing.T) {
	cases := []struct {
		reps       int
		rpe        float64
		contested  [2]float64
		wantWithin float64
	}{
		{3, 6, [2]float64{75, 82}, 82},
		{3, 8, [2]float64{81, 87}, 87},
		{4, 8, [2]float64{81, 84}, 84},
		{5, 8, [2]float64{78, 82}, 82},
		{7, 8, [2]float64{77, 78}, 77},
	}
	for _, c := range cases {
		got := Intensity(c.reps, f(c.rpe)) * 100
		if math.Abs(got-c.wantWithin) > 1.75 {
			t.Errorf("%d reps @ RPE %.0f: file says %v, chart resolves to %.1f%% (expected near %.0f%%)",
				c.reps, c.rpe, c.contested, got, c.wantWithin)
		}
	}
}

// TestChartIsMonotonic guards the interpolation and both extrapolation edges:
// more reps is always lighter, and a higher RPE is always heavier.
func TestChartIsMonotonic(t *testing.T) {
	for reps := 1; reps <= 20; reps++ {
		previous := math.Inf(1)
		for rpe := 10.0; rpe >= 4.0; rpe -= 0.5 {
			got := Intensity(reps, f(rpe))
			if got > previous {
				t.Errorf("%d reps: RPE %.1f (%.3f) is heavier than the RPE above it (%.3f)", reps, rpe, got, previous)
			}
			if got <= 0 || got > 1 {
				t.Errorf("%d reps @ RPE %.1f: intensity %.3f out of range", reps, rpe, got)
			}
			previous = got
		}
	}
	for rpe := 10.0; rpe >= 5.0; rpe -= 0.5 {
		previous := math.Inf(1)
		for reps := 1; reps <= 20; reps++ {
			got := Intensity(reps, f(rpe))
			if got > previous {
				t.Errorf("RPE %.1f: %d reps (%.3f) is heavier than one rep fewer (%.3f)", rpe, reps, got, previous)
			}
			previous = got
		}
	}
}

// TestSingleAtRPE10IsTheMax is the anchor the whole chart hangs off: one rep
// taken to failure is, by definition, the one-rep max.
func TestSingleAtRPE10IsTheMax(t *testing.T) {
	got := Load(Prescription{Mode: ModeRPE, Reps: 1, RPE: f(10)}, f(200), 0)
	if got.Load != 200 {
		t.Errorf("1 rep @ RPE 10 off a 200kg max = %.1f, want 200", got.Load)
	}
	if got.Percentage != 100 {
		t.Errorf("1 rep @ RPE 10 = %.1f%%, want 100%%", got.Percentage)
	}
}

// TestPercentageModeUsesMembersOwn1RM covers the sets the spreadsheet leaves
// without an RPE ("?"), which keep their authored percentage but must still
// follow the member's own max rather than the file author's.
func TestPercentageModeUsesMembersOwn1RM(t *testing.T) {
	p := Prescription{Mode: ModePercentage, Reps: 3, Percentage: f(82)}

	if got := Load(p, f(95), 0); got.Load != 77.9 {
		t.Errorf("82%% of 95kg = %.1f, want 77.9", got.Load)
	}
	if got := Load(p, f(140), 0); got.Load != 114.8 {
		t.Errorf("82%% of 140kg = %.1f, want 114.8", got.Load)
	}
}

func TestUnknownWithoutOneRM(t *testing.T) {
	for _, mode := range []string{ModeRPE, ModePercentage} {
		got := Load(Prescription{Mode: mode, Reps: 3, RPE: f(8), Percentage: f(80)}, nil, 2.5)
		if got.Known {
			t.Errorf("%s: expected unknown load without a recorded 1RM", mode)
		}
	}
	// A zero or negative max is as good as none - it would otherwise scale the
	// whole program down to nothing.
	if Load(Prescription{Mode: ModeRPE, Reps: 3, RPE: f(8)}, f(0), 2.5).Known {
		t.Error("expected unknown load for a 0kg 1RM")
	}
}

func TestAbsoluteAndBodyweight(t *testing.T) {
	abs := Load(Prescription{Mode: ModeAbsolute, Reps: 8, AbsoluteLoad: f(46)}, nil, 2.5)
	if !abs.Known || abs.Load != 46 {
		t.Errorf("absolute load = %+v, want 46 and known", abs)
	}
	if abs.Percentage != 0 {
		t.Errorf("absolute load should not report a percentage, got %.1f", abs.Percentage)
	}

	bw := Load(Prescription{Mode: ModeBodyweight, Reps: 5}, nil, 2.5)
	if !bw.Known || bw.Load != 0 {
		t.Errorf("bodyweight load = %+v, want 0 and known", bw)
	}
}

func TestRoundToIncrement(t *testing.T) {
	cases := []struct{ in, increment, want float64 }{
		{77.9, 2.5, 77.5},
		{79.0, 2.5, 80.0},
		{182.4, 2.5, 182.5},
		{100.0, 2.5, 100.0},
		{77.9, 0, 77.9}, // no increment configured: leave it alone
		{77.94, 1, 78},
	}
	for _, c := range cases {
		if got := RoundToIncrement(c.in, c.increment); got != c.want {
			t.Errorf("RoundToIncrement(%.2f, %.1f) = %.2f, want %.2f", c.in, c.increment, got, c.want)
		}
	}
}

// TestE1RMFromPerceivedRPE is the member-facing direction: they log what they
// actually lifted and how hard it felt, and that becomes the estimate their
// coach reads.
func TestE1RMFromPerceivedRPE(t *testing.T) {
	// Prescribed off a 95kg max but it moved like an RPE 6 instead of the
	// RPE 8 asked for: the estimate must come out above 95.
	prescribed := Load(Prescription{Mode: ModeRPE, Reps: 3, RPE: f(8)}, f(95), 0)
	if got := E1RM(prescribed.Load, 3, f(6)); got <= 95 {
		t.Errorf("e1RM = %.1f, want above the 95kg max it was prescribed from", got)
	}
	if got := E1RM(0, 5, f(8)); got != 0 {
		t.Errorf("bodyweight set should have no e1RM, got %.1f", got)
	}
	if got := E1RM(100, 0, f(8)); got != 0 {
		t.Errorf("a set with no reps should have no e1RM, got %.1f", got)
	}
}

// TestMissingRPEReadsAsFailure matches how the spreadsheet's e1RM column
// treats a blank RPE.
func TestMissingRPEReadsAsFailure(t *testing.T) {
	if Intensity(5, nil) != Intensity(5, f(10)) {
		t.Error("a set with no prescribed RPE should read as taken to failure")
	}
}
