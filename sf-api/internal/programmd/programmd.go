// Package programmd renders the sheets as markdown.
//
// The fourth renderer beside the PDF, the picture and the spreadsheet, laid
// out from the same tables (see programsheet) so the four cannot disagree
// about what a sheet contains. This one is not for a member to read: it is
// what a language model is shown when it is asked about a sheet, and a model
// reads a markdown table about as well as a person reads the PDF.
//
// A chart becomes a table. Markdown cannot draw bars, and the numbers behind
// them are what the question is about anyway - which week was idle, which day
// peaked - so the series is written out with its labels instead.
package programmd

import (
	"fmt"
	"strings"

	"strong-fish-api/internal/programsheet"
)

// RenderTables writes titled sections as markdown, one after the other.
func RenderTables(title, subtitle string, sections []programsheet.Section, footer string) string {
	var out strings.Builder

	if title != "" {
		fmt.Fprintf(&out, "# %s\n\n", title)
	}
	if subtitle != "" {
		fmt.Fprintf(&out, "%s\n\n", subtitle)
	}

	for _, section := range sections {
		if section.Heading != "" {
			fmt.Fprintf(&out, "## %s\n\n", section.Heading)
		}
		if section.Chart != nil {
			writeChart(&out, *section.Chart)
			continue
		}
		writeTable(&out, section.Columns, section.Rows)
	}

	if footer != "" {
		fmt.Fprintf(&out, "%s\n", footer)
	}
	return out.String()
}

func writeTable(out *strings.Builder, columns []programsheet.Column, rows []programsheet.Line) {
	if len(columns) == 0 {
		return
	}

	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = escape(column.Header)
	}
	fmt.Fprintf(out, "| %s |\n", strings.Join(headers, " | "))
	fmt.Fprintf(out, "|%s\n", strings.Repeat(" --- |", len(columns)))

	for _, row := range rows {
		cells := make([]string, len(columns))
		for i := range columns {
			if i < len(row.Cells) {
				cells[i] = escape(row.Cells[i])
			}
		}
		fmt.Fprintf(out, "| %s |\n", strings.Join(cells, " | "))
	}
	out.WriteString("\n")
}

// writeChart writes a histogram as its series: the label of each bar, the
// figure it was drawn from, and what its colour meant.
//
// A day with no reading is written as a blank rather than a zero. The
// distinction matters to whoever reads this: a day the watch was not worn is
// not a day of no activity, and a model shown a column of zeroes will say the
// member stopped training.
func writeChart(out *strings.Builder, chart programsheet.Chart) {
	// English headers, unlike everything else here: this document is read by
	// a model under an English instruction, not by the member.
	named := len(chart.Legend) > 0
	if named {
		out.WriteString("| Day | Value | Band |\n| --- | --- | --- |\n")
	} else {
		out.WriteString("| Day | Value |\n| --- | --- |\n")
	}

	for _, bar := range chart.Bars {
		value := bar.Caption
		if value == "" && bar.Value > 0 {
			value = fmt.Sprintf("%.0f", bar.Value)
		}
		if named {
			fmt.Fprintf(out, "| %s | %s | %s |\n", escape(bar.Label), escape(value),
				escape(legendLabel(chart, bar)))
			continue
		}
		fmt.Fprintf(out, "| %s | %s |\n", escape(bar.Label), escape(value))
	}
	out.WriteString("\n")
}

// legendLabel is what a bar's colour meant, read off the chart's own legend -
// the band a day's steps fell into, the hardest heart-rate zone it reached.
func legendLabel(chart programsheet.Chart, bar programsheet.Bar) string {
	for _, entry := range chart.Legend {
		if entry.Color == bar.Color {
			return entry.Label
		}
	}
	return ""
}

// escape keeps a cell inside its cell: a pipe in a value would otherwise open
// a column that is not there.
func escape(value string) string {
	v := strings.ReplaceAll(strings.TrimSpace(value), "|", "\\|")
	// A multi-line cell (the VBT column's per-rep speeds) keeps its line breaks
	// as <br>, which a markdown table renders - and the AI reads - as lines.
	return strings.ReplaceAll(v, "\n", "<br>")
}
