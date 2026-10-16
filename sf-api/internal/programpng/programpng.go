// Package programpng renders a training program as a single image.
//
// It exists for the way coaches and lifters actually pass a session around:
// pasted into WhatsApp. A PDF there is an attachment somebody has to open, and
// on a phone it opens in a viewer, away from the conversation. An image is the
// message - it shows in the thread, it is read without a tap, and it forwards.
//
// So the shape is deliberately not the PDF's. There are no pages: the whole
// block is one tall picture, because a picture split across files is no longer
// one thing to share. What it draws is the same table all the same, from
// programsheet, so the image and the printout cannot disagree.
package programpng

import (
	"bytes"
	"image"
	"image/color"
	imagedraw "image/draw"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programsheet"
)

// Options is everything the renderer needs that is not the program itself.
// Shared with the PDF and XLSX renderers (see programsheet).
type Options = programsheet.Options

const (
	// Wide enough that a phone screenshotting the thread still has readable
	// text, narrow enough that WhatsApp does not scale it away. A feedback
	// sheet has nine columns instead of five and needs the room.
	width     = 1200
	wideWidth = 1760

	padding    = 44
	titleSize  = 36
	subSize    = 19
	weekSize   = 27
	daySize    = 21
	cellSize   = 17
	footerSize = 14

	cellPadX   = 10
	cellPadY   = 7
	lineHeight = 23
)

var (
	colText     = color.RGBA{17, 24, 39, 255}
	colMuted    = color.RGBA{110, 110, 110, 255}
	colHeaderBg = color.RGBA{14, 94, 155, 255}
	colHeaderFg = color.RGBA{255, 255, 255, 255}
	colZebra    = color.RGBA{244, 246, 249, 255}
	colDone     = color.RGBA{223, 246, 228, 255}
	colBorder   = color.RGBA{206, 212, 222, 255}
	colPaper    = color.RGBA{255, 255, 255, 255}
)

// Render lays the program out as one PNG and returns the bytes.
//
// days must carry their sets (see models.ProgramDay.Sets); a day with none
// still gets its heading, because an empty session in the middle of a block is
// information - it is a rest day the coach wrote down.
func Render(program models.Program, days []models.ProgramDay, options Options) ([]byte, error) {
	faces, err := loadFaces()
	if err != nil {
		return nil, err
	}

	canvasWidth := width
	if options.Feedback {
		canvasWidth = wideWidth
	}

	// Laid out twice: once against a canvas that draws nothing, only to learn
	// how tall the picture has to be, then again for real. The alternative is
	// predicting the height from row counts, which goes wrong the moment a
	// long exercise name wraps onto a second line.
	measure := &canvas{faces: faces, width: canvasWidth, dry: true}
	layout(measure, program, days, options)

	img := image.NewRGBA(image.Rect(0, 0, canvasWidth, measure.y+padding))
	imagedraw.Draw(img, img.Bounds(), &image.Uniform{colPaper}, image.Point{}, imagedraw.Src)
	layout(&canvas{faces: faces, width: canvasWidth, img: img}, program, days, options)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderTable draws one titled table as a picture.
//
// The program renderer below is this with a block's worth of structure on top;
// anything that is simply rows - a member's health data, say - needs only
// this. Kept in the same package because it is the same canvas: a second
// implementation of "draw a table into a PNG" would drift in its padding and
// its zebra within a release.
func RenderTable(title, subtitle string, columns []programsheet.Column,
	rows []programsheet.Line, footer string) ([]byte, error) {
	faces, err := loadFaces()
	if err != nil {
		return nil, err
	}

	draw := func(c *canvas) {
		c.y = padding
		c.text(padding, title, c.faces.title, colText)
		c.y += titleSize + 10
		if subtitle != "" {
			c.text(padding, subtitle, c.faces.sub, colMuted)
			c.y += subSize + 8
		}
		c.y += 14
		c.table(columns, rows)
		if footer != "" {
			c.y += 16
			c.text(padding, footer, c.faces.footer, colMuted)
			c.y += footerSize
		}
	}

	// Measured first, drawn second - the same two passes Render makes, and
	// for the same reason: the height is not known until the cells have been
	// wrapped.
	measure := &canvas{faces: faces, width: width, dry: true}
	draw(measure)

	img := image.NewRGBA(image.Rect(0, 0, width, measure.y+padding))
	imagedraw.Draw(img, img.Bounds(), &image.Uniform{colPaper}, image.Point{}, imagedraw.Src)
	draw(&canvas{faces: faces, width: width, img: img})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// layout writes the whole picture top to bottom. Called once to measure and
// once to draw, so there is one description of the page rather than two that
// have to agree.
func layout(c *canvas, program models.Program, days []models.ProgramDay, options Options) {
	c.y = padding

	c.text(padding, program.Name, c.faces.title, colText)
	c.y += titleSize + 10
	if subtitle := programsheet.Subtitle(program, options.MemberName); subtitle != "" {
		c.text(padding, subtitle, c.faces.sub, colMuted)
		c.y += subSize + 8
	}
	c.y += 14

	weeks := programsheet.Weeks(days)
	for _, week := range weeks {
		c.text(padding, programsheet.WeekLabel(options.Locale, week.Number), c.faces.week, colText)
		c.y += weekSize + 14

		for _, day := range week.Days {
			c.text(padding, programsheet.DayTitle(day, options.Locale), c.faces.day, colText)
			c.y += daySize + 10

			if len(day.Sets) == 0 {
				c.text(padding, programsheet.RestLabel(options.Locale), c.faces.cell, colMuted)
				c.y += lineHeight + 16
				continue
			}

			sets := programsheet.SortedSets(day)
			c.table(programsheet.Columns(options.Locale, options.Feedback),
				programsheet.Lines(sets, options.Feedback, options.Locale))
			c.y += 22
		}
		c.y += 10
	}

	// An empty program still produces a picture rather than a blank one: what
	// somebody gets back should say the block is empty, not look like the
	// export broke.
	if len(weeks) == 0 {
		c.text(padding, programsheet.RestLabel(options.Locale), c.faces.cell, colMuted)
		c.y += lineHeight
	}

	if options.Footer != "" {
		c.y += 8
		c.text(padding, options.Footer, c.faces.footer, colMuted)
		c.y += footerSize
	}
}

// --- the canvas -------------------------------------------------------------

type faceSet struct {
	title, sub, week, day, cell, cellBold, footer font.Face
}

// canvas is a cursor down the page. With dry set it draws nothing and only
// advances y, which is how the height is known before the image exists.
type canvas struct {
	faces *faceSet
	img   *image.RGBA
	width int
	y     int
	dry   bool
}

func (c *canvas) text(x int, s string, face font.Face, col color.Color) {
	if c.dry || s == "" {
		return
	}
	d := &font.Drawer{
		Dst:  c.img,
		Src:  &image.Uniform{col},
		Face: face,
		// y is the top of the line; a face draws from its baseline.
		Dot: fixed.P(x, c.y+face.Metrics().Ascent.Ceil()),
	}
	d.DrawString(s)
}

func (c *canvas) fill(r image.Rectangle, col color.Color) {
	if c.dry {
		return
	}
	imagedraw.Draw(c.img, r, &image.Uniform{col}, image.Point{}, imagedraw.Src)
}

// table draws the sheet's rows, headings included.
func (c *canvas) table(columns []programsheet.Column, rows []programsheet.Line) {
	usable := c.width - padding*2
	total := 0.0
	for _, col := range columns {
		total += col.Weight
	}
	widths := make([]int, len(columns))
	x := padding
	for i, col := range columns {
		widths[i] = int(float64(usable) * col.Weight / total)
		x += widths[i]
	}
	// Rounding leaves a few pixels over; the last column takes them so the
	// table's right edge lands exactly on the margin.
	widths[len(widths)-1] += padding + usable - x

	headerHeight := lineHeight + cellPadY*2
	c.fill(image.Rect(padding, c.y, padding+usable, c.y+headerHeight), colHeaderBg)
	x = padding
	for i, col := range columns {
		c.cellText(x, widths[i], []string{col.Header}, c.faces.cellBold, colHeaderFg, true)
		x += widths[i]
	}
	c.y += headerHeight

	zebra := false
	for _, row := range rows {
		wrapped := make([][]string, len(row.Cells))
		tallest := 1
		for i, cell := range row.Cells {
			lines := wrap(cell, c.faces.cell, widths[i]-cellPadX*2)
			wrapped[i] = lines
			if len(lines) > tallest {
				tallest = len(lines)
			}
		}
		height := tallest*lineHeight + cellPadY*2

		// A ticked-off set is a green line rather than a column of crosses:
		// what a coach reads a feedback sheet for is which sets were missed,
		// and that is a shape on the page. The alternating grey underneath is
		// what keeps a long unfinished week readable.
		switch {
		case row.Done:
			c.fill(image.Rect(padding, c.y, padding+usable, c.y+height), colDone)
		case zebra:
			c.fill(image.Rect(padding, c.y, padding+usable, c.y+height), colZebra)
		}

		x = padding
		for i := range columns {
			c.cellText(x, widths[i], wrapped[i], c.faces.cell, colText, false)
			x += widths[i]
		}

		// One rule under each row, and one down each column boundary: enough
		// to read a row across without the ink of a full grid.
		c.fill(image.Rect(padding, c.y+height-1, padding+usable, c.y+height), colBorder)
		x = padding
		for i := range columns[:len(columns)-1] {
			x += widths[i]
			c.fill(image.Rect(x-1, c.y, x, c.y+height), colBorder)
		}

		c.y += height
		zebra = !zebra
	}
}

// cellText draws one cell's lines, centred when it is a heading and left
// aligned otherwise, without advancing the cursor - every cell in a row starts
// at the same y.
func (c *canvas) cellText(x, w int, lines []string, face font.Face, col color.Color, centre bool) {
	top := c.y + cellPadY
	for i, line := range lines {
		at := x + cellPadX
		if centre {
			at = x + (w-textWidth(line, face))/2
		}
		if !c.dry && line != "" {
			d := &font.Drawer{
				Dst:  c.img,
				Src:  &image.Uniform{col},
				Face: face,
				Dot:  fixed.P(at, top+i*lineHeight+face.Metrics().Ascent.Ceil()),
			}
			d.DrawString(line)
		}
	}
}

// --- text -------------------------------------------------------------------

func textWidth(s string, face font.Face) int {
	return font.MeasureString(face, s).Ceil()
}

// wrap breaks a cell's text to the width it has. A word longer than the column
// is cut rather than allowed to run into the next cell, which is the one
// failure that loses information instead of merely looking wrong.
func wrap(s string, face font.Face, maxWidth int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{""}
	}
	if maxWidth <= 0 || textWidth(s, face) <= maxWidth {
		return []string{s}
	}

	lines := []string{}
	current := ""
	for _, word := range strings.Fields(s) {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if textWidth(candidate, face) <= maxWidth {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		for textWidth(word, face) > maxWidth && len(word) > 1 {
			cut := len(word)
			for cut > 1 && textWidth(word[:cut], face) > maxWidth {
				cut--
			}
			lines = append(lines, word[:cut])
			word = word[cut:]
		}
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// loadFaces rasterizes the Go fonts, which ship with golang.org/x/image under
// a BSD licence - so the image carries real type without the repository having
// to vendor a font of its own.
func loadFaces() (*faceSet, error) {
	regular, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	bold, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}

	face := func(f *opentype.Font, size float64) (font.Face, error) {
		// DPI 72 makes a point a pixel, so the sizes above read as what they
		// measure on the picture.
		return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	}

	set := &faceSet{}
	for _, spec := range []struct {
		dst  *font.Face
		f    *opentype.Font
		size float64
	}{
		{&set.title, bold, titleSize},
		{&set.sub, regular, subSize},
		{&set.week, bold, weekSize},
		{&set.day, bold, daySize},
		{&set.cell, regular, cellSize},
		{&set.cellBold, bold, cellSize},
		{&set.footer, regular, footerSize},
	} {
		made, err := face(spec.f, spec.size)
		if err != nil {
			return nil, err
		}
		*spec.dst = made
	}
	return set, nil
}
