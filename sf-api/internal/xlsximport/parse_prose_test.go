package xlsximport

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

// The prose layout is tested against workbooks built here rather than against
// the coaches' own files: those are somebody's paid programming, and this
// repository is public. What is reproduced is the *shape* - the punctuation,
// the spacing and the wording the real files use - with invented movements and
// numbers, which is what the parser actually reads.

// buildProse writes a one-sheet workbook from a grid of cells.
func buildProse(t *testing.T, rows [][]string) []byte {
	t.Helper()

	file := excelize.NewFile()
	defer file.Close()

	for r, row := range rows {
		for c, value := range row {
			if value == "" {
				continue
			}
			cell, err := excelize.CoordinatesToCellName(c+1, r+1)
			if err != nil {
				t.Fatalf("addressing r%dc%d: %v", r, c, err)
			}
			if err := file.SetCellStr("Sheet1", cell, value); err != nil {
				t.Fatalf("writing %s: %v", cell, err)
			}
		}
	}

	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		t.Fatalf("writing the workbook: %v", err)
	}
	return buf.Bytes()
}

// frenchProse is the layout as the French files write it: a week and its RPE
// down the first column, sessions across, and each session a column of
// sentences. The odd spacing is deliberate - see the nbsp test below.
func frenchProse() [][]string {
	return [][]string{
		{"PROGRAMME 2 SEMAINES"},
		{},
		{"Semaine 1", "Mardi Séance 1 :", "", "Jeudi Séance 2"},
		{"RPE 7"},
		{"", "Squat :", "", "Bench :"},
		{"", "LWU 1 X 5 : 70kg", "", "LWU 1 X 6 : 35kg"},
		{"", "Top set 3 X 5 : 85kg", "", "Top set 3 X 6 : 45kg"},
		{"", "Back off : 1 X 5 : 80kg", "", ""},
		{"", "Renfo :", "", "Renfo :"},
		{"", "Tirage vertical", "", "Biceps sur banc"},
		{"", "3 x 8-12 reps RPE 7", "", "3 x 8-12 reps RPE 7"},
		{},
		{"Semaine 2", "Mardi Séance 1 :", "", "Jeudi Séance 2"},
		{"RPE 7,5/8"},
		{"", "Squat", "", "Bench"},
		{"", "LWU 1 X 5 : 75kg", "", "LWU 1 X 6 : 40kg"},
		{"", "Top set 2 X 5 : 90-95kg", "", "Top set 3 X 6 : 47,5kg"},
		{"", "Back off 1 X 5 : 85k", "", ""},
	}
}

func parseProse(t *testing.T, rows [][]string) *ParsedProgram {
	t.Helper()
	program, err := Parse(buildProse(t, rows))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	return program
}

// findDay returns the day of a week, by its position within that week.
func findDay(t *testing.T, program *ParsedProgram, week, day int) ParsedDay {
	t.Helper()
	for _, candidate := range program.Days {
		if candidate.Week == week && candidate.Day == day {
			return candidate
		}
	}
	t.Fatalf("no week %d day %d among %d days", week, day, len(program.Days))
	return ParsedDay{}
}

// TestProseSetsFollowInOrder is the instruction in one assertion: the last
// warm-up, the top sets and the back-off become sets in the order they are
// written, and "3 X 5" is three sets of five rather than one row saying three.
func TestProseSetsFollowInOrder(t *testing.T) {
	program := parseProse(t, frenchProse())
	day := findDay(t, program, 1, 1)

	// 1 warm-up + 3 top sets + 1 back-off, then the accessory's 3.
	if len(day.Sets) != 8 {
		t.Fatalf("read %d sets, want 8", len(day.Sets))
	}

	want := []struct {
		exercise string
		reps     int
		notes    string
		part     int
	}{
		{"Squat", 5, "LWU 70kg", 1},
		{"Squat", 5, "Top set 85kg", 1},
		{"Squat", 5, "Top set 85kg", 1},
		{"Squat", 5, "Top set 85kg", 1},
		{"Squat", 5, "Back off 80kg", 1},
		{"Tirage vertical", 8, "8-12 reps", 2},
	}
	for i, expected := range want {
		set := day.Sets[i]
		if set.ExerciseName != expected.exercise {
			t.Errorf("set %d is %q, want %q", i+1, set.ExerciseName, expected.exercise)
		}
		if set.Reps != expected.reps {
			t.Errorf("set %d has %d reps, want %d", i+1, set.Reps, expected.reps)
		}
		if set.Notes != expected.notes {
			t.Errorf("set %d notes = %q, want %q", i+1, set.Notes, expected.notes)
		}
		// The accessory work is a part of its own, the way the tabular formats
		// separate main work from what follows it.
		if set.Part != expected.part {
			t.Errorf("set %d is in part %d, want %d", i+1, set.Part, expected.part)
		}
		if set.Position != i+1 {
			t.Errorf("set %d has position %d", i+1, set.Position)
		}
	}
}

// TestProseIgnoresTheLoadWhenAnRPEIsGiven covers the instruction's other rule.
// The week states an RPE, so that is what each set is stored as, and the kilos
// the coach wrote stay in the notes rather than becoming this member's weight.
func TestProseIgnoresTheLoadWhenAnRPEIsGiven(t *testing.T) {
	program := parseProse(t, frenchProse())

	for _, day := range program.Days {
		for _, set := range day.Sets {
			if set.RPE == nil {
				t.Errorf("%s in %q has no RPE", set.ExerciseName, day.Title)
			}
			if set.AbsoluteLoad != nil {
				t.Errorf("%s in %q kept a load of %.1f despite an RPE",
					set.ExerciseName, day.Title, *set.AbsoluteLoad)
			}
		}
	}

	// "RPE 7,5/8" is a range: the lower end is the one a lifter can always
	// meet, and the comma is a decimal point in these files.
	second := findDay(t, program, 2, 1)
	if got := *second.Sets[0].RPE; got != 7.5 {
		t.Errorf("week 2 is RPE %.2f, want 7.5", got)
	}
}

// TestProseReadsTheAwkwardWritings covers the punctuation the real files
// actually contain: the same workbook writes a back-off three different ways
// and ends a load "kg", "k" or as a range.
func TestProseReadsTheAwkwardWritings(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		reps  int
		count int
		notes string
	}{
		{"a colon after the label", "Back off : 1 X 5 : 80kg", 5, 1, "Back off 80kg"},
		{"no colon after the label", "Back off 1 X 5 : 80kg", 5, 1, "Back off 80kg"},
		{"two spaces after the label", "Back off  1 X 5 : 80kg", 5, 1, "Back off 80kg"},
		{"no space before the colon", "LWU 1 X 2: 90kg", 2, 1, "LWU 90kg"},
		{"no space after the colon", "Back off : 1 X 2 :100kg", 2, 1, "Back off 100kg"},
		{"a lowercase x", "Top set 1 x 1 : 105kg", 1, 1, "Top set 105kg"},
		{"kilos written k", "Back off 2 X 5 : 100k", 5, 2, "Back off 100kg"},
		{"a load range takes the lower end", "Top set 2 X 3 : 80-85kg", 3, 2, "Top set 80kg"},
		{"a decimal comma", "Top set 1 X 6 : 47,5kg", 6, 1, "Top set 47.5kg"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program := parseProse(t, [][]string{
				{"Semaine 1", "Séance 1"},
				{"RPE 8"},
				{"", "Squat :"},
				{"", tc.line},
			})
			day := findDay(t, program, 1, 1)

			if len(day.Sets) != tc.count {
				t.Fatalf("%q made %d sets, want %d", tc.line, len(day.Sets), tc.count)
			}
			if day.Sets[0].Reps != tc.reps {
				t.Errorf("%q has %d reps, want %d", tc.line, day.Sets[0].Reps, tc.reps)
			}
			if day.Sets[0].Notes != tc.notes {
				t.Errorf("%q kept notes %q, want %q", tc.line, day.Sets[0].Notes, tc.notes)
			}
		})
	}
}

// TestProseReadsNonBreakingSpaces covers what actually broke first, and what
// nothing else here would catch: cells typed in a word processor are full of
// no-break and narrow spaces, and Go's `\s` matches neither. Every pattern in
// this parser failed on them, and the lines fell through to be read as prose -
// so the file imported, quietly, with four fifths of its sets missing.
func TestProseReadsNonBreakingSpaces(t *testing.T) {
	program := parseProse(t, [][]string{
		{"Semaine\u00a01", "Mardi\u202fSéance 1\u202f"},
		{"RPE\u00a07"},
		{"", "Squat\u202f:"},
		{"", "LWU\u00a01 X 5\u202f: 70kg"},
		{"", "Top set\u00a03 X 5\u202f: 85kg"},
		{"", "Back off\u202f:\u00a01 X 5\u202f: 80kg"},
	})

	day := findDay(t, program, 1, 1)
	if len(day.Sets) != 5 {
		t.Fatalf("read %d sets, want 5 - the exotic spacing was not normalized", len(day.Sets))
	}
	if day.Title != "Mardi Séance 1" {
		t.Errorf("day title = %q, want the spacing cleaned out of it", day.Title)
	}
	if day.Sets[0].ExerciseName != "Squat" {
		t.Errorf("exercise = %q, want %q", day.Sets[0].ExerciseName, "Squat")
	}
}

// TestProseReadsEnglishAndMixed covers the instruction's "english or french or
// mixed": the same sheet can name its weeks in one language and its sets in
// another, and neither is a special case.
func TestProseReadsEnglishAndMixed(t *testing.T) {
	program := parseProse(t, [][]string{
		{"Week 1", "Monday Session 1", "", "Jeudi Séance 2"},
		{"RPE 7.5", "Back day", "", "Séance Jambes"},
		{"", "Deadlift:", "", "Squat :"},
		{"", "LWU 1 x 5 : 70kg", "", "LWU 1 X 5 : 50kg"},
		{"", "Top set 2 x 5 : 85kg", "", "Top set 3 X 5 : 60kg"},
		{"", "Accessories:", "", "Renfo :"},
		{"", "Lat pulldown", "", "Tirage vertical"},
		{"", "3 x 8-12 reps RPE 7", "", "3 x 8-12 reps RPE 7"},
	})

	if program.Weeks != 1 {
		t.Errorf("read %d weeks, want 1", program.Weeks)
	}

	english := findDay(t, program, 1, 1)
	// The subtitle beside the week's RPE says what the session is for.
	if english.Title != "Monday Session 1 - Back day" {
		t.Errorf("english day title = %q", english.Title)
	}
	if len(english.Sets) != 6 {
		t.Errorf("english day has %d sets, want 6", len(english.Sets))
	}
	if english.Sets[0].ExerciseName != "Deadlift" {
		t.Errorf("english exercise = %q", english.Sets[0].ExerciseName)
	}
	// "Accessories:" is a heading, not a movement, exactly as "Renfo :" is.
	if last := english.Sets[len(english.Sets)-1]; last.ExerciseName != "Lat pulldown" || last.Part != 2 {
		t.Errorf("english accessory = %q part %d, want %q part 2", last.ExerciseName, last.Part, "Lat pulldown")
	}

	french := findDay(t, program, 1, 2)
	if french.Sets[0].ExerciseName != "Squat" {
		t.Errorf("french exercise = %q", french.Sets[0].ExerciseName)
	}
}

// TestProseKeepsSentencesOutOfTheCatalogue covers a line the coach wrote to the
// athlete rather than a movement. Reading it as an exercise would put it in the
// shared catalogue, where every club would then see it in their autocomplete.
func TestProseKeepsSentencesOutOfTheCatalogue(t *testing.T) {
	program := parseProse(t, [][]string{
		{"Semaine 1", "Séance 1"},
		{"RPE 7"},
		{"", "Squat :"},
		{"", "Top set 1 X 5 : 85kg"},
		{"", "Ton entrainement de body à la suite"},
	})

	for _, exercise := range program.Exercises {
		if exercise.Name != "Squat" {
			t.Errorf("a sentence became the exercise %q", exercise.Name)
		}
	}

	day := findDay(t, program, 1, 1)
	if len(day.Sets) != 1 {
		t.Fatalf("read %d sets, want 1", len(day.Sets))
	}
	// Kept, not dropped: it is an instruction about the session.
	if day.Sets[0].Notes != "Top set 85kg Ton entrainement de body à la suite" {
		t.Errorf("the sentence was lost: notes = %q", day.Sets[0].Notes)
	}
}

// TestProseNumbersADeloadWeek covers the week this format writes without a
// number, which is where the block ends.
func TestProseNumbersADeloadWeek(t *testing.T) {
	program := parseProse(t, [][]string{
		{"Semaine 1", "Séance 1"},
		{"RPE 7"},
		{"", "Squat :"},
		{"", "Top set 1 X 5 : 85kg"},
		{"DELOAD", "Séance 1"},
		{"RPE 6"},
		{"", "Squat :"},
		{"", "Top set 1 X 5 : 60kg"},
	})

	if program.Weeks != 2 {
		t.Fatalf("read %d weeks, want 2 - the deload is the week after the last", program.Weeks)
	}
	deload := findDay(t, program, 2, 1)
	if got := *deload.Sets[0].RPE; got != 6 {
		t.Errorf("the deload week is RPE %.1f, want 6", got)
	}
}

// TestProseLayoutIsNotConfusedWithTheTabularOnes guards the dispatch: three
// formats now arrive through the same upload button, and picking the wrong
// reader produces a plausible-looking wrong program rather than an error.
func TestProseLayoutIsNotConfusedWithTheTabularOnes(t *testing.T) {
	if isProseSheet([][]string{
		{"WEEK 1 DAY 2"},
		{"Exercice", "Reps", "RPE", "Percentage", "Load", "e1RM", "Part"},
		{"Squat", "3", "5", "79", "140", "175", "1"},
	}) {
		t.Error("a week-per-sheet table was read as prose")
	}

	// A week marker with nothing beside it is a title, not a session grid.
	if isProseSheet([][]string{{"Semaine 1"}, {"Exercice", "Reps"}}) {
		t.Error("a lone week marker was read as prose")
	}

	if !isProseSheet(frenchProse()) {
		t.Error("the prose layout was not recognized")
	}
}
