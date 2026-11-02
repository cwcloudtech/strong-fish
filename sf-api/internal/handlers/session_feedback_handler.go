package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/ai"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programmd"
	"strong-fish-api/internal/programsheet"
	"strong-fish-api/internal/templates"
)

// What the model is shown of the block behind the session being reviewed.
//
// Enough to see a direction, not the whole history: a dozen sessions of
// context costs tokens on every reading and adds little, because what a coach
// looks at is the last few times this lift was trained.
const feedbackContextSessions = 6

// SessionFeedback returns what the model already said about one session.
//
// Authorized exactly like the session itself: the member, a manager of the
// club the program belongs to, or a superadmin. A coach reads their athlete's
// feedback the way they read their comments - it is part of the same record.
func (h *TrainingHandler) SessionFeedback(w http.ResponseWriter, r *http.Request) {
	assignment, ok := h.authorizeAssignment(w, r)
	if !ok {
		return
	}

	feedback, err := h.aiStore.FindFeedback(r.Context(), assignment.ID, chi.URLParam(r, "dayId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, feedback)
}

// DeleteSessionFeedback removes a reading, which is how a member asks for
// another one.
func (h *TrainingHandler) DeleteSessionFeedback(w http.ResponseWriter, r *http.Request) {
	assignment, ok := h.authorizeAssignment(w, r)
	if !ok {
		return
	}
	if err := h.aiStore.DeleteFeedback(r.Context(), assignment.ID, chi.URLParam(r, "dayId")); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GenerateSessionFeedback asks the model to read one finished session.
//
// The connection spent is the caller's own, not the athlete's: a coach
// reviewing somebody's week pays for it out of their own subscription, which
// is what lets a coach buy one key for a roster that has none.
//
// It refuses a session that is not finished. A reading of three logged sets
// out of eight is a reading of a session that did not happen that way, and a
// member would take it for a verdict on their training rather than on an
// incomplete record of it.
func (h *TrainingHandler) GenerateSessionFeedback(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	assignment, ok := h.authorizeAssignment(w, r)
	if !ok {
		return
	}
	dayID := chi.URLParam(r, "dayId")

	days, sets, err := h.sessionsOf(r, assignment)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	var subject models.ProgramDay
	found := false
	for _, day := range days {
		if day.ID == dayID {
			subject, found = day, true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "Session not found", CodeNotFound)
		return
	}
	if !sessionComplete(subject) {
		writeError(w, http.StatusConflict,
			"Finish the session before asking for a reading of it", CodeSessionIncomplete)
		return
	}

	// The athlete's language, not the caller's: a coach generating a reading
	// for somebody writes it into that member's record, where that member is
	// the one who will read it. English when nothing is configured.
	locale := "en"
	if athlete, err := h.users.FindByID(r.Context(), assignment.UserID); err == nil {
		locale = localeOf(athlete, r)
	}
	program, err := h.programs.FindByID(r.Context(), assignment.ProgramID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	sheet := feedbackSheet(program, subject, earlierSessions(days, subject), locale)

	answer, ok := h.ai.ask(w, r, callerID, "session", ai.Request{
		// Both skills: how a coach answers at all, then what a session review
		// specifically is. Kept apart so the common half is edited once. The
		// language is pinned here because nothing in a session table tells the
		// model which one to answer in.
		System: templates.CoachSkill + "\n\n" + templates.FeedbackSkill +
			"\n\n" + LanguageRule(locale),
		Messages: []models.ChatMessage{
			{Role: models.ChatRoleUser, Message: sheet},
		},
	})
	if !ok {
		return
	}

	connection, _ := h.aiStore.Usable(r.Context(), callerID)
	stored, err := h.aiStore.SaveFeedback(r.Context(), models.SessionFeedback{
		AssignmentID: assignment.ID, DayID: dayID, AuthorID: callerID,
		Content: answer.Message, Provider: connection.Provider, Model: answer.Model,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, stored)
	_ = sets
}

// sessionsOf loads the block's sessions with their sets resolved against the
// athlete and their logs attached - the same data the export builds, for the
// same reason: what the model reads has to be what the member sees.
func (h *TrainingHandler) sessionsOf(r *http.Request, assignment models.ProgramAssignment) (
	[]models.ProgramDay, []models.ProgramSet, error) {
	days, err := h.programs.ListDays(r.Context(), assignment.ProgramID)
	if err != nil {
		return nil, nil, err
	}
	sets, err := h.programs.ListSetsForProgram(r.Context(), assignment.ProgramID)
	if err != nil {
		return nil, nil, err
	}
	logs, err := h.programs.ListLogsForAssignment(r.Context(), assignment.ID)
	if err != nil {
		return nil, nil, err
	}

	resolved, _, err := h.sets.resolveSets(r.Context(), sets, assignment.UserID, logs)
	if err != nil {
		return nil, nil, err
	}

	byDay := map[string][]models.ProgramSet{}
	for _, set := range resolved {
		byDay[set.DayID] = append(byDay[set.DayID], set)
	}
	for i := range days {
		days[i].Sets = programsheet.SortedSets(models.ProgramDay{Sets: byDay[days[i].ID]})
	}
	return days, resolved, nil
}

// sessionComplete reports whether every prescribed set was ticked off.
//
// Every set, not "some logs exist": a session half entered is a record of
// something that has not finished happening, and the button that asks for a
// reading stays off until it has.
func sessionComplete(day models.ProgramDay) bool {
	if len(day.Sets) == 0 {
		return false
	}
	for _, set := range day.Sets {
		if set.Log == nil || !set.Log.Done {
			return false
		}
	}
	return true
}

// earlierSessions is the finished sessions before this one, most recent last,
// bounded by feedbackContextSessions.
func earlierSessions(days []models.ProgramDay, subject models.ProgramDay) []models.ProgramDay {
	earlier := []models.ProgramDay{}
	for _, day := range days {
		if day.ID == subject.ID {
			break
		}
		if len(day.Sets) > 0 && anyLogged(day) {
			earlier = append(earlier, day)
		}
	}
	if len(earlier) > feedbackContextSessions {
		earlier = earlier[len(earlier)-feedbackContextSessions:]
	}
	return earlier
}

func anyLogged(day models.ProgramDay) bool {
	for _, set := range day.Sets {
		if set.Log != nil {
			return true
		}
	}
	return false
}

// feedbackSheet writes the sessions out for the model, as markdown.
//
// The same document the member exports, rendered from the same tables: every
// column of the feedback sheet - the prescription, the reps done, the RPE
// felt, the estimated 1RM and the athlete's own comment - rather than a
// summary of it. A reading made from less than what the member can see is a
// reading they cannot check, and the comment they left on a set is often the
// only thing that explains the number beside it.
//
// The subject session comes last, after the ones before it, because that is
// the order they happened in.
func feedbackSheet(program models.Program, subject models.ProgramDay,
	earlier []models.ProgramDay, locale string) string {
	sections := make([]programsheet.Section, 0, len(earlier)+1)
	columns := programsheet.Columns(locale, true)

	for _, day := range earlier {
		sections = append(sections, programsheet.Section{
			Heading: sessionHeading(day, locale),
			Columns: columns,
			Rows:    programsheet.Lines(day.Sets, true, locale),
		})
	}
	sections = append(sections, programsheet.Section{
		Heading: sessionHeading(subject, locale),
		Columns: columns,
		Rows:    programsheet.Lines(subject.Sets, true, locale),
	})

	// English framing, like the skill it arrives with: it says which table is
	// the subject, which the tables themselves cannot.
	subtitle := fmt.Sprintf(
		"The session to review is the last one below: %s. The sections before it are "+
			"earlier sessions of the same block, oldest first, for context.",
		sessionHeading(subject, locale))
	if len(earlier) == 0 {
		subtitle = "This is the first completed session of the block; there is nothing " +
			"earlier to compare it against."
	}

	return programmd.RenderTables(program.Name, subtitle, sections, "")
}

func sessionHeading(day models.ProgramDay, locale string) string {
	return fmt.Sprintf("%s - %s",
		programsheet.WeekLabel(locale, day.Week), programsheet.DayTitle(day, locale))
}
