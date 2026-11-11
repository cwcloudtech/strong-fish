package models

import (
	"testing"
	"time"
)

func TestSleepNightStatus(t *testing.T) {
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
	}
	for _, c := range cases {
		if got := (SleepNight{AsleepMin: c.asleepMin}).Status(); got != c.want {
			t.Errorf("Status(%d min) = %q, want %q", c.asleepMin, got, c.want)
		}
	}
}

// A night from 23:30 to 06:30 with a short awake patch at 02:00 should bucket
// its asleep minutes under the right hours of the 21:00-09:00 window, and the
// awake patch should read as in-bed-but-not-asleep.
func TestSleepNightHours(t *testing.T) {
	// Spanning midnight: the first segment is on the 1st, the rest on the 2nd.
	night := SleepNight{
		Start: time.Date(2026, 1, 1, 23, 30, 0, 0, time.UTC),
		End:   time.Date(2026, 1, 2, 6, 30, 0, 0, time.UTC),
		Segments: []SleepSegment{
			{
				Start: time.Date(2026, 1, 1, 23, 30, 0, 0, time.UTC),
				End:   time.Date(2026, 1, 2, 2, 0, 0, 0, time.UTC),
			},
			{Start: time.Date(2026, 1, 2, 2, 20, 0, 0, time.UTC), End: time.Date(2026, 1, 2, 6, 30, 0, 0, time.UTC)},
		},
	}

	hours := night.Hours()
	if len(hours) != 12 {
		t.Fatalf("want 12 hourly buckets, got %d", len(hours))
	}
	if hours[0].Hour != 21 || hours[11].Hour != 8 {
		t.Errorf("window should run 21h..08h, got %dh..%dh", hours[0].Hour, hours[11].Hour)
	}

	byHour := map[int]SleepHour{}
	for _, h := range hours {
		byHour[h.Hour] = h
	}
	// 21:00 and 22:00 are before bed - not in bed, nothing asleep.
	if byHour[21].InBedMin != 0 || byHour[22].InBedMin != 0 {
		t.Errorf("hours before bed should be empty: 21h=%+v 22h=%+v", byHour[21], byHour[22])
	}
	// 23:00: in bed from 23:30, asleep the last 30 minutes.
	if byHour[23].AsleepMin != 30 {
		t.Errorf("23h asleep = %d, want 30", byHour[23].AsleepMin)
	}
	// 02:00: in bed the whole hour, awake 02:00-02:20, so 40 min asleep.
	if byHour[2].InBedMin != 60 || byHour[2].AsleepMin != 40 {
		t.Errorf("02h = %+v, want inbed 60 asleep 40", byHour[2])
	}
	// 07:00 and 08:00 are after waking - empty.
	if byHour[7].InBedMin != 0 || byHour[8].InBedMin != 0 {
		t.Errorf("hours after waking should be empty: 07h=%+v 08h=%+v", byHour[7], byHour[8])
	}
}
