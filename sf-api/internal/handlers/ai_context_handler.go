package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programmd"
	"strong-fish-api/internal/programsheet"
)

// What the model is told about the member before their question.
//
// A coach does not ask what you squat every time you speak to them, and a
// chat that answers "it depends on your maxes" to every question is a chat
// nobody asks twice. So the same facts travel with every question: who is
// asking, what they lift, and what block they are in the middle of.
//
// It goes in the system prompt rather than in the conversation, for two
// reasons: it is not something the member said, and it has to be *current* -
// a figure quoted into the thread in January would still be being read in
// March, after the max it describes had moved.

// memberContext writes the member's file for the model, or "" when there is
// nothing to say about them.
//
// Failures are skipped, never fatal. A question that cannot be answered
// because the 1RM query was slow is worse than one answered without the
// maxes, and the model is told what is missing rather than left to assume.
func (h *AIHandler) memberContext(r *http.Request, userID, locale string) string {
	var out strings.Builder
	out.WriteString("# The member you are talking to\n\n")

	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		return ""
	}
	out.WriteString(profileLines(user))

	if h.oneRMs != nil {
		maxes, err := h.oneRMs.ListForUser(r.Context(), userID)
		if err == nil {
			out.WriteString("\n" + maxLines(maxes, locale))
		}
	}

	if block := h.currentBlock(r, userID, locale); block != "" {
		out.WriteString("\n" + block)
	}

	// Their fitness data: what their watch counted today, their latest body
	// composition, and their last bar-speed set of each lift. A question asked
	// in the evening is about the day they have actually had - "should I train
	// tonight" reads differently after twelve thousand steps than after four
	// hundred - and "is my deadlift moving well" needs the bar-speed set in
	// front of it.
	if h.lastDay != nil {
		if fitness := h.lastDay(r, userID, locale); fitness != "" {
			out.WriteString("\n## Fitness data (steps and heart rate are the last 24h; bar speed is the last set)\n\n" + fitness)
		}
	}

	out.WriteString("\nUse these without being asked for them, and never read them back as a list.\n")
	return out.String()
}

// profileLines is the member themselves: the numbers a coach would know.
func profileLines(user models.User) string {
	var out strings.Builder

	if name := strings.TrimSpace(user.Name + " " + user.Surname); name != "" {
		fmt.Fprintf(&out, "- Name: %s\n", name)
	}
	if user.Gender != "" {
		fmt.Fprintf(&out, "- Gender: %s\n", user.Gender)
	}
	if user.Birthdate != "" {
		if age := ageOn(user.Birthdate, time.Now()); age > 0 {
			fmt.Fprintf(&out, "- Age: %d (born %s)\n", age, user.Birthdate)
		} else {
			fmt.Fprintf(&out, "- Born: %s\n", user.Birthdate)
		}
	}
	if user.Bodyweight > 0 {
		fmt.Fprintf(&out, "- Bodyweight: %s kg\n", trimFloat(user.Bodyweight))
	}
	if user.Height > 0 {
		fmt.Fprintf(&out, "- Height: %s cm\n", trimFloat(user.Height))
	}
	if out.Len() == 0 {
		return "Nothing is recorded about them.\n"
	}
	return out.String()
}

// maxLines is every max they have recorded, heaviest first.
func maxLines(maxes []models.OneRM, locale string) string {
	if len(maxes) == 0 {
		return "## Current maxes\n\nNone recorded. The loads in their program cannot be computed, " +
			"and you should say so rather than guessing at numbers.\n"
	}

	var out strings.Builder
	out.WriteString("## Current maxes (1RM)\n\n")
	for _, max := range maxes {
		fmt.Fprintf(&out, "- %s: %s kg\n", exerciseLabel(max, locale), trimFloat(max.Value))
	}
	return out.String()
}

func exerciseLabel(max models.OneRM, locale string) string {
	if label := max.Labels[locale]; label != "" {
		return label
	}
	if label := max.Labels["en"]; label != "" {
		return label
	}
	return max.Slug
}

// currentBlock is the program the member is in the middle of, as the sheet
// they can export: the whole thing, prescription and what they have done so
// far, because "how is my block going" is the question this chat exists for.
//
// The active assignment, most recent first. A member running two blocks at
// once is rare enough that the newest is the right guess, and the sheet names
// the program so the model can say which one it is reading.
func (h *AIHandler) currentBlock(r *http.Request, userID, locale string) string {
	if h.programs == nil || h.sets == nil {
		return ""
	}

	assignments, err := h.programs.ListAssignmentsForUser(r.Context(), userID)
	if err != nil {
		return ""
	}
	var current models.ProgramAssignment
	for _, assignment := range assignments {
		if assignment.Status == "" || assignment.Status == models.AssignmentStatusActive {
			current = assignment
			break
		}
	}
	if current.ID == "" {
		return "## Current block\n\nThey are not running a program at the moment.\n"
	}

	program, err := h.programs.FindByID(r.Context(), current.ProgramID)
	if err != nil {
		return ""
	}
	days, err := h.programs.ListDays(r.Context(), current.ProgramID)
	if err != nil {
		return ""
	}
	sets, err := h.programs.ListSetsForProgram(r.Context(), current.ProgramID)
	if err != nil {
		return ""
	}
	logs, err := h.programs.ListLogsForAssignment(r.Context(), current.ID)
	if err != nil {
		return ""
	}
	resolved, _, err := h.sets.resolveSets(r.Context(), sets, current.UserID, logs)
	if err != nil {
		return ""
	}

	byDay := map[string][]models.ProgramSet{}
	for _, set := range resolved {
		byDay[set.DayID] = append(byDay[set.DayID], set)
	}

	columns := programsheet.Columns(locale, true)
	sections := make([]programsheet.Section, 0, len(days))
	for _, day := range days {
		day.Sets = programsheet.SortedSets(models.ProgramDay{Sets: byDay[day.ID]})
		sections = append(sections, programsheet.Section{
			Heading: fmt.Sprintf("%s - %s", programsheet.WeekLabel(locale, day.Week),
				programsheet.DayTitle(day, locale)),
			Columns: columns,
			Rows:    aiLines(day.Sets, locale),
		})
	}

	return "## Current block\n\n" + programmd.RenderTables(program.Name,
		"The block they are running now: the prescription, and what they have logged against it so far.",
		sections, "")
}

// ageOn is their age today, or 0 when the date cannot be read.
func ageOn(birthdate string, now time.Time) int {
	born, err := time.Parse("2006-01-02", birthdate)
	if err != nil {
		return 0
	}
	age := now.Year() - born.Year()
	// Not had the birthday yet this year.
	if now.YearDay() < born.YearDay() {
		age--
	}
	if age < 0 || age > 120 {
		return 0
	}
	return age
}

// trimFloat writes a measurement without a pointless decimal: 92 kg rather
// than 92.0 kg, 92.5 kg when that is what was entered.
func trimFloat(value float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
}

// aiLines is the feedback sheet's rows with the prescribed load kept where
// nothing has been lifted yet.
//
// On paper that cell is deliberately blank - the feedback sheet is about what
// was done, and a coach reading a half-finished week wants the gaps to show.
// A model reading the same table needs the number: "five at 80%" is not an
// answer it can give in kilos, and the whole point of this app is that the
// kilos are computed.
func aiLines(sets []models.ProgramSet, locale string) []programsheet.Line {
	lines := programsheet.Lines(sets, true, locale)
	for index, set := range sets {
		if index >= len(lines) || len(lines[index].Cells) <= loadColumn {
			continue
		}
		if lines[index].Cells[loadColumn] == "" {
			lines[index].Cells[loadColumn] = programsheet.FormatLoad(set)
		}
	}
	return lines
}

// loadColumn is where the load sits in a sheet's row: exercise, reps,
// intensity, load (see programsheet.Row).
const loadColumn = 3
