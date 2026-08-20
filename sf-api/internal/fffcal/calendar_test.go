package fffcal

import (
	"os"
	"strings"
	"testing"
	"time"
)

// The two seasons the federation published, kept as fixtures because this
// package's whole job is reading *these* documents. Nothing here can be tested
// against a synthetic PDF: the layout inference is only meaningful against a
// real one, and a hand-made fixture would only prove the parser agrees with
// whatever the fixture's author assumed.
const (
	season2026 = "testdata/fff-calendar-1.pdf"
	season2027 = "testdata/fff-calendar-2.pdf"
)

func load(t *testing.T, name string) Result {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	result, err := Calendar(data)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return result
}

func find(t *testing.T, result Result, title string) Event {
	t.Helper()
	for _, event := range result.Events {
		if event.Title == title {
			return event
		}
	}
	t.Fatalf("no event titled %q; got %d events", title, len(result.Events))
	return Event{}
}

func date(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("bad test date %q: %v", value, err)
	}
	return parsed
}

// TestCalendarReadsKnownCompetitions pins a handful of entries that can be
// checked against the printed page, one for each way a date is written there.
func TestCalendarReadsKnownCompetitions(t *testing.T) {
	result := load(t, season2027)

	cases := []struct {
		title       string
		start, end  string
		color       string
		description string
	}{
		// "17/12 - 19/12": both ends spelled out.
		{"France Elite", "2027-12-17", "2027-12-20", "#92d050", "Villenave d'Ornon (33)"},
		// "18 au 26/09": one month, stated once, at the end.
		{"EU DC/BP", "2027-09-18", "2027-09-27", "#ffc000", "Georgie"},
		// "24 au 27/07": the same form with the venue on its own line.
		{"EU SA FA", "2027-07-24", "2027-07-28", "#ffc000", "Croatie"},
		// "30 au 1er Nov": a range that crosses into the month it names.
		{"Finale PL / BP ?", "2027-10-30", "2027-11-02", "#92d050", "Fourchambault (58)"},
	}

	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			event := find(t, result, tc.title)
			if !event.Start.Equal(date(t, tc.start)) {
				t.Errorf("start = %s, want %s", event.Start.Format("2006-01-02"), tc.start)
			}
			// End is exclusive, so a competition on the 17th to the 19th ends
			// on the 20th - the same convention iCalendar uses for a DATE.
			if !event.End.Equal(date(t, tc.end)) {
				t.Errorf("end = %s, want %s", event.End.Format("2006-01-02"), tc.end)
			}
			if event.Color != tc.color {
				t.Errorf("color = %q, want %q", event.Color, tc.color)
			}
			if !strings.Contains(event.Description, tc.description) {
				t.Errorf("description = %q, want it to mention %q", event.Description, tc.description)
			}
		})
	}
}

// TestCalendarReadsBothSeasons checks the whole of each file rather than
// individual entries: a change that quietly halves the number of competitions
// read, or starts inventing them, shows up here.
func TestCalendarReadsBothSeasons(t *testing.T) {
	cases := []struct {
		file      string
		year      int
		atLeast   int
		atMost    int
		mustCover []string
	}{
		{season2026, 2026, 20, 30, []string{"Pyrenee Cup", "France Elite", "W PL Open"}},
		{season2027, 2027, 18, 28, []string{"EU FA", "France Elite", "W FA Open"}},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			result := load(t, tc.file)

			if len(result.Events) < tc.atLeast || len(result.Events) > tc.atMost {
				t.Fatalf("read %d events, want between %d and %d", len(result.Events), tc.atLeast, tc.atMost)
			}
			for _, title := range tc.mustCover {
				find(t, result, title)
			}

			for _, event := range result.Events {
				if strings.TrimSpace(event.Title) == "" {
					t.Errorf("an event has no title: %+v", event)
				}
				// Every entry is a whole day or several: the planner records
				// days, never a time, and the app stores them as midnight.
				if event.Start.Hour() != 0 || event.End.Hour() != 0 {
					t.Errorf("%q is not a whole day: %s to %s", event.Title, event.Start, event.End)
				}
				if !event.End.After(event.Start) {
					t.Errorf("%q ends before it starts: %s to %s", event.Title, event.Start, event.End)
				}
				// The season is the year on the page. December entries may run
				// a few days into January, and nothing may predate the season.
				if event.Start.Year() != tc.year {
					t.Errorf("%q starts in %d, want %d", event.Title, event.Start.Year(), tc.year)
				}
			}
		})
	}
}

// TestCalendarIsOrdered covers the order the events are returned in, because
// the import writes them in that order and a coach reads the result as a list.
func TestCalendarIsOrdered(t *testing.T) {
	result := load(t, season2026)
	for i := 1; i < len(result.Events); i++ {
		if result.Events[i].Start.Before(result.Events[i-1].Start) {
			t.Fatalf("event %d (%s) comes before event %d (%s)",
				i, result.Events[i].Start.Format("2006-01-02"),
				i-1, result.Events[i-1].Start.Format("2006-01-02"))
		}
	}
}

// TestCalendarKeepsTheCategoryColours checks that the shading survives the
// import. On the printed page the category - federal, European, world - is
// carried by colour and by nothing else, so an import that dropped it would
// turn a legible season into forty identical rows.
func TestCalendarKeepsTheCategoryColours(t *testing.T) {
	result := load(t, season2026)

	colored, palette := 0, map[string]bool{}
	for _, event := range result.Events {
		if event.Color == "" {
			continue
		}
		colored++
		palette[event.Color] = true
		if len(event.Color) != 7 || event.Color[0] != '#' {
			t.Errorf("%q has colour %q, want #rrggbb", event.Title, event.Color)
		}
	}

	if colored < len(result.Events)/2 {
		t.Errorf("only %d of %d events kept a colour", colored, len(result.Events))
	}
	// The legend lists five categories. Reading one colour for everything
	// would mean the shading was being found but not told apart.
	if len(palette) < 3 {
		t.Errorf("read %d distinct colours, want at least 3: %v", len(palette), palette)
	}
}

// TestCalendarRejectsWhatIsNotACalendar covers the wrong-file case, which is
// what a coach will actually do first.
func TestCalendarRejectsWhatIsNotACalendar(t *testing.T) {
	cases := map[string][]byte{
		"a text file":              []byte("just some words"),
		"an image":                 {0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a},
		"a zip that is not a book": []byte("PK\x03\x04 not really a spreadsheet"),
	}

	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Calendar(data); err == nil {
				t.Fatal("this was accepted as a calendar")
			}
		})
	}
}

// TestFontsAreResolvedPerPage covers a bug that does not fail, it lies: the
// two pages of the federation's calendar both define /F1 and /F4 and point
// them at different font objects. Resolving font names document-wide decodes
// one page through the other's table and returns real-looking wrong text -
// "Qualif" came back as "ualif" and "CALENDRIER" as "CALENDE".
func TestFontsAreResolvedPerPage(t *testing.T) {
	data, err := os.ReadFile(season2026)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("read %d pages, want 2", len(pages))
	}

	// One string from each page, each set in a font whose name the other page
	// reuses for something else.
	wanted := []string{"CALENDRIER FA, PL et DC, BP 2026", "Qualif"}
	for _, want := range wanted {
		found := false
		for _, page := range pages {
			for _, text := range page.Texts {
				if strings.Contains(text.S, want) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("no page contains %q - the fonts were decoded through the wrong table", want)
		}
	}
}
