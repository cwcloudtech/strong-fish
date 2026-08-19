// Package programpdf renders a training program as a printable PDF, one sheet
// per week.
//
// A program is written to be taken into a gym, and a phone is a poor thing to
// hold while your hands are chalked. A sheet per week is how coaches hand
// blocks out on paper, so that is the shape: one page per week, one table per
// session on it, and the loads already worked out for whoever it is printed
// for.
//
// Built with go-pdf/fpdf, the library ~/cwclock renders its reports with,
// including a port of its drawTable - fpdf has no table of its own, and a cell
// wider than its column silently overflows into the next one unless every
// row's height is measured up front.
package programpdf

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/go-pdf/fpdf"

	"strong-fish-api/internal/models"
)

const (
	fontSizePt   = 9.0
	lineHeightPt = 13.0
	titleSizePt  = 16.0
	weekSizePt   = 13.0
	daySizePt    = 10.5
)

// Options is everything the renderer needs that is not the program itself.
type Options struct {
	// MemberName is who the sheet was printed for, when it was printed against
	// somebody's own maxes. Blank prints the program as authored.
	MemberName string
	// Locale picks the language of the column headings.
	Locale string
	// Footer is the attribution line at the bottom of every page.
	Footer string
}

// Render lays the program out as a PDF and returns the bytes.
//
// days must carry their sets (see models.ProgramDay.Sets); a day with none
// still gets its heading, because an empty session in the middle of a block is
// information - it is a rest day the coach wrote down.
func Render(program models.Program, days []models.ProgramDay, options Options) ([]byte, error) {
	pdf := fpdf.New("P", "pt", "A4", "")
	pdf.SetMargins(36, 40, 36)
	pdf.SetAutoPageBreak(true, 40)

	// cp1252 covers the accented Latin characters French needs. Without it a
	// non-ASCII rune is written as raw UTF-8 into a cp1252-encoded font and
	// comes out as mojibake - the same reason cwclock's renderer sets it.
	translate := pdf.UnicodeTranslatorFromDescriptor("")

	if options.Footer != "" {
		pdf.SetFooterFunc(func() {
			pdf.SetY(-24)
			pdf.SetFont("Helvetica", "I", 7)
			pdf.SetTextColor(170, 170, 170)
			pdf.CellFormat(0, 10, translate(options.Footer), "", 0, "C", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
		})
	}

	for _, week := range weeksOf(days) {
		pdf.AddPage()
		drawWeekHeader(pdf, translate, program, week.number, options)

		for _, day := range week.days {
			drawDay(pdf, translate, day, options.Locale)
		}
	}

	// A program with no sessions still produces a sheet rather than an empty
	// file: what a coach gets back should say the block is empty, not look
	// like the export broke.
	if pdf.PageNo() == 0 {
		pdf.AddPage()
		drawWeekHeader(pdf, translate, program, 0, options)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type week struct {
	number int
	days   []models.ProgramDay
}

// weeksOf groups the sessions into weeks, in order.
//
// Grouped rather than assumed contiguous: an imported spreadsheet can skip a
// week number, and a page headed "week 3" with week 4's sessions on it would be
// worse than a missing page.
func weeksOf(days []models.ProgramDay) []week {
	byNumber := map[int][]models.ProgramDay{}
	for _, day := range days {
		byNumber[day.Week] = append(byNumber[day.Week], day)
	}

	numbers := make([]int, 0, len(byNumber))
	for number := range byNumber {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)

	weeks := make([]week, 0, len(numbers))
	for _, number := range numbers {
		sessions := byNumber[number]
		sort.SliceStable(sessions, func(a, b int) bool {
			if sessions[a].Day != sessions[b].Day {
				return sessions[a].Day < sessions[b].Day
			}
			return sessions[a].Position < sessions[b].Position
		})
		weeks = append(weeks, week{number: number, days: sessions})
	}
	return weeks
}

func drawWeekHeader(pdf *fpdf.Fpdf, translate func(string) string, program models.Program,
	number int, options Options) {
	pdf.SetFont("Helvetica", "B", titleSizePt)
	pdf.CellFormat(0, 20, translate(program.Name), "", 1, "L", false, 0, "")

	// The subtitle carries what the sheet is *for*: whose maxes the loads came
	// from, and which club's program it is. On paper there is nothing else to
	// tell two printouts apart.
	subtitle := []string{}
	if options.MemberName != "" {
		subtitle = append(subtitle, options.MemberName)
	}
	if program.ClubName != "" {
		subtitle = append(subtitle, program.ClubName)
	}
	if len(subtitle) > 0 {
		pdf.SetFont("Helvetica", "", fontSizePt)
		pdf.SetTextColor(110, 110, 110)
		pdf.CellFormat(0, 12, translate(strings.Join(subtitle, "  -  ")), "", 1, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	pdf.Ln(6)
	pdf.SetFont("Helvetica", "B", weekSizePt)
	pdf.CellFormat(0, 16, translate(weekLabel(options.Locale, number)), "", 1, "L", false, 0, "")
	pdf.Ln(2)
}

func drawDay(pdf *fpdf.Fpdf, translate func(string) string, day models.ProgramDay, locale string) {
	title := strings.TrimSpace(day.Title)
	if title == "" {
		title = dayLabel(locale, day.Day)
	}

	// Kept with its table: a session heading stranded at the foot of a page,
	// with its sets overleaf, is the one layout failure that actually matters
	// on a printed sheet.
	_, pageHeight := pdf.GetPageSize()
	_, _, _, bottom := pdf.GetMargins()
	if pdf.GetY()+lineHeightPt*3 > pageHeight-bottom {
		pdf.AddPage()
	}

	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", daySizePt)
	pdf.CellFormat(0, 14, translate(title), "", 1, "L", false, 0, "")

	if len(day.Sets) == 0 {
		pdf.SetFont("Helvetica", "I", fontSizePt)
		pdf.SetTextColor(110, 110, 110)
		pdf.CellFormat(0, 12, translate(restLabel(locale)), "", 1, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		return
	}

	sets := append([]models.ProgramSet(nil), day.Sets...)
	sort.SliceStable(sets, func(a, b int) bool { return sets[a].Position < sets[b].Position })

	rows := make([][]string, 0, len(sets))
	for _, set := range sets {
		rows = append(rows, []string{
			exerciseName(set),
			formatReps(set.Reps),
			formatIntensity(set),
			formatLoad(set),
			set.Notes,
		})
	}

	drawTable(pdf, translate, columnsFor(locale), rows)
}

// exerciseName is what to call the movement on paper: its label in the sheet's
// language, falling back to the slug, which is at least unambiguous.
func exerciseName(set models.ProgramSet) string {
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

func formatReps(reps int) string {
	if reps <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", reps)
}

// formatIntensity prints what the coach prescribed, not what was derived: an
// RPE set says RPE, a percentage set says the percentage. Both when both were
// written, because a coach who wrote both meant both.
func formatIntensity(set models.ProgramSet) string {
	parts := []string{}
	if set.RPE != nil {
		parts = append(parts, fmt.Sprintf("RPE %s", trimFloat(*set.RPE)))
	}
	if set.Percentage != nil {
		parts = append(parts, fmt.Sprintf("%s%%", trimFloat(*set.Percentage)))
	}
	return strings.Join(parts, " / ")
}

// formatLoad prints the weight to put on the bar.
//
// The rounded load is what a lifter loads, so that is what is printed; the
// exact figure is a computation, not an instruction. A set whose load could not
// be worked out - no 1RM recorded - prints blank rather than zero, which would
// read as an empty bar.
func formatLoad(set models.ProgramSet) string {
	if set.AbsoluteLoad != nil && *set.AbsoluteLoad > 0 {
		return fmt.Sprintf("%s kg", trimFloat(*set.AbsoluteLoad))
	}
	if !set.LoadKnown || set.RoundedLoad <= 0 {
		return ""
	}
	return fmt.Sprintf("%s kg", trimFloat(set.RoundedLoad))
}

// trimFloat prints a number the way a coach writes it: 7.5 stays 7.5, and 100.0
// becomes 100.
func trimFloat(value float64) string {
	text := fmt.Sprintf("%.1f", value)
	return strings.TrimSuffix(text, ".0")
}
