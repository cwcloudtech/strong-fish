// Package programsheet is what a program looks like as a table: which columns
// a sheet has, what goes in each cell, and what the headings are called.
//
// It exists because the same document is produced in two formats. A PDF is
// what a coach prints and hands out; an XLSX is what an athlete opens, fills
// in and mails back. If each renderer decided for itself what "Intensity"
// means, the two would drift - and a coach reading a spreadsheet next to a
// printout would have to work out which one was lying.
//
// So the renderers here own layout and nothing else: programpdf draws these
// rows with fpdf, programxlsx writes them with excelize, and both ask this
// package what the rows are.
package programsheet

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/velocity"
)

// Options is everything a sheet needs that is not the program itself.
type Options struct {
	// MemberName is who the sheet was made for, when it was made against
	// somebody's own maxes. Blank renders the program as authored.
	MemberName string
	// Locale picks the language of the headings.
	Locale string
	// Footer is the attribution line, where the format has somewhere to put
	// one.
	Footer string
	// Feedback adds what the member actually did beside what was prescribed:
	// the sheet a lifter sends their coach at the end of a week.
	Feedback bool
	// EffortMetric is how effort is shown on this sheet - "rpe" (the default) or
	// "rir". It only flips the display; effort is always computed as RPE.
	EffortMetric string
	// Readings is what the AI said about a session, by day id - one entry per
	// reading, since a session may be read more than once. Printed under the
	// table they are about: a reading filed separately from the week it
	// concerns is a reading nobody reads again.
	Readings map[string][]string
}

// Column is one column: its heading, and its share of the width relative to
// the others. The weight is only meaningful to a renderer that has to fit a
// page; a spreadsheet uses it to size the column sensibly.
type Column struct {
	Header string
	Weight float64
}

// Columns is the sheet's layout. The exercise gets the space, because it is
// the only cell whose text is a name rather than a number.
//
// With feedback on, what happened is added to the right of the prescription -
// read left to right, a row says what was asked and then what was done, which
// is the comparison the sheet exists for. Two exceptions, both to spend the
// page on numbers rather than on columns: the one Load column carries what was
// actually lifted (see Row), and whether a set was ticked off is the row's
// colour (see Line) rather than a column of crosses.
func Columns(locale string, feedback bool, metric string) []Column {
	columns := []Column{
		{Header: Heading(locale, "exercise"), Weight: 3.4},
		{Header: Heading(locale, "reps"), Weight: 0.8},
		{Header: Heading(locale, "intensity"), Weight: 1.3},
		{Header: Heading(locale, "load"), Weight: 1.1},
		{Header: Heading(locale, "notes"), Weight: 2.2},
	}
	if !feedback {
		return columns
	}
	return append(columns,
		Column{Header: Heading(locale, "repsDone"), Weight: 0.9},
		Column{Header: effortHeading(locale, metric), Weight: 0.9},
		Column{Header: Heading(locale, "e1rm"), Weight: 1.0},
		Column{Header: Heading(locale, "vbt"), Weight: 1.6},
		Column{Header: Heading(locale, "comment"), Weight: 2.0},
	)
}

// effortHeading names the logged-effort column in the member's chosen scale -
// "RPE felt" or, for a member who reads in reps-in-reserve, "RIR left".
func effortHeading(locale, metric string) string {
	if models.NormalizeEffortMetric(metric) == models.EffortMetricRIR {
		return Heading(locale, "rirLeft")
	}
	return Heading(locale, "rpeFelt")
}

// Row is one set as a line of cells, in the order Columns gives.
//
// On a feedback sheet the Load column carries the weight the member logged,
// never the computed one. A sheet of what was done should not print a load
// nobody lifted: a blank there says the set was run without a weight being
// recorded, which is a different fact from "you were asked for 122.5 kg" - and
// the prescription is still on the row, in Reps and Intensity.
func Row(set models.ProgramSet, feedback bool, locale, metric string) []string {
	log := set.Log

	load := FormatLoad(set)
	if feedback {
		load = ""
		if log != nil {
			load = formatKgPtr(log.ActualLoad)
		}
	}

	cells := []string{
		ExerciseName(set),
		formatInt(set.Reps),
		FormatIntensity(set),
		load,
		set.Notes,
	}
	if !feedback {
		return cells
	}

	if log == nil {
		// A set with no log at all still gets its cells, empty: a gap in the
		// week is what the coach is looking for.
		return append(cells, "", "", "", "", "")
	}
	return append(cells,
		repsDone(*log, locale),
		formatEffortPtr(log.ActualRPE, metric),
		formatKg(log.E1RM),
		formatVBT(log.VBT, set.ExerciseOneRMRef, metric, locale),
		log.Comment,
	)
}

// formatEffortPtr writes a logged RPE in the member's chosen scale: the RPE as
// stored, or the reps-in-reserve it implies (10 - RPE) when they read in RIR.
// Blank when nothing was logged.
func formatEffortPtr(rpe *float64, metric string) string {
	if rpe == nil {
		return ""
	}
	if models.NormalizeEffortMetric(metric) == models.EffortMetricRIR {
		return TrimFloat(models.RIRFromRPE(*rpe))
	}
	return TrimFloat(*rpe)
}

// formatVBT is the export's VBT cell: what the velocity tracker read for the
// set. The headline first - velocity loss, then mean and peak speed - and then
// each rep's concentric speed on its own line, so the sheet (and the AI reading
// it) has the set rep by rep, not just its summary. Empty when nothing recorded.
func formatVBT(vbt *models.BarSetVBT, movement, metric, locale string) string {
	if vbt == nil || vbt.MeanVelocity <= 0 {
		return ""
	}
	var head []string
	if vbt.VelocityLoss > 0 {
		head = append(head, fmt.Sprintf(Heading(locale, "vbtLoss"), fmt.Sprintf("%.0f", vbt.VelocityLoss)))
	}
	head = append(head, fmt.Sprintf(Heading(locale, "vbtMean"), fmt.Sprintf("%.2f", vbt.MeanVelocity)))
	if vbt.PeakVelocity > 0 {
		head = append(head, fmt.Sprintf(Heading(locale, "vbtPeak"), fmt.Sprintf("%.2f", vbt.PeakVelocity)))
	}
	// The velocity-based effort estimate, read per movement off the last rep's
	// speed - the headline, right after the loss, since it is the one number a
	// reader wants from the velocities. Nothing when the movement is not one of
	// the three lifts (no model to read it against).
	if effort := EffortLine(movement, lastRep(vbt), metric, locale); effort != "" {
		head = append(head, effort)
	}
	lines := []string{strings.Join(head, " · ")}
	for i, v := range vbt.RepVelocities {
		lines = append(lines, fmt.Sprintf(Heading(locale, "vbtRep"), i+1, fmt.Sprintf("%.2f", v)))
	}
	return strings.Join(lines, "\n")
}

// lastRep is the set's effort signal: its last (slowest) rep's mean velocity,
// or the set mean when the per-rep series was not carried.
func lastRep(vbt *models.BarSetVBT) float64 {
	if n := len(vbt.RepVelocities); n > 0 {
		return vbt.RepVelocities[n-1]
	}
	return vbt.MeanVelocity
}

// EffortLine is the velocity-based effort estimate as one labelled cell - "est.
// RPE 7" or, for a member who reads in reps-in-reserve, "est. RIR 3". Blank when
// the movement has no model or the velocity is unreadable, so a caller can add
// it unconditionally.
func EffortLine(movement string, lastRepVelocity float64, metric, locale string) string {
	estimate, ok := velocity.EstimateEffort(movement, lastRepVelocity)
	if !ok {
		return ""
	}
	if models.NormalizeEffortMetric(metric) == models.EffortMetricRIR {
		return fmt.Sprintf(Heading(locale, "vbtEffortRir"), TrimFloat(estimate.RIR))
	}
	return fmt.Sprintf(Heading(locale, "vbtEffortRpe"), TrimFloat(estimate.RPE))
}

// Line is one set as a row of cells, plus whether the member ticked it off.
//
// Done is not a cell: a column of crosses costs a tenth of the width on every
// page to say something a colour says at a glance, and what a coach reads a
// feedback sheet for is which sets were missed - which is a shape on the page,
// not a value to scan for.
type Line struct {
	Cells []string
	Done  bool
}

// Section is one titled part of a sheet that has several - the health
// export's steps, then its heart rate, then the latest body composition.
// Heading may be blank for a sheet whose title already says what it is.
//
// A section is either a table (Columns and Rows) or, when Chart is set, a
// histogram. Days of numbers read as a shape - which week was idle, which day
// peaked - and a column of ninety figures hides exactly that.
type Section struct {
	Heading string
	Columns []Column
	Rows    []Line
	Chart   *Chart
}

// Chart is a histogram: one bar per entry, oldest first, each in its own
// colour, with the legend that says what the colours mean.
type Chart struct {
	Bars   []Bar
	Legend []LegendEntry
}

// Bar is one day of a histogram.
type Bar struct {
	// Label goes under the bar - a short date. The renderer thins the labels
	// out when there are too many bars for all of them to fit.
	Label string
	// Value is the bar's height, against the tallest bar in the chart.
	Value float64
	// Caption is printed above the bar when there is room - the figure
	// itself, so a short range can be read without an axis.
	Caption string
	Color   color.RGBA
}

// LegendEntry is one colour of a chart and what it means.
type LegendEntry struct {
	Label string
	Color color.RGBA
}

// Lines is a session's sets as rows, in the order they are performed.
func Lines(sets []models.ProgramSet, feedback bool, locale, metric string) []Line {
	lines := make([]Line, 0, len(sets))
	for _, set := range sets {
		lines = append(lines, Line{
			Cells: Row(set, feedback, locale, metric),
			Done:  feedback && set.Log != nil && set.Log.Done,
		})
	}
	return lines
}

// repsDone is what the member actually got, and whether they got it without
// their belt: "2 beltless".
//
// A mention on the reps rather than a column of its own. A column would spend
// width on every sheet - including every bench and every accessory - to answer
// a question only the squat and the deadlift ask, and it would be empty almost
// everywhere it was printed.
func repsDone(log models.SetLog, locale string) string {
	reps := formatIntPtr(log.ActualReps)
	if !log.Beltless {
		return reps
	}
	if reps == "" {
		// Somebody who ticked the switch but never wrote a rep count still
		// said something worth printing.
		return Heading(locale, "beltless")
	}
	return reps + " " + Heading(locale, "beltless")
}

// Weeks groups the sessions into weeks, in order.
//
// Grouped rather than assumed contiguous: an imported spreadsheet can skip a
// week number, and a page headed "week 3" carrying week 4's sessions would be
// worse than a missing page.
func Weeks(days []models.ProgramDay) []Week {
	byNumber := map[int][]models.ProgramDay{}
	for _, day := range days {
		byNumber[day.Week] = append(byNumber[day.Week], day)
	}

	numbers := make([]int, 0, len(byNumber))
	for number := range byNumber {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)

	weeks := make([]Week, 0, len(numbers))
	for _, number := range numbers {
		sessions := byNumber[number]
		sort.SliceStable(sessions, func(a, b int) bool {
			if sessions[a].Day != sessions[b].Day {
				return sessions[a].Day < sessions[b].Day
			}
			return sessions[a].Position < sessions[b].Position
		})
		weeks = append(weeks, Week{Number: number, Days: sessions})
	}
	return weeks
}

// Week is one week of the block with its sessions in order.
type Week struct {
	Number int
	Days   []models.ProgramDay
}

// SortedSets is a session's sets in the order they are performed.
func SortedSets(day models.ProgramDay) []models.ProgramSet {
	sets := append([]models.ProgramSet(nil), day.Sets...)
	sort.SliceStable(sets, func(a, b int) bool { return sets[a].Position < sets[b].Position })
	return sets
}

// DayTitle is what a session is called: the coach's own title when they gave
// it one, and the day number otherwise.
func DayTitle(day models.ProgramDay, locale string) string {
	if title := strings.TrimSpace(day.Title); title != "" {
		return title
	}
	return DayLabel(locale, day.Day)
}

// Subtitle carries what the sheet is *for*: whose maxes the loads came from,
// and which club's program it is. On paper there is nothing else to tell two
// printouts apart.
func Subtitle(program models.Program, memberName string) string {
	parts := []string{}
	if memberName != "" {
		parts = append(parts, memberName)
	}
	if program.ClubName != "" {
		parts = append(parts, program.ClubName)
	}
	return strings.Join(parts, "  -  ")
}

// ExerciseName is what to call the movement: its label in the sheet's
// language, falling back to the slug, which is at least unambiguous.
func ExerciseName(set models.ProgramSet) string {
	for _, key := range []string{"en", "fr"} {
		if label := strings.TrimSpace(set.ExerciseLabels[key]); label != "" {
			return label
		}
	}
	for _, label := range set.ExerciseLabels {
		if trimmed := strings.TrimSpace(label); trimmed != "" {
			return trimmed
		}
	}
	return set.ExerciseSlug
}

// FormatIntensity prints what the coach prescribed, not what was derived: an
// RPE set says RPE, a percentage set says the percentage. Both when both were
// written, because a coach who wrote both meant both.
func FormatIntensity(set models.ProgramSet) string {
	parts := []string{}
	if set.RPE != nil {
		parts = append(parts, fmt.Sprintf("RPE %s", TrimFloat(*set.RPE)))
	}
	if set.Percentage != nil {
		parts = append(parts, fmt.Sprintf("%s%%", TrimFloat(*set.Percentage)))
	}
	return strings.Join(parts, " / ")
}

// FormatLoad prints the weight to put on the bar.
//
// The rounded load is what a lifter loads, so that is what is printed; the
// exact figure is a computation, not an instruction. A set whose load could
// not be worked out - no 1RM recorded - prints blank rather than zero, which
// would read as an empty bar.
func FormatLoad(set models.ProgramSet) string {
	if set.AbsoluteLoad != nil && *set.AbsoluteLoad > 0 {
		return formatKg(*set.AbsoluteLoad)
	}
	if !set.LoadKnown || set.RoundedLoad <= 0 {
		return ""
	}
	return formatKg(set.RoundedLoad)
}

// TrimFloat prints a number the way a coach writes it: 7.5 stays 7.5, and
// 100.0 becomes 100.
func TrimFloat(value float64) string {
	return strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
}

func formatInt(value int) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func formatIntPtr(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func formatFloatPtr(value *float64) string {
	if value == nil {
		return ""
	}
	return TrimFloat(*value)
}

func formatKg(value float64) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%s kg", TrimFloat(value))
}

func formatKgPtr(value *float64) string {
	if value == nil {
		return ""
	}
	return formatKg(*value)
}

// ReadingOf is what the model said about one session, stripped down to lines a
// renderer can print, or nothing when it was never asked.
func ReadingOf(options Options, dayID string) []string {
	if len(options.Readings) == 0 {
		return nil
	}

	lines := []string{}
	for _, reading := range options.Readings[dayID] {
		// A rule between them, because two readings of the same session run
		// together read as one that contradicts itself.
		if len(lines) > 0 {
			lines = append(lines, readingSeparator)
		}
		lines = append(lines, PlainLines(reading)...)
	}
	return lines
}

// readingSeparator is what sits between two readings of one session. A row of
// dashes rather than a blank line: every renderer here drops empty lines.
const readingSeparator = "---"

// PlainLines strips a markdown answer down to what it says, one line per line.
//
// The model answers in markdown and none of the three renderers draws
// markdown: a spreadsheet cell, a PDF paragraph and a PNG line are all plain
// text. The line breaks are kept because they carry the shape of the answer -
// a heading, then its bullets - which is most of what the formatting was
// doing; the markers themselves would just be noise on paper.
func PlainLines(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if plain := plainLine(line); plain != "" {
			lines = append(lines, plain)
		}
	}
	return lines
}

func plainLine(line string) string {
	trimmed := strings.TrimSpace(line)

	switch {
	case strings.HasPrefix(trimmed, "#"):
		trimmed = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
	case strings.HasPrefix(trimmed, "> "):
		trimmed = strings.TrimSpace(trimmed[2:])
	case strings.HasPrefix(trimmed, "* "):
		// Kept as a bullet rather than flattened: a list that reads as a
		// paragraph of sentences is a list nobody can scan.
		trimmed = "- " + strings.TrimSpace(trimmed[2:])
	}

	// The emphasis markers, longest first so "**" is gone before "*" is
	// looked for. Underscores are left alone except in pairs: a single one is
	// more often part of a word than emphasis around it.
	for _, marker := range []string{"**", "__", "~~", "*", "`"} {
		trimmed = strings.ReplaceAll(trimmed, marker, "")
	}
	return strings.TrimSpace(trimmed)
}
