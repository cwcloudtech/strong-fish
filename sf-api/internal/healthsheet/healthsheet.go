// Package healthsheet is what a member's health data looks like as a table:
// which columns it has, and what goes in each cell.
//
// The same reason programsheet exists. The export is produced in two formats -
// a picture to paste into a conversation, a PDF to keep or print - and if each
// renderer decided for itself what a cell meant, the two would drift. So the
// renderers own layout and nothing else: they are handed these rows.
package healthsheet

import (
	"fmt"
	"strconv"
	"strings"

	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programsheet"
)

// Columns is the sheet's layout: a day per row, steps beside the heart rate
// recorded on it.
//
// The date gets the width because it is the only cell that is not a number.
// Zone is a word rather than a colour: an export is printed, forwarded and
// screenshotted, and a colour is the first thing to be lost on the way.
func Columns(locale string, heart bool) []programsheet.Column {
	columns := []programsheet.Column{
		{Header: programsheet.Heading(locale, "date"), Weight: 1.6},
		{Header: programsheet.Heading(locale, "steps"), Weight: 1.2},
	}
	if !heart {
		return columns
	}
	return append(columns,
		programsheet.Column{Header: programsheet.Heading(locale, "heartAvg"), Weight: 1.0},
		programsheet.Column{Header: programsheet.Heading(locale, "heartMax"), Weight: 1.0},
		programsheet.Column{Header: programsheet.Heading(locale, "zone"), Weight: 1.2},
	)
}

// Lines is the range as rows, oldest first, with a totals row at the end.
//
// A day the watch recorded nothing leaves its heart cells empty rather than
// printing zero: nobody's heart beat zero times, and a nought in that column
// would be read as a measurement.
func Lines(steps models.StepSeries, heart *models.HeartSeries, locale string) []programsheet.Line {
	byDate := map[string]models.HeartDay{}
	if heart != nil {
		for _, day := range heart.Days {
			byDate[day.Date] = day
		}
	}
	withHeart := heart != nil

	lines := make([]programsheet.Line, 0, len(steps.Days)+1)
	for _, day := range steps.Days {
		cells := []string{day.Date, formatInt(day.Steps)}
		if withHeart {
			reading, known := byDate[day.Date]
			if known && reading.Known {
				cells = append(cells,
					strconv.Itoa(reading.Avg),
					strconv.Itoa(reading.Max),
					ZoneLabel(locale, reading.Zone))
			} else {
				cells = append(cells, "", "", "")
			}
		}
		lines = append(lines, programsheet.Line{Cells: cells})
	}

	// The totals row is marked Done so the renderers tint it: it is the line
	// somebody looks for first, and in a table of ninety days it would
	// otherwise be the one that disappears.
	totals := []string{programsheet.Heading(locale, "totals"), formatInt(steps.Total)}
	if withHeart {
		totals = append(totals, strconv.Itoa(heart.AverageBPM), strconv.Itoa(heart.PeakBPM), "")
	}
	return append(lines, programsheet.Line{Cells: totals, Done: true})
}

// Title is what the sheet is called: whose data, over what.
func Title(memberName string, locale string) string {
	name := strings.TrimSpace(memberName)
	if name == "" {
		return strings.TrimSuffix(programsheet.Heading(locale, "healthFor"), " - %s")
	}
	return fmt.Sprintf(programsheet.Heading(locale, "healthFor"), name)
}

// Subtitle says what the numbers add up to, in a sentence - the thing somebody
// repeats when they forward the picture.
func Subtitle(steps models.StepSeries, heart *models.HeartSeries, locale string) string {
	parts := []string{
		fmt.Sprintf("%s - %s", steps.From, steps.To),
		fmt.Sprintf(programsheet.Heading(locale, "stepsAvg"), formatInt(steps.DailyAverage)),
	}

	switch {
	case heart == nil || heart.AverageBPM == 0:
		parts = append(parts, programsheet.Heading(locale, "bpmNone"))
	default:
		// Said explicitly, because the heart rate covers fewer days than the
		// steps whenever the range is a wide one - a reader comparing the two
		// columns deserves to know why one stops early.
		parts = append(parts, fmt.Sprintf(programsheet.Heading(locale, "bpmRange"),
			len(heart.Days), heart.AverageBPM, heart.PeakBPM))
	}
	return strings.Join(parts, "  -  ")
}

// ZoneLabel names one of the provider's four bands.
func ZoneLabel(locale, zone string) string {
	switch zone {
	case devicelink.ZoneLight:
		return programsheet.Heading(locale, "zoneLight")
	case devicelink.ZoneModerate:
		return programsheet.Heading(locale, "zoneModerate")
	case devicelink.ZoneVigorous:
		return programsheet.Heading(locale, "zoneVigorous")
	case devicelink.ZonePeak:
		return programsheet.Heading(locale, "zonePeak")
	}
	return programsheet.Heading(locale, "zoneNone")
}

// formatInt groups thousands with a thin space, which reads the same in both
// languages the sheet is produced in.
func formatInt[T int | int64](value T) string {
	digits := strconv.FormatInt(int64(value), 10)
	var out strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
	}
	return out.String()
}
