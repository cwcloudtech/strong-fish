package fffcal

import (
	"testing"
	"time"
)

// TestParseSpan covers every way a date is written on this planner, including
// the three forms the instruction names. The federation's own file uses all of
// them, sometimes for the same competition in different years.
func TestParseSpan(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		month time.Month
		start string
		// end is the last day of the competition, stated inclusively here
		// because that is how a reader thinks about it; the parser's own End
		// is the day after.
		end string
	}{
		{"both ends dated with a slash", "13/05 - 15/05", time.May, "2026-05-13", "2026-05-15"},
		{"both ends dated with a dash", "13-05 / 15-05", time.May, "2026-05-13", "2026-05-15"},
		{"days then the month", "13-15/05", time.May, "2026-05-13", "2026-05-15"},
		{"no spaces at all", "26/02-1/03", time.February, "2026-02-26", "2026-03-01"},
		{"written with au", "24 au 27/07", time.July, "2026-07-24", "2026-07-27"},
		{"au with the month named", "30 au 1er Nov", time.October, "2026-10-30", "2026-11-01"},
		{"days alone take the column's month", "9-15", time.November, "2026-11-09", "2026-11-15"},
		{"au with days alone", "6 au 12", time.December, "2026-12-06", "2026-12-12"},
		{"a slashed pair that is a range", "17/20", time.December, "2026-12-17", "2026-12-20"},
		{"a single date in its own column", "13/05", time.May, "2026-05-13", "2026-05-13"},
		{"a date with the venue beside it", "1-10 Pilsen (CZE)", time.May, "2026-05-01", "2026-05-10"},
		{"a range crossing into the new year", "28/12-3/01", time.December, "2026-12-28", "2027-01-03"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, matched, ok := parseSpan(tc.line, tc.month, 2026)
			if !ok {
				t.Fatalf("parseSpan(%q) did not match", tc.line)
			}
			if matched == "" {
				t.Errorf("parseSpan(%q) reported no matched text", tc.line)
			}

			wantStart, _ := time.Parse("2006-01-02", tc.start)
			if !parsed.start.Equal(wantStart) {
				t.Errorf("start = %s, want %s", parsed.start.Format("2006-01-02"), tc.start)
			}
			// End is exclusive: the day after the last day of the event.
			wantEnd, _ := time.Parse("2006-01-02", tc.end)
			if !parsed.end.Equal(wantEnd.AddDate(0, 0, 1)) {
				t.Errorf("end = %s, want the day after %s", parsed.end.Format("2006-01-02"), tc.end)
			}
		})
	}
}

// TestParseSpanRejects covers the lines that must *not* be read as dates.
// A wrong date is worse than a missing one: a missing one falls through to the
// shading, while a wrong one silently puts a competition in the wrong week.
func TestParseSpanRejects(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"a title", "France Elite"},
		{"a venue with a department number", "Villenave d'Ornon (33)"},
		{"a lone year", "2027"},
		{"a lone number", "58"},
		{"nothing at all", ""},
		{"a span longer than any competition", "1/01 - 30/11"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := parseSpan(tc.line, time.June, 2026); ok {
				t.Fatalf("parseSpan(%q) matched, want no date", tc.line)
			}
		})
	}
}

// TestNamedMonth covers the abbreviations that turn up inside labels, where
// four letters and three letters can disagree - "juil" is July, "jui" is June.
func TestNamedMonth(t *testing.T) {
	cases := map[string]time.Month{
		"Nov":      time.November,
		"nov.":     time.November,
		"Novembre": time.November,
		"juil":     time.July,
		"Juillet":  time.July,
		"jui":      time.June,
		"AOÛT":     time.August,
		"aou":      time.August,
		"Déc":      time.December,
	}

	for text, want := range cases {
		t.Run(text, func(t *testing.T) {
			got, ok := namedMonth(text)
			if !ok {
				t.Fatalf("namedMonth(%q) did not match", text)
			}
			if got != want {
				t.Errorf("namedMonth(%q) = %v, want %v", text, got, want)
			}
		})
	}

	if _, ok := namedMonth("Croatie"); ok {
		t.Error("namedMonth read a country as a month")
	}
}
