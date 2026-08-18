package xlsximport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"strong-fish-api/internal/loadcalc"
)

// The spreadsheets the two import formats were reverse-engineered from. They
// live outside the module (they're the instruction's own assets, not
// fixtures), so a checkout without them skips these tests rather than failing.
const (
	referenceProgram      = "../../../ai-gen/assets/program_1.xlsx"
	referenceBlockProgram = "../../../ai-gen/assets/program_2.xlsx"
)

func loadProgram(t *testing.T, path string) *ParsedProgram {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Skipf("reference program not available (%v)", err)
	}
	program, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return program
}

func loadReference(t *testing.T) *ParsedProgram {
	t.Helper()
	return loadProgram(t, referenceProgram)
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

// --- block-per-sheet layout (program_2.xlsx) ---

func TestParseBlockProgramStructure(t *testing.T) {
	program := loadProgram(t, referenceBlockProgram)

	// Two block sheets, four W columns and four sessions each. The blocks are
	// consecutive, not parallel: the second block's W1 is program week 5.
	if program.Weeks != 8 {
		t.Errorf("weeks = %d, want 8", program.Weeks)
	}
	if len(program.Days) != 32 {
		t.Errorf("days = %d, want 32", len(program.Days))
	}

	seen := map[[2]int]string{}
	for _, day := range program.Days {
		key := [2]int{day.Week, day.Day}
		if previous, clash := seen[key]; clash {
			t.Errorf("week %d day %d parsed twice (%q and %q)", day.Week, day.Day, previous, day.Title)
		}
		seen[key] = day.Title
		if day.Week < 1 || day.Week > 8 || day.Day < 1 || day.Day > 4 {
			t.Errorf("day %q numbered week %d day %d, out of range", day.Title, day.Week, day.Day)
		}
		if len(day.Sets) == 0 {
			t.Errorf("week %d day %d has no sets", day.Week, day.Day)
		}
	}
}

// TestBlockWeeksRepeatThePrescription pins the defining property of this
// layout: the coach writes each session once and it runs every week of the
// block. Week 2 exists as its own set of days so a member can log it
// separately, and it has to prescribe exactly what week 1 did.
func TestBlockWeeksRepeatThePrescription(t *testing.T) {
	program := loadProgram(t, referenceBlockProgram)

	byKey := map[[2]int]ParsedDay{}
	for _, day := range program.Days {
		byKey[[2]int{day.Week, day.Day}] = day
	}

	for day := 1; day <= 4; day++ {
		first, ok := byKey[[2]int{1, day}]
		if !ok {
			t.Fatalf("week 1 day %d missing", day)
		}
		for week := 2; week <= 4; week++ {
			other := byKey[[2]int{week, day}]
			if len(other.Sets) != len(first.Sets) {
				t.Errorf("week %d day %d has %d sets, week 1 has %d", week, day, len(other.Sets), len(first.Sets))
				continue
			}
			for i := range first.Sets {
				if other.Sets[i].ExerciseSlug != first.Sets[i].ExerciseSlug ||
					other.Sets[i].Reps != first.Sets[i].Reps {
					t.Errorf("week %d day %d set %d = %s x%d, want %s x%d", week, day, i,
						other.Sets[i].ExerciseSlug, other.Sets[i].Reps,
						first.Sets[i].ExerciseSlug, first.Sets[i].Reps)
				}
			}
		}
	}
}

// TestBlockImportsNoAthleteLog guards the instruction that the W* columns are
// one athlete's feedback, not the program: a block sheet prescribes effort
// only, so nothing it imports may carry a weight.
func TestBlockImportsNoAthleteLog(t *testing.T) {
	program := loadProgram(t, referenceBlockProgram)

	for _, day := range program.Days {
		for _, set := range day.Sets {
			if set.AbsoluteLoad != nil {
				t.Errorf("%s: %s carries an absolute load of %g", day.Title, set.ExerciseSlug, *set.AbsoluteLoad)
			}
			if set.Percentage != nil {
				t.Errorf("%s: %s carries a percentage of %g", day.Title, set.ExerciseSlug, *set.Percentage)
			}
			if set.LoadMode != loadcalc.ModeRPE && set.LoadMode != loadcalc.ModeBodyweight {
				t.Errorf("%s: %s has load mode %q", day.Title, set.ExerciseSlug, set.LoadMode)
			}
		}
	}
}

// TestBlockExpandsSetsAndReps covers the "3 x 8" cell: a set count and a rep
// target, one prescribed set per count. "3 x AMRAP" has no rep number to work
// a load out of, so it keeps the coach's word instead of inventing one.
func TestBlockExpandsSetsAndReps(t *testing.T) {
	cases := []struct {
		cell      string
		count     int
		reps      int
		repsLabel string
		ok        bool
	}{
		{"1 x 3", 1, 3, "", true},
		{"3 x 3", 3, 3, "", true},
		{"4 x 10", 4, 10, "", true},
		{"3 x AMRAP", 3, 0, "AMRAP", true},
		{"5", 1, 5, "", true},
		{"", 0, 0, "", false},
		{"as it comes", 0, 0, "", false},
		// A mis-typed cell must not expand into hundreds of sets.
		{"300 x 5", 0, 0, "", false},
	}

	for _, c := range cases {
		count, reps, label, ok := parseSetsReps(c.cell)
		if ok != c.ok || count != c.count || reps != c.reps || label != c.repsLabel {
			t.Errorf("parseSetsReps(%q) = (%d, %d, %q, %t), want (%d, %d, %q, %t)",
				c.cell, count, reps, label, ok, c.count, c.reps, c.repsLabel, c.ok)
		}
	}
}

func TestBlockAmrapKeepsTheInstruction(t *testing.T) {
	program := loadProgram(t, referenceBlockProgram)

	found := false
	for _, day := range program.Days {
		for _, set := range day.Sets {
			if set.ExerciseSlug != "dips-tractions" {
				continue
			}
			found = true
			if set.Reps != 0 {
				t.Errorf("AMRAP set has reps = %d, want 0 (no rep target was prescribed)", set.Reps)
			}
			if !strings.Contains(set.Notes, "AMRAP") {
				t.Errorf("AMRAP set notes = %q, want the instruction kept", set.Notes)
			}
			if set.LoadMode != loadcalc.ModeBodyweight {
				t.Errorf("AMRAP set load mode = %q, want %q", set.LoadMode, loadcalc.ModeBodyweight)
			}
		}
	}
	if !found {
		t.Error("the AMRAP movement was not imported")
	}
}

// TestBlockNamesAreTidied covers the trailing colon these sheets end every
// movement with: "COMP.DEADLIFT: " and "COMP.DEADLIFT" are the same exercise,
// and importing both would split one movement's history in two.
func TestBlockNamesAreTidied(t *testing.T) {
	program := loadProgram(t, referenceBlockProgram)

	slugs := map[string]bool{}
	for _, exercise := range program.Exercises {
		if strings.HasSuffix(exercise.Name, ":") || exercise.Name != strings.TrimSpace(exercise.Name) {
			t.Errorf("exercise name %q was not tidied", exercise.Name)
		}
		if slugs[exercise.Slug] {
			t.Errorf("exercise slug %q collected twice", exercise.Slug)
		}
		slugs[exercise.Slug] = true
	}

	// The deadlift is written "COMP.DEADLIFT:" in one session and
	// "COMP.DEADLIFT: " in another; both must land on one entry.
	if !slugs["comp-deadlift"] {
		t.Errorf("comp-deadlift missing from %v", slugs)
	}
}

// TestBothFormatsAreDetected is the instruction's actual requirement: the same
// upload path takes either layout, told apart by the file's own shape.
func TestBothFormatsAreDetected(t *testing.T) {
	week := loadProgram(t, referenceProgram)
	block := loadProgram(t, referenceBlockProgram)

	// The week-per-sheet file is numbered by its own sheets and carries the
	// reference maxes its percentages were authored against.
	if len(week.RefOneRMs) == 0 {
		t.Error("the week-per-sheet program lost its reference 1RMs")
	}
	// The block file has no refs sheet at all, and prescribes no percentages.
	if len(block.RefOneRMs) != 0 {
		t.Errorf("the block program invented reference 1RMs: %v", block.RefOneRMs)
	}
	if block.Weeks <= week.Weeks {
		t.Errorf("block weeks = %d, week-per-sheet weeks = %d; the two files were parsed the same way",
			block.Weeks, week.Weeks)
	}
}
