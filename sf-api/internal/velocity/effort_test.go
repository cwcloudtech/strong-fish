package velocity

import "testing"

func TestEstimateEffort(t *testing.T) {
	cases := []struct {
		name     string
		movement string
		velocity float64
		wantOK   bool
		wantRPE  float64
		wantRIR  float64
	}{
		// The tutorial's worked example: a squat last rep at 0.45 m/s against
		// MVT 0.30 and k 0.05 is three reps in reserve, RPE 7.
		{"squat worked example", "squat", 0.45, true, 7, 3},
		// The same set a touch slower - one rep left, RPE 9.
		{"squat near failure", "squat", 0.35, true, 9, 1},
		// At or below the MVT the bar is at the last possible rep: RPE 10.
		{"squat at failure", "squat", 0.30, true, 10, 0},
		{"squat past failure", "squat", 0.20, true, 10, 0},
		// Far from failure the estimate floors at RPE 5 rather than claim a
		// precise low number velocity cannot support.
		{"squat warm-up speed", "squat", 0.90, true, 5, 5},
		// Bench and deadlift read against their own MVT/k.
		{"bench", "bench", 0.26, true, 8, 2},
		{"deadlift", "deadlift", 0.26, true, 8, 2},
		// No model, no estimate.
		{"unknown movement", "press", 0.45, false, 0, 0},
		{"zero velocity", "squat", 0, false, 0, 0},
		{"negative velocity", "squat", -0.1, false, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := EstimateEffort(c.movement, c.velocity)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if got.RPE != c.wantRPE {
				t.Errorf("RPE = %v, want %v", got.RPE, c.wantRPE)
			}
			if got.RIR != c.wantRIR {
				t.Errorf("RIR = %v, want %v", got.RIR, c.wantRIR)
			}
			if got.RPE+got.RIR != 10 {
				t.Errorf("RPE+RIR = %v, want 10 (they are the same number two ways)", got.RPE+got.RIR)
			}
		})
	}
}
