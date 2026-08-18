package xlsximport

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"strong-fish-api/internal/loadcalc"
	"strong-fish-api/internal/utils"
)

// The second layout this package understands: one sheet per *block*, holding
// several weeks at once (ai-gen/assets/program_2.xlsx).
//
//	                                                  <- sheet: "BLOCK 1 - (06072026-12072026)"
//	EXERCICES | SÉRIES/REPS | RPE | ESTIMATIONS | REMARQUES | W1 | RPE | COMMENTAIRE | W2 | ...
//	SÉANCE 1 in column A, then:
//	TEMPO COMP.SQUAT: | 1 x 3 | 6 |   |  Ajouter 1 rpe/semaine
//	                  | 3 x 3 | 7                              <- same movement, back-off sets
//
// The coach prescribes each session once; W1..Wn are the columns the athlete
// fills in week by week. So a block sheet expands to (number of W columns) x
// (number of sessions) training days, all carrying the same prescription.
//
// The W* columns themselves are deliberately not imported. They are one
// athlete's log - what they actually lifted, and what they said about it -
// whereas a program here is generic: it is handed to every member of a club,
// or published to anyone. Importing one person's numbers as everybody's
// prescription would be wrong, and the app collects the same feedback itself.
//
// A block sheet also carries material this parser has no use for: a warm-up
// column, a video-link glossary, a free-text health section. All of it sits
// outside the prescription columns and is simply never read.

var (
	// blockSetsRepsHeader is the column that identifies this layout. Both the
	// accented and unaccented spellings show up depending on who typed it.
	blockSetsRepsHeader = regexp.MustCompile(`(?i)^s[ée]ries?\s*/\s*reps?$`)
	// blockWeekHeader matches a week column: "W1", "W 2", ...
	blockWeekHeader = regexp.MustCompile(`(?i)^w\s*(\d+)$`)
	// blockNameHeader matches the exercise-name column.
	blockNameHeader = regexp.MustCompile(`(?i)^exerci[cs]es?$`)
	// blockNotesHeader matches the coach's remarks column.
	blockNotesHeader = regexp.MustCompile(`(?i)^(remarques?|notes?)$`)
	// blockSessionLabel matches the session marker in column A.
	blockSessionLabel = regexp.MustCompile(`(?i)^\s*(s[ée]ance|session|jour|day)\s*(\d+)`)
	// blockSetsReps reads "3 x 8": a set count and a rep target. The rep half
	// is captured loosely because it isn't always a number ("3 x AMRAP").
	blockSetsReps = regexp.MustCompile(`^\s*(\d+)\s*[x×*]\s*(.+?)\s*$`)
)

// blockSkipLabels are the words that appear in the exercise-name column
// without naming an exercise: the little COACH/ATHLETE banner repeated above
// each session, and the header echo itself.
var blockSkipLabels = map[string]bool{"coach": true, "athlete": true, "athlète": true, "exercices": true, "exercise": true, "exercises": true}

// blockLayout is where each field sits in one block sheet, discovered from its
// header row rather than assumed: the columns before the first week column are
// the prescription, everything from there on is somebody's log.
type blockLayout struct {
	name     int
	setsReps int
	rpe      int
	notes    int
	weeks    int // how many W columns the sheet has
	firstLog int // index of the first W column, or -1 when there is none
}

// blockHeaderLayout reads a row as a block header, reporting false for any row
// that isn't one.
func blockHeaderLayout(row []string) (blockLayout, bool) {
	layout := blockLayout{name: -1, setsReps: -1, rpe: -1, notes: -1, firstLog: -1}

	for i, raw := range row {
		value := strings.TrimSpace(raw)
		switch {
		case blockSetsRepsHeader.MatchString(value):
			if layout.setsReps < 0 {
				layout.setsReps = i
			}
		case blockNameHeader.MatchString(value):
			if layout.name < 0 {
				layout.name = i
			}
		case blockWeekHeader.MatchString(value):
			layout.weeks++
			if layout.firstLog < 0 {
				layout.firstLog = i
			}
		}
	}
	if layout.setsReps < 0 || layout.name < 0 {
		return blockLayout{}, false
	}

	// The prescription's own RPE and remarks are the ones *before* the first
	// week column - past it, "RPE" and "COMMENTAIRE" repeat once per week and
	// belong to the athlete's log.
	limit := len(row)
	if layout.firstLog >= 0 {
		limit = layout.firstLog
	}
	for i := layout.setsReps + 1; i < limit; i++ {
		value := strings.TrimSpace(row[i])
		if layout.rpe < 0 && strings.EqualFold(value, "rpe") {
			layout.rpe = i
		}
		if layout.notes < 0 && blockNotesHeader.MatchString(value) {
			layout.notes = i
		}
	}

	// A sheet with no W column at all is still a valid block - it just holds a
	// single week.
	if layout.weeks == 0 {
		layout.weeks = 1
	}
	return layout, true
}

// isBlockSheet reports whether a sheet uses the block layout, which is what
// picks a parser for the workbook.
func isBlockSheet(rows [][]string) bool {
	for _, row := range rows {
		if _, ok := blockHeaderLayout(row); ok {
			return true
		}
	}
	return false
}

// parseBlockSheet expands one block sheet into training days.
//
// weekOffset is how many weeks earlier sheets already contributed: a workbook
// with two blocks of four weeks is an eight-week program, so the second
// block's "W1" is program week 5. It returns the days and how many weeks this
// sheet added.
func parseBlockSheet(sheetName string, rows [][]string, weekOffset, startPosition int, program *ParsedProgram) ([]ParsedDay, int) {
	// Each header row opens a session; the session runs until the next one.
	type session struct {
		layout blockLayout
		from   int
		to     int
	}
	var sessions []session
	for i, row := range rows {
		layout, ok := blockHeaderLayout(row)
		if !ok {
			continue
		}
		if len(sessions) > 0 {
			sessions[len(sessions)-1].to = i
		}
		sessions = append(sessions, session{layout: layout, from: i + 1, to: len(rows)})
	}
	if len(sessions) == 0 {
		return nil, 0
	}

	weeks := 0
	for _, s := range sessions {
		if s.layout.weeks > weeks {
			weeks = s.layout.weeks
		}
	}

	// Parse each session's prescription once...
	type parsed struct {
		label string
		sets  []ParsedSet
	}
	var prescriptions []parsed
	for _, s := range sessions {
		label, sets := parseBlockSession(sheetName, rows, s.from, s.to, s.layout, program)
		if len(sets) == 0 {
			continue
		}
		if utils.IsBlank(label) {
			label = fmt.Sprintf("Session %d", len(prescriptions)+1)
		}
		prescriptions = append(prescriptions, parsed{label: label, sets: sets})
	}
	if len(prescriptions) == 0 {
		return nil, 0
	}

	// ...then repeat it across the block's weeks. The coach wrote one session
	// and expected it run every week of the block; the athlete's week-by-week
	// columns are what differed, and those aren't imported.
	days := make([]ParsedDay, 0, weeks*len(prescriptions))
	for week := 1; week <= weeks; week++ {
		for day, p := range prescriptions {
			sets := make([]ParsedSet, len(p.sets))
			copy(sets, p.sets)
			days = append(days, ParsedDay{
				Week:     weekOffset + week,
				Day:      day + 1,
				Title:    fmt.Sprintf("%s - W%d - %s", strings.TrimSpace(sheetName), week, p.label),
				Position: startPosition + len(days),
				Sets:     sets,
			})
		}
	}
	return days, weeks
}

// parseBlockSession reads one session's prescription rows, returning its label
// (the "SÉANCE 1" marker in column A) and its sets.
//
// A row with a name opens a movement; the rows under it that carry only a
// sets/reps value are that same movement's remaining work - the top single
// followed by its back-off sets, which is how these sheets are written.
func parseBlockSession(sheetName string, rows [][]string, from, to int, layout blockLayout, program *ParsedProgram) (string, []ParsedSet) {
	label := utils.EMPTY
	var sets []ParsedSet
	current := utils.EMPTY
	currentNotes := utils.EMPTY
	// The last RPE typed for the movement being read. These sheets leave the
	// cell blank on a row that continues the one above it - "1 x 4 @ 8" then
	// "2 x 4" - and the same slot is filled in explicitly elsewhere in the
	// same workbook, so blank means "as above" rather than "no target". Left
	// unresolved, a barbell back-off set would import as having no load to
	// work out at all.
	var currentRPE *float64

	for i := from; i < to && i < len(rows); i++ {
		row := rows[i]

		if match := blockSessionLabel.FindStringSubmatch(strings.TrimSpace(cell(row, 0))); match != nil && utils.IsBlank(label) {
			label = strings.TrimSpace(cell(row, 0))
		}

		if name := blockExerciseName(cell(row, layout.name)); utils.IsNotBlank(name) {
			if blockSkipLabels[strings.ToLower(name)] {
				current, currentNotes, currentRPE = utils.EMPTY, utils.EMPTY, nil
				continue
			}
			current = name
			currentNotes = strings.TrimSpace(cell(row, layout.notes))
			currentRPE = nil
		}

		setsReps := strings.TrimSpace(cell(row, layout.setsReps))
		if utils.IsBlank(setsReps) || utils.IsBlank(current) {
			continue
		}

		count, reps, repsLabel, ok := parseSetsReps(setsReps)
		if !ok {
			program.Warnings = append(program.Warnings, fmt.Sprintf(
				"%s row %d: could not read %q as a number of sets and reps, skipped", sheetName, i+1, setsReps))
			continue
		}

		slug := utils.Slugify(current)
		if utils.IsBlank(slug) {
			program.Warnings = append(program.Warnings, fmt.Sprintf(
				"%s row %d: exercise name %q has no usable letters, skipped", sheetName, i+1, current))
			continue
		}

		var rpe *float64
		if value, hasRPE := number(cell(row, layout.rpe)); hasRPE && value > 0 && value <= 10 {
			rpe = &value
			currentRPE = &value
		} else if currentRPE != nil {
			rpe = currentRPE
			program.Warnings = append(program.Warnings, fmt.Sprintf(
				"%s row %d: %q has no RPE; carried %g down from the row above", sheetName, i+1, current, *currentRPE))
		}

		notes := currentNotes
		if utils.IsNotBlank(repsLabel) {
			// The rep target wasn't a number ("AMRAP"): keep what the coach
			// actually wrote, since it's the instruction.
			notes = strings.TrimSpace(repsLabel + " " + notes)
		}

		// A block sheet prescribes effort, never a weight: there is no
		// percentage and no load column to read. So an RPE row resolves
		// against the member's own 1RM, and a row without one - an AMRAP set
		// taken to failure, an unloaded movement - carries no computable load
		// at all.
		mode := loadcalc.ModeBodyweight
		if rpe != nil && reps > 0 {
			mode = loadcalc.ModeRPE
		}

		for n := 0; n < count; n++ {
			sets = append(sets, ParsedSet{
				ExerciseName: current,
				ExerciseSlug: slug,
				Position:     len(sets),
				Reps:         reps,
				RPE:          rpe,
				LoadMode:     mode,
				Notes:        notes,
			})
		}
	}
	return label, sets
}

// blockExerciseName tidies a name cell. These sheets end every movement with a
// colon ("COMP.DEADLIFT: ") and sometimes wrap it over spaces.
func blockExerciseName(value string) string {
	name := strings.TrimSpace(value)
	name = strings.TrimRight(name, ": \t")
	return strings.Join(strings.Fields(name), " ")
}

// parseSetsReps reads a "3 x 8" cell into a set count and a rep target.
//
// The rep half is not always a number: "3 x AMRAP" is three sets taken to
// failure. Those come back with reps 0 and the original text as repsLabel, so
// the caller can keep the coach's instruction instead of inventing a rep count
// that would drive a load calculation nobody asked for.
func parseSetsReps(value string) (count, reps int, repsLabel string, ok bool) {
	value = strings.TrimSpace(value)
	if utils.IsBlank(value) {
		return 0, 0, utils.EMPTY, false
	}

	match := blockSetsReps.FindStringSubmatch(value)
	if match == nil {
		// A bare number is one set of that many reps.
		if single, isNumber := number(value); isNumber && single > 0 {
			return 1, int(math.Round(single)), utils.EMPTY, true
		}
		return 0, 0, utils.EMPTY, false
	}

	count, err := strconv.Atoi(match[1])
	if err != nil || count <= 0 {
		return 0, 0, utils.EMPTY, false
	}
	if count > maxSetsPerRow {
		// Guards against a mis-typed cell ("30 x 5") turning one row into
		// hundreds of sets.
		return 0, 0, utils.EMPTY, false
	}

	if value, isNumber := number(match[2]); isNumber && value > 0 {
		return count, int(math.Round(value)), utils.EMPTY, true
	}
	return count, 0, strings.ToUpper(strings.TrimSpace(match[2])), true
}

// maxSetsPerRow bounds how many sets one "n x r" cell may expand into.
const maxSetsPerRow = 20
