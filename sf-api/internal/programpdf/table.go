package programpdf

import (
	"github.com/go-pdf/fpdf"

	"strong-fish-api/internal/programsheet"
)

// drawTable renders a bordered table directly with fpdf, ported from
// ~/cwclock's report tables.
//
// fpdf has no table: CellFormat draws one cell, and a cell whose text is wider
// than its column overflows past its border rather than wrapping - where the
// next cell's opaque fill then paints over it, so the text is not merely ugly
// but gone. Every cell's wrapped lines are measured up front with fpdf's own
// SplitLines (so the measurement matches how the text will really break), every
// cell in a row is given the tallest one's height, and each line is drawn at an
// explicit (x, y) rather than wherever the previous cell left the cursor.
func drawTable(pdf *fpdf.Fpdf, translate func(string) string, columns []programsheet.Column, rows []programsheet.Line) {
	left, _, right, bottom := pdf.GetMargins()
	pageWidth, pageHeight := pdf.GetPageSize()
	usable := pageWidth - left - right

	total := 0.0
	for _, c := range columns {
		total += c.Weight
	}
	widths := make([]float64, len(columns))
	for i, c := range columns {
		widths[i] = usable * c.Weight / total
	}

	drawHeader := func() {
		pdf.SetFont("Helvetica", "B", fontSizePt)
		pdf.SetFillColor(14, 94, 155)
		pdf.SetTextColor(255, 255, 255)
		y := pdf.GetY()
		x := left
		for i, c := range columns {
			pdf.SetXY(x, y)
			pdf.CellFormat(widths[i], lineHeightPt, translate(c.Header), "1", 0, "C", true, 0, "")
			x += widths[i]
		}
		pdf.SetXY(left, y+lineHeightPt)
		pdf.SetFillColor(244, 246, 249)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Helvetica", "", fontSizePt)
	}

	drawHeader()

	zebra := false
	for _, row := range rows {
		lines := make([][]string, len(row.Cells))
		tallest := 1
		for i, cell := range row.Cells {
			split := pdf.SplitLines([]byte(translate(cell)), widths[i])
			wrapped := make([]string, len(split))
			for j, line := range split {
				wrapped[j] = string(line)
			}
			if len(wrapped) == 0 {
				wrapped = []string{""}
			}
			lines[i] = wrapped
			if len(wrapped) > tallest {
				tallest = len(wrapped)
			}
		}

		height := float64(tallest) * lineHeightPt
		if pdf.GetY()+height > pageHeight-bottom {
			pdf.AddPage()
			drawHeader()
		}

		// A ticked-off set is a green line rather than a column of crosses:
		// what a coach reads a feedback sheet for is which sets were missed,
		// and that is a shape on the page. The alternating grey underneath is
		// what keeps a long unfinished week readable.
		filled := true
		switch {
		case row.Done:
			pdf.SetFillColor(223, 246, 228)
		case zebra:
			pdf.SetFillColor(244, 246, 249)
		default:
			filled = false
		}

		y := pdf.GetY()
		x := left
		for i := range columns {
			for j := 0; j < tallest; j++ {
				line := ""
				if j < len(lines[i]) {
					line = lines[i][j]
				}
				// The border is split across a wrapped cell's lines so the box
				// closes once around the whole cell rather than once per line.
				border := "LR"
				switch {
				case tallest == 1:
					border = "1"
				case j == 0:
					border = "LRT"
				case j == tallest-1:
					border = "LRB"
				}
				pdf.SetXY(x, y+float64(j)*lineHeightPt)
				pdf.CellFormat(widths[i], lineHeightPt, line, border, 0, "", filled, 0, "")
			}
			x += widths[i]
		}
		pdf.SetXY(left, y+height)
		zebra = !zebra
	}
}
