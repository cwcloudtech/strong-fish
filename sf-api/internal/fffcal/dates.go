package fffcal

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// span is a run of whole days: start inclusive, end exclusive.
type span struct {
	start, end time.Time
}

// The forms a date is written in on this planner, in the order they are tried.
// Order matters: the fuller a form is, the earlier it has to be tested, or a
// shorter pattern matches the front of a longer date and reads "26/02-1/03" as
// the twenty-sixth of February alone.
var (
	// 13/05 - 15/05, 13-05 / 15-05, 26/02-1/03, 6/12-13/12
	dayMonthToDayMonth = regexp.MustCompile(`(\d{1,2})\s*[/-]\s*(\d{1,2})\s*(?:-|–|au|/)\s*(\d{1,2})\s*[/-]\s*(\d{1,2})`)
	// 13-15/05, 22-31/05, 24 au 27/07
	daysThenMonth = regexp.MustCompile(`(\d{1,2})\s*(?:-|–|au)\s*(\d{1,2})\s*/\s*(\d{1,2})`)
	// 30 au 1er Nov
	daysThenNamedMonth = regexp.MustCompile(`(?i)(\d{1,2})\s*(?:-|–|au)\s*(\d{1,2})\s*(?:er|ème|eme)?\s*([a-zéûôA-ZÉÛÔ]{3,10})`)
	// 9-15, 6 au 12, 1-10
	daysOnly = regexp.MustCompile(`(\d{1,2})\s*(?:-|–|au)\s*(\d{1,2})`)
	// 17/20, 3/ 9 - a pair that is a range rather than a day and a month
	slashPair = regexp.MustCompile(`(\d{1,2})\s*/\s*(\d{1,2})`)
)

// parseSpan reads the date written into a label. It returns the span, the text
// it read the date out of - so the caller can keep the rest of the line, since
// a date is usually written with the venue beside it - and whether it matched.
//
// This is the instruction's first choice, and the reliable one: the planner's
// shading is drawn to look right rather than to be read, but a coach who wrote
// "24 au 27/07" meant those four days.
//
// month is the column the label sits in, used when the date names days without
// a month. year comes from the page.
func parseSpan(line string, month time.Month, year int) (span, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return span{}, "", false
	}

	if m := dayMonthToDayMonth.FindStringSubmatch(line); m != nil {
		parsed, ok := build(year, atoi(m[2]), atoi(m[1]), atoi(m[4]), atoi(m[3]))
		return parsed, m[0], ok
	}

	if m := daysThenMonth.FindStringSubmatch(line); m != nil {
		// One month for both ends: "13-15/05" is the thirteenth to the
		// fifteenth of May, not the thirteenth of some other month.
		parsed, ok := build(year, atoi(m[3]), atoi(m[1]), atoi(m[3]), atoi(m[2]))
		return parsed, m[0], ok
	}

	if m := daysThenNamedMonth.FindStringSubmatch(line); m != nil {
		if named, ok := namedMonth(m[3]); ok {
			first, last := atoi(m[1]), atoi(m[2])
			// "30 au 1er Nov" names the month the range *ends* in, and starts
			// in the one before: a range that would run backwards inside one
			// month is really a range across two.
			startMonth := named
			if last < first {
				startMonth = named - 1
			}
			parsed, ok := build(year, int(startMonth), first, int(named), last)
			return parsed, m[0], ok
		}
	}

	if m := daysOnly.FindStringSubmatch(line); m != nil {
		parsed, ok := build(year, int(month), atoi(m[1]), int(month), atoi(m[2]))
		return parsed, m[0], ok
	}

	if m := slashPair.FindStringSubmatch(line); m != nil {
		first, second := atoi(m[1]), atoi(m[2])
		switch {
		case second == int(month):
			// "13/05" in the May column is a single day.
			parsed, ok := build(year, int(month), first, int(month), first)
			return parsed, m[0], ok
		case second > first:
			// "17/20" is the seventeenth to the twentieth. A month cannot come
			// after its own day here, so this pair is a range.
			parsed, ok := build(year, int(month), first, int(month), second)
			return parsed, m[0], ok
		}
	}

	return span{}, "", false
}

// build turns a day range into whole-day instants, with End exclusive.
//
// A range that ends before it starts has crossed into the next year - the
// planner's December column carries competitions running into January - so the
// end is rolled forward rather than rejected.
func build(year, startMonth, startDay, endMonth, endDay int) (span, bool) {
	if !validMonth(startMonth) || !validMonth(endMonth) || !validDay(startDay) || !validDay(endDay) {
		return span{}, false
	}

	start := day(year, time.Month(startMonth), startDay)
	end := day(year, time.Month(endMonth), endDay)
	if end.Before(start) {
		end = day(year+1, time.Month(endMonth), endDay)
	}
	// A calendar entry spanning most of a year is a misread, not a competition.
	if end.Sub(start) > 120*24*time.Hour {
		return span{}, false
	}
	return span{start: start, end: end.AddDate(0, 0, 1)}, true
}

// day is midnight UTC on a date. The planner states wall-clock days with no
// zone of their own, and UTC midnight is how the app stores a whole day.
func day(year int, month time.Month, dayOfMonth int) time.Time {
	return time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

func validMonth(month int) bool { return month >= 1 && month <= 12 }
func validDay(day int) bool     { return day >= 1 && day <= 31 }

// atoi reads a day or month out of a matched group. Named apart from pdf.go's
// number, which reads the float coordinates of the page.
func atoi(text string) int {
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0
	}
	return value
}

// namedMonth matches a month written out or abbreviated inside a label.
func namedMonth(text string) (time.Month, bool) {
	normalized := normalize(text)
	if month, ok := months[normalized]; ok {
		return month, true
	}
	if len(normalized) >= 3 {
		// Four letters first, so "juil" is July rather than June.
		if len(normalized) >= 4 {
			if month, ok := shortMonths[normalized[:4]]; ok {
				return month, true
			}
		}
		if month, ok := shortMonths[normalized[:3]]; ok {
			return month, true
		}
	}
	return 0, false
}

// bandSpan reads the dates off the shading - the instruction's second choice,
// used when the label carries no date the parser recognized.
//
// The shaded cells in a competition's own column say which days it occupies,
// which is the only other place the planner records that.
func bandSpan(page Page, col column, b block, year int) (span, bool) {
	days := bandDays(page, col, b)
	if len(days) == 0 {
		return span{}, false
	}

	first, last := days[0], days[0]
	for _, d := range days {
		first = min(first, d)
		last = max(last, d)
	}
	return build(year, int(col.month), first, int(col.month), last)
}

// bandDays returns the days of the month whose cells are shaded beside a label.
//
// Only cells that vertically overlap the label are taken: a month column holds
// several competitions, each shaded on its own rows, and the whole column's
// shading would run them together into one long event.
func bandDays(page Page, col column, b block) []int {
	// Half a row of slack at each end: a label is set inside its band rather
	// than flush with it, and the band often runs a row further than the text.
	top := b.top - col.rowHeight
	bottom := b.bottom + col.rowHeight

	seen := map[int]bool{}
	var days []int
	for _, rect := range page.Rects {
		if !isBand(rect, col) {
			continue
		}
		centre := (rect.Y0 + rect.Y1) / 2
		if centre < top || centre > bottom {
			continue
		}
		if d, ok := col.day(centre); ok && !seen[d] {
			seen[d] = true
			days = append(days, d)
		}
	}
	return days
}

// bandColor is the shading a competition is printed in, which the imported
// event keeps: the categories - federal, European, world - are told apart by
// colour on the page and by nothing else.
//
// The colour covering the most rows wins, so a band crossed by a single
// neutral cell keeps its own colour.
func bandColor(page Page, col column, b block) string {
	top := b.top - col.rowHeight
	bottom := b.bottom + col.rowHeight

	counts := map[string]int{}
	for _, rect := range page.Rects {
		if !isBand(rect, col) {
			continue
		}
		centre := (rect.Y0 + rect.Y1) / 2
		if centre < top || centre > bottom {
			continue
		}
		counts[rect.Hex()]++
	}

	best, bestCount := "", 0
	for hex, count := range counts {
		// Ties break on the hex so the same file always imports the same way.
		if count > bestCount || (count == bestCount && hex < best) {
			best, bestCount = hex, count
		}
	}
	return best
}

// isBand reports whether a filled rectangle is category shading in this month's
// column, as opposed to the grid itself.
//
// White is the empty grid, and the near-greys are its ruling and its weekend
// tint: neither says a competition happens. What is left is the printed
// category colours.
func isBand(rect Rect, col column) bool {
	if rect.IsWhite() || isNeutral(rect) {
		return false
	}
	if rect.Height() < col.rowHeight*0.6 || rect.Height() > col.rowHeight*1.4 {
		return false
	}
	centre := (rect.X0 + rect.X1) / 2
	return centre >= col.left && centre < col.right
}

// isNeutral reports whether a fill is a shade of grey. The planner rules its
// grid and tints its weekends in greys; every category colour is saturated.
func isNeutral(rect Rect) bool {
	high := math.Max(rect.R, math.Max(rect.G, rect.B))
	low := math.Min(rect.R, math.Min(rect.G, rect.B))
	return high-low < 0.08
}
