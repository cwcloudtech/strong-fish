package programmd

import (
	"image/color"
	"strings"
	"testing"

	"strong-fish-api/internal/programsheet"
)

// TestRenderTablesWritesATableAModelCanRead covers the document a language
// model is shown in place of the PDF: the cells have to land in the right
// columns, and a pipe inside a value would otherwise open one that is not
// there.
func TestRenderTablesWritesATableAModelCanRead(t *testing.T) {
	out := RenderTables("Health data - Ada", "12 000 steps a day", []programsheet.Section{{
		Heading: "Body composition",
		Columns: []programsheet.Column{{Header: "Measure"}, {Header: "Value"}},
		Rows: []programsheet.Line{
			{Cells: []string{"Weight", "92.4 kg"}},
			{Cells: []string{"Body fat | estimated", "18 %"}},
		},
	}}, "strong-fish.com")

	for _, want := range []string{
		"# Health data - Ada",
		"| Measure | Value |",
		"| Weight | 92.4 kg |",
		`| Body fat \| estimated | 18 % |`,
		"strong-fish.com",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the sheet does not contain %q:\n%s", want, out)
		}
	}
}

// TestRenderTablesNamesTheBandsOfAChart covers the part that cannot survive
// the trip as a picture: a bar's colour means something, and a model reading
// the document has only the words.
//
// A day nothing was recorded must stay blank. Written as a zero it reads as a
// day of no activity, and the whole reading is then about a collapse that
// never happened.
func TestRenderTablesNamesTheBandsOfAChart(t *testing.T) {
	good := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	out := RenderTables("Steps", "", []programsheet.Section{{
		Heading: "Steps per day",
		Chart: &programsheet.Chart{
			Legend: []programsheet.LegendEntry{{Label: "10 000 - 15 000", Color: good}},
			Bars: []programsheet.Bar{
				{Label: "3 Nov", Value: 12000, Caption: "12 000", Color: good},
				{Label: "4 Nov"},
			},
		},
	}}, "")

	if !strings.Contains(out, "| 3 Nov | 12 000 | 10 000 - 15 000 |") {
		t.Errorf("the bar lost its figure or its band:\n%s", out)
	}
	if !strings.Contains(out, "| 4 Nov |  |  |") {
		t.Errorf("a day with no reading should stay blank:\n%s", out)
	}
}
