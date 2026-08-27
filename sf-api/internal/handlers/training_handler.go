package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
)

// TrainingHandler is the member's own view of the programs assigned to them:
// the sessions to run, with every load computed against their current 1RMs, and
// the feedback they leave on each set.
type TrainingHandler struct {
	programs *store.ProgramStore
	clubs    *store.ClubStore
	users    *store.UserStore
	sets     *ProgramHandler
}

// NewTrainingHandler reuses ProgramHandler's set resolution rather than
// duplicating it - both views must agree on what a set's load is.
func NewTrainingHandler(programs *store.ProgramStore, clubs *store.ClubStore, users *store.UserStore, sets *ProgramHandler) *TrainingHandler {
	return &TrainingHandler{programs: programs, clubs: clubs, users: users, sets: sets}
}

// ListAssignments returns the programs the caller has been given.
func (h *TrainingHandler) ListAssignments(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	assignments, err := h.programs.ListAssignmentsForUser(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, assignments)
}

// assignmentResponse is one assigned program with all its sessions resolved for
// the member running it.
type assignmentResponse struct {
	models.ProgramAssignment
	Program       models.Program      `json:"program"`
	Days          []models.ProgramDay `json:"days"`
	MissingOneRMs []models.Exercise   `json:"missingOneRms"`
}

// authorizeAssignment loads an assignment the caller is allowed to see: their
// own, or - for a coach - one belonging to a member of a club they manage.
func (h *TrainingHandler) authorizeAssignment(w http.ResponseWriter, r *http.Request) (models.ProgramAssignment, bool) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	assignment, err := h.programs.FindAssignment(r.Context(), chi.URLParam(r, "assignmentId"))
	if err != nil {
		writeStoreError(w, err)
		return models.ProgramAssignment{}, false
	}
	if assignment.UserID == callerID {
		return assignment, true
	}

	membership, err := h.clubs.FindMembership(r.Context(), assignment.ClubID, callerID)
	if err == nil && models.CanManageClub(membership.Role) {
		return assignment, true
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeStoreError(w, err)
		return models.ProgramAssignment{}, false
	}

	user, err := h.users.FindByID(r.Context(), callerID)
	if err == nil && user.Role == models.GlobalRoleSuperadmin {
		return assignment, true
	}

	// Same reasoning as ClubMembership: an assignment's existence is private.
	writeError(w, http.StatusNotFound, "Assignment not found", CodeNotFound)
	return models.ProgramAssignment{}, false
}

// Get returns one assigned program, fully resolved: every set carries the weight
// this member should lift, recomputed from whatever 1RMs they last recorded.
func (h *TrainingHandler) Get(w http.ResponseWriter, r *http.Request) {
	assignment, ok := h.authorizeAssignment(w, r)
	if !ok {
		return
	}

	program, err := h.programs.FindByID(r.Context(), assignment.ProgramID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	days, err := h.programs.ListDays(r.Context(), assignment.ProgramID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	sets, err := h.programs.ListSetsForProgram(r.Context(), assignment.ProgramID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	logs, err := h.programs.ListLogsForAssignment(r.Context(), assignment.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	resolved, missing, err := h.sets.resolveSets(r.Context(), sets, assignment.UserID, logs)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	byDay := map[string][]models.ProgramSet{}
	for _, set := range resolved {
		byDay[set.DayID] = append(byDay[set.DayID], set)
	}
	for i := range days {
		days[i].Sets = byDay[days[i].ID]
	}

	writeJSON(w, http.StatusOK, assignmentResponse{
		ProgramAssignment: assignment, Program: program, Days: days, MissingOneRMs: missing,
	})
}

type logSetPayload struct {
	ActualReps *int     `json:"actualReps"`
	ActualRPE  *float64 `json:"actualRpe"`
	ActualLoad *float64 `json:"actualLoad"`
	// Beltless is the member's own answer for this set. Sent for any set: a
	// movement nobody belts anyway simply never asks, so a true here is either
	// somebody's honest note or a client bug, and neither is worth refusing.
	Beltless bool   `json:"beltless"`
	Comment  string `json:"comment"`
	Done     bool   `json:"done"`
}

// LogSet records what the member actually did on a prescribed set: the RPE they
// perceived, and a comment for their coach. Only the member running the block
// may write one - a coach reads feedback, they don't author it.
func (h *TrainingHandler) LogSet(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	setID := chi.URLParam(r, "setId")

	assignment, err := h.programs.FindAssignment(r.Context(), chi.URLParam(r, "assignmentId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if assignment.UserID != callerID {
		writeError(w, http.StatusForbidden, "You can only log your own sets", CodeForbidden)
		return
	}

	var p logSetPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if p.ActualRPE != nil && (*p.ActualRPE <= 0 || *p.ActualRPE > 10) {
		writeError(w, http.StatusBadRequest, "An RPE must be between 1 and 10", CodeInvalidSet)
		return
	}
	if p.ActualReps != nil && *p.ActualReps < 0 {
		writeError(w, http.StatusBadRequest, "Reps cannot be negative", CodeInvalidSet)
		return
	}
	if p.ActualLoad != nil && *p.ActualLoad < 0 {
		writeError(w, http.StatusBadRequest, "A load cannot be negative", CodeInvalidSet)
		return
	}

	// The set must belong to the program this assignment is for, otherwise a
	// member could log against someone else's block.
	prescribed, err := h.programs.FindSet(r.Context(), setID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if prescribed.ProgramID != assignment.ProgramID {
		writeError(w, http.StatusBadRequest, "This set is not part of the assigned program", CodeInvalidSet)
		return
	}

	log, err := h.programs.LogSet(r.Context(), assignment.ID, setID, callerID, store.SetLogFields{
		ActualReps: p.ActualReps, ActualRPE: p.ActualRPE, ActualLoad: p.ActualLoad,
		Beltless: p.Beltless, Comment: p.Comment, Done: p.Done,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	// Echo the estimated max the performance demonstrates, so the member sees
	// straight away whether the set beat the 1RM it was prescribed from.
	//
	// Resolved through the same code the session screen uses rather than
	// computed here: the load a member who logged only an RPE is credited with
	// is the one they were prescribed, and working that out twice is how the
	// two answers come to differ.
	if resolved, _, err := h.sets.resolveSets(r.Context(), []models.ProgramSet{prescribed},
		assignment.UserID, map[string]models.SetLog{setID: log}); err == nil && resolved[0].Log != nil {
		log = *resolved[0].Log
	}
	writeJSON(w, http.StatusOK, log)
}

type dayDonePayload struct {
	Done bool `json:"done"`
}

// SetDayDone ticks off a whole session, or puts it back.
//
// The same thing the per-set button does, for every set at once: what a member
// actually does in the gym is finish a session, and tapping twelve buttons to
// say so is how a log stops being kept at all.
func (h *TrainingHandler) SetDayDone(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	dayID := chi.URLParam(r, "dayId")

	assignment, err := h.programs.FindAssignment(r.Context(), chi.URLParam(r, "assignmentId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if assignment.UserID != callerID {
		writeError(w, http.StatusForbidden, "You can only log your own sets", CodeForbidden)
		return
	}

	var p dayDonePayload
	if !decodeJSON(w, r, &p) {
		return
	}

	// The session has to belong to the program this assignment is for -
	// otherwise a member could tick off somebody else's block.
	day, err := h.programs.FindDay(r.Context(), dayID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if day.ProgramID != assignment.ProgramID {
		writeError(w, http.StatusBadRequest, "This session is not part of the assigned program", CodeInvalidSet)
		return
	}

	count, err := h.programs.SetDayDone(r.Context(), assignment.ID, dayID, callerID, p.Done)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dayId": dayID, "done": p.Done, "sets": count})
}

// DeleteLog clears the member's feedback on a set.
func (h *TrainingHandler) DeleteLog(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	assignmentID := chi.URLParam(r, "assignmentId")
	setID := chi.URLParam(r, "setId")

	assignment, err := h.programs.FindAssignment(r.Context(), assignmentID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if assignment.UserID != callerID {
		writeError(w, http.StatusForbidden, "You can only clear your own sets", CodeForbidden)
		return
	}
	if err := h.programs.DeleteSetLog(r.Context(), assignmentID, setID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"setId": setID})
}

type assignmentStatusPayload struct {
	Status string `json:"status"`
}

// SetStatus marks a block active, finished or archived.
func (h *TrainingHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	assignment, ok := h.authorizeAssignment(w, r)
	if !ok {
		return
	}

	var p assignmentStatusPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !models.IsValidAssignmentStatus(p.Status) {
		writeError(w, http.StatusBadRequest, "Invalid status", CodeInvalidStatus)
		return
	}

	updated, err := h.programs.SetAssignmentStatus(r.Context(), assignment.ID, p.Status)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
