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

	"github.com/go-pdf/fpdf"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programsheet"
)

const (
	fontSizePt   = 9.0
	lineHeightPt = 13.0
	titleSizePt  = 16.0
	weekSizePt   = 13.0
	daySizePt    = 10.5
)

// Options is everything the renderer needs that is not the program itself.
// Shared with the XLSX renderer, so the two documents cannot disagree about
// what a sheet contains (see programsheet).
type Options = programsheet.Options

// Render lays the program out as a PDF and returns the bytes.
//
// days must carry their sets (see models.ProgramDay.Sets); a day with none
// still gets its heading, because an empty session in the middle of a block is
// information - it is a rest day the coach wrote down.
func Render(program models.Program, days []models.ProgramDay, options Options) ([]byte, error) {
	// Landscape for a feedback sheet: eleven columns on a portrait page leaves
	// the exercise names two characters wide, and the whole point of the
	// document is reading the prescription against what was done.
	orientation := "P"
	if options.Feedback {
		orientation = "L"
	}
	pdf := fpdf.New(orientation, "pt", "A4", "")
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

	for _, week := range programsheet.Weeks(days) {
		pdf.AddPage()
		drawWeekHeader(pdf, translate, program, week.Number, options)

		for _, day := range week.Days {
			drawDay(pdf, translate, day, options)
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

func drawWeekHeader(pdf *fpdf.Fpdf, translate func(string) string, program models.Program,
	number int, options Options) {
	pdf.SetFont("Helvetica", "B", titleSizePt)
	pdf.CellFormat(0, 20, translate(program.Name), "", 1, "L", false, 0, "")

	// The subtitle carries what the sheet is *for*: whose maxes the loads came
	// from, and which club's program it is. On paper there is nothing else to
	// tell two printouts apart.
	if subtitle := programsheet.Subtitle(program, options.MemberName); subtitle != "" {
		pdf.SetFont("Helvetica", "", fontSizePt)
		pdf.SetTextColor(110, 110, 110)
		pdf.CellFormat(0, 12, translate(subtitle), "", 1, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	pdf.Ln(6)
	pdf.SetFont("Helvetica", "B", weekSizePt)
	pdf.CellFormat(0, 16, translate(programsheet.WeekLabel(options.Locale, number)), "", 1, "L", false, 0, "")
	pdf.Ln(2)
}

func drawDay(pdf *fpdf.Fpdf, translate func(string) string, day models.ProgramDay, options Options) {
	title := programsheet.DayTitle(day, options.Locale)

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
		pdf.CellFormat(0, 12, translate(programsheet.RestLabel(options.Locale)), "", 1, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		return
	}

	sets := programsheet.SortedSets(day)
	rows := make([][]string, 0, len(sets))
	for _, set := range sets {
		rows = append(rows, programsheet.Row(set, options.Feedback))
	}

	drawTable(pdf, translate, programsheet.Columns(options.Locale, options.Feedback), rows)
}
