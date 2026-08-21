package handlers

import (
	"context"
	"errors"
	"fmt"
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

	program, ok := h.authorizeProgram(w, r)
	if !ok {
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
	mainLiftsByID := make(map[string]models.Exercise, len(mainLifts))
	for _, lift := range mainLifts {
		mainLiftsByID[lift.ID] = lift
	}

	missingIDs := map[string]bool{}
	resolved := make([]models.ProgramSet, len(sets))

	// What this session has shown each lift to be worth (see autoregulate.go).
	session := newSessionMaxes()

	for i, set := range sets {
		session.enter(set.DayID)
		oneRM, sourceID := memberOneRM(set, maxes, mainLifts)
		// Projected, never stored: when the load came off a lift matched from
		// the name, the set has to say so, or the screen credits the max to
		// the movement itself.
		if utils.IsBlank(set.ExerciseOneRMRef) && sourceID != set.ExerciseID {
			if lift, ok := mainLiftsByID[sourceID]; ok {
				// The category, not the slug: a reference names the lift's
				// family, which is what FindMainLifts keys on and what a
				// catalog entry stores.
				set.ExerciseOneRMRef = lift.Category
			}
		}
		if shown, ok := session.forLift(sourceID); ok {
			oneRM = &shown
			set.Autoregulated = true
		}

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
			// An estimate needs the member to have said something about how
			// the set went: the RPE it came out at, or the weight they
			// actually used. Ticking a set off says neither, and an e1RM
			// computed from the prescription alone would be quoting a max
			// nobody demonstrated - E1RM reads a missing RPE as an all-out
			// effort.
			if log.ActualRPE != nil || log.ActualLoad != nil {
				// The weight they lifted, falling back to the weight they were
				// told to lift: somebody who only picks the RPE has said
				// enough, and asking them to retype the number already on
				// screen would be asking twice.
				load := set.Load
				if log.ActualLoad != nil {
					load = *log.ActualLoad
				}
				reps := set.Reps
				if log.ActualReps != nil {
					reps = *log.ActualReps
				}
				log.E1RM = loadcalc.E1RM(load, reps, log.ActualRPE)
			}
			set.Log = &log
			session.record(sourceID, log)
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

	ref := set.ExerciseOneRMRef
	if utils.IsBlank(ref) {
		// Nothing on the catalog entry says which lift this is a variation of,
		// so read it out of the name: a highbar squat is loaded off the squat,
		// a paused deadlift off the deadlift (see models.LiftRules). This is
		// the last resort - a max recorded for the movement itself and a
		// reference a coach set both win over it, above.
		ref = models.MatchOneRMRef(set.ExerciseSlug, set.ExerciseLabels["en"], set.ExerciseLabels["fr"])
	}

	if utils.IsNotBlank(ref) {
		if lift, ok := mainLifts[ref]; ok {
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
	// Visibility is optional on create (a new program is club-only) and
	// authoritative on update, which is how a coach publishes or unpublishes
	// one. Anything unrecognized normalizes to club-only rather than being
	// rejected, so a client that doesn't know about sharing can keep sending
	// the payload it always sent without silently making a block public.
	Visibility string `json:"visibility"`
}

// Create opens an empty program for a coach to build session by session, which
// is the alternative to importing a spreadsheet. It starts with no sessions: a
// program's shape comes from the sessions added to it, not from a week count
// declared up front.
// ListMine returns the programs the caller wrote for themselves.
//
// Their own only, and only the club-less ones: a coach's club programs are
// listed under their club, where the people who may read them look for them.
func (h *ProgramHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	programs, err := h.programs.ListForAuthor(r.Context(), callerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, programs)
}

func (h *ProgramHandler) Create(w http.ResponseWriter, r *http.Request) {
	authorID, _ := middleware.UserIDFromContext(r.Context())

	var p programMetaPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if utils.IsBlank(p.Name) {
		writeError(w, http.StatusBadRequest, "Please add a name", CodeNameRequired)
		return
	}

	clubID := chi.URLParam(r, "clubId")
	visibility := p.Visibility
	// A program somebody writes for themselves starts private. The default
	// everywhere else is club-only, which for a program with no club means
	// "everybody I train with" - a wider audience than an athlete jotting
	// down their own block is asking for.
	if utils.IsBlank(clubID) && utils.IsBlank(visibility) {
		visibility = models.ProgramVisibilityPrivate
	}

	program, err := h.programs.Create(r.Context(), store.NewProgram{
		ClubID: clubID, AuthorID: authorID,
		Name: p.Name, Description: p.Description, Visibility: visibility,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, program)
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

	program, err := h.programs.UpdateMeta(r.Context(), chi.URLParam(r, "programId"),
		p.Name, p.Description, p.Visibility)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, program)
}

// GetPublic serves a program that has been shared publicly, to a caller who
// may well be anonymous.
//
// It is deliberately not Get with the membership check skipped: there is no
// member to resolve loads against, so every set comes back as the coach
// authored it - reps, RPE and percentage - with no weights, no 1RM and no
// logs. A visitor sees the prescription; what anybody actually lifted stays
// inside the club.
func (h *ProgramHandler) GetPublic(w http.ResponseWriter, r *http.Request) {
	programID := chi.URLParam(r, "programId")

	program, err := h.programs.FindPublicByID(r.Context(), programID)
	if err != nil {
		// A private program is reported as missing rather than forbidden: the
		// difference would confirm the id exists to somebody guessing.
		writeError(w, http.StatusNotFound, "Program not found", CodeNotFound)
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

	byDay := map[string][]models.ProgramSet{}
	for _, set := range sets {
		byDay[set.DayID] = append(byDay[set.DayID], set)
	}
	for i := range days {
		days[i].Sets = byDay[days[i].ID]
	}

	// program carries its club's name already (see programSelect), which is the
	// provenance a reader outside the club wants without implying membership.
	writeJSON(w, http.StatusOK, map[string]any{
		"program": program,
		"days":    days,
	})
}

func (h *ProgramHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "programId")
	if err := h.programs.Delete(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

// --- sessions ---

type dayPayload struct {
	Week  int    `json:"week"`
	Day   int    `json:"day"`
	Title string `json:"title"`
}

// AddDay appends a session to a program. Week and day are optional: left out,
// they continue the program's existing numbering, which is what a coach adding
// sessions one after another wants.
func (h *ProgramHandler) AddDay(w http.ResponseWriter, r *http.Request) {
	programID := chi.URLParam(r, "programId")

	program, ok := h.authorizeProgram(w, r)
	if !ok {
		return
	}

	var p dayPayload
	if !decodeJSON(w, r, &p) {
		return
	}

	week, day, position, err := h.programs.NextDayNumber(r.Context(), program.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if p.Week > 0 {
		week = p.Week
	}
	if p.Day > 0 {
		day = p.Day
	}

	created, err := h.programs.AddDay(r.Context(), programID, store.NewDay{
		Week: week, Day: day, Title: dayTitle(p.Title, week, day), Position: position,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// UpdateDay renumbers or renames a session.
func (h *ProgramHandler) UpdateDay(w http.ResponseWriter, r *http.Request) {
	day, ok := h.dayOfProgram(w, r)
	if !ok {
		return
	}

	var p dayPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if p.Week <= 0 || p.Day <= 0 {
		writeError(w, http.StatusBadRequest, "A session needs a week and a day number", CodeInvalidSet)
		return
	}

	updated, err := h.programs.UpdateDay(r.Context(), day.ID, store.NewDay{
		Week: p.Week, Day: p.Day, Title: dayTitle(p.Title, p.Week, p.Day), Position: day.Position,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteDay removes a session and the sets in it.
func (h *ProgramHandler) DeleteDay(w http.ResponseWriter, r *http.Request) {
	day, ok := h.dayOfProgram(w, r)
	if !ok {
		return
	}
	if err := h.programs.DeleteDay(r.Context(), day.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": day.ID})
}

// dayTitle falls back to the same "Week n Day m" shape the importer generates,
// so a hand-built session reads like an imported one.
func dayTitle(title string, week, day int) string {
	if utils.IsNotBlank(title) {
		return title
	}
	return fmt.Sprintf("Week %d Day %d", week, day)
}

// authorizeProgram loads the addressed program and decides whether this caller
// may work on it. It is the one place that answers that question, so the two
// route groups cannot drift apart on it.
//
// Through a club's path, the club in the URL is what the membership middleware
// authorized: a program belonging to a different club must not be reachable
// there, whoever asks. Through the personal path there is no club to check, so
// the rule is authorship - a member writing their own block - plus a
// superadmin, who moderates everything.
func (h *ProgramHandler) authorizeProgram(w http.ResponseWriter, r *http.Request) (models.Program, bool) {
	program, err := h.programs.FindByID(r.Context(), chi.URLParam(r, "programId"))
	if err != nil {
		writeStoreError(w, err)
		return models.Program{}, false
	}

	// Not found rather than forbidden throughout: whether a program id exists
	// is not something a caller who may not read it should learn.
	if clubID := chi.URLParam(r, "clubId"); utils.IsNotBlank(clubID) {
		if program.ClubID != clubID {
			writeError(w, http.StatusNotFound, "Program not found", CodeNotFound)
			return models.Program{}, false
		}
		return program, true
	}

	callerID, _ := middleware.UserIDFromContext(r.Context())
	if program.AuthorID != callerID && !h.isSuperadmin(r, callerID) {
		writeError(w, http.StatusNotFound, "Program not found", CodeNotFound)
		return models.Program{}, false
	}
	return program, true
}

// isSuperadmin reports whether the caller moderates everything.
func (h *ProgramHandler) isSuperadmin(r *http.Request, callerID string) bool {
	if utils.IsBlank(callerID) {
		return false
	}
	user, err := h.users.FindByID(r.Context(), callerID)
	return err == nil && user.Role == models.GlobalRoleSuperadmin
}

// dayOfProgram loads the addressed session, checking the same way.
func (h *ProgramHandler) dayOfProgram(w http.ResponseWriter, r *http.Request) (models.ProgramDay, bool) {
	program, ok := h.authorizeProgram(w, r)
	if !ok {
		return models.ProgramDay{}, false
	}

	day, err := h.programs.FindDay(r.Context(), chi.URLParam(r, "dayId"))
	if err != nil {
		writeStoreError(w, err)
		return models.ProgramDay{}, false
	}
	if day.ProgramID != program.ID {
		writeError(w, http.StatusNotFound, "Session not found", CodeNotFound)
		return models.ProgramDay{}, false
	}
	return day, true
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

	if _, ok := h.dayOfProgram(w, r); !ok {
		return
	}

	fields := p.fields()
	if fields.Position <= 0 {
		var err error
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
	// UserIDs are everybody the program is being handed to. A coach starting a
	// block runs it with a group, not with one person at a time.
	UserIDs []string `json:"userIds"`
	// UserID is the singular form an older client sends; folded into UserIDs.
	UserID    string `json:"userId"`
	StartDate string `json:"startDate"`
	Note      string `json:"note"`
}

// targets folds the two forms into one list, without duplicates.
func (p assignPayload) targets() []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(p.UserIDs)+1)
	for _, id := range append(append([]string{}, p.UserIDs...), p.UserID) {
		if utils.IsBlank(id) || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// Assign hands a program to a member of the club.
func (h *ProgramHandler) Assign(w http.ResponseWriter, r *http.Request) {
	programID := chi.URLParam(r, "programId")
	clubID := chi.URLParam(r, "clubId")

	var p assignPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	members := p.targets()
	if len(members) == 0 {
		writeError(w, http.StatusBadRequest, "Please add a user id", CodeAllFieldsRequired)
		return
	}
	// Everybody is checked before anybody is written: half a group assigned and
	// then a refusal would leave a coach to work out who got it and who did not.
	callerID, _ := middleware.UserIDFromContext(r.Context())
	for _, userID := range members {
		// A program of somebody's own has no club to be a member of, so the
		// rule there is simply that it is theirs: an athlete running their own
		// block, or a coach following the one they wrote for themselves.
		if utils.IsBlank(clubID) {
			if userID != callerID {
				writeError(w, http.StatusForbidden,
					"A personal program can only be assigned to yourself", CodeForbidden)
				return
			}
			continue
		}
		// A club's program goes to a member of that club - which includes the
		// coach assigning it, since coaching a club means belonging to it.
		// Nothing here excludes the caller: a coach running their own block is
		// the ordinary case, not a special one.
		if _, err := h.clubs.FindMembership(r.Context(), clubID, userID); err != nil {
			writeError(w, http.StatusBadRequest, "This user is not a member of the club", CodeNotAClubMember)
			return
		}
	}

	assignments := make([]models.ProgramAssignment, 0, len(members))
	for _, userID := range members {
		assignment, err := h.programs.Assign(r.Context(), programID, userID, p.StartDate, p.Note)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		h.notifyAssignment(r, assignment)
		assignments = append(assignments, assignment)
	}

	// A list, always - even for one - so a client never has to branch on how
	// many it asked for.
	writeJSON(w, http.StatusCreated, assignments)
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
