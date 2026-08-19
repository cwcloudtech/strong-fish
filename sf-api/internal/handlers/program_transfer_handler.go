package handlers

import (
	"net/http"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// copyProgramPayload names where a program is going, and whether the original
// stays behind.
type copyProgramPayload struct {
	// ClubID is the destination. Blank moves the program out of every club and
	// into the caller's own library, which is how a coach turns a club block
	// into one of their own.
	ClubID string `json:"clubId"`
	// Move transfers rather than duplicates. Copying is the default because it
	// cannot lose anything: a transfer takes the program away from everybody
	// who was reading it in the club it came from.
	Move bool `json:"move"`
}

// CopyToClub duplicates a program into another club, into the caller's own
// library, or moves it.
//
// Two things at once, because they are the same operation with a different
// destination. A coach moves a block that worked for one club to another
// rather than retyping six weeks of sessions. A member copies a club's program
// into their own library so they can adapt it - which is why this is not
// gated on coaching: reaching the source through a club's path already means
// being in that club, and what they make is theirs alone.
//
// The copy is a real one - its own days and sets - so editing it afterwards
// leaves the original alone, which is the whole point of copying rather than
// sharing.
//
// Assignments and logged sets are deliberately not carried over: they belong to
// the members who did the work, not to the program's text.
func (h *ProgramHandler) CopyToClub(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	program, ok := h.authorizeProgram(w, r)
	if !ok {
		return
	}

	var p copyProgramPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if p.ClubID == program.ClubID {
		writeError(w, http.StatusBadRequest, "This program is already in that club", CodeInvalidRequestBody)
		return
	}

	// Checked against the *destination*, which nothing else has authorized:
	// reaching the source already required being able to read it. Landing it
	// in a club needs a coach there; landing it in your own library needs
	// nothing more, since nobody else can see it.
	if !h.canWriteInClub(r, callerID, p.ClubID) {
		writeError(w, http.StatusForbidden, "You cannot add programs to that club", CodeForbidden)
		return
	}

	// Moving is a coaching decision about the source, not just the
	// destination: it takes the program away from everybody reading it where
	// it is now. Copying is what a member does.
	if p.Move && !h.canWriteInClub(r, callerID, program.ClubID) {
		writeError(w, http.StatusForbidden, "You cannot move this program", CodeForbidden)
		return
	}

	if p.Move {
		moved, err := h.programs.SetClub(r.Context(), program.ID, p.ClubID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, moved)
		return
	}

	copied, err := h.duplicate(r, program, p.ClubID, callerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, copied)
}

// canWriteInClub reports whether the caller may add a program to a club.
//
// A blank club is the caller's own library, which is always theirs to write in.
func (h *ProgramHandler) canWriteInClub(r *http.Request, callerID, clubID string) bool {
	if utils.IsBlank(callerID) {
		return false
	}
	if utils.IsBlank(clubID) {
		return true
	}
	if h.isSuperadmin(r, callerID) {
		return true
	}

	membership, err := h.clubs.FindMembership(r.Context(), clubID, callerID)
	if err != nil {
		return false
	}
	return models.CanManageClub(membership.Role)
}

// duplicate writes a fresh program with the same sessions and sets.
//
// Read out and written back through the same NewProgram the importer uses, so
// a copy is built exactly like any other program rather than through a
// row-copying shortcut that would have to be kept in step with the schema.
func (h *ProgramHandler) duplicate(r *http.Request, program models.Program,
	clubID, authorID string) (models.Program, error) {
	days, err := h.programs.ListDays(r.Context(), program.ID)
	if err != nil {
		return models.Program{}, err
	}
	sets, err := h.programs.ListSetsForProgram(r.Context(), program.ID)
	if err != nil {
		return models.Program{}, err
	}

	setsByDay := make(map[string][]models.ProgramSet, len(days))
	for _, set := range sets {
		setsByDay[set.DayID] = append(setsByDay[set.DayID], set)
	}

	newDays := make([]store.NewDay, 0, len(days))
	for _, day := range days {
		newSets := make([]store.NewSet, 0, len(setsByDay[day.ID]))
		for _, set := range setsByDay[day.ID] {
			newSets = append(newSets, store.NewSet{
				ExerciseID: set.ExerciseID, Position: set.Position, Reps: set.Reps,
				RPE: set.RPE, Percentage: set.Percentage, AbsoluteLoad: set.AbsoluteLoad,
				LoadMode: set.LoadMode, Part: set.Part, Notes: set.Notes,
			})
		}
		newDays = append(newDays, store.NewDay{
			Week: day.Week, Day: day.Day, Title: day.Title, Position: day.Position, Sets: newSets,
		})
	}

	// The copy is the copier's, not the original author's: they are the one
	// who may now edit it, and the club it lands in should see who put it
	// there. Its visibility comes across as it was.
	return h.programs.Create(r.Context(), store.NewProgram{
		ClubID: clubID, AuthorID: authorID,
		Name: program.Name, Description: program.Description,
		SourceFileName: program.SourceFileName, Visibility: program.Visibility,
		Days: newDays,
	})
}
