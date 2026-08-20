package handlers

import (
	"testing"

	"strong-fish-api/internal/loadcalc"
	"strong-fish-api/internal/models"
)

// The catalog as these tests see it: the three lifts a member records, keyed
// the way FindMainLifts keys them.
func mainLiftFixtures() map[string]models.Exercise {
	return map[string]models.Exercise{
		models.CategorySquat:    {ID: "squat-id", Slug: "squat", Category: models.CategorySquat, Main: true},
		models.CategoryBench:    {ID: "bench-id", Slug: "bench", Category: models.CategoryBench, Main: true},
		models.CategoryDeadlift: {ID: "deadlift-id", Slug: "deadlift", Category: models.CategoryDeadlift, Main: true},
	}
}

// TestMemberOneRMPrefersWhatWasRecorded pins the order the answer is looked
// for in. A member who has actually tested the movement must be loaded off
// that, and a coach who set a reference on the catalog entry must be obeyed -
// the name is only consulted when neither exists.
func TestMemberOneRMPrefersWhatWasRecorded(t *testing.T) {
	lifts := mainLiftFixtures()
	maxes := map[string]float64{
		"paused-dl-id": 180, // tested for the movement itself
		"squat-id":     200,
		"deadlift-id":  220,
	}

	cases := []struct {
		name     string
		set      models.ProgramSet
		want     float64
		wantFrom string
	}{
		{
			name: "their own max for the movement wins",
			set: models.ProgramSet{
				ExerciseID: "paused-dl-id", ExerciseSlug: "paused-deadlift",
				LoadMode: loadcalc.ModeRPE,
			},
			want: 180, wantFrom: "paused-dl-id",
		},
		{
			name: "the reference a coach set wins over the name",
			set: models.ProgramSet{
				ExerciseID: "odd-id", ExerciseSlug: "highbar-squat",
				ExerciseOneRMRef: models.CategoryDeadlift, LoadMode: loadcalc.ModeRPE,
			},
			want: 220, wantFrom: "deadlift-id",
		},
		{
			name: "with neither, the name decides",
			set: models.ProgramSet{
				ExerciseID: "highbar-id", ExerciseSlug: "highbar-squat",
				ExerciseLabels: map[string]string{"en": "Highbar squat"}, LoadMode: loadcalc.ModeRPE,
			},
			want: 200, wantFrom: "squat-id",
		},
		{
			name: "the label carries it when the slug does not",
			set: models.ProgramSet{
				ExerciseID: "mvt-1", ExerciseSlug: "mouvement-1",
				ExerciseLabels: map[string]string{"fr": "Soulevé de terre avec pause"}, LoadMode: loadcalc.ModeRPE,
			},
			want: 220, wantFrom: "deadlift-id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oneRM, from := memberOneRM(tc.set, maxes, lifts)
			if oneRM == nil {
				t.Fatalf("no 1RM resolved, want %v", tc.want)
			}
			if *oneRM != tc.want {
				t.Errorf("1RM = %v, want %v", *oneRM, tc.want)
			}
			if from != tc.wantFrom {
				t.Errorf("resolved against %q, want %q", from, tc.wantFrom)
			}
		})
	}
}

// TestMemberOneRMReportsWhatIsMissing covers the other half: when the matched
// lift has no max either, the prompt has to name *that* lift rather than the
// variation, because recording a max for the variation is not what unlocks the
// load.
func TestMemberOneRMReportsWhatIsMissing(t *testing.T) {
	lifts := mainLiftFixtures()
	empty := map[string]float64{}

	set := models.ProgramSet{
		ExerciseID: "tempo-id", ExerciseSlug: "tempo-squat-3-3-0",
		ExerciseLabels: map[string]string{"en": "Tempo squat 3:3:0"}, LoadMode: loadcalc.ModeRPE,
	}
	oneRM, from := memberOneRM(set, empty, lifts)
	if oneRM != nil {
		t.Errorf("resolved a 1RM of %v from an empty record", *oneRM)
	}
	if from != "squat-id" {
		t.Errorf("missing max reported against %q, want the squat", from)
	}

	// A movement the table deliberately keeps out stays on its own: recording
	// a squat max must not start prescribing goblet squats at 70% of it.
	goblet := models.ProgramSet{
		ExerciseID: "goblet-id", ExerciseSlug: "goblet-squat",
		ExerciseLabels: map[string]string{"en": "Goblet squat"}, LoadMode: loadcalc.ModeRPE,
	}
	if _, from := memberOneRM(goblet, map[string]float64{"squat-id": 200}, lifts); from != "goblet-id" {
		t.Errorf("goblet squat resolved against %q, want its own movement", from)
	}
}
