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
	"image/color"
	"strconv"
	"strings"

	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programsheet"
)

// The colours of the app's own screens, so a member reads the export the way
// they read the page: four walking bands for steps, and the provider's four
// heart rate zones from blue to red.
var (
	stepsLow  = color.RGBA{220, 38, 38, 255}
	stepsFair = color.RGBA{217, 119, 6, 255}
	stepsGood = color.RGBA{101, 195, 122, 255}
	stepsHigh = color.RGBA{22, 163, 74, 255}

	zoneColors = map[string]color.RGBA{
		devicelink.ZoneLight:    {59, 130, 246, 255},
		devicelink.ZoneModerate: {234, 179, 8, 255},
		devicelink.ZoneVigorous: {249, 115, 22, 255},
		devicelink.ZonePeak:     {220, 38, 38, 255},
	}
)

// StepTone is the band a day's count falls in - the same thresholds as the
// app's bars.
func StepTone(steps int64) color.RGBA {
	switch {
	case steps < 6000:
		return stepsLow
	case steps < 10000:
		return stepsFair
	case steps < 15000:
		return stepsGood
	}
	return stepsHigh
}

// Sections is the whole export: steps, then heart rate when there is any,
// then the latest body composition when a scale is connected.
func Sections(steps models.StepSeries, heart *models.HeartSeries,
	body *models.BodyComposition, locale string) []programsheet.Section {
	sections := []programsheet.Section{StepsSection(steps, locale)}
	if heart != nil {
		sections = append(sections, HeartSection(*heart, locale))
	}
	if body != nil && len(body.Measures) > 0 {
		sections = append(sections, BodySection(*body, locale))
	}
	return sections
}

// StepsSection is a histogram of the range, a bar a day coloured by its band.
func StepsSection(steps models.StepSeries, locale string) programsheet.Section {
	chart := &programsheet.Chart{
		Legend: []programsheet.LegendEntry{
			{Label: programsheet.Heading(locale, "stepsLow"), Color: stepsLow},
			{Label: programsheet.Heading(locale, "stepsFair"), Color: stepsFair},
			{Label: programsheet.Heading(locale, "stepsGood"), Color: stepsGood},
			{Label: programsheet.Heading(locale, "stepsHigh"), Color: stepsHigh},
		},
	}
	for _, day := range steps.Days {
		chart.Bars = append(chart.Bars, programsheet.Bar{
			Label:   shortDate(day.Date),
			Value:   float64(day.Steps),
			Caption: formatInt(day.Steps),
			Color:   StepTone(day.Steps),
		})
	}
	return programsheet.Section{Heading: programsheet.Heading(locale, "stepsChart"), Chart: chart}
}

// HeartSection is a histogram of the days with heart rate: each bar as tall as
// the day's highest reading, captioned with its average, coloured by the
// hardest zone reached. A day the watch recorded nothing is a flat sliver -
// nobody's heart beat zero times, and a bar there would be read as one.
func HeartSection(heart models.HeartSeries, locale string) programsheet.Section {
	chart := &programsheet.Chart{}
	for _, zone := range []string{devicelink.ZoneLight, devicelink.ZoneModerate,
		devicelink.ZoneVigorous, devicelink.ZonePeak} {
		chart.Legend = append(chart.Legend,
			programsheet.LegendEntry{Label: ZoneLabel(locale, zone), Color: zoneColors[zone]})
	}
	for _, day := range heart.Days {
		bar := programsheet.Bar{Label: shortDate(day.Date)}
		if day.Known {
			bar.Value = float64(day.Max)
			if bar.Value == 0 {
				bar.Value = float64(day.Avg)
			}
			bar.Caption = strconv.Itoa(day.Avg)
			bar.Color = zoneColors[day.Zone]
		}
		chart.Bars = append(chart.Bars, bar)
	}
	return programsheet.Section{Heading: programsheet.Heading(locale, "heartChart"), Chart: chart}
}

// BodySection is the latest reading of each measure, as a table: one value per
// measure is a list, not a series, and a bar chart of kilograms beside
// percentages beside kilocalories would compare things that do not compare.
func BodySection(body models.BodyComposition, locale string) programsheet.Section {
	rows := make([]programsheet.Line, 0, len(body.Measures))
	for _, measure := range body.Measures {
		value := strconv.FormatFloat(measure.Value, 'f', -1, 64)
		if unit := displayUnit(measure.Unit); unit != "" {
			value += " " + unit
		}
		rows = append(rows, programsheet.Line{Cells: []string{
			programsheet.Heading(locale, "bc."+measure.Type),
			value,
			measure.TakenAt.Format("2006-01-02"),
		}})
	}
	return programsheet.Section{
		Heading: programsheet.Heading(locale, "bodyComposition"),
		Columns: []programsheet.Column{
			{Header: programsheet.Heading(locale, "measure"), Weight: 1.8},
			{Header: programsheet.Heading(locale, "value"), Weight: 1.2},
			{Header: programsheet.Heading(locale, "measuredOn"), Weight: 1.2},
		},
		Rows: rows,
	}
}

// displayUnit is the unit as printed. Squared metres are spelt out by the API
// so a client is never handed a character it cannot encode.
func displayUnit(unit string) string {
	if unit == "kg/m2" {
		return "kg/m²"
	}
	return unit
}

// shortDate is the label under a bar: day and month, which is all the room a
// bar of a ninety-day range has.
func shortDate(date string) string {
	if len(date) == len("2006-01-02") {
		return date[8:10] + "/" + date[5:7]
	}
	return date
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
