package programpdf

import (
	"bytes"
	"regexp"
	"strconv"
	"testing"

	"strong-fish-api/internal/models"
)

func rpe(value float64) *float64 { return &value }

// sample is a two-week block: two sessions in the first week, one in the
// second, plus a week the coach left empty.
func sample() (models.Program, []models.ProgramDay) {
	program := models.Program{Name: "Bloc force 1", ClubName: "Barbell Club"}

	set := func(name string, reps int, r *float64, load float64) models.ProgramSet {
		return models.ProgramSet{
			ExerciseLabels: map[string]string{"en": name},
			Reps:           reps, RPE: r,
			RoundedLoad: load, LoadKnown: load > 0,
		}
	}

	return program, []models.ProgramDay{
		{ID: "1", Week: 1, Day: 1, Title: "WEEK 1 DAY 1", Sets: []models.ProgramSet{
			set("Squat", 5, rpe(7), 140),
			set("Bench press", 8, rpe(8), 92.5),
		}},
		{ID: "2", Week: 1, Day: 2, Sets: []models.ProgramSet{set("Deadlift", 3, rpe(8.5), 200)}},
		{ID: "3", Week: 2, Day: 1, Sets: []models.ProgramSet{set("Squat", 3, rpe(9), 160)}},
		{ID: "4", Week: 3, Day: 1},
	}
}

// pageCount reads how many pages the document declares. A PDF's page tree
// carries the total in /Count, which is the only number here that says whether
// a sheet per week really happened.
func pageCount(t *testing.T, pdf []byte) int {
	t.Helper()
	match := regexp.MustCompile(`/Type\s*/Pages(?:[^>]|>[^>])*?/Count\s+(\d+)`).FindSubmatch(pdf)
	if match == nil {
		t.Fatal("no page tree in the output - this is not a PDF")
	}
	count, err := strconv.Atoi(string(match[1]))
	if err != nil {
		t.Fatalf("unreadable page count %q", match[1])
	}
	return count
}

// TestRenderMakesASheetPerWeek is the whole instruction in one assertion: a
// week is a page, however many sessions are on it.
func TestRenderMakesASheetPerWeek(t *testing.T) {
	program, days := sample()

	out, err := Render(program, days, Options{Locale: "en", MemberName: "Alex Martin"})
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output does not start with a PDF header: %q", out[:min(8, len(out))])
	}

	// Three weeks appear in the sessions - 1, 2 and the empty 3 - so three
	// sheets, even though the third has nothing on it.
	if got := pageCount(t, out); got != 3 {
		t.Errorf("rendered %d pages, want one per week (3)", got)
	}
}

// TestRenderHandlesAnEmptyProgram covers the case a coach hits by exporting a
// block they have only just named: it must produce a sheet saying so, not an
// empty file that looks like a broken download.
func TestRenderHandlesAnEmptyProgram(t *testing.T) {
	out, err := Render(models.Program{Name: "New block"}, nil, Options{Locale: "en"})
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	if got := pageCount(t, out); got != 1 {
		t.Errorf("rendered %d pages, want 1", got)
	}
}

// TestRenderAcceptsAccents covers the encoding: the PDF fonts are cp1252, and
// writing UTF-8 bytes into them straight produces mojibake rather than an
// error - so a French program name would come out unreadable and nothing would
// have failed.
func TestRenderAcceptsAccents(t *testing.T) {
	program := models.Program{Name: "Préparation générale"}
	days := []models.ProgramDay{{
		ID: "1", Week: 1, Day: 1, Title: "Séance légère",
		Sets: []models.ProgramSet{{
			ExerciseLabels: map[string]string{"fr": "Développé couché"},
			Reps:           5, Notes: "à l'échauffement",
		}},
	}}

	out, err := Render(program, days, Options{Locale: "fr"})
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	if pageCount(t, out) != 1 {
		t.Error("an accented program did not render a page")
	}
}

// TestRenderWithFeedbackTurnsThePage covers the sheet a lifter sends their
// coach. Eleven columns do not fit an A4 portrait page: fpdf does not complain
// about that, it simply draws the exercise names two characters wide, so the
// orientation is what has to be pinned.
func TestRenderWithFeedbackTurnsThePage(t *testing.T) {
	program, days := sample()
	days[0].Sets[0].Log = &models.SetLog{Done: true, ActualRPE: rpe(9), E1RM: 187.9, Comment: "Tough"}

	portrait, err := Render(program, days, Options{Locale: "en"})
	if err != nil {
		t.Fatalf("rendering the prescription: %v", err)
	}
	landscape, err := Render(program, days, Options{Locale: "en", Feedback: true})
	if err != nil {
		t.Fatalf("rendering the feedback sheet: %v", err)
	}

	tall, wide := mediaBox(t, portrait), mediaBox(t, landscape)
	if tall.width >= tall.height {
		t.Errorf("the prescription is %v x %v, want portrait", tall.width, tall.height)
	}
	if wide.width <= wide.height {
		t.Errorf("the feedback sheet is %v x %v, want landscape", wide.width, wide.height)
	}
	if pageCount(t, landscape) != pageCount(t, portrait) {
		t.Error("the two documents disagree about how many weeks the block has")
	}
}

type box struct{ width, height float64 }

// mediaBox reads the page size the document declares.
func mediaBox(t *testing.T, pdf []byte) box {
	t.Helper()
	match := regexp.MustCompile(`/MediaBox\s*\[\s*[\d.]+\s+[\d.]+\s+([\d.]+)\s+([\d.]+)`).FindSubmatch(pdf)
	if match == nil {
		t.Fatal("the document declares no page size")
	}
	width, _ := strconv.ParseFloat(string(match[1]), 64)
	height, _ := strconv.ParseFloat(string(match[2]), 64)
	return box{width: width, height: height}
}
