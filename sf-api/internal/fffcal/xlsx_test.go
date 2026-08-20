package fffcal

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

const season2026Workbook = "testdata/fff-calendar-3.xlsx"

func loadWorkbook(t *testing.T) Result {
	t.Helper()

	data, err := os.ReadFile(season2026Workbook)
	if err != nil {
		t.Fatalf("reading %s: %v", season2026Workbook, err)
	}
	result, err := Calendar(data)
	if err != nil {
		t.Fatalf("parsing %s: %v", season2026Workbook, err)
	}
	return result
}

// buildWorkbook writes a one-sheet workbook from a grid, for the cases the
// real file does not happen to contain.
func buildWorkbook(t *testing.T, rows [][]string) []byte {
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

func on(t *testing.T, date string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatalf("bad test date %q: %v", date, err)
	}
	return parsed
}

// TestWorkbookReadsTheSeason pins the file the federation published, entry by
// entry: ten competitions, the two-day ones spanning both days, and each
// carrying its discipline and its organiser.
func TestWorkbookReadsTheSeason(t *testing.T) {
	result := loadWorkbook(t)

	if len(result.Events) != 10 {
		t.Fatalf("read %d competitions, want 10", len(result.Events))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("warnings on a file that parses cleanly: %v", result.Warnings)
	}

	cases := []struct {
		title       string
		start, end  string
		color       string
		description string
	}{
		{"CHALLENGE JEAN-CLAUDE LAPOSTOLLE", "2026-01-11", "2026-01-12", "#0e5e9b", "FA/PL - CAHC BERK"},
		// Two days, written as a second dated row with no name.
		{"CHAMPIONNATS DE FRANCE UNIVERSITAIRE", "2026-03-28", "2026-03-30", "#0e5e9b", "FA/PL - PLANETE FORME AMIENS"},
		{"OPEN DU LION", "2026-06-06", "2026-06-08", "#0e5e9b", "FA/PL - POWERCELLHOUSE COMPIÈGNE"},
		// A bench meet: a different discipline, and so a different colour.
		{"CHAMPIONNAT RÉGIONAL", "2026-04-26", "2026-04-27", "#d97706", "DC/BP - WINGLES FORME"},
	}

	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			var found *Event
			for i := range result.Events {
				if result.Events[i].Title == tc.title {
					found = &result.Events[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("no competition called %q", tc.title)
			}

			if !found.Start.Equal(on(t, tc.start)) {
				t.Errorf("starts %s, want %s", found.Start.Format("2006-01-02"), tc.start)
			}
			// End is exclusive: a one-day meet ends the next morning, and a
			// two-day one the morning after that.
			if !found.End.Equal(on(t, tc.end)) {
				t.Errorf("ends %s, want %s", found.End.Format("2006-01-02"), tc.end)
			}
			if found.Color != tc.color {
				t.Errorf("colour %q, want %q", found.Color, tc.color)
			}
			if found.Description != tc.description {
				t.Errorf("description %q, want %q", found.Description, tc.description)
			}
		})
	}
}

// TestWorkbookKeepsTheFootnotes covers the row that carries a note in no column
// of its own, and the date the sheet marks as still moving.
func TestWorkbookKeepsTheFootnotes(t *testing.T) {
	result := loadWorkbook(t)

	for _, event := range result.Events {
		if event.Title != "THE NORTH MEET" {
			continue
		}
		// "8 mars 2026 *" - a French date, and the asterisk is the sheet's
		// footnote for a date that may change, not part of the date.
		if !event.Start.Equal(on(t, "2026-03-08")) {
			t.Errorf("starts %s, want 2026-03-08", event.Start.Format("2006-01-02"))
		}
		if event.Description != "FA/PL - THE NORTH STRENGTH - PRIORITÉ JEUNES" {
			t.Errorf("description %q lost the note beside the row", event.Description)
		}
		return
	}
	t.Fatal("THE NORTH MEET is missing")
}

// TestWorkbookIgnoresTheSmallPrint covers the paragraph at the foot of the
// sheet, which sits in the date column and is not a competition. Reading it as
// one would put a wall of text in somebody's calendar.
func TestWorkbookIgnoresTheSmallPrint(t *testing.T) {
	result := loadWorkbook(t)

	for _, event := range result.Events {
		if len(event.Title) > 60 {
			t.Errorf("a paragraph became a competition: %q", event.Title)
		}
	}
}

// TestWorkbookDates covers the forms the date column arrives in. The same file
// carries two languages: a real date cell is rendered in English by the
// spreadsheet library, while one typed as text stays French.
func TestWorkbookDates(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"english, as a date cell is rendered", "11 January 2026", "2026-01-11"},
		{"french, as somebody typed it", "8 mars 2026", "2026-03-08"},
		{"french with the sheet's asterisk", "6 juin 2026 *", "2026-06-06"},
		{"an abbreviated french month", "3 déc 2026", "2026-12-03"},
		{"iso, should a cell hold one", "2026-01-11", "2026-01-11"},
		{"iso with a time after it", "2026-01-11 00:00:00", "2026-01-11"},
		{"day first, with slashes", "11/01/2026", "2026-01-11"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseWorkbookDate(tc.text)
			if !ok {
				t.Fatalf("parseWorkbookDate(%q) did not match", tc.text)
			}
			if got.Format("2006-01-02") != tc.want {
				t.Errorf("parseWorkbookDate(%q) = %s, want %s", tc.text, got.Format("2006-01-02"), tc.want)
			}
		})
	}

	for _, text := range []string{"", "Toutes les dates ci dessus sont celles qui ont été arrêtées", "somewhen"} {
		if _, ok := parseWorkbookDate(text); ok {
			t.Errorf("parseWorkbookDate(%q) matched, want no date", text)
		}
	}
}

// TestWorkbookFindsItsColumnsByHeading covers a sheet laid out differently from
// the one file this was written against: the columns are found by what they are
// headed, because their positions are a property of one spreadsheet rather than
// of the format.
func TestWorkbookFindsItsColumnsByHeading(t *testing.T) {
	data := buildWorkbook(t, [][]string{
		{"Ligue de Force - saison 2027"},
		{},
		{"Organisateur", "Nature", "Compétitions", "Dates"},
		{"CLUB DU NORD", "DC/BP", "OPEN D'HIVER", "17 January 2027"},
		{"", "", "", "18 January 2027"},
	})

	result, err := Calendar(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("read %d competitions, want 1", len(result.Events))
	}

	event := result.Events[0]
	if event.Title != "OPEN D'HIVER" {
		t.Errorf("title = %q", event.Title)
	}
	if !event.Start.Equal(on(t, "2027-01-17")) || !event.End.Equal(on(t, "2027-01-19")) {
		t.Errorf("runs %s to %s, want the 17th to the 19th (exclusive)",
			event.Start.Format("2006-01-02"), event.End.Format("2006-01-02"))
	}
	if event.Color != "#d97706" {
		t.Errorf("colour = %q, want the bench-press one", event.Color)
	}
}

// TestWorkbookWithoutHeadingsIsNotACalendar covers a spreadsheet that is not
// one of these at all - a coach's program, say, uploaded to the wrong button.
// It must come back empty rather than inventing events, so the handler can say
// nothing was found.
func TestWorkbookWithoutHeadingsIsNotACalendar(t *testing.T) {
	data := buildWorkbook(t, [][]string{
		{"WEEK 1 DAY 2"},
		{"Exercice", "Reps", "RPE", "Percentage"},
		{"Squat", "3", "5", "79"},
	})

	result, err := Calendar(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if len(result.Events) != 0 {
		t.Errorf("a training program produced %d events", len(result.Events))
	}
}

// TestBothPlannersStillRead guards the formats that already worked: three
// shapes now arrive through one upload button, and the dispatch has to keep
// sending each to the reader that understands it.
func TestBothPlannersStillRead(t *testing.T) {
	cases := []struct {
		file    string
		atLeast int
	}{
		{season2026, 20},
		{season2027, 18},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Calendar(data)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if len(result.Events) < tc.atLeast {
				t.Errorf("read %d competitions, want at least %d", len(result.Events), tc.atLeast)
			}
		})
	}
}
