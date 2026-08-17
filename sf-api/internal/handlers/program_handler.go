package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/email"
	"strong-fish-api/internal/loadcalc"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

type ProgramHandler struct {
	programs       *store.ProgramStore
	clubs          *store.ClubStore
	exercises      *store.ExerciseStore
	oneRMs         *store.OneRMStore
	users          *store.UserStore
	mailer         *email.Sender
	maxUploadSize  int64
	plateIncrement float64
	uiBaseURL      string
}

func NewProgramHandler(programs *store.ProgramStore, clubs *store.ClubStore, exercises *store.ExerciseStore,
	oneRMs *store.OneRMStore, users *store.UserStore, mailer *email.Sender,
	maxUploadSize int64, plateIncrement float64, uiBaseURL string) *ProgramHandler {
	return &ProgramHandler{
		programs: programs, clubs: clubs, exercises: exercises, oneRMs: oneRMs, users: users,
		mailer: mailer, maxUploadSize: maxUploadSize, plateIncrement: plateIncrement, uiBaseURL: uiBaseURL,
	}
}

// List returns a club's programs.
func (h *ProgramHandler) List(w http.ResponseWriter, r *http.Request) {
	programs, err := h.programs.ListForClub(r.Context(), chi.URLParam(r, "clubId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, programs)
}

// programResponse is a program with its sessions, each session's sets resolved
// against one member's 1RMs.
type programResponse struct {
	models.Program
	Days []models.ProgramDay `json:"days"`
	// ResolvedFor is the member the loads were computed for. A coach browsing
	// their own program sees it resolved against their own maxes, which is what
	// makes the numbers concrete while writing it.
	ResolvedFor string `json:"resolvedFor"`
	// MissingOneRMs lists the exercises whose 1RM the member hasn't recorded,
	// so the UI can prompt for exactly those rather than showing bare "?"s.
	MissingOneRMs []models.Exercise `json:"missingOneRms"`
	// Assignment is the member's own assignment of this program, when they have
	// one - its id is what logging a set needs.
	Assignment *models.ProgramAssignment `json:"assignment,omitempty"`
}

// Get returns a whole program, resolved for the caller (or, with ?memberId=,
// for one of the club's members - how a coach reviews someone's block).
func (h *ProgramHandler) Get(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())
	programID := chi.URLParam(r, "programId")

	program, err := h.programs.FindByID(r.Context(), programID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if program.ClubID != chi.URLParam(r, "clubId") {
		writeError(w, http.StatusNotFound, "Program not found", CodeNotFound)
		return
	}

	memberID, ok := h.resolveMember(w, r, callerID)
	if !ok {
		return
	}

	days, err := h.programs.ListDays(r.Context(), programID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	sets, err := h.programs.ListSetsForProgram(r.Context(), programID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	assignment, assignmentErr := h.programs.FindAssignmentFor(r.Context(), programID, memberID)
	logs := map[string]models.SetLog{}
	if assignmentErr == nil {
		if logs, err = h.programs.ListLogsForAssignment(r.Context(), assignment.ID); err != nil {
			writeStoreError(w, err)
			return
		}
	} else if !errors.Is(assignmentErr, store.ErrNotFound) {
		writeStoreError(w, assignmentErr)
		return
	}

	resolved, missing, err := h.resolveSets(r.Context(), sets, memberID, logs)
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

	response := programResponse{Program: program, Days: days, ResolvedFor: memberID, MissingOneRMs: missing}
	if assignmentErr == nil {
		response.Assignment = &assignment
	}
	writeJSON(w, http.StatusOK, response)
}

// resolveMember decides whose 1RMs a program is rendered against: the caller by
// default, or ?memberId= for a coach reviewing someone else's block. Only a club
// manager may look at another member's numbers.
func (h *ProgramHandler) resolveMember(w http.ResponseWriter, r *http.Request, callerID string) (string, bool) {
	memberID := r.URL.Query().Get("memberId")
	if utils.IsBlank(memberID) || memberID == callerID {
		return callerID, true
	}

	role, _ := middleware.ClubRoleFromContext(r.Context())
	if !models.CanManageClub(role) {
		writeError(w, http.StatusForbidden, "Only a coach can view another member's program", CodeForbidden)
		return utils.EMPTY, false
	}
	if _, err := h.clubs.FindMembership(r.Context(), chi.URLParam(r, "clubId"), memberID); err != nil {
		writeError(w, http.StatusBadRequest, "This user is not a member of the club", CodeNotAClubMember)
		return utils.EMPTY, false
	}
	return memberID, true
}

// resolveSets computes each prescribed set's load against one member's current
// 1RMs. This is where "update your 1RM and the whole program recalculates"
// actually happens: nothing derived is stored, so every read reflects whatever
// the member last recorded.
//
// It also returns the exercises whose max is missing, so the client can prompt
// for exactly the ones that would unlock a load rather than for all of them.
func (h *ProgramHandler) resolveSets(ctx context.Context, sets []models.ProgramSet, memberID string,
	logs map[string]models.SetLog) ([]models.ProgramSet, []models.Exercise, error) {
	maxes, err := h.oneRMs.MapForUser(ctx, memberID)
	if err != nil {
		return nil, nil, err
	}
	mainLifts, err := h.exercises.FindMainLifts(ctx)
	if err != nil {
		return nil, nil, err
	}

	missingIDs := map[string]bool{}
	resolved := make([]models.ProgramSet, len(sets))

	for i, set := range sets {
		oneRM, sourceID := memberOneRM(set, maxes, mainLifts)

		result := loadcalc.Load(loadcalc.Prescription{
			Mode: set.LoadMode, Reps: set.Reps, RPE: set.RPE,
			Percentage: set.Percentage, AbsoluteLoad: set.AbsoluteLoad,
		}, oneRM, h.plateIncrement)

		set.Load = result.Load
		set.RoundedLoad = result.RoundedLoad
		set.ComputedPercentage = result.Percentage
		set.LoadKnown = result.Known
		if oneRM != nil {
			set.OneRM = *oneRM
			// For an RPE set this comes back out as the member's own max, which
			// is the self-consistency the source spreadsheet lacked.
			set.TargetE1RM = loadcalc.E1RM(result.Load, set.Reps, set.RPE)
		}
		if !result.Known && utils.IsNotBlank(sourceID) {
			missingIDs[sourceID] = true
		}

		if log, ok := logs[set.ID]; ok {
			if log.ActualLoad != nil {
				reps := set.Reps
				if log.ActualReps != nil {
					reps = *log.ActualReps
				}
				log.E1RM = loadcalc.E1RM(*log.ActualLoad, reps, log.ActualRPE)
			}
			set.Log = &log
		}
		resolved[i] = set
	}

	missing, err := h.loadMissing(ctx, missingIDs)
	return resolved, missing, err
}

// memberOneRM picks which max loads a set, and returns the exercise id that max
// would have to be recorded against.
//
// A member's own max for the exact movement wins when they've recorded one - an
// athlete who has actually tested their paused deadlift should be loaded off
// that rather than off their competition pull. Otherwise the movement's
// reference lift is used, which is how the source spreadsheet programs a Larsen
// press off the bench max.
func memberOneRM(set models.ProgramSet, maxes map[string]float64, mainLifts map[string]models.Exercise) (*float64, string) {
	switch set.LoadMode {
	case loadcalc.ModeAbsolute, loadcalc.ModeBodyweight:
		// Neither is tied to a max, so nothing is missing when none is recorded.
		return nil, utils.EMPTY
	}

	if value, ok := maxes[set.ExerciseID]; ok && value > 0 {
		return &value, set.ExerciseID
	}

	if utils.IsNotBlank(set.ExerciseOneRMRef) {
		if lift, ok := mainLifts[set.ExerciseOneRMRef]; ok {
			if value, recorded := maxes[lift.ID]; recorded && value > 0 {
				return &value, lift.ID
			}
			return nil, lift.ID
		}
	}
	return nil, set.ExerciseID
}

// loadMissing turns the set of unresolved exercise ids into the catalog entries
// the client needs to prompt for.
func (h *ProgramHandler) loadMissing(ctx context.Context, ids map[string]bool) ([]models.Exercise, error) {
	if len(ids) == 0 {
		return []models.Exercise{}, nil
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}

	byID, err := h.exercises.ListByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	missing := make([]models.Exercise, 0, len(byID))
	for _, exercise := range byID {
		missing = append(missing, exercise)
	}
	return missing, nil
}

type programMetaPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *ProgramHandler) Update(w http.ResponseWriter, r *http.Request) {
	var p programMetaPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Name) {
		writeError(w, http.StatusBadRequest, "Please add a name", CodeNameRequired)
		return
	}

	program, err := h.programs.UpdateMeta(r.Context(), chi.URLParam(r, "programId"), p.Name, p.Description)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, program)
}

func (h *ProgramHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "programId")
	if err := h.programs.Delete(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

// --- sets ---

type setPayload struct {
	ExerciseID   string   `json:"exerciseId"`
	Reps         int      `json:"reps"`
	RPE          *float64 `json:"rpe"`
	Percentage   *float64 `json:"percentage"`
	AbsoluteLoad *float64 `json:"absoluteLoad"`
	LoadMode     string   `json:"loadMode"`
	Part         int      `json:"part"`
	Notes        string   `json:"notes"`
	Position     int      `json:"position"`
}

// validate checks a hand-edited set carries whatever its load mode needs. The
// modes are deliberately explicit rather than inferred here: a coach editing a
// set is stating how it should be loaded, unlike the importer, which has to
// guess from a spreadsheet's columns.
func (p setPayload) validate() (string, bool) {
	if utils.IsBlank(p.ExerciseID) {
		return CodeInvalidSet, false
	}
	if p.Reps <= 0 {
		return CodeInvalidSet, false
	}
	if !loadcalc.IsValidMode(p.LoadMode) {
		return CodeInvalidLoadMode, false
	}
	if p.RPE != nil && (*p.RPE <= 0 || *p.RPE > 10) {
		return CodeInvalidSet, false
	}

	switch p.LoadMode {
	case loadcalc.ModeRPE:
		if p.RPE == nil {
			return CodeInvalidSet, false
		}
	case loadcalc.ModePercentage:
		if p.Percentage == nil || *p.Percentage <= 0 {
			return CodeInvalidSet, false
		}
	case loadcalc.ModeAbsolute:
		if p.AbsoluteLoad == nil || *p.AbsoluteLoad <= 0 {
			return CodeInvalidSet, false
		}
	}
	return utils.EMPTY, true
}

func (p setPayload) fields() store.NewSet {
	return store.NewSet{
		ExerciseID: p.ExerciseID, Position: p.Position, Reps: p.Reps, RPE: p.RPE,
		Percentage: p.Percentage, AbsoluteLoad: p.AbsoluteLoad, LoadMode: p.LoadMode,
		Part: p.Part, Notes: p.Notes,
	}
}

// AddSet appends a set to a session.
func (h *ProgramHandler) AddSet(w http.ResponseWriter, r *http.Request) {
	programID := chi.URLParam(r, "programId")
	dayID := chi.URLParam(r, "dayId")

	var p setPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if code, ok := p.validate(); !ok {
		writeError(w, http.StatusBadRequest, "This set is incomplete", code)
		return
	}

	day, err := h.programs.FindDay(r.Context(), dayID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if day.ProgramID != programID {
		writeError(w, http.StatusNotFound, "Session not found", CodeNotFound)
		return
	}

	fields := p.fields()
	if fields.Position <= 0 {
		if fields.Position, err = h.programs.NextSetPosition(r.Context(), dayID); err != nil {
			writeStoreError(w, err)
			return
		}
	}

	set, err := h.programs.AddSet(r.Context(), programID, dayID, p.ExerciseID, fields)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, set)
}

// UpdateSet edits one prescribed set.
func (h *ProgramHandler) UpdateSet(w http.ResponseWriter, r *http.Request) {
	setID := chi.URLParam(r, "setId")

	var p setPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if code, ok := p.validate(); !ok {
		writeError(w, http.StatusBadRequest, "This set is incomplete", code)
		return
	}

	existing, err := h.programs.FindSet(r.Context(), setID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if existing.ProgramID != chi.URLParam(r, "programId") {
		writeError(w, http.StatusNotFound, "Set not found", CodeNotFound)
		return
	}

	fields := p.fields()
	if fields.Position <= 0 {
		fields.Position = existing.Position
	}

	set, err := h.programs.UpdateSet(r.Context(), setID, p.ExerciseID, fields)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, set)
}

func (h *ProgramHandler) DeleteSet(w http.ResponseWriter, r *http.Request) {
	setID := chi.URLParam(r, "setId")

	existing, err := h.programs.FindSet(r.Context(), setID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if existing.ProgramID != chi.URLParam(r, "programId") {
		writeError(w, http.StatusNotFound, "Set not found", CodeNotFound)
		return
	}
	if err := h.programs.DeleteSet(r.Context(), setID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": setID})
}

// --- assignments ---

type assignPayload struct {
	UserID    string `json:"userId"`
	StartDate string `json:"startDate"`
	Note      string `json:"note"`
}

// Assign hands a program to a member of the club.
func (h *ProgramHandler) Assign(w http.ResponseWriter, r *http.Request) {
	programID := chi.URLParam(r, "programId")
	clubID := chi.URLParam(r, "clubId")

	var p assignPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.UserID) {
		writeError(w, http.StatusBadRequest, "Please add a user id", CodeAllFieldsRequired)
		return
	}
	if _, err := h.clubs.FindMembership(r.Context(), clubID, p.UserID); err != nil {
		writeError(w, http.StatusBadRequest, "This user is not a member of the club", CodeNotAClubMember)
		return
	}

	assignment, err := h.programs.Assign(r.Context(), programID, p.UserID, p.StartDate, p.Note)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	h.notifyAssignment(r, assignment)
	writeJSON(w, http.StatusCreated, assignment)
}

// notifyAssignment emails the member (best-effort).
func (h *ProgramHandler) notifyAssignment(r *http.Request, assignment models.ProgramAssignment) {
	member, err := h.users.FindByID(r.Context(), assignment.UserID)
	if err != nil {
		return
	}
	coachID, _ := middleware.UserIDFromContext(r.Context())
	coach, err := h.users.FindByID(r.Context(), coachID)
	if err != nil {
		return
	}
	h.mailer.SendProgramAssigned(r.Context(), member.Email, localeOf(member, r),
		assignment.ProgramName, coach.Name+" "+coach.Surname,
		h.uiBaseURL+"/dashboard/training/"+assignment.ID)
}

// ListAssignments returns everyone running a program, for its coach.
func (h *ProgramHandler) ListAssignments(w http.ResponseWriter, r *http.Request) {
	assignments, err := h.programs.ListAssignmentsForProgram(r.Context(), chi.URLParam(r, "programId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, assignments)
}

func (h *ProgramHandler) Unassign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "assignmentId")
	if err := h.programs.Unassign(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

// ListFeedback is the coach's inbox: every set one of the club's members left a
// perceived RPE or a comment on.
func (h *ProgramHandler) ListFeedback(w http.ResponseWriter, r *http.Request) {
	page, size := pagination(r)
	feedback, total, err := h.programs.ListFeedbackForClub(r.Context(), chi.URLParam(r, "clubId"), page, size)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, models.Page[models.SetFeedback]{Results: feedback, TotalResults: total})
}
