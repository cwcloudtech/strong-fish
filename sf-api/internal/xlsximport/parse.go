// Package xlsximport turns a coach's program spreadsheet into the data model.
//
// Two layouts are supported, told apart by their header rows so that either
// can be dropped on the same upload button:
//
//   - week per sheet (ai-gen/assets/program_1.xlsx), described below;
//   - block per sheet (ai-gen/assets/program_2.xlsx), where one sheet holds
//     several weeks at once - see parse_block.go.
//
// Neither reads the workbook's own computed weights. Every load in this app is
// derived from the member reading it (see package loadcalc), so a spreadsheet
// whose formulas are stale, or wrong, or written against somebody else's maxes
// imports exactly as well as a correct one.
//
// The week-per-sheet file is laid out as one sheet per week plus a "refs"
// sheet, and each week sheet holds several day blocks:
//
//	WEEK 1 DAY 2                                       <- title row
//	Exercice | Reps | RPE | Percentage | Load | e1RM | Part   <- header row
//	Squat    |  3   |  5  |     79     | ...             <- set rows
//	...
//	(blank row separates one day from the next)
//
// The refs sheet carries the 1RMs the percentages were authored against, and a
// "Renfo" (accessory) block of movements loaded in absolute kilos.
//
// Parsing is deliberately tolerant, because the reference file is not clean:
// several day-title cells were clobbered by a stray fill-down formula and
// evaluate to an exercise name (or a number) instead of "WEEK n DAY m". A
// block is therefore recognized by its header row, not its title, and the
// week/day numbering falls back to the sheet name plus the block's position
// when the title doesn't parse.
package xlsximport

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"strong-fish-api/internal/loadcalc"
	"strong-fish-api/internal/utils"
)

// ErrNoProgramFound is returned when no sheet in the workbook contains a
// recognizable day block.
var ErrNoProgramFound = errors.New("no program found in this spreadsheet")

// ParsedProgram is a whole workbook, ready to be persisted.
type ParsedProgram struct {
	Weeks int
	Days  []ParsedDay
	// RefOneRMs are the 1RMs the spreadsheet's percentages were authored
	// against, keyed by exercise slug. They're never stored as anyone's max -
	// they're only used to work out which lift each set is programmed off (see
	// resolveReference) and are offered to the importing coach as a starting
	// point for members who have no 1RM recorded yet.
	RefOneRMs map[string]float64
	// Exercises is every distinct movement the workbook mentions, in first-seen
	// order, with whatever could be inferred about it.
	Exercises []ParsedExercise
	// Warnings collects rows that were skipped or guessed at, surfaced to the
	// coach after the import rather than failing the whole upload.
	Warnings []string
}

// ParsedExercise is one movement encountered in the workbook.
type ParsedExercise struct {
	Name string
	Slug string
	// OneRMRef is which competition lift this movement's percentages resolve
	// against, inferred from the numbers (see resolveReference). Empty for an
	// accessory.
	OneRMRef   string
	Bodyweight bool
}

// ParsedDay is one training session.
type ParsedDay struct {
	Week     int
	Day      int
	Title    string
	Position int
	Sets     []ParsedSet
}

// ParsedSet is one prescribed set.
type ParsedSet struct {
	ExerciseName string
	ExerciseSlug string
	Position     int
	Reps         int
	RPE          *float64
	Percentage   *float64
	AbsoluteLoad *float64
	LoadMode     string
	Part         int
	// Notes is whatever the coach wrote alongside the prescription: a cue, a
	// weekly progression instruction, or a rep target that wasn't a number.
	Notes string
	// sourceLoad is the weight the spreadsheet had cached for this row. It is
	// never persisted - loads are derived per member from now on - but it's
	// what identifies which lift a percentage was authored against (see
	// referenceFromLoad), so it's kept for the duration of the parse.
	sourceLoad float64
}

// Column indices in a day block's set rows, as laid out by the reference
// spreadsheet's header row.
const (
	colExercise = iota
	colReps
	colRPE
	colPercentage
	colLoad
	colE1RM
	colPart
)

var (
	// exerciseHeaders are the column-A values that mark a block's header row.
	// Both spellings appear in the wild (the reference file is French-ish).
	exerciseHeaders = map[string]bool{"exercice": true, "exercise": true}
	// weekDayPattern reads "WEEK 2 DAY 3" out of a title row.
	weekDayPattern = regexp.MustCompile(`(?i)week\s*(\d+)\s*[-/ ]?\s*day\s*(\d+)`)
	// sheetWeekPattern reads the week number out of a sheet name ("week 3").
	sheetWeekPattern = regexp.MustCompile(`(?i)(\d+)`)
	// refsSheetNames are the sheet names holding the 1RM/accessory reference
	// tables rather than a week of training.
	refsSheetNames = map[string]bool{"refs": true, "ref": true, "references": true, "1rm": true}
)

// Parse reads a program workbook.
func Parse(data []byte) (*ParsedProgram, error) {
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("this file is not a readable spreadsheet: %w", err)
	}
	defer file.Close()

	program := &ParsedProgram{RefOneRMs: map[string]float64{}}

	// The refs sheet must be read first: the reference 1RMs are what let
	// resolveReference work out which lift each week's percentages are
	// programmed off.
	accessories := map[string]bool{}
	for _, name := range file.GetSheetList() {
		if refsSheetNames[strings.ToLower(strings.TrimSpace(name))] {
			rows, err := file.GetRows(name)
			if err != nil {
				return nil, err
			}
			parseRefs(rows, program.RefOneRMs, accessories)
		}
	}

	// Weeks accumulate across block sheets: two four-week blocks make an
	// eight-week program, so the second block's "W1" is program week 5. A
	// week-per-sheet workbook numbers its own weeks and leaves this at zero.
	weekOffset := 0
	weeks := 0
	for _, sheet := range file.GetSheetList() {
		if refsSheetNames[strings.ToLower(strings.TrimSpace(sheet))] {
			continue
		}
		rows, err := file.GetRows(sheet)
		if err != nil {
			return nil, err
		}

		// The two layouts are told apart by their header row, not by the sheet
		// name or the file name - a coach renames sheets, and both formats are
		// uploaded through the same button.
		var days []ParsedDay
		if isBlockSheet(rows) {
			var added int
			days, added = parseBlockSheet(sheet, rows, weekOffset, len(program.Days), program)
			weekOffset += added
		} else {
			days = parseSheet(sheet, rows, len(program.Days), program)
		}

		for _, day := range days {
			if day.Week > weeks {
				weeks = day.Week
			}
		}
		program.Days = append(program.Days, days...)
	}

	if len(program.Days) == 0 {
		return nil, ErrNoProgramFound
	}
	program.Weeks = weeks
	program.Exercises = collectExercises(program.Days, program.RefOneRMs, accessories)
	return program, nil
}

// collectExercises reduces every set down to the distinct movements the
// workbook mentions, in first-seen order, working out what can be inferred
// about each: whether it's loaded by bodyweight, and which competition lift its
// percentages are programmed off.
func collectExercises(days []ParsedDay, refOneRMs map[string]float64, accessories map[string]bool) []ParsedExercise {
	index := map[string]int{}
	exercises := []ParsedExercise{}

	for _, day := range days {
		for _, set := range day.Sets {
			position, known := index[set.ExerciseSlug]
			if !known {
				position = len(exercises)
				index[set.ExerciseSlug] = position
				exercises = append(exercises, ParsedExercise{
					Name: set.ExerciseName,
					Slug: set.ExerciseSlug,
					// The refs sheet's accessory block is the author's own
					// statement that a movement isn't tied to a 1RM, so trust
					// it over anything inferred from a single row.
					Bodyweight: set.LoadMode == loadcalc.ModeBodyweight,
				})
			}
			if accessories[set.ExerciseSlug] {
				continue
			}
			// One resolvable row is enough to pin the reference lift, and every
			// row of the same movement agrees, so stop at the first that
			// resolves.
			if utils.IsBlank(exercises[position].OneRMRef) && set.Percentage != nil {
				exercises[position].OneRMRef = referenceFromLoad(set.sourceLoad, *set.Percentage, refOneRMs)
			}
		}
	}
	return exercises
}

// parseRefs reads the reference sheet's two blocks: the 1RM table
// (Exercice | Load) and the accessory table (Exercice | Reps | Load). Rows
// outside a recognized block, and the "Total" summary row, are ignored.
func parseRefs(rows [][]string, oneRMs map[string]float64, accessories map[string]bool) {
	const (
		blockNone = iota
		blockOneRM
		blockAccessory
	)
	block := blockNone

	for _, row := range rows {
		name := strings.TrimSpace(cell(row, colExercise))
		if utils.IsBlank(name) {
			block = blockNone
			continue
		}
		if exerciseHeaders[strings.ToLower(name)] {
			// Two columns of header means the 1RM table, three the accessory
			// one - the accessory table has a Reps column in between.
			if len(row) >= 3 && utils.IsNotBlank(cell(row, 2)) {
				block = blockAccessory
			} else {
				block = blockOneRM
			}
			continue
		}
		if strings.EqualFold(name, "total") {
			continue
		}

		switch block {
		case blockOneRM:
			if value, ok := number(cell(row, 1)); ok && value > 0 {
				oneRMs[utils.Slugify(name)] = value
			}
		case blockAccessory:
			accessories[utils.Slugify(name)] = true
		}
	}
}

// parseSheet reads every day block in one week sheet. startPosition keeps day
// positions unique and ordered across the whole workbook.
func parseSheet(sheetName string, rows [][]string, startPosition int, program *ParsedProgram) []ParsedDay {
	week := weekFromSheetName(sheetName)
	var days []ParsedDay

	for i, row := range rows {
		if !isHeaderRow(row) {
			continue
		}

		// The title sits on the row above the header. In the reference file
		// several of those cells were overwritten by a stray formula, so the
		// title is only trusted when it actually parses as "WEEK n DAY m";
		// otherwise the day is numbered by its position in the sheet, which is
		// what the clobbered blocks turn out to be anyway.
		day := len(days) + 1
		title := utils.EMPTY
		if i > 0 {
			title = strings.TrimSpace(cell(rows[i-1], colExercise))
		}
		if match := weekDayPattern.FindStringSubmatch(title); match != nil {
			if w, err := strconv.Atoi(match[1]); err == nil && w > 0 {
				week = w
			}
			if d, err := strconv.Atoi(match[2]); err == nil && d > 0 {
				day = d
			}
		} else {
			if utils.IsNotBlank(title) {
				program.Warnings = append(program.Warnings, fmt.Sprintf(
					"%s: the title above row %d reads %q instead of \"WEEK n DAY m\"; numbered it week %d day %d from its position",
					sheetName, i+1, title, week, day))
			}
			title = utils.EMPTY
		}
		if utils.IsBlank(title) {
			title = fmt.Sprintf("Week %d Day %d", week, day)
		}

		sets := parseSets(sheetName, rows, i+1, program)
		if len(sets) == 0 {
			continue
		}
		days = append(days, ParsedDay{
			Week:     week,
			Day:      day,
			Title:    title,
			Position: startPosition + len(days),
			Sets:     sets,
		})
	}
	return days
}

// parseSets reads the set rows following a header row, stopping at the first
// blank row or the next block's title/header.
func parseSets(sheetName string, rows [][]string, from int, program *ParsedProgram) []ParsedSet {
	var sets []ParsedSet

	for i := from; i < len(rows); i++ {
		row := rows[i]
		name := strings.TrimSpace(cell(row, colExercise))
		if utils.IsBlank(name) || isHeaderRow(row) {
			break
		}

		reps, hasReps := number(cell(row, colReps))
		if !hasReps || reps <= 0 {
			// A row with a name but no reps is the next block's title, not a
			// set: stop here rather than importing the title as an exercise.
			break
		}

		set := ParsedSet{
			ExerciseName: name,
			ExerciseSlug: utils.Slugify(name),
			Position:     len(sets),
			Reps:         int(math.Round(reps)),
		}

		if rpe, ok := number(cell(row, colRPE)); ok && rpe > 0 && rpe <= 10 {
			set.RPE = &rpe
		}
		if pct, ok := number(cell(row, colPercentage)); ok && pct > 0 {
			set.Percentage = &pct
		}
		load, hasLoad := number(cell(row, colLoad))
		set.sourceLoad = load
		if part, ok := number(cell(row, colPart)); ok && part > 0 {
			set.Part = int(math.Round(part))
		}

		switch {
		// An RPE prescription against a main lift is authoritative: the load
		// follows from the member's own 1RM, and the authored percentage is
		// kept only for reference (see loadcalc's package comment).
		case set.Percentage != nil && set.RPE != nil:
			set.LoadMode = loadcalc.ModeRPE
		// The spreadsheet writes "?" in the RPE column for sets the coach
		// deliberately left open; those keep their authored percentage.
		case set.Percentage != nil:
			set.LoadMode = loadcalc.ModePercentage
		// No percentage at all: an accessory, loaded either in absolute kilos
		// or by the athlete's own bodyweight (the sheet writes 0).
		case hasLoad && load > 0:
			set.LoadMode = loadcalc.ModeAbsolute
			set.AbsoluteLoad = &load
		default:
			set.LoadMode = loadcalc.ModeBodyweight
		}

		if utils.IsBlank(set.ExerciseSlug) {
			program.Warnings = append(program.Warnings, fmt.Sprintf(
				"%s row %d: exercise name %q has no usable letters, skipped", sheetName, i+1, name))
			continue
		}
		sets = append(sets, set)
	}
	return sets
}

// referenceTolerance is how far the 1RM implied by a row may sit from a
// reference 1RM and still be considered the same lift. The spreadsheet's
// percentages are whole numbers, so the implied max is off by up to about half
// a percent even when the match is right.
const referenceTolerance = 0.02

// referenceFromLoad identifies which competition lift a row is programmed off.
// The spreadsheet doesn't say so anywhere readable - a Larsen press row simply
// computes "82% of refs!B4" - so it's recovered from the arithmetic: the row's
// cached load and its percentage imply the 1RM the author used
// (load*100/percentage), and that lands on one of the reference 1RMs.
func referenceFromLoad(load, percentage float64, refOneRMs map[string]float64) string {
	if load <= 0 || percentage <= 0 {
		return utils.EMPTY
	}
	implied := load * 100 / percentage

	best, bestDelta := utils.EMPTY, math.MaxFloat64
	for slug, value := range refOneRMs {
		if value <= 0 {
			continue
		}
		if delta := math.Abs(implied-value) / value; delta < bestDelta {
			best, bestDelta = slug, delta
		}
	}
	if bestDelta > referenceTolerance {
		return utils.EMPTY
	}
	return best
}

// isHeaderRow reports whether row is a block's "Exercice | Reps | RPE | ..."
// header.
func isHeaderRow(row []string) bool {
	return exerciseHeaders[strings.ToLower(strings.TrimSpace(cell(row, colExercise)))]
}

// weekFromSheetName reads the week number out of a sheet name like "week 3",
// defaulting to 1 for a sheet with no number in it.
func weekFromSheetName(name string) int {
	if match := sheetWeekPattern.FindStringSubmatch(name); match != nil {
		if week, err := strconv.Atoi(match[1]); err == nil && week > 0 {
			return week
		}
	}
	return 1
}

// cell reads one column of a row, tolerating rows shorter than the header
// (excelize trims trailing empty cells).
func cell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return utils.EMPTY
	}
	return row[index]
}

// number parses a numeric cell. The spreadsheet writes "?" for an RPE the
// coach left open, which is not an error - it just isn't a number.
func number(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if utils.IsBlank(value) || value == "?" {
		return 0, false
	}
	// Some locales export decimals with a comma.
	parsed, err := strconv.ParseFloat(strings.Replace(value, ",", ".", 1), 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
