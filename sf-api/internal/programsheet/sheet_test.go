package programsheet

import (
	"testing"

	"strong-fish-api/internal/models"
)

// TestFormatLoad covers what actually goes on the bar.
func TestFormatLoad(t *testing.T) {
	absolute := 22.5
	cases := []struct {
		name string
		set  models.ProgramSet
		want string
	}{
		{"a resolved load is rounded to what a lifter can load",
			models.ProgramSet{RoundedLoad: 102.5, LoadKnown: true}, "102.5 kg"},
		{"a whole number loses its decimal",
			models.ProgramSet{RoundedLoad: 100, LoadKnown: true}, "100 kg"},
		{"an accessory's absolute load wins over any derived one",
			models.ProgramSet{AbsoluteLoad: &absolute, RoundedLoad: 999, LoadKnown: true}, "22.5 kg"},
		{"no recorded 1RM prints nothing rather than zero",
			models.ProgramSet{RoundedLoad: 0, LoadKnown: false}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatLoad(tc.set); got != tc.want {
				t.Errorf("FormatLoad() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPageOrderFollowsTheWeeks covers a program whose sessions arrive out of
// order, which an imported spreadsheet's do.
func TestWeeksAreOrdered(t *testing.T) {
	weeks := Weeks([]models.ProgramDay{
		{Week: 3, Day: 1}, {Week: 1, Day: 2}, {Week: 1, Day: 1}, {Week: 2, Day: 1},
	})

	if len(weeks) != 3 {
		t.Fatalf("grouped into %d weeks, want 3", len(weeks))
	}
	for i, want := range []int{1, 2, 3} {
		if weeks[i].Number != want {
			t.Errorf("week %d of the document is week %d, want %d", i, weeks[i].Number, want)
		}
	}
	// And the sessions within a week keep their own order.
	if weeks[0].Days[0].Day != 1 || weeks[0].Days[1].Day != 2 {
		t.Error("sessions inside a week are out of order")
	}
}
