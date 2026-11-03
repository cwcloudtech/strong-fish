package stance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMatchLiftReadsFreeText covers the one judgement the integration makes on
// its own: Stance stores a free-text exercise, and the charts key on three
// lifts. Getting it wrong drops a set off the screen silently, so the shapes a
// member actually types are pinned.
func TestMatchLiftReadsFreeText(t *testing.T) {
	cases := map[string]string{
		"Comp Squat":      LiftSquat,
		"Back Squat":      LiftSquat,
		"Bench Press":     LiftBench,
		"Paused Bench":    LiftBench,
		"Deadlift (comp)": LiftDeadlift,
		"Dead Lift":       LiftDeadlift,
		"Overhead Press":  "",
		"Barbell Row":     "",
	}
	for exercise, want := range cases {
		if got := matchLift(exercise); got != want {
			t.Errorf("matchLift(%q) = %q, want %q", exercise, got, want)
		}
	}
}

// TestLastSetsKeepsTheNewestPerLift covers the read's whole job: newest first,
// one set per lift. A second, older squat in the feed must not shadow the
// first, and a lift never tracked must be absent rather than a zero row.
func TestLastSetsKeepsTheNewestPerLift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Two squats (newest first), one bench, no deadlift.
		_, _ = w.Write([]byte(`[
			{"exercise_raw":"Comp Squat","weight_kg":180,"reps":3,"rpe":8,"predicted_rpe":8.5,
			 "set_ended_at_epoch_ms":200,"repetitions":[{"rep_index":0,"mean_concentric_velocity":0.4}]},
			{"exercise_raw":"Bench Press","weight_kg":120,"reps":2,"set_ended_at_epoch_ms":150,
			 "repetitions":[{"rep_index":0,"mean_concentric_velocity":0.2}]},
			{"exercise_raw":"Back Squat","weight_kg":170,"reps":5,"set_ended_at_epoch_ms":100,
			 "repetitions":[{"rep_index":0,"mean_concentric_velocity":0.5}]}
		]`))
	}))
	defer server.Close()

	client := New(Config{SupabaseURL: server.URL, SupabaseAnonKey: "anon"})
	sets, err := client.LastSets(context.Background(), "token", "user")
	if err != nil {
		t.Fatalf("LastSets: %v", err)
	}

	byLift := map[string]Set{}
	for _, set := range sets {
		if _, seen := byLift[set.Lift]; seen {
			t.Fatalf("lift %q returned twice", set.Lift)
		}
		byLift[set.Lift] = set
	}
	if squat, ok := byLift[LiftSquat]; !ok || squat.WeightKg != 180 {
		t.Errorf("squat should be the newest (180kg), got %+v", byLift[LiftSquat])
	}
	if _, ok := byLift[LiftBench]; !ok {
		t.Error("bench should be present")
	}
	if _, ok := byLift[LiftDeadlift]; ok {
		t.Error("deadlift was never tracked and should be absent")
	}
}

// TestPairRefusesBadCredentials covers the one error the member can act on: a
// wrong password has to come back as ErrRefused, not a generic failure, or the
// screen cannot tell them to check what they typed.
func TestPairRefusesBadCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid login credentials"}`))
	}))
	defer server.Close()

	client := New(Config{SupabaseURL: server.URL, SupabaseAnonKey: "anon"})
	if _, err := client.Pair(context.Background(), "a@b.co", "wrong"); !isRefused(err) {
		t.Errorf("want ErrRefused, got %v", err)
	}
}

func isRefused(err error) bool {
	for err != nil {
		if err == ErrRefused {
			return true
		}
		type wrapped interface{ Unwrap() error }
		if w, ok := err.(wrapped); ok {
			err = w.Unwrap()
			continue
		}
		return false
	}
	return false
}
