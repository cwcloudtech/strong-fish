// Package programxlsx renders a training program as a spreadsheet, one sheet
// per week.
//
// The PDF beside it (see programpdf) is what a coach prints and hands out.
// This is the other half: a workbook an athlete opens on a laptop, fills in
// and mails back, and that a coach can sort and filter. Both are laid out from
// the same tables (see programsheet), so the two documents cannot come to
// disagree about what a set says.
//
// Built with excelize, already in this module for reading the programs coaches
// upload - the same library now writing what it used to only read.
package programxlsx

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programsheet"
)

// Options is everything the renderer needs that is not the program itself.
type Options = programsheet.Options

const (
	// Column widths are the sheet's weights scaled to something a spreadsheet
	// reads in characters rather than points.
	widthPerWeight = 9.0
	minWidth       = 8.0
)

// Render lays the program out as a workbook and returns the bytes.
//
// days must carry their sets (see models.ProgramDay.Sets); a session with none
// still gets its heading, because an empty session in the middle of a block is
// information - it is a rest day the coach wrote down.
func Render(program models.Program, days []models.ProgramDay, options Options) ([]byte, error) {
	file := excelize.NewFile()
	defer file.Close()

	styles, err := newStyles(file)
	if err != nil {
		return nil, err
	}

	weeks := programsheet.Weeks(days)
	// A program with no sessions still produces a workbook rather than an
	// empty file: what somebody gets back should say the block is empty, not
	// look like the export broke.
	if len(weeks) == 0 {
		weeks = []programsheet.Week{{Number: 0}}
	}

	for index, week := range weeks {
		name := sheetName(options.Locale, week.Number, index)
		if index == 0 {
			// excelize starts every workbook with a "Sheet1"; renaming it is
			// how the first week avoids being an empty tab beside its own.
			if err := file.SetSheetName(file.GetSheetName(0), name); err != nil {
				return nil, err
			}
		} else if _, err := file.NewSheet(name); err != nil {
			return nil, err
		}

		if err := writeWeek(file, name, program, week, options, styles); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// styles are the few formats the workbook uses, created once: excelize numbers
// styles per workbook, so building them per row would grow the file for
// nothing.
type styles struct {
	title    int
	subtitle int
	day      int
	header   int
	cell     int
	// cellDone is the same cell on a green ground: a set the member ticked off.
	// A colour rather than a "Done" column, so the width goes to the numbers.
	cellDone int
}

func newStyles(file *excelize.File) (styles, error) {
	var s styles
	var err error

	if s.title, err = file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14},
	}); err != nil {
		return s, err
	}
	if s.subtitle, err = file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 10, Color: "6E6E6E"},
	}); err != nil {
		return s, err
	}
	if s.day, err = file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
	}); err != nil {
		return s, err
	}
	// The brand navy the PDF's header row uses, so the two documents look like
	// they came from the same app.
	if s.header, err = file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"0E5E9B"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	}); err != nil {
		return s, err
	}
	if s.cell, err = file.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	}); err != nil {
		return s, err
	}
	// Light enough that black text on it still reads, and that a page of them
	// prints without soaking a cartridge.
	if s.cellDone, err = file.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"DFF6E4"}},
	}); err != nil {
		return s, err
	}
	return s, nil
}

func writeWeek(file *excelize.File, sheet string, program models.Program,
	week programsheet.Week, options Options, styles styles) error {
	columns := programsheet.Columns(options.Locale, options.Feedback)
	if err := setWidths(file, sheet, columns); err != nil {
		return err
	}

	row := 1
	if err := writeCell(file, sheet, 1, row, program.Name, styles.title); err != nil {
		return err
	}
	row++
	if subtitle := programsheet.Subtitle(program, options.MemberName); subtitle != "" {
		if err := writeCell(file, sheet, 1, row, subtitle, styles.subtitle); err != nil {
			return err
		}
		row++
	}
	if err := writeCell(file, sheet, 1, row, programsheet.WeekLabel(options.Locale, week.Number), styles.day); err != nil {
		return err
	}
	row += 2

	for _, day := range week.Days {
		if err := writeCell(file, sheet, 1, row, programsheet.DayTitle(day, options.Locale), styles.day); err != nil {
			return err
		}
		row++

		sets := programsheet.SortedSets(day)
		if len(sets) == 0 {
			if err := writeCell(file, sheet, 1, row, programsheet.RestLabel(options.Locale), styles.subtitle); err != nil {
				return err
			}
			row += 2
			continue
		}

		for column, definition := range columns {
			if err := writeCell(file, sheet, column+1, row, definition.Header, styles.header); err != nil {
				return err
			}
		}
		row++

		for _, line := range programsheet.Lines(sets, options.Feedback, options.Locale) {
			style := styles.cell
			if line.Done {
				style = styles.cellDone
			}
			for column, value := range line.Cells {
				if err := writeCell(file, sheet, column+1, row, value, style); err != nil {
					return err
				}
			}
			row++
		}
		// A blank line between sessions: without it a week reads as one long
		// table and the session headings disappear into it.
		row++
	}

	if options.Footer != "" {
		if err := writeCell(file, sheet, 1, row, options.Footer, styles.subtitle); err != nil {
			return err
		}
	}
	return nil
}

func setWidths(file *excelize.File, sheet string, columns []programsheet.Column) error {
	for index, column := range columns {
		name, err := excelize.ColumnNumberToName(index + 1)
		if err != nil {
			return err
		}
		width := column.Weight * widthPerWeight
		if width < minWidth {
			width = minWidth
		}
		if err := file.SetColWidth(sheet, name, name, width); err != nil {
			return err
		}
	}
	return nil
}

func writeCell(file *excelize.File, sheet string, column, row int, value string, style int) error {
	address, err := excelize.CoordinatesToCellName(column, row)
	if err != nil {
		return err
	}
	// SetCellStr rather than SetCellValue: every cell here is text a person
	// wrote or this app formatted ("RPE 8", "102.5 kg"), and letting the
	// library guess would turn some of them into numbers or dates.
	if err := file.SetCellStr(sheet, address, value); err != nil {
		return err
	}
	return file.SetCellStyle(sheet, address, address, style)
}

// sheetName is what the tab is called. Excel refuses a few characters in a
// sheet name and caps it at 31, and two tabs may not share a name - so a week
// with no number falls back to its position rather than colliding with the
// next one.
func sheetName(locale string, number, index int) string {
	name := programsheet.WeekLabel(locale, number)
	if number <= 0 {
		name = fmt.Sprintf("%s %d", name, index+1)
	}
	for _, banned := range []string{"[", "]", ":", "*", "?", "/", "\\"} {
		name = strings.ReplaceAll(name, banned, " ")
	}
	if runes := []rune(name); len(runes) > 31 {
		name = string(runes[:31])
	}
	return strings.TrimSpace(name)
}
