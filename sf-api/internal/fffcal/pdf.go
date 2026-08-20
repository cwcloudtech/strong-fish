// Package fffcal reads the FFForce season calendar and turns it into events.
//
// The federation publishes its season in more than one shape, and a coach who
// wants those dates in this app would otherwise retype forty of them. Two are
// read here, told apart by the file itself (see Calendar):
//
//   - a PDF year planner - months across the page, days down it, competitions
//     written into the grid and shaded by category (pdf.go, calendar.go);
//   - a spreadsheet - one row per competition, with its date, its discipline
//     and its organiser in columns (xlsx.go).
//
// This file is the PDF reader: enough of the format to get a page's
// *positioned* text and its *filled rectangles*, which is all the layout
// information that calendar carries. Nothing here understands calendars - see
// calendar.go for that.
//
// It is written against the standard library rather than a PDF dependency,
// because what it needs is a narrow slice of the format: FlateDecode streams,
// ToUnicode CMaps, and the handful of content-stream operators that place text
// and fill paths. A general PDF library would bring a great deal more.
package fffcal

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ErrUnsupportedCalendar is returned for a file that is neither of the shapes
// the federation publishes, so the handler can tell a coach they attached the
// wrong thing rather than reporting a parse failure.
var ErrUnsupportedCalendar = errors.New("fffcal: not a season calendar")

// maxPages bounds the work a single upload can cause. The federation's season
// is two pages; a file claiming thousands is not this calendar, whatever else
// it may be.
const maxPages = 64

// Text is one run of characters and where it sits on the page.
//
// The position is the text-matrix translation - the run's origin, near the
// baseline's left end. That is enough to group runs into blocks and to line
// them up with the grid, which is all the calendar needs; a full text-layout
// model (advance widths, kerning, rise) would be a lot of machinery for no
// extra fidelity here.
type Text struct {
	X, Y float64
	S    string
}

// Rect is one filled rectangle: the day cells, the category shading and the
// colour bands are all drawn as these.
//
// PDF paths are general, so this is the special case the calendar happens to
// use - a `m`/`l`/`l`/`l` box followed by a fill. Anything more elaborate is
// ignored rather than approximated.
type Rect struct {
	X0, Y0, X1, Y1 float64
	// R, G, B are in the PDF's 0..1 range, not 0..255.
	R, G, B float64
}

// Width is the rectangle's horizontal extent.
func (r Rect) Width() float64 { return r.X1 - r.X0 }

// Height is the rectangle's vertical extent.
func (r Rect) Height() float64 { return r.Y1 - r.Y0 }

// IsWhite reports whether this is background rather than shading. Colour is
// what the calendar uses to say "something happens here", and white says
// nothing - the grid is mostly white cells.
func (r Rect) IsWhite() bool {
	return r.R > 0.97 && r.G > 0.97 && r.B > 0.97
}

// Hex renders the fill as #rrggbb, the form the rest of the app stores colours
// in (see models.NormalizeHexColor).
func (r Rect) Hex() string {
	channel := func(v float64) int {
		scaled := int(v*255 + 0.5)
		if scaled < 0 {
			return 0
		}
		if scaled > 255 {
			return 255
		}
		return scaled
	}
	return fmt.Sprintf("#%02x%02x%02x", channel(r.R), channel(r.G), channel(r.B))
}

// font is everything needed to read one text run: what its glyphs mean, and
// how wide they are.
//
// The widths matter because this calendar positions some rows by writing a
// long run of spaces and then the text - the run's origin is at the left
// margin while the words appear halfway across the page. Without advancing
// through the glyphs, such a label lands in January when it belongs in May.
type font struct {
	toUnicode map[int]string
	// widths are in thousandths of an em, the PDF convention, keyed by CID.
	widths map[int]float64
	// missing is the width for a CID the /W array does not mention.
	missing float64
}

// advance returns the width of one glyph at the given font size.
func (f font) advance(cid int, size float64) float64 {
	width, ok := f.widths[cid]
	if !ok {
		width = f.missing
	}
	return width * size / 1000
}

// Page is one page's worth of extracted content.
type Page struct {
	Texts []Text
	Rects []Rect
}

var (
	objectPattern    = regexp.MustCompile(`(?s)(\d+)\s+0\s+obj(.*?)endobj`)
	toUnicodePattern = regexp.MustCompile(`/ToUnicode\s+(\d+)\s+0\s+R`)
	fontRefPattern   = regexp.MustCompile(`/(F\d+)\s+(\d+)\s+0\s+R`)
	streamPattern    = regexp.MustCompile(`stream\r?\n`)
	bfCharPattern    = regexp.MustCompile(`(?s)beginbfchar(.*?)endbfchar`)
	bfRangePattern   = regexp.MustCompile(`(?s)beginbfrange(.*?)endbfrange`)
	hexPairPattern   = regexp.MustCompile(`<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>`)
	hexTriplePattern = regexp.MustCompile(`<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>`)
	hexRunPattern    = regexp.MustCompile(`<([0-9A-Fa-f]+)>`)
	pageTypePattern  = regexp.MustCompile(`/Type\s*/Page[^s]`)
	contentsPattern  = regexp.MustCompile(`/Contents\s*(\[[^\]]*\]|\d+\s+0\s+R)`)
	refPattern       = regexp.MustCompile(`(\d+)\s+0\s+R`)
	resourcesPattern = regexp.MustCompile(`(?s)/Resources\s*(<<.*?>>|\d+\s+0\s+R)`)
	widthsRefPattern = regexp.MustCompile(`/W\s+(\d+)\s+0\s+R`)
	widthsPattern    = regexp.MustCompile(`(?s)/W\s*\[(.*?)\]`)
	numberPattern    = regexp.MustCompile(`\[|\]|[-\d.]+`)
)

// operators matches the content-stream operators that place text or fill a
// box. Everything else - clipping, graphics state, images - is skipped, since
// none of it changes where a word or a coloured cell ends up.
var operators = regexp.MustCompile(`` +
	`/(F\d+)\s+([-\d.]+)\s+Tf` + // 1-2: font and size
	`|([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s+([-\d.]+)\s+Tm` + // 3-8: text matrix
	`|([-\d.]+)\s+([-\d.]+)\s+Td` + // 9-10: relative move
	`|\[([^\]]*)\]\s*TJ` + // 11: show text
	`|([\d.]+)\s+([\d.]+)\s+([\d.]+)\s+rg` + // 12-14: non-stroking colour
	`|([-\d.]+)\s+([-\d.]+)\s+m\b` + // 15-16: start a path
	`|([-\d.]+)\s+([-\d.]+)\s+l\b`) // 17-18: line to

// runElements splits a TJ array into its parts: hex strings to show, and the
// numbers between them, which nudge the pen by -n/1000 of the font size.
var runElements = regexp.MustCompile(`<([0-9A-Fa-f]*)>|([-\d.]+)`)

// Parse reads a PDF's pages.
//
// Pages are resolved through their /Contents rather than by scanning for
// streams that look like page content, because one page can be drawn by
// several streams: this calendar paints the shaded grid in one and the day
// numbers over it in another. Taking each stream as a page would split the
// grid from the dates written into it, and neither half means anything alone.
func Parse(data []byte) ([]Page, error) {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, ErrUnsupportedCalendar
	}

	objects := indexObjects(data)
	// One decoded font per font object, shared by every page that uses it: the
	// same font is usually referenced from both pages, and parsing a
	// thousand-entry CMap twice is pure waste.
	loaded := make(map[int]font)

	var pages []Page
	for _, body := range pageObjects(objects) {
		if len(pages) >= maxPages {
			break
		}
		var content strings.Builder
		for _, number := range contentRefs(body) {
			if stream := streamBody(objects[number]); stream != nil {
				content.Write(stream)
				// Streams are concatenated as if they were one, which is what
				// the spec says they are - but only at a lexical break, so the
				// last operator of one and the first of the next cannot merge
				// into a token that was never written.
				content.WriteByte('\n')
			}
		}
		if content.Len() == 0 {
			continue
		}
		pages = append(pages, extract(content.String(), pageFonts(body, objects, loaded)))
	}

	if len(pages) == 0 {
		return nil, errors.New("fffcal: no page content found")
	}
	return pages, nil
}

// pageObjects returns the body of every page object, in object-number order so
// a document reads front to back. A calendar's pages are numbered in the order
// they were written, which is the order they appear in.
func pageObjects(objects map[int][]byte) [][]byte {
	numbers := make([]int, 0, len(objects))
	for number, body := range objects {
		// "/Type /Page" also prefixes "/Type /Pages", the tree node, which has
		// /Kids instead of /Contents and would contribute nothing.
		if pageTypePattern.Match(body) && contentsPattern.Match(body) {
			numbers = append(numbers, number)
		}
	}
	sort.Ints(numbers)

	bodies := make([][]byte, 0, len(numbers))
	for _, number := range numbers {
		bodies = append(bodies, objects[number])
	}
	return bodies
}

// contentRefs reads the object numbers in a page's /Contents, which is either
// one reference or an array of them.
func contentRefs(page []byte) []int {
	match := contentsPattern.FindSubmatch(page)
	if match == nil {
		return nil
	}
	var numbers []int
	for _, ref := range refPattern.FindAllSubmatch(match[1], -1) {
		if number, err := strconv.Atoi(string(ref[1])); err == nil {
			numbers = append(numbers, number)
		}
	}
	return numbers
}

// indexObjects maps object numbers to their bodies.
//
// Deliberately a scan for `N 0 obj ... endobj` rather than a walk of the xref
// table: an incrementally-updated file has several xref sections and objects
// that supersede each other, and for reading fonts the last definition wins -
// which is what a forward scan into a map gives, for far less machinery.
func indexObjects(data []byte) map[int][]byte {
	objects := make(map[int][]byte)
	for _, match := range objectPattern.FindAllSubmatch(data, -1) {
		number, err := strconv.Atoi(string(match[1]))
		if err != nil {
			continue
		}
		objects[number] = match[2]
	}
	return objects
}

// pageFonts builds the reading table for each font *this page* names.
//
// Per page, not per document, because the names are page-local: in the
// federation's own calendar the two pages both define /F1 and /F4 and swap
// which font object each points at. Resolving them globally decodes half the
// document through the wrong table - which does not fail, it silently returns
// the wrong letters, and "Qualif" comes back as "ualif".
//
// The calendar's fonts are subset and Identity-encoded: a show-text operator
// carries glyph indices, not characters, and they mean nothing without the
// font's own ToUnicode CMap.
func pageFonts(page []byte, objects map[int][]byte, cache map[int]font) map[string]font {
	resources := resourcesOf(page, objects)
	fonts := make(map[string]font)

	for _, match := range fontRefPattern.FindAllSubmatch(resources, -1) {
		name := string(match[1])
		number, err := strconv.Atoi(string(match[2]))
		if err != nil {
			continue
		}
		if loaded, ok := cache[number]; ok {
			fonts[name] = loaded
			continue
		}
		loaded := loadFont(objects, number)
		cache[number] = loaded
		fonts[name] = loaded
	}
	return fonts
}

// loadFont reads one font's ToUnicode CMap and its glyph widths.
func loadFont(objects map[int][]byte, number int) font {
	// 500 is the usual "unknown glyph" width, and only applies to glyphs the
	// /W array omits - which for a subset font means glyphs it never draws.
	loaded := font{missing: 500}

	body, ok := objects[number]
	if !ok {
		return loaded
	}
	loaded.toUnicode = toUnicodeTable(objects, body)
	loaded.widths = widthTable(objects, body)
	return loaded
}

// widthTable reads a CIDFont's /W array: runs of `first last width` and
// `first [w1 w2 ...]`, in thousandths of an em.
func widthTable(objects map[int][]byte, body []byte) map[int]float64 {
	array := body
	if ref := widthsRefPattern.FindSubmatch(body); ref != nil {
		if number, err := strconv.Atoi(string(ref[1])); err == nil {
			if resolved, ok := objects[number]; ok {
				array = resolved
			}
		}
	}
	match := widthsPattern.FindSubmatch(array)
	if match == nil {
		return nil
	}

	widths := make(map[int]float64)
	tokens := numberPattern.FindAllString(string(match[1]), -1)
	for i := 0; i < len(tokens); {
		first, err := strconv.Atoi(tokens[i])
		if err != nil {
			i++
			continue
		}
		i++
		if i >= len(tokens) {
			break
		}

		if tokens[i] == "[" {
			// `first [ w1 w2 ... ]`: consecutive CIDs from first.
			i++
			for cid := first; i < len(tokens) && tokens[i] != "]"; i, cid = i+1, cid+1 {
				widths[cid] = number(tokens[i])
			}
			if i < len(tokens) {
				i++ // the closing bracket
			}
			continue
		}

		// `first last width`: one width for the whole range.
		last, err := strconv.Atoi(tokens[i])
		i++
		if err != nil || i >= len(tokens) || last < first || last-first > 0xFFFF {
			continue
		}
		width := number(tokens[i])
		i++
		for cid := first; cid <= last; cid++ {
			widths[cid] = width
		}
	}
	return widths
}

// resourcesOf returns a page's resource dictionary, following the indirect
// reference when it is one and taking it inline otherwise.
func resourcesOf(page []byte, objects map[int][]byte) []byte {
	match := resourcesPattern.FindSubmatch(page)
	if match == nil {
		return page
	}
	if ref := refPattern.FindSubmatch(match[1]); ref != nil {
		if number, err := strconv.Atoi(string(ref[1])); err == nil {
			if body, ok := objects[number]; ok {
				return body
			}
		}
	}
	return match[1]
}

// toUnicodeTable reads one font's ToUnicode CMap, or nil when it has none - a
// font without one cannot be decoded, and guessing would put plausible-looking
// wrong text into somebody's calendar.
func toUnicodeTable(objects map[int][]byte, body []byte) map[int]string {
	ref := toUnicodePattern.FindSubmatch(body)
	if ref == nil {
		return nil
	}
	number, err := strconv.Atoi(string(ref[1]))
	if err != nil {
		return nil
	}
	raw := streamBody(objects[number])
	if raw == nil {
		return nil
	}
	return parseCMap(string(raw))
}

// streamBody returns a stream object's payload, inflated when it is deflated.
func streamBody(object []byte) []byte {
	start := streamPattern.FindIndex(object)
	if start == nil {
		return nil
	}
	end := bytes.LastIndex(object, []byte("endstream"))
	if end < start[1] {
		return nil
	}
	raw := object[start[1]:end]
	if !bytes.Contains(object[:start[0]], []byte("/FlateDecode")) {
		return raw
	}
	inflated, err := inflate(raw)
	if err != nil {
		return nil
	}
	return inflated
}

// inflate decompresses a zlib stream, tolerating a truncated tail.
//
// io.ErrUnexpectedEOF is accepted with whatever was decoded: a content stream's
// declared /Length can disagree with where `endstream` actually is, and the
// bytes recovered before the cut are still the top of the page.
func inflate(raw []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var out bytes.Buffer
	if _, err := io.Copy(&out, reader); err != nil {
		if !errors.Is(err, io.ErrUnexpectedEOF) || out.Len() == 0 {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

// parseCMap reads a ToUnicode CMap's bfchar and bfrange sections.
func parseCMap(text string) map[int]string {
	table := make(map[int]string)

	for _, block := range bfCharPattern.FindAllStringSubmatch(text, -1) {
		for _, pair := range hexPairPattern.FindAllStringSubmatch(block[1], -1) {
			code, err := strconv.ParseInt(pair[1], 16, 32)
			if err != nil {
				continue
			}
			table[int(code)] = utf16BEString(pair[2])
		}
	}

	for _, block := range bfRangePattern.FindAllStringSubmatch(text, -1) {
		for _, triple := range hexTriplePattern.FindAllStringSubmatch(block[1], -1) {
			low, err1 := strconv.ParseInt(triple[1], 16, 32)
			high, err2 := strconv.ParseInt(triple[2], 16, 32)
			base, err3 := strconv.ParseInt(triple[3], 16, 32)
			if err1 != nil || err2 != nil || err3 != nil || high < low {
				continue
			}
			// A malformed range could otherwise ask for millions of entries.
			if high-low > 0xFFFF {
				continue
			}
			for code := low; code <= high; code++ {
				table[int(code)] = string(rune(base + code - low))
			}
		}
	}
	return table
}

// utf16BEString decodes a CMap destination, which is UTF-16BE.
func utf16BEString(hex string) string {
	var out strings.Builder
	for i := 0; i+4 <= len(hex); i += 4 {
		value, err := strconv.ParseInt(hex[i:i+4], 16, 32)
		if err != nil {
			continue
		}
		out.WriteRune(rune(value))
	}
	return out.String()
}

// extract walks one content stream, collecting placed text and filled boxes.
func extract(content string, fonts map[string]font) Page {
	var page Page
	var current font
	var size float64
	var x, y float64
	var haveColor bool
	var red, green, blue float64
	var points [][2]float64

	for _, match := range operators.FindAllStringSubmatch(content, -1) {
		switch {
		case match[1] != "":
			current = fonts[match[1]]
			size = number(match[2])

		case match[3] != "":
			// Only the translation is kept: this planner sets text upright, and
			// a rotated or scaled run would still be at this origin.
			x = number(match[7])
			y = number(match[8])

		case match[9] != "":
			x += number(match[9])
			y += number(match[10])

		case match[11] != "":
			run, offset := decodeRun(match[11], current, size)
			if strings.TrimSpace(run) != "" {
				// The reported position is where the first visible glyph sits,
				// not where the run began: a row positioned by a long run of
				// spaces would otherwise be filed under the wrong month.
				page.Texts = append(page.Texts, Text{X: x + offset, Y: y, S: strings.TrimSpace(run)})
			}

		case match[12] != "":
			red, green, blue = number(match[12]), number(match[13]), number(match[14])
			haveColor = true
			// A colour change starts a new shape; anything half-drawn was not
			// going to be filled with this colour.
			points = nil

		case match[15] != "":
			points = [][2]float64{{number(match[15]), number(match[16])}}

		case match[17] != "":
			points = append(points, [2]float64{number(match[17]), number(match[18])})
			// Three corners already fix a box; the calendar closes each cell
			// with `h`, which this scan does not need to see.
			if len(points) >= 3 && haveColor {
				page.Rects = append(page.Rects, boundingRect(points, red, green, blue))
				points = nil
			}
		}
	}
	return page
}

// decodeRun turns a TJ array into text, and reports how far into the run the
// first visible character sits.
//
// The pen is walked rather than the strings simply concatenated, because this
// calendar indents by writing spaces: the offset is what tells "  14/05" in
// the January column from the same run meaning May.
func decodeRun(array string, f font, size float64) (string, float64) {
	var out strings.Builder
	var pen, offset float64
	visible := false

	for _, element := range runElements.FindAllStringSubmatch(array, -1) {
		if element[2] != "" {
			// A kerning number moves the pen without drawing anything.
			pen -= number(element[2]) * size / 1000
			continue
		}
		hex := element[1]
		// Identity encoding: two bytes per glyph.
		for i := 0; i+4 <= len(hex); i += 4 {
			code, err := strconv.ParseInt(hex[i:i+4], 16, 32)
			if err != nil {
				continue
			}
			cid := int(code)
			glyph := f.toUnicode[cid]
			if !visible && strings.TrimSpace(glyph) != "" {
				visible = true
				offset = pen
			}
			out.WriteString(glyph)
			pen += f.advance(cid, size)
		}
	}
	return out.String(), offset
}

func boundingRect(points [][2]float64, red, green, blue float64) Rect {
	rect := Rect{
		X0: points[0][0], Y0: points[0][1],
		X1: points[0][0], Y1: points[0][1],
		R: red, G: green, B: blue,
	}
	for _, point := range points[1:] {
		rect.X0 = min(rect.X0, point[0])
		rect.Y0 = min(rect.Y0, point[1])
		rect.X1 = max(rect.X1, point[0])
		rect.Y1 = max(rect.Y1, point[1])
	}
	return rect
}

func number(text string) float64 {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return value
}
