package models

import "testing"

func TestSleepStatusFor(t *testing.T) {
	cases := []struct {
		asleepMin int
		want      string
	}{
		{8 * 60, SleepStatusGood},
		{7 * 60, SleepStatusGood},
		{7*60 - 1, SleepStatusFair},
		{6 * 60, SleepStatusFair},
		{6*60 - 1, SleepStatusPoor},
		{4 * 60, SleepStatusPoor},
		{1, SleepStatusPoor},
		{0, ""},
	}
	for _, c := range cases {
		if got := SleepStatusFor(c.asleepMin); got != c.want {
			t.Errorf("SleepStatusFor(%d min) = %q, want %q", c.asleepMin, got, c.want)
		}
	}
}

// A week is "recorded" as soon as one of its nights has sleep; a week of only
// empty nights (a chart of seven blank bars) is not.
func TestSleepWeekRecorded(t *testing.T) {
	empty := SleepWeek{Days: []SleepDay{{Date: "2026-11-10"}, {Date: "2026-11-11"}}}
	if empty.Recorded() {
		t.Error("a week with no slept minutes should not be Recorded")
	}
	some := SleepWeek{Days: []SleepDay{{Date: "2026-11-10"}, {Date: "2026-11-11", AsleepMin: 430}}}
	if !some.Recorded() {
		t.Error("a week with a slept night should be Recorded")
	}
}
