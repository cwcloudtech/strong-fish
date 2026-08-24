package programxlsx

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programsheet"
)

func rpe(value float64) *float64 { return &value }

func fixture() (models.Program, []models.ProgramDay) {
	program := models.Program{Name: "Bloc force", ClubName: "Club du Nord"}
	days := []models.ProgramDay{
		{
			ID: "d1", Week: 1, Day: 1, Sets: []models.ProgramSet{
				{
					ID: "s1", Position: 0, ExerciseSlug: "squat",
					ExerciseLabels: map[string]string{"en": "Squat", "fr": "Squat"},
					Reps:           5, RPE: rpe(8), RoundedLoad: 162.5, LoadKnown: true,
					Notes: "Belt on",
					Log: &models.SetLog{
						Done: true, ActualReps: intp(5), ActualRPE: rpe(9),
						ActualLoad: rpe(162.5), E1RM: 187.9, Comment: "Harder than it looked",
					},
				},
				{
					ID: "s2", Position: 1, ExerciseSlug: "bench",
					ExerciseLabels: map[string]string{"en": "Bench press"},
					Reps:           3, RPE: rpe(7), LoadKnown: false,
				},
			},
		},
		// An empty session: a rest day the coach wrote down.
		{ID: "d2", Week: 1, Day: 2},
		{ID: "d3", Week: 2, Day: 1, Sets: []models.ProgramSet{
			{ID: "s3", Position: 0, ExerciseSlug: "deadlift",
				ExerciseLabels: map[string]string{"en": "Deadlift"}, Reps: 1, RPE: rpe(9)},
		}},
	}
	return program, days
}

func intp(value int) *int { return &value }

func open(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the workbook does not open: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

// TestRenderSheetPerWeek pins the shape of the workbook: a tab per week, named
// after it, in order. A coach opening this looks for "Week 3" as a tab, which
// is the only navigation a spreadsheet has.
func TestRenderSheetPerWeek(t *testing.T) {
	program, days := fixture()

	data, err := Render(program, days, Options{Locale: "en", MemberName: "Marie Dubois"})
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}

	file := open(t, data)
	sheets := file.GetSheetList()
	if len(sheets) != 2 {
		t.Fatalf("%d sheets (%v), want one per week", len(sheets), sheets)
	}
	if sheets[0] != "Week 1" || sheets[1] != "Week 2" {
		t.Errorf("sheets are %v, want [Week 1 Week 2]", sheets)
	}

	rows, err := file.GetRows("Week 1")
	if err != nil {
		t.Fatalf("reading the first sheet: %v", err)
	}
	flat := flatten(rows)
	for _, want := range []string{
		"Bloc force", "Marie Dubois  -  Club du Nord", "Week 1",
		"Squat", "RPE 8", "162.5 kg", "Belt on",
		"Bench press", "RPE 7",
		programsheet.RestLabel("en"),
	} {
		if !contains(flat, want) {
			t.Errorf("the sheet does not carry %q", want)
		}
	}

	// Without feedback the sheet is a prescription: nothing about what was
	// done belongs on it, even though the fixture's set carries a log.
	for _, unwanted := range []string{"Harder than it looked", "187.9 kg"} {
		if contains(flat, unwanted) {
			t.Errorf("a prescription sheet leaked feedback: %q", unwanted)
		}
	}
}

// TestRenderWithFeedback covers the sheet a lifter sends their coach: what was
// asked, and beside it what actually happened.
func TestRenderWithFeedback(t *testing.T) {
	program, days := fixture()

	data, err := Render(program, days, Options{Locale: "en", Feedback: true})
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}

	rows, err := open(t, data).GetRows("Week 1")
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	flat := flatten(rows)

	for _, want := range []string{
		"Done", "RPE felt", "e1RM", "Athlete's comment",
		"X", "9", "187.9 kg", "Harder than it looked",
	} {
		if !contains(flat, want) {
			t.Errorf("the feedback sheet does not carry %q", want)
		}
	}
}

// TestSheetNamesSurviveExcelsRules covers the tab names Excel would refuse:
// a name may not carry :/\?*[] and may not repeat, and a program whose
// sessions were never numbered produces several week-zero groups.
func TestSheetNamesSurviveExcelsRules(t *testing.T) {
	first := sheetName("en", 0, 0)
	second := sheetName("en", 0, 1)
	if first == second {
		t.Errorf("two unnumbered weeks both produced %q, which Excel refuses", first)
	}
	for _, name := range []string{first, second, sheetName("fr", 3, 2)} {
		if len([]rune(name)) > 31 || name == "" {
			t.Errorf("%q is not a usable sheet name", name)
		}
		for _, banned := range []string{"[", "]", ":", "*", "?", "/", "\\"} {
			if bytes.Contains([]byte(name), []byte(banned)) {
				t.Errorf("%q carries %q, which Excel refuses", name, banned)
			}
		}
	}
}

func flatten(rows [][]string) []string {
	cells := []string{}
	for _, row := range rows {
		cells = append(cells, row...)
	}
	return cells
}

func contains(cells []string, want string) bool {
	for _, cell := range cells {
		if cell == want {
			return true
		}
	}
	return false
}
