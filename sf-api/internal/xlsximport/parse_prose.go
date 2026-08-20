package xlsximport

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"strong-fish-api/internal/loadcalc"
	"strong-fish-api/internal/utils"
)

// The third layout this importer reads: a program written as prose rather than
// as a table.
//
// One sheet holds the whole block. Column A marks the weeks and their target
// RPE; the columns to its right are the sessions, side by side, and each
// session is a column of sentences a coach typed:
//
//	Semaine 1 | Mardi Séance 1 :        | Jeudi Séance 2
//	RPE 7     | Deadlift :              | Squat :
//	          | LWU 1 X 5 : 70kg        | LWU 1 X 5 : 50kg
//	          | Top set 3 X 5 : 85kg    | Top set 3 X 5 : 60kg
//	          | Back off : 1 X 5 : 80kg | Back off : 1 X 5 : 50kg
//	          | Renfo :                 |
//	          | Tirage vertical         |
//	          | 3 x 8-12 reps RPE 7     |
//
// Nothing here is in a fixed cell: the same file writes "Back off : 1 X 5",
// "Back off 1 X 5" and "Back off  1 X 5", ends a load "70kg", "70k" or
// "80-85kg", and switches between French and English mid-sheet. So this reads
// by shape rather than by position, and anything it cannot recognize becomes a
// warning rather than a wrong set.
//
// The layout carries no percentages and no reference maxes: it prescribes
// kilos, and a target RPE per week. Since this app derives every load from the
// member's own maxes, the week's RPE is what each set is stored as, and the
// coach's kilos are kept in the set's notes - the instruction being to ignore
// the load whenever an RPE is given.

var (
	// A week marker in the first column. "Semaine 3", "Week 3", or a deload
	// week, which this format writes without a number.
	proseWeek   = regexp.MustCompile(`(?i)^(?:semaine|week)\s*(\d+)`)
	proseDeload = regexp.MustCompile(`(?i)^(?:deload|décharge|decharge)`)

	// The week's target RPE, on its own line under the marker. "RPE 7,5/8" is a
	// range: the lower end is what gets used, being the one a lifter can always
	// meet.
	proseWeekRPE = regexp.MustCompile(`(?i)^rpe\s*(\d+(?:[.,]\d+)?)`)

	// A prescribed set: an optional label, then sets × reps, then the load.
	//   LWU 1 X 5 : 70kg      Top set 3 X 5 : 85kg      Back off 2 X 5 : 100k
	//   LWU 1 X 2: 90kg       Back off : 1 X 2 :100kg   Top set 1 x 1 : 105-110kg
	// The load is optional, and may be a range - a coach writing "80-85kg"
	// means either, and the first is the one to start from.
	proseSet = regexp.MustCompile(`(?i)^(.*?)(\d+)\s*[x×]\s*(\d+)\s*:?\s*(\d+(?:[.,]\d+)?)?(?:\s*-\s*(\d+(?:[.,]\d+)?))?\s*(?:kg|k)?\s*$`)

	// An accessory, prescribed by rep range and RPE rather than by weight:
	//   3 x 8-12 reps RPE 7
	proseAccessory = regexp.MustCompile(`(?i)^(\d+)\s*[x×]\s*(\d+)(?:\s*-\s*(\d+))?\s*(?:reps?|répétitions?|repetitions?)?\s*(?:@|rpe)\s*(\d+(?:[.,]\d+)?)`)

	// An RPE written anywhere in a line, for a set line that carries its own.
	proseInlineRPE = regexp.MustCompile(`(?i)(?:@|rpe)\s*(\d+(?:[.,]\d+)?)`)
)

// proseSections are headings that group what follows without being a movement
// themselves - "Renfo :" opens the accessory work, it is not an exercise.
var proseSections = map[string]bool{
	"renfo": true, "renforcement": true, "accessoires": true, "accessoire": true,
	"accessories": true, "accessory": true, "assistance": true,
}

// proseSpaces are the space characters a word processor leaves in a cell that
// Go's regexp does not treat as whitespace: `\s` is ASCII-only, so a
// non-breaking space between "LWU" and "1" is enough to stop every pattern
// here from matching - and the line then reads as prose rather than as a set.
// Typed text from a Mac is full of them.
var proseSpaces = strings.NewReplacer(
	"\u00a0", " ", // no-break space
	"\u202f", " ", // narrow no-break space
	"\u2009", " ", // thin space
	"\u2007", " ", // figure space
	"\u200b", "", // zero-width space
	"\t", " ",
)

// cleanProse normalizes a cell before anything tries to read it.
func cleanProse(text string) string {
	return strings.Join(strings.Fields(proseSpaces.Replace(text)), " ")
}

// maxProseSets caps how many rows one "N X R" line can become. A coach writes
// "3 X 5"; a misread cell could say 300.
const maxProseSets = 20

// isProseSheet reports whether a sheet is written in this layout.
//
// The test is a week marker in the first column with something beside it: a
// tabular sheet has its weeks in a title row or a sheet name, never stacked
// down column A with the sessions laid out across.
func isProseSheet(rows [][]string) bool {
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		first := cleanProse(row[0])
		if !proseWeek.MatchString(first) && !proseDeload.MatchString(first) {
			continue
		}
		for _, cell := range row[1:] {
			if cleanProse(cell) != "" {
				return true
			}
		}
	}
	return false
}

// proseSession is one session column while it is being read.
type proseSession struct {
	column int
	title  string
	sets   []ParsedSet
	// exercise is the movement the set lines are currently describing.
	exercise string
	// part groups the session: everything before the accessory heading is 1,
	// everything after it 2 - the same grouping the tabular formats carry.
	part int
	// position numbers the sets within the session.
	position int
}

// parseProseSheet reads one sheet of the prose layout into days.
func parseProseSheet(sheet string, rows [][]string, dayOffset int, program *ParsedProgram) []ParsedDay {
	var days []ParsedDay

	week := 0
	var weekRPE *float64
	var sessions []*proseSession

	// flush turns the sessions read so far into days.
	flush := func() {
		for index, session := range sessions {
			if len(session.sets) == 0 {
				continue
			}
			days = append(days, ParsedDay{
				Week:     week,
				Day:      index + 1,
				Title:    session.title,
				Position: dayOffset + len(days),
				Sets:     session.sets,
			})
		}
		sessions = nil
	}

	for index, row := range rows {
		first := ""
		if len(row) > 0 {
			first = cleanProse(row[0])
		}

		// A week marker starts a new block of sessions, so whatever was being
		// read belongs to the week before it.
		if proseWeek.MatchString(first) || proseDeload.MatchString(first) {
			flush()
			weekRPE = nil

			if match := proseWeek.FindStringSubmatch(first); match != nil {
				week, _ = strconv.Atoi(match[1])
			} else {
				// A deload written without a number is the week after the last
				// one, which is where it sits on the page.
				week++
			}
			sessions = proseSessions(row)
			continue
		}

		if len(sessions) == 0 {
			continue
		}

		// The week's RPE sits under its marker, and applies to every set in it.
		// The same row carries each session's subtitle - "Séance dos" - which
		// says what the session is for and belongs in its title. Reading it
		// here rather than in the loop below is what keeps it from being taken
		// for the name of a movement.
		if match := proseWeekRPE.FindStringSubmatch(first); match != nil {
			weekRPE = parseProseNumber(match[1])
			for _, session := range sessions {
				if session.column >= len(row) {
					continue
				}
				if subtitle := cleanProse(row[session.column]); subtitle != "" {
					session.title = strings.TrimSpace(session.title + " - " + subtitle)
				}
			}
			continue
		}

		for _, session := range sessions {
			if session.column >= len(row) {
				continue
			}
			readProseCell(session, cleanProse(row[session.column]), weekRPE, program, sheet, index+1)
		}
	}

	flush()
	return days
}

// proseSessions reads the session headers on a week's row. Their column
// positions are what tie the rows below to a session, since the sheet has no
// other structure.
func proseSessions(row []string) []*proseSession {
	var sessions []*proseSession
	for column := 1; column < len(row); column++ {
		title := cleanProse(row[column])
		if title == "" {
			continue
		}
		sessions = append(sessions, &proseSession{
			column: column,
			title:  strings.TrimRight(title, " :"),
			part:   1,
		})
	}
	return sessions
}

// readProseCell reads one cell of a session column.
func readProseCell(session *proseSession, text string, weekRPE *float64,
	program *ParsedProgram, sheet string, rowNumber int) {
	if text == "" {
		return
	}

	heading := strings.ToLower(strings.TrimRight(text, " :"))
	if proseSections[heading] {
		// The accessory heading: what follows is a new part of the session,
		// and the heading itself is not a movement.
		session.part++
		session.exercise = ""
		return
	}

	if sets := parseProseSet(text, session, weekRPE); len(sets) > 0 {
		session.sets = append(session.sets, sets...)
		return
	}

	// Not a set line. Either the name of the movement the next lines describe,
	// or a sentence the coach wrote to the athlete.
	if isProseNote(text) {
		// Attached to the last set rather than dropped: it is an instruction
		// about the session, and the session is where it should still be
		// readable.
		if len(session.sets) > 0 {
			last := &session.sets[len(session.sets)-1]
			last.Notes = strings.TrimSpace(last.Notes + " " + text)
		}
		return
	}

	session.exercise = strings.TrimRight(text, " :")
}

// parseProseSet turns one line into the sets it prescribes.
//
// "Top set 3 X 5 : 85kg" is three sets of five, not one row saying three: the
// app ticks sets off one at a time, and the tabular formats are stored that
// way too.
func parseProseSet(text string, session *proseSession, weekRPE *float64) []ParsedSet {
	var count, reps int
	var rpe *float64
	var load *float64
	var label string

	if match := proseAccessory.FindStringSubmatch(text); match != nil {
		// An accessory: sets, a rep range, and its own RPE. No load to read -
		// which is the point of writing it this way.
		count, _ = strconv.Atoi(match[1])
		reps, _ = strconv.Atoi(match[2])
		rpe = parseProseNumber(match[4])
		// The lower end of the range is what gets prescribed, and the range
		// itself is kept: "8-12" tells a lifter to stop somewhere in there,
		// and storing only the 8 would read as a harder instruction.
		if match[3] != "" {
			label = fmt.Sprintf("%s-%s reps", match[2], match[3])
		}
	} else if match := proseSet.FindStringSubmatch(text); match != nil {
		label = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(match[1]), ":"))
		count, _ = strconv.Atoi(match[2])
		reps, _ = strconv.Atoi(match[3])
		if match[4] != "" {
			load = parseProseNumber(match[4])
		}
		if inline := proseInlineRPE.FindStringSubmatch(text); inline != nil {
			rpe = parseProseNumber(inline[1])
		}
	} else {
		return nil
	}

	if count <= 0 || count > maxProseSets || reps <= 0 || session.exercise == "" {
		return nil
	}

	// The week's RPE stands in when the line does not carry one of its own.
	if rpe == nil {
		rpe = weekRPE
	}

	// "Ignore the load if there's an RPE": an RPE set is stored as an RPE set,
	// and the kilos the coach wrote stay in the notes, where they are still
	// the author's own numbers rather than this member's.
	authored := loadValue(load)
	notes := label
	mode := loadcalc.ModeAbsolute
	switch {
	case rpe != nil:
		mode = loadcalc.ModeRPE
		if load != nil {
			notes = strings.TrimSpace(fmt.Sprintf("%s %s", label, formatProseLoad(*load)))
		}
		load = nil
	case load == nil:
		// Neither a load nor an RPE: nothing to work a weight out from, so it
		// is recorded as prescribed and left to the coach.
		mode = loadcalc.ModeAbsolute
	}

	slug := utils.Slugify(session.exercise)
	sets := make([]ParsedSet, 0, count)
	for i := 0; i < count; i++ {
		session.position++
		sets = append(sets, ParsedSet{
			ExerciseName: session.exercise,
			ExerciseSlug: slug,
			Position:     session.position,
			Reps:         reps,
			RPE:          rpe,
			AbsoluteLoad: load,
			LoadMode:     mode,
			Part:         session.part,
			Notes:        strings.TrimSpace(notes),
			sourceLoad:   authored,
		})
	}
	return sets
}

// isProseNote reports whether a line is a sentence to the athlete rather than
// the name of a movement.
//
// Movements are short - "Bench close Grip", "DC haltères inclinés". A sentence
// like "Ton entrainement de body à la suite" runs longer, and turning it into
// an exercise would put it in the catalogue for every club to see.
func isProseNote(text string) bool {
	return len(strings.Fields(text)) >= 5
}

// parseProseNumber reads a number written either way round: this format is
// French, so "47,5" and "47.5" both turn up in the same file.
func parseProseNumber(text string) *float64 {
	value, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(text), ",", "."), 64)
	if err != nil {
		return nil
	}
	return &value
}

// formatProseLoad prints a load the way a coach writes it: 82.5 stays 82.5,
// and 80.0 becomes 80.
func formatProseLoad(value float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(value, 'f', 1, 64), ".0") + "kg"
}

func loadValue(load *float64) float64 {
	if load == nil {
		return 0
	}
	return *load
}
