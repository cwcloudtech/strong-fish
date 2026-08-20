package handlers

import (
	"testing"

	"strong-fish-api/internal/loadcalc"
	"strong-fish-api/internal/models"
)

func logged(rpe *float64, e1rm float64) models.SetLog {
	return models.SetLog{ActualRPE: rpe, E1RM: e1rm, Done: true}
}

func rpe(value float64) *float64 { return &value }

// TestSessionMaxesFollowsTheSession covers what a logged set changes and what
// it leaves alone. Each of these is a load somebody puts on a bar, so being
// wrong here is worse than showing nothing at all.
func TestSessionMaxesFollowsTheSession(t *testing.T) {
	const squat, bench = "squat-id", "bench-id"

	session := newSessionMaxes()
	session.enter("monday")

	if _, ok := session.forLift(squat); ok {
		t.Error("a lift with nothing logged already carries a demonstrated max")
	}

	// A top single that came out heavier than prescribed.
	session.record(squat, logged(rpe(9), 182.5))

	shown, ok := session.forLift(squat)
	if !ok || shown != 182.5 {
		t.Fatalf("squat resolves against %v (%v), want 182.5", shown, ok)
	}
	if _, ok := session.forLift(bench); ok {
		t.Error("a squat single moved the bench work")
	}
	if _, ok := session.forLift(""); ok {
		t.Error("a set with no 1RM source picked up somebody else's max")
	}

	// The next session starts from the 1RM on file again.
	session.enter("wednesday")
	if _, ok := session.forLift(squat); ok {
		t.Error("yesterday's single is still loading today's squats")
	}
}

// TestSessionMaxesNeedsAPerceivedRPE is the guard that keeps a member who
// merely ticked a set off from having every later load recomputed: with no RPE,
// E1RM reads the set as an all-out effort.
func TestSessionMaxesNeedsAPerceivedRPE(t *testing.T) {
	session := newSessionMaxes()
	session.enter("monday")

	session.record("squat-id", logged(nil, 200))
	if _, ok := session.forLift("squat-id"); ok {
		t.Error("a set logged without an RPE moved the rest of the session")
	}

	// Nor does a set with no weight behind it - a bodyweight movement
	// estimates no max at all.
	session.record("squat-id", logged(rpe(8), 0))
	if _, ok := session.forLift("squat-id"); ok {
		t.Error("a set that demonstrated no max was recorded anyway")
	}
}

// TestAutoregulationMovesTheBackOffs is the arithmetic the feature exists for,
// end to end through loadcalc: a top set that felt harder than prescribed
// leaves the back-offs lighter, and one that felt easier leaves them heavier.
func TestAutoregulationMovesTheBackOffs(t *testing.T) {
	const oneRM = 200.0
	backOff := loadcalc.Prescription{Mode: loadcalc.ModeRPE, Reps: 5, RPE: rpe(7)}

	prescribed := loadcalc.Load(backOff, ptr(oneRM), 2.5).Load

	cases := []struct {
		name       string
		feltRPE    float64
		wantLess   bool
		wantEqualP bool
	}{
		{"the single felt harder than asked", 9, true, false},
		{"it felt exactly as asked", 8, false, true},
		{"it felt easy", 7, false, false},
	}

	// The prescription for the top set: a single at RPE 8.
	top := loadcalc.Load(loadcalc.Prescription{Mode: loadcalc.ModeRPE, Reps: 1, RPE: rpe(8)}, ptr(oneRM), 2.5)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			felt := tc.feltRPE
			demonstrated := loadcalc.E1RM(top.Load, 1, &felt)

			session := newSessionMaxes()
			session.enter("monday")
			session.record("squat-id", models.SetLog{ActualRPE: &felt, E1RM: demonstrated})

			shown, ok := session.forLift("squat-id")
			if !ok {
				t.Fatal("nothing demonstrated")
			}
			got := loadcalc.Load(backOff, &shown, 2.5).Load

			switch {
			case tc.wantEqualP && got != prescribed:
				t.Errorf("back-off = %v, want the prescribed %v when the set felt as asked", got, prescribed)
			case tc.wantLess && got >= prescribed:
				t.Errorf("back-off = %v, want less than the prescribed %v", got, prescribed)
			case !tc.wantLess && !tc.wantEqualP && got <= prescribed:
				t.Errorf("back-off = %v, want more than the prescribed %v", got, prescribed)
			}
		})
	}
}

func ptr(v float64) *float64 { return &v }
