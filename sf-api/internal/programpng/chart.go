package programpng

import (
	"bytes"
	"image"
	"image/color"
	imagedraw "image/draw"
	"image/png"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"strong-fish-api/internal/programsheet"
)

const (
	// chartHeight is the plot itself, between the captions and the baseline.
	// Kept short on purpose: a fitness export can carry several charts - steps,
	// heart rate, and one per tracked lift - and each is placed in the PDF at
	// the page's full width, so a tall plot scaled to that width eats a whole
	// A4 page. Short enough that the usual set fits a page or two.
	chartHeight = 150
	// chartMaxBar keeps a short range from drawing a single bar the width of
	// the page: one day of steps is a bar, not a wall.
	chartMaxBar = 90
	// chartCaptionMax is the most bars that still get their figure printed
	// above them; past it the figures run into each other.
	chartCaptionMax = 16
	legendSwatch    = 14
)

// ChartPNG draws one histogram on its own, for a document that places it as
// an image - the PDF, which has no drawing of its own to match this one.
// width is the picture's width in pixels; its height follows from the chart.
func ChartPNG(chart programsheet.Chart, width int) ([]byte, error) {
	faces, err := loadFaces()
	if err != nil {
		return nil, err
	}

	// Two passes, as with every picture here: the legend wraps, so the height
	// is only known once it has been laid out.
	measure := &canvas{faces: faces, width: width, dry: true}
	measure.chart(chartPad, width-chartPad*2, chart)

	img := image.NewRGBA(image.Rect(0, 0, width, measure.y+chartPad))
	imagedraw.Draw(img, img.Bounds(), &image.Uniform{colPaper}, image.Point{}, imagedraw.Src)
	draw := &canvas{faces: faces, width: width, img: img}
	draw.chart(chartPad, width-chartPad*2, chart)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// chartPad is the margin around a stand-alone chart: the PDF supplies its own
// page margins, so this only keeps the outermost bars off the image's edge.
const chartPad = 8

// chart draws a histogram from the cursor down, between x and x+w.
//
// Each bar is scaled against the tallest, coloured by its caller, and
// captioned with its figure when there are few enough bars for figures to
// fit. Labels under the bars are thinned to every n-th when they would
// overlap, always keeping the last - the most recent day is the one a reader
// looks for.
func (c *canvas) chart(x, w int, chart programsheet.Chart) {
	bars := chart.Bars
	if len(bars) == 0 {
		return
	}

	peak := 0.0
	for _, bar := range bars {
		peak = math.Max(peak, bar.Value)
	}
	if peak <= 0 {
		peak = 1
	}

	slot := float64(w) / float64(len(bars))
	gap := math.Max(1, math.Min(slot*0.2, 10))
	barWidth := math.Min(slot-gap, chartMaxBar)
	if barWidth < 1 {
		barWidth = 1
	}

	captions := len(bars) <= chartCaptionMax
	captionSpace := 0
	if captions {
		captionSpace = cellSize + 6
	}

	top := c.y + captionSpace
	baseline := top + chartHeight

	// Faint guides at the peak and at half of it: enough to read a height off
	// without the ink of a full grid.
	for _, fraction := range []float64{1, 0.5} {
		y := baseline - int(math.Round(chartHeight*fraction))
		c.fill(image.Rect(x, y, x+w, y+1), colZebra)
	}

	for i, bar := range bars {
		centre := float64(x) + slot*(float64(i)+0.5)
		left := int(math.Round(centre - barWidth/2))
		right := int(math.Round(centre + barWidth/2))
		if right <= left {
			right = left + 1
		}

		height := int(math.Round(chartHeight * bar.Value / peak))
		// A day of nothing still draws a sliver, so the row reads as "nothing
		// that day" rather than as a gap in the data.
		colour := bar.Color
		if height < 2 {
			height = 2
			if bar.Value <= 0 {
				colour = colBorder
			}
		}
		c.fill(image.Rect(left, baseline-height, right, baseline), colour)

		if captions && bar.Caption != "" {
			c.textAt(int(centre), baseline-height-cellSize-4, bar.Caption, c.faces.cell, colText, true)
		}
	}

	// The baseline, drawn over the bars' feet.
	c.fill(image.Rect(x, baseline, x+w, baseline+1), colBorder)

	// Every n-th label, n as small as fits.
	widest := 0
	for _, bar := range bars {
		if width := measureText(c.faces.footer, bar.Label); width > widest {
			widest = width
		}
	}
	every := 1
	if widest > 0 {
		every = int(math.Ceil(float64(widest+12) / slot))
		if every < 1 {
			every = 1
		}
	}
	labelY := baseline + 6
	last := len(bars) - 1
	for i, bar := range bars {
		// Counted back from the last, so the most recent day always has its
		// date under it.
		if (last-i)%every != 0 {
			continue
		}
		// Kept inside the plot: the first and last labels are centred on bars
		// that sit against its edges.
		width := measureText(c.faces.footer, bar.Label)
		left := int(float64(x)+slot*(float64(i)+0.5)) - width/2
		left = max(x, min(left, x+w-width))
		c.textAt(left, labelY, bar.Label, c.faces.footer, colMuted, false)
	}
	c.y = labelY + footerSize + 14

	// The legend, wrapping onto as many lines as it needs.
	if len(chart.Legend) == 0 {
		return
	}
	cursor := x
	for _, entry := range chart.Legend {
		width := legendSwatch + 6 + measureText(c.faces.footer, entry.Label) + 22
		if cursor > x && cursor+width > x+w {
			cursor = x
			c.y += footerSize + 10
		}
		c.fill(image.Rect(cursor, c.y+2, cursor+legendSwatch, c.y+2+legendSwatch), entry.Color)
		c.textAt(cursor+legendSwatch+6, c.y, entry.Label, c.faces.footer, colMuted, false)
		cursor += width
	}
	c.y += footerSize + 6
}

// textAt draws one string with its top at y, either starting at x or centred
// on it. Unlike text it does not use the cursor, so a chart can place its
// figures wherever its bars end.
func (c *canvas) textAt(x, y int, s string, face font.Face, col color.Color, centre bool) {
	if c.dry || s == "" {
		return
	}
	if centre {
		x -= measureText(face, s) / 2
	}
	d := &font.Drawer{
		Dst:  c.img,
		Src:  &image.Uniform{col},
		Face: face,
		Dot:  fixed.P(x, y+face.Metrics().Ascent.Ceil()),
	}
	d.DrawString(s)
}

func measureText(face font.Face, s string) int {
	return font.MeasureString(face, s).Ceil()
}
