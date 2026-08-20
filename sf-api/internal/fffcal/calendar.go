package fffcal

import (
	"bytes"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Event is one competition read off the calendar.
//
// The dates are whole days, and End is exclusive - the start of the day after
// the last one - which is what iCalendar means by DTEND and what the app
// stores for an all-day event.
type Event struct {
	Title       string
	Description string
	Start       time.Time
	End         time.Time
	// Color is the shading the federation printed this competition in, as
	// #rrggbb, or empty when it sits on an unshaded row. The categories are
	// only distinguishable by colour on the printed page, so keeping it is the
	// difference between a legible imported calendar and forty identical rows.
	Color string
}

// Result is a parsed calendar: what was read, and what could not be.
type Result struct {
	Events []Event
	// Warnings name the labels that were found but not dated. A calendar is a
	// human document - some entries carry no date at all, only a shaded band
	// that turned out to be missing - and a coach should be told which ones
	// were skipped rather than left to notice the gap themselves.
	Warnings []string
}

// months are the column headers, lowercased and stripped of accents so a
// header reads the same however the PDF encoded it.
var months = map[string]time.Month{
	"janvier": time.January, "fevrier": time.February, "mars": time.March,
	"avril": time.April, "mai": time.May, "juin": time.June,
	"juillet": time.July, "aout": time.August, "septembre": time.September,
	"octobre": time.October, "novembre": time.November, "decembre": time.December,
}

// shortMonths cover the abbreviations that appear inside labels, where a date
// is written out ("30 au 1er Nov") rather than implied by the column.
var shortMonths = map[string]time.Month{
	"jan": time.January, "fev": time.February, "mar": time.March,
	"avr": time.April, "mai": time.May, "jui": time.June,
	"juil": time.July, "aou": time.August, "sep": time.September,
	"oct": time.October, "nov": time.November, "dec": time.December,
}

var (
	dayNumber  = regexp.MustCompile(`^\d{1,2}$`)
	yearInPage = regexp.MustCompile(`\b(20\d{2})\b`)
	// weekdays are the day-of-week column, which is neither a label nor a day
	// number and would otherwise be read as competition titles.
	weekdays = map[string]bool{
		"lun": true, "mar": true, "mer": true, "jeu": true,
		"ven": true, "sam": true, "dim": true, "d": true,
	}
)

// Calendar reads an uploaded FFForce season calendar, in whichever shape the
// federation published it.
//
// Told apart by the file's own first bytes rather than by its name or by what
// the browser called its content type: an upload is whatever it is, and a coach
// renaming a download should not decide how it is read.
func Calendar(data []byte) (Result, error) {
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return parsePlanner(data)
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		// A zip, which every .xlsx is. Whether it is one of these calendars is
		// a question about its headings, and parseWorkbook answers it by
		// finding nothing.
		return parseWorkbook(data)
	}
	return Result{}, ErrUnsupportedCalendar
}

// parsePlanner reads the year planner: six month columns per page, the days of
// each month down its column, and competitions written into the cells and
// shaded by category. Everything here is inferred from where things sit on the
// page, because that is the only structure the file has - a PDF records ink,
// not tables.
func parsePlanner(data []byte) (Result, error) {
	pages, err := Parse(data)
	if err != nil {
		return Result{}, err
	}

	var result Result
	for _, page := range pages {
		read(page, &result)
	}

	result.Events = withoutFragments(result.Events)

	sort.SliceStable(result.Events, func(a, b int) bool {
		if !result.Events[a].Start.Equal(result.Events[b].Start) {
			return result.Events[a].Start.Before(result.Events[b].Start)
		}
		return result.Events[a].Title < result.Events[b].Title
	})
	return result, nil
}

// withoutFragments drops the leftovers of an event that spans a month boundary.
//
// The planner repeats a competition's venue at the top of the next month to
// show it carries over. That repeat is a label like any other, in a column of
// its own, and it becomes a one-day event named after a town. It is recognized
// by being wholly contained in another event that already names it.
func withoutFragments(events []Event) []Event {
	// A fresh slice, not a filter in place: the inner loop reads every event,
	// and reusing the backing array would have it comparing against entries
	// already overwritten by the ones being kept.
	kept := make([]Event, 0, len(events))
	for i, event := range events {
		fragment := false
		for j, other := range events {
			if i == j || event.Title == "" {
				continue
			}
			if !strings.Contains(normalize(other.Description), normalize(event.Title)) {
				continue
			}
			if !event.Start.Before(other.Start) && !event.End.After(other.End) {
				fragment = true
				break
			}
		}
		if !fragment {
			kept = append(kept, event)
		}
	}
	return kept
}

// column is one month of the planner: where it sits across the page, and which
// row each of its days occupies down it.
type column struct {
	month       time.Month
	left, right float64
	// dayY maps a day of the month to the vertical middle of its row.
	dayY map[int]float64
	// rowHeight is how tall one day's row is, used to decide which row a label
	// or a shaded cell belongs to.
	rowHeight float64
	// top and bottom are the first and last day rows. Anything outside them is
	// page furniture - the document title above the grid, a footnote below it.
	top, bottom float64
}

// day returns the day of the month whose row contains y, and whether any does.
func (c column) day(y float64) (int, bool) {
	best, bestDistance := 0, math.MaxFloat64
	for day, centre := range c.dayY {
		if distance := math.Abs(centre - y); distance < bestDistance {
			best, bestDistance = day, distance
		}
	}
	// Half a row: past that, the label belongs to the neighbouring day.
	if best == 0 || bestDistance > c.rowHeight*0.6 {
		return 0, false
	}
	return best, true
}

func read(page Page, result *Result) {
	year, ok := pageYear(page)
	if !ok {
		return
	}
	columns := layout(page)
	if len(columns) == 0 {
		return
	}

	for _, col := range columns {
		for _, block := range blocks(page, col) {
			event, warning := interpret(block, col, year, page)
			if warning != "" {
				result.Warnings = append(result.Warnings, warning)
				continue
			}
			result.Events = append(result.Events, event)
		}
	}
}

// pageYear finds the season the page belongs to.
//
// The most common four-digit year on the page wins rather than the first:
// a planner carries its year in the title, in the legend and beside several
// deadlines, while a stray one appears at most once.
func pageYear(page Page) (int, bool) {
	counts := map[int]int{}
	for _, text := range page.Texts {
		for _, match := range yearInPage.FindAllStringSubmatch(text.S, -1) {
			if year, err := strconv.Atoi(match[1]); err == nil {
				counts[year]++
			}
		}
	}

	best, bestCount := 0, 0
	for year, count := range counts {
		// Ties break towards the earlier year, so the same file read twice
		// gives the same answer.
		if count > bestCount || (count == bestCount && year < best) {
			best, bestCount = year, count
		}
	}
	return best, best != 0
}

// layout works out where each month column is and which row each day occupies.
//
// The columns are found from the day numbers rather than from the month
// headings: a heading is one centred word whose position says little about the
// column's width, while the days are a column of thirty-odd numbers in a
// straight line, which fixes it exactly.
func layout(page Page) []column {
	clusters := dayClusters(page)
	if len(clusters) == 0 {
		return nil
	}
	headings := monthHeadings(page, gridTop(clusters))
	// One heading per column or the page was not read correctly, and a
	// calendar imported against the wrong months is worse than none.
	if len(clusters) != len(headings) {
		return nil
	}

	pitch := columnPitch(clusters)
	if pitch <= 0 {
		return nil
	}
	// The day numbers are not at the left edge of their column: the day-of-week
	// abbreviation sits to their left. A fifth of the pitch reaches back over
	// it without crossing into the previous month.
	margin := pitch * 0.2

	columns := make([]column, 0, len(clusters))
	for i, cluster := range clusters {
		left := cluster.x - margin
		right := left + pitch
		if i+1 < len(clusters) {
			right = clusters[i+1].x - margin
		}
		columns = append(columns, column{
			month:     headings[i],
			left:      left,
			right:     right,
			dayY:      cluster.dayY,
			rowHeight: cluster.rowHeight,
			top:       cluster.top,
			bottom:    cluster.bottom,
		})
	}
	return columns
}

// gridTop is the top of the day grid: no month heading sits below it.
func gridTop(clusters []cluster) float64 {
	top := math.MaxFloat64
	for _, c := range clusters {
		top = math.Min(top, c.top)
	}
	return top
}

type cluster struct {
	x         float64
	dayY      map[int]float64
	rowHeight float64
	// top and bottom are the y of this column's first and last day rows.
	top, bottom float64
}

// dayClusters finds the columns of day numbers, left to right.
//
// Grouped by sorting on x and breaking where the gap widens, rather than by
// rounding x into buckets: within one column the single-digit days and the
// two-digit ones are set a couple of points apart, and any fixed bucket
// eventually falls between them - splitting a month into two half-columns,
// each too short to be recognized as one at all.
func dayClusters(page Page) []cluster {
	var numbers []Text
	for _, text := range page.Texts {
		if !dayNumber.MatchString(text.S) {
			continue
		}
		if day, err := strconv.Atoi(text.S); err == nil && day >= 1 && day <= 31 {
			numbers = append(numbers, text)
		}
	}
	sort.Slice(numbers, func(a, b int) bool { return numbers[a].X < numbers[b].X })

	var clusters []cluster
	var group []Text
	flush := func() {
		// A month has 28 to 31 days. Well under that is a date written into a
		// label, not a column of days.
		if len(group) >= 20 {
			clusters = append(clusters, newCluster(group))
		}
		group = nil
	}
	for i, text := range numbers {
		// A column is a few points wide; the next column is a hundred away.
		if i > 0 && text.X-numbers[i-1].X > maxColumnSpread {
			flush()
		}
		group = append(group, text)
	}
	flush()

	return clusters
}

// maxColumnSpread is how far apart two day numbers can sit and still belong to
// the same column. Generous enough for the digit-width difference, far short
// of the distance to the next month.
const maxColumnSpread = 12

func newCluster(texts []Text) cluster {
	dayY := make(map[int]float64, len(texts))
	var sumX float64
	for _, text := range texts {
		day, _ := strconv.Atoi(text.S)
		// A day drawn twice keeps its first row: the planner sometimes
		// repeats a number in the margin.
		if _, seen := dayY[day]; !seen {
			dayY[day] = text.Y
		}
		sumX += text.X
	}
	top, bottom := extentOf(texts)
	return cluster{
		x:         sumX / float64(len(texts)),
		dayY:      dayY,
		rowHeight: rowHeight(dayY),
		top:       top,
		bottom:    bottom,
	}
}

// extentOf is the first and last day rows of a column: the vertical span of
// the grid itself.
func extentOf(texts []Text) (top, bottom float64) {
	top, bottom = math.MaxFloat64, -math.MaxFloat64
	for _, text := range texts {
		top = math.Min(top, text.Y)
		bottom = math.Max(bottom, text.Y)
	}
	return top, bottom
}

// rowHeight is the spacing between consecutive days, taken as a median so one
// misplaced number cannot stretch it.
func rowHeight(dayY map[int]float64) float64 {
	var gaps []float64
	for day := 1; day < 31; day++ {
		this, ok1 := dayY[day]
		next, ok2 := dayY[day+1]
		if ok1 && ok2 && next > this {
			gaps = append(gaps, next-this)
		}
	}
	if len(gaps) == 0 {
		return 0
	}
	sort.Float64s(gaps)
	return gaps[len(gaps)/2]
}

// columnPitch is the horizontal distance from one month to the next, again a
// median so a page with an odd column keeps a sane width.
func columnPitch(clusters []cluster) float64 {
	if len(clusters) < 2 {
		return 0
	}
	gaps := make([]float64, 0, len(clusters)-1)
	for i := 1; i < len(clusters); i++ {
		gaps = append(gaps, clusters[i].x-clusters[i-1].x)
	}
	sort.Float64s(gaps)
	return gaps[len(gaps)/2]
}

// monthHeadings returns the month of each column heading, left to right.
//
// Only headings above the grid count. A month is named inside the grid too -
// in a label like "30 au 1er Nov", or a competition called after where it is
// held - and counting those would leave more headings than there are columns,
// which is how this function reports that it did not understand the page.
func monthHeadings(page Page, top float64) []time.Month {
	type heading struct {
		x     float64
		month time.Month
	}
	var headings []heading
	for _, text := range page.Texts {
		if text.Y >= top {
			continue
		}
		if month, ok := months[normalize(text.S)]; ok {
			headings = append(headings, heading{x: text.X, month: month})
		}
	}
	sort.Slice(headings, func(a, b int) bool { return headings[a].x < headings[b].x })

	out := make([]time.Month, 0, len(headings))
	for i, h := range headings {
		// The same heading drawn twice - a shadow, or a redraw - is still one
		// column.
		if i > 0 && h.month == out[len(out)-1] && h.x-headings[i-1].x < maxColumnSpread {
			continue
		}
		out = append(out, h.month)
	}
	return out
}

// block is one competition's label: the lines written into a cell, and the
// vertical space they occupy.
type block struct {
	lines       []string
	top, bottom float64
}

// blocks groups a column's label lines into the competitions they describe.
//
// Grouping is by vertical adjacency alone. Within one month there is a single
// lane for labels, so anything in this column at consecutive line spacing is
// part of the same entry - and the entries themselves are separated by empty
// rows, since a competition takes up days that the next one cannot.
func blocks(page Page, col column) []block {
	var lines []Text
	for _, text := range page.Texts {
		if text.X < col.left || text.X >= col.right {
			continue
		}
		// Outside the day rows there is no day for a label to belong to: what
		// is up there is the document's own title, and below it a footnote.
		if text.Y < col.top-col.rowHeight || text.Y > col.bottom+col.rowHeight {
			continue
		}
		if isGridFurniture(text.S) {
			continue
		}
		lines = append(lines, text)
	}
	sort.Slice(lines, func(a, b int) bool { return lines[a].Y < lines[b].Y })

	// Two and a half rows: labels run to four lines at roughly half a row each,
	// and an entry sometimes leaves a blank line in the middle. Two separate
	// competitions in one month are far further apart than that.
	gap := col.rowHeight * 2.5

	var out []block
	var current *block
	var lastY float64
	for _, line := range lines {
		if current == nil || line.Y-lastY > gap {
			out = append(out, block{top: line.Y, bottom: line.Y})
			current = &out[len(out)-1]
		}
		current.lines = append(current.lines, line.S)
		current.bottom = line.Y
		lastY = line.Y
	}
	return out
}

// isGridFurniture reports whether a run is part of the grid rather than
// something written into it: the day numbers and the day-of-week column.
func isGridFurniture(text string) bool {
	trimmed := strings.TrimSpace(text)
	if dayNumber.MatchString(trimmed) {
		return true
	}
	if _, ok := months[normalize(trimmed)]; ok {
		return true
	}
	return weekdays[normalize(trimmed)]
}

// interpret turns one label into an event, or explains why it could not.
//
// The second return value is a warning: a label that was found but could not
// be dated. An empty title and an empty warning together mean there was
// nothing here to import.
func interpret(b block, col column, year int, page Page) (Event, string) {
	title, description, dated, ok := readLines(b.lines, col.month, year)
	if title == "" {
		return Event{}, ""
	}

	if !ok {
		// The label carried no date the parser recognized, so the shading is
		// what is left - the instruction's second choice, and the only other
		// place the planner records when something happens.
		dated, ok = bandSpan(page, col, b, year)
	}
	if !ok {
		return Event{}, title
	}

	return Event{
		Title:       title,
		Description: description,
		Start:       dated.start,
		End:         dated.end,
		Color:       bandColor(page, col, b),
	}, ""
}

// readLines splits a label into its title, its detail and its dates.
//
// Every line is offered to the date parser rather than one line being picked
// out as "the date" beforehand, because the planner writes the date and the
// venue together - "1-10 Pilsen (CZE)", "15-22 (Malta)", "3/ 9 Lituanie". A
// rule based on how a date line looks either rejects those or, loosened enough
// to accept them, starts reading titles as dates.
//
// Whatever is left of the date's own line stays in the detail, so the venue
// written beside it survives.
func readLines(lines []string, month time.Month, year int) (title, description string, dated span, ok bool) {
	var rest []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if !ok {
			if parsed, matched, found := parseSpan(trimmed, month, year); found {
				dated, ok = parsed, true
				if remainder := strings.TrimSpace(strings.Replace(trimmed, matched, "", 1)); remainder != "" {
					rest = append(rest, remainder)
				}
				continue
			}
		}

		if title == "" {
			title = trimmed
			continue
		}
		rest = append(rest, trimmed)
	}
	return title, strings.Join(rest, " - "), dated, ok
}

// normalize lowercases a run and strips the accents and punctuation that make
// the same word compare unequal - "AOÛT" against "aout", "Déc." against "dec".
func normalize(text string) string {
	replacer := strings.NewReplacer(
		"à", "a", "â", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
		"î", "i", "ï", "i", "ô", "o", "ö", "o", "û", "u", "ù", "u", "ü", "u", "ç", "c",
	)
	lowered := replacer.Replace(strings.ToLower(strings.TrimSpace(text)))
	return strings.Trim(lowered, ". \t")
}
