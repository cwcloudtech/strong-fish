package handlers

import (
	"strings"
	"testing"
	"time"

	"strong-fish-api/internal/models"
)

func at(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

// TestBuildICSStructure covers what a calendar client actually parses: CRLF
// line endings, the VCALENDAR/VEVENT nesting, and a UID per event.
func TestBuildICSStructure(t *testing.T) {
	ics := BuildICS([]models.Event{{
		ID: "ev-1", Title: "French Nationals", StartsAt: at("2026-09-12T08:00:00Z"),
		EndsAt: at("2026-09-12T18:00:00Z"), Visibility: models.VisibilityPublic,
	}})

	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "END:VCALENDAR\r\n",
		"BEGIN:VEVENT\r\n", "END:VEVENT\r\n",
		"VERSION:2.0\r\n",
		"UID:ev-1@strong-fish\r\n",
		"DTSTART:20260912T080000Z\r\n",
		"DTEND:20260912T180000Z\r\n",
		"SUMMARY:French Nationals\r\n",
	} {
		if !strings.Contains(ics, want) {
			t.Errorf("ICS is missing %q\n%s", want, ics)
		}
	}

	// A bare LF anywhere would break clients that split on CRLF.
	if strings.Contains(strings.ReplaceAll(ics, "\r\n", ""), "\n") {
		t.Error("ICS contains a line ending that is not CRLF")
	}
}

// TestBuildICSAllDayEndIsExclusive pins RFC 5545's rule that a DATE-valued
// DTEND is the day *after* the last one. Outlook renders a same-day DTEND as
// no day at all, so getting this wrong makes the event vanish.
func TestBuildICSAllDayEndIsExclusive(t *testing.T) {
	ics := BuildICS([]models.Event{{
		ID: "ev-2", Title: "Camp", AllDay: true, StartsAt: at("2026-07-06T00:00:00Z"),
	}})

	if !strings.Contains(ics, "DTSTART;VALUE=DATE:20260706\r\n") {
		t.Errorf("all-day DTSTART wrong:\n%s", ics)
	}
	if !strings.Contains(ics, "DTEND;VALUE=DATE:20260707\r\n") {
		t.Errorf("all-day DTEND must be the next day:\n%s", ics)
	}
}

// TestBuildICSEscapesText covers RFC 5545's TEXT escaping. A raw newline or
// comma in a summary doesn't merely look wrong - it ends the content line and
// corrupts every property after it.
func TestBuildICSEscapesText(t *testing.T) {
	ics := BuildICS([]models.Event{{
		ID: "ev-3", Title: "Meet, day 1; heavy", StartsAt: at("2026-03-01T09:00:00Z"),
		Location:    "Lyon, France",
		Description: "Weigh-in 07:00\nStart 09:00",
	}})

	if !strings.Contains(ics, `SUMMARY:Meet\, day 1\; heavy`) {
		t.Errorf("comma/semicolon not escaped:\n%s", ics)
	}
	if !strings.Contains(ics, `LOCATION:Lyon\, France`) {
		t.Errorf("location not escaped:\n%s", ics)
	}
	if !strings.Contains(ics, `DESCRIPTION:Weigh-in 07:00\nStart 09:00`) {
		t.Errorf("newline not escaped:\n%s", ics)
	}
}

// TestBuildICSDefaultsADuration guards against a zero-length event: a meet
// whose author gave no end time still has to occupy a slot in the calendar.
func TestBuildICSDefaultsADuration(t *testing.T) {
	ics := BuildICS([]models.Event{{
		ID: "ev-4", Title: "Session", StartsAt: at("2026-03-01T09:00:00Z"),
	}})

	if !strings.Contains(ics, "DTEND:20260301T100000Z\r\n") {
		t.Errorf("an event with no end should last an hour:\n%s", ics)
	}
}

// TestBuildICSPrefixesTheClub: one feed mixes several clubs' calendars with
// the open one, so an entry has to say whose it is.
func TestBuildICSPrefixesTheClub(t *testing.T) {
	ics := BuildICS([]models.Event{{
		ID: "ev-5", Title: "Mock meet", ClubName: "Iron Gym", StartsAt: at("2026-03-01T09:00:00Z"),
	}})

	if !strings.Contains(ics, "SUMMARY:Iron Gym - Mock meet\r\n") {
		t.Errorf("club event should name its club:\n%s", ics)
	}
}

// TestBuildICSSkipsUndatedEvents: an event with no start has nowhere to go in
// a calendar, and emitting a VEVENT with no DTSTART makes the whole feed
// invalid rather than just that entry.
func TestBuildICSSkipsUndatedEvents(t *testing.T) {
	ics := BuildICS([]models.Event{
		{ID: "ev-6", Title: "Undated"},
		{ID: "ev-7", Title: "Real", StartsAt: at("2026-03-01T09:00:00Z")},
	})

	if strings.Count(ics, "BEGIN:VEVENT") != 1 {
		t.Errorf("expected exactly one event:\n%s", ics)
	}
	if strings.Contains(ics, "Undated") {
		t.Errorf("an event with no start date should be skipped:\n%s", ics)
	}
}
