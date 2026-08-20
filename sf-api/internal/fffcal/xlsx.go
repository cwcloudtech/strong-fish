package fffcal

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// The other shape the federation publishes a season in: a spreadsheet with one
// competition per row.
//
//	DATES              | COMPÉTITIONS/FORMATIONS  | NATURE | ORGANISATEUR
//	11 January 2026    | CHALLENGE J.-C. L.       | FA/PL  | CAHC BERK
//	28 March 2026      | CHAMPIONNATS DE FRANCE   | FA/PL  | PLANETE FORME AMIENS
//	29 March 2026      |                          |        |
//	8 mars 2026 *      | THE NORTH MEET           | FA/PL  | THE NORTH STRENGTH
//
// Easier to read than the year planner, with two things to know. A dated row
// with no name is the *second day* of the competition above it - which is how
// this format writes a weekend. And the dates arrive in two languages at once:
// a real date cell is rendered by the spreadsheet library in English, while one
// somebody typed as text stays in French, asterisk and all.

var (
	// The columns are found by their headings rather than by position: this
	// sheet spaces them out with merged cells, so "the fourth column" is a
	// property of one file rather than of the format.
	xlsxDateHeading      = regexp.MustCompile(`(?i)^dates?$`)
	xlsxTitleHeading     = regexp.MustCompile(`(?i)comp[ée]titions?|formations?|events?`)
	xlsxNatureHeading    = regexp.MustCompile(`(?i)^nature|discipline`)
	xlsxOrganiserHeading = regexp.MustCompile(`(?i)^organisateur|organiser|organizer|club$`)

	// "8 mars 2026 *", "11 January 2026" - a day, a month by name, a year. The
	// asterisk is this sheet's footnote for a date that may still move, and is
	// not part of the date.
	xlsxNamedDate = regexp.MustCompile(`^(\d{1,2})\s+([^\s\d]+)\s+(\d{4})`)
	// 2026-01-11, and the same with a time after it.
	xlsxISODate = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})`)
	// 11/01/2026 - day first, as everything else in these files is.
	xlsxSlashDate = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})/(\d{4})`)
)

// englishMonths complete the French ones in dates.go: a date cell is formatted
// by the spreadsheet library, which writes month names in English whatever
// language the file was authored in.
var englishMonths = map[string]time.Month{
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June,
	"july": time.July, "august": time.August, "september": time.September,
	"october": time.October, "november": time.November, "december": time.December,
}

// natureColors give each discipline its own colour, the way the year planner
// shades its categories: on both, what tells a bench meet from a full
// powerlifting one at a glance is the colour.
//
// Keyed by the discipline as the sheet writes it, and matched on the part
// before the slash so "FA/PL" and "FA" are the same thing.
var natureColors = map[string]string{
	"fa":  "#0e5e9b", // force athlétique / powerlifting
	"pl":  "#0e5e9b",
	"dc":  "#d97706", // développé couché / bench press
	"bp":  "#d97706",
	"str": "#16a34a", // strongman and the rest
}

// defaultNatureColor is for a discipline this app has not seen. Anything with a
// nature at all gets a colour, so a new one is legible rather than invisible.
const defaultNatureColor = "#7c3aed"

// parseWorkbook reads a season from the spreadsheet form.
func parseWorkbook(data []byte) (Result, error) {
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return Result{}, ErrUnsupportedCalendar
	}
	defer file.Close()

	var result Result
	for _, sheet := range file.GetSheetList() {
		rows, err := file.GetRows(sheet)
		if err != nil {
			return Result{}, err
		}
		readWorkbookSheet(rows, &result)
	}

	sort.SliceStable(result.Events, func(a, b int) bool {
		if !result.Events[a].Start.Equal(result.Events[b].Start) {
			return result.Events[a].Start.Before(result.Events[b].Start)
		}
		return result.Events[a].Title < result.Events[b].Title
	})
	return result, nil
}

// workbookColumns is where each field sits, once the heading row has been found.
type workbookColumns struct {
	date, title, nature, organiser int
	// note is whatever column follows the organiser - this sheet uses it for
	// "PRIORITÉ JEUNES" and the like, under no heading of its own.
	note int
}

func readWorkbookSheet(rows [][]string, result *Result) {
	columns, headingRow, found := findWorkbookColumns(rows)
	if !found {
		return
	}

	for _, row := range rows[headingRow+1:] {
		date := cell(row, columns.date)
		if date == "" {
			continue
		}

		start, ok := parseWorkbookDate(date)
		if !ok {
			// A line of prose in the date column is this sheet's footer, not a
			// competition. Only something that looked like it was meant to be
			// a date is worth reporting.
			if looksLikeADate(date) {
				result.Warnings = append(result.Warnings, date)
			}
			continue
		}

		title := cell(row, columns.title)
		if title == "" {
			// A date with no name continues the competition above it: this is
			// how the sheet writes a two-day meet. End is exclusive, so the
			// second day extends it to the morning after.
			if len(result.Events) > 0 {
				last := &result.Events[len(result.Events)-1]
				if next := start.AddDate(0, 0, 1); next.After(last.End) {
					last.End = next
				}
			}
			continue
		}

		nature := cell(row, columns.nature)
		result.Events = append(result.Events, Event{
			Title:       title,
			Description: workbookDescription(nature, cell(row, columns.organiser), cell(row, columns.note)),
			Start:       start,
			End:         start.AddDate(0, 0, 1),
			Color:       natureColor(nature),
		})
	}
}

// findWorkbookColumns locates the heading row and the columns under it.
func findWorkbookColumns(rows [][]string) (workbookColumns, int, bool) {
	for index, row := range rows {
		columns := workbookColumns{date: -1, title: -1, nature: -1, organiser: -1, note: -1}
		for column, value := range row {
			switch value = strings.TrimSpace(value); {
			case xlsxDateHeading.MatchString(value):
				columns.date = column
			case xlsxTitleHeading.MatchString(value):
				columns.title = column
			case xlsxNatureHeading.MatchString(value):
				columns.nature = column
			case xlsxOrganiserHeading.MatchString(value):
				columns.organiser = column
			}
		}

		// A date and a name are the minimum: without both there is no event to
		// make, whatever else the row has.
		if columns.date >= 0 && columns.title >= 0 {
			if columns.organiser >= 0 {
				columns.note = columns.organiser + 2
			}
			return columns, index, true
		}
	}
	return workbookColumns{}, 0, false
}

// parseWorkbookDate reads the forms this column arrives in.
func parseWorkbookDate(text string) (time.Time, bool) {
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "*"))

	if match := xlsxISODate.FindStringSubmatch(text); match != nil {
		return buildDate(atoi(match[1]), atoi(match[2]), atoi(match[3]))
	}
	if match := xlsxSlashDate.FindStringSubmatch(text); match != nil {
		return buildDate(atoi(match[3]), atoi(match[2]), atoi(match[1]))
	}
	if match := xlsxNamedDate.FindStringSubmatch(text); match != nil {
		if month, ok := workbookMonth(match[2]); ok {
			return buildDate(atoi(match[3]), int(month), atoi(match[1]))
		}
	}
	return time.Time{}, false
}

// workbookMonth reads a month name in either language: the file is French, and
// the spreadsheet library renders its real date cells in English.
func workbookMonth(name string) (time.Month, bool) {
	if month, ok := englishMonths[strings.ToLower(strings.TrimSpace(name))]; ok {
		return month, true
	}
	return namedMonth(name)
}

func buildDate(year, month, day int) (time.Time, bool) {
	if !validMonth(month) || !validDay(day) || year < 1970 || year > 2200 {
		return time.Time{}, false
	}
	// Midnight UTC, as the whole-day events elsewhere in this app are stored.
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), true
}

// looksLikeADate distinguishes a date this reader could not manage from the
// paragraph of small print at the bottom of the sheet.
func looksLikeADate(text string) bool {
	return len(strings.Fields(text)) <= 5 && strings.ContainsAny(text, "0123456789")
}

// workbookDescription is what the row says besides its date and its name.
func workbookDescription(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " - ")
}

// natureColor picks a colour for a discipline, or none when the row does not
// say - an event with no category should fall back to the app's own colour for
// its kind rather than being given an arbitrary one.
func natureColor(nature string) string {
	nature = strings.TrimSpace(nature)
	if nature == "" {
		return ""
	}

	first := strings.ToLower(strings.TrimSpace(strings.Split(nature, "/")[0]))
	if color, ok := natureColors[first]; ok {
		return color
	}
	return defaultNatureColor
}

func cell(row []string, column int) string {
	if column < 0 || column >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[column])
}
