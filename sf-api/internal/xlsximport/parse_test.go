package xlsximport

import (
	"os"
	"path/filepath"
	"testing"

	"strong-fish-api/internal/loadcalc"
)

// referenceProgram is the spreadsheet the import format was reverse-engineered
// from. It lives outside the module (it's the instruction's own asset, not a
// fixture), so a checkout without it skips these tests rather than failing.
const referenceProgram = "../../../ai-gen/assets/program.xlsx"

func loadReference(t *testing.T) *ParsedProgram {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(referenceProgram))
	if err != nil {
		t.Skipf("reference program not available (%v)", err)
	}
	program, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return program
}

func TestParseReferenceProgramStructure(t *testing.T) {
	program := loadReference(t)

	if program.Weeks != 5 {
		t.Errorf("weeks = %d, want 5", program.Weeks)
	}
	// Five weeks of four sessions each.
	if len(program.Days) != 20 {
		t.Errorf("days = %d, want 20", len(program.Days))
	}

	// Every week/day pair must be distinct: several title cells in the source
	// file were clobbered by a stray formula, and the fallback numbering has to
	// recover the right day rather than repeating one.
	seen := map[[2]int]string{}
	for _, day := range program.Days {
		key := [2]int{day.Week, day.Day}
		if previous, clash := seen[key]; clash {
			t.Errorf("week %d day %d parsed twice (%q and %q)", day.Week, day.Day, previous, day.Title)
		}
		seen[key] = day.Title
		if day.Week < 1 || day.Week > 5 || day.Day < 1 || day.Day > 4 {
			t.Errorf("day %q numbered week %d day %d, out of range", day.Title, day.Week, day.Day)
		}
		if len(day.Sets) == 0 {
			t.Errorf("week %d day %d has no sets", day.Week, day.Day)
		}
	}

	// The three sessions whose title cells are clobbered in the source file
	// (week 2 day 3, week 4 day 2, week 5 day 4) must still come through.
	for _, want := range [][2]int{{2, 3}, {4, 2}, {5, 4}} {
		if _, ok := seen[[2]int{want[0], want[1]}]; !ok {
			t.Errorf("week %d day %d is missing - its title cell is clobbered in the source file", want[0], want[1])
		}
	}
}

func TestParseReferenceOneRMs(t *testing.T) {
	program := loadReference(t)

	want := map[string]float64{"squat": 120, "bench": 95, "deadlift": 225}
	for slug, value := range want {
		if got := program.RefOneRMs[slug]; got != value {
			t.Errorf("reference 1RM for %s = %v, want %v", slug, got, value)
		}
	}
	// The refs sheet's "Total" summary row is not a lift.
	if _, ok := program.RefOneRMs["total"]; ok {
		t.Error("the refs sheet's Total row was imported as a 1RM")
	}
}

// TestResolvesReferenceLift is the inference that makes an imported program
// recompute per member: the spreadsheet never says a Larsen press is programmed
// off the bench max (its formula just points at a cell), so it's recovered from
// the arithmetic.
func TestResolvesReferenceLift(t *testing.T) {
	program := loadReference(t)

	byslug := map[string]ParsedExercise{}
	for _, ex := range program.Exercises {
		byslug[ex.Slug] = ex
	}

	cases := map[string]string{
		"squat":             "squat",
		"tempo-squat-3-1-3": "squat",
		"bench":             "bench",
		"2ct-paused-bench":  "bench",
		"larsen-2ct-paused": "bench",
		"larsen-press":      "bench",
		"close-grip-bench":  "bench",
		"larsen-close-grip": "bench",
		"deadlift":          "deadlift",
		"paused-deadlift":   "deadlift",
	}
	for slug, want := range cases {
		ex, ok := byslug[slug]
		if !ok {
			t.Errorf("%s not found in the parsed exercises", slug)
			continue
		}
		if ex.OneRMRef != want {
			t.Errorf("%s is programmed off %q, want %q", slug, ex.OneRMRef, want)
		}
	}

	// Accessories are loaded in absolute kilos or by bodyweight - they must not
	// be attached to a competition lift's max.
	for _, slug := range []string{"pull-ups", "dips", "lateral-raises", "strict-curl", "hammer-curl"} {
		if ex, ok := byslug[slug]; ok && ex.OneRMRef != "" {
			t.Errorf("accessory %s was attached to the %s max", slug, ex.OneRMRef)
		}
	}

	// Pull-ups and dips carry a 0 load in the source: bodyweight, not a 0kg
	// barbell.
	for _, slug := range []string{"pull-ups", "dips"} {
		if ex, ok := byslug[slug]; ok && !ex.Bodyweight {
			t.Errorf("%s should be a bodyweight movement", slug)
		}
	}
}

// TestLoadModes checks each of the spreadsheet's three ways of prescribing a
// set is recognized.
func TestLoadModes(t *testing.T) {
	program := loadReference(t)

	counts := map[string]int{}
	for _, day := range program.Days {
		for _, set := range day.Sets {
			counts[set.LoadMode]++

			switch set.LoadMode {
			case loadcalc.ModeRPE:
				if set.RPE == nil || set.Percentage == nil {
					t.Errorf("%s: rpe set missing rpe or percentage", set.ExerciseSlug)
				}
			case loadcalc.ModePercentage:
				// The spreadsheet's "?" rows: a percentage, deliberately no RPE.
				if set.RPE != nil || set.Percentage == nil {
					t.Errorf("%s: percentage set should have a percentage and no rpe", set.ExerciseSlug)
				}
			case loadcalc.ModeAbsolute:
				if set.AbsoluteLoad == nil || *set.AbsoluteLoad <= 0 {
					t.Errorf("%s: absolute set has no weight", set.ExerciseSlug)
				}
			case loadcalc.ModeBodyweight:
				if set.AbsoluteLoad != nil {
					t.Errorf("%s: bodyweight set should carry no weight", set.ExerciseSlug)
				}
			default:
				t.Errorf("%s: unknown load mode %q", set.ExerciseSlug, set.LoadMode)
			}

			if set.Reps <= 0 {
				t.Errorf("%s: set with %d reps", set.ExerciseSlug, set.Reps)
			}
		}
	}

	for _, mode := range []string{loadcalc.ModeRPE, loadcalc.ModePercentage, loadcalc.ModeAbsolute, loadcalc.ModeBodyweight} {
		if counts[mode] == 0 {
			t.Errorf("no %s sets parsed - the reference program contains all four kinds", mode)
		}
	}
}

// TestNoTitleRowsImportedAsExercises guards the tolerant block detection: a
// day title must never end up looking like a movement.
func TestNoTitleRowsImportedAsExercises(t *testing.T) {
	program := loadReference(t)

	for _, ex := range program.Exercises {
		if ex.Slug == "" {
			t.Error("an exercise was parsed with an empty slug")
		}
		// "week-1-day-2" and friends would mean a title row leaked into a
		// block's sets.
		if len(ex.Slug) >= 4 && ex.Slug[:4] == "week" {
			t.Errorf("day title %q was imported as an exercise", ex.Name)
		}
	}
}

func TestParseRejectsNonSpreadsheet(t *testing.T) {
	if _, err := Parse([]byte("this is not a workbook")); err == nil {
		t.Error("expected an error for a file that isn't a spreadsheet")
	}
}
