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
	"sort"
	"strings"

	"strong-fish-api/internal/models"
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
// is the comparison the sheet exists for. The load is the exception: rather
// than printing the computed weight and the lifted one in two columns, the one
// Load column carries what was actually lifted (see Row), so there is no
// second load column here.
func Columns(locale string, feedback bool) []Column {
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
		Column{Header: Heading(locale, "done"), Weight: 0.7},
		Column{Header: Heading(locale, "repsDone"), Weight: 0.9},
		Column{Header: Heading(locale, "rpeFelt"), Weight: 0.9},
		Column{Header: Heading(locale, "e1rm"), Weight: 1.0},
		Column{Header: Heading(locale, "comment"), Weight: 2.2},
	)
}

// Row is one set as a line of cells, in the order Columns gives.
//
// On a feedback sheet the Load column carries the weight the member logged,
// never the computed one. A sheet of what was done should not print a load
// nobody lifted: a blank there says the set was run without a weight being
// recorded, which is a different fact from "you were asked for 122.5 kg" - and
// the prescription is still on the row, in Reps and Intensity.
func Row(set models.ProgramSet, feedback bool) []string {
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
		doneMark(log.Done),
		formatIntPtr(log.ActualReps),
		formatFloatPtr(log.ActualRPE),
		formatKg(log.E1RM),
		log.Comment,
	)
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

// doneMark is an X rather than a tick: the PDF's font is cp1252, which has no
// check mark, and an X reads the same in a spreadsheet.
func doneMark(done bool) string {
	if done {
		return "X"
	}
	return ""
}
