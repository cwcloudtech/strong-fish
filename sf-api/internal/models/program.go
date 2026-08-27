package models

import "time"

// Program is a training block a coach uploaded into a club - typically an
// imported spreadsheet, one sheet per week.
type Program struct {
	ID string `json:"id"`
	// ClubID is empty for a program somebody wrote for themselves, which
	// belongs to no club and is governed by its visibility alone.
	ClubID      string `json:"clubId"`
	ClubName    string `json:"clubName,omitempty"`
	AuthorID    string `json:"authorId"`
	AuthorName  string `json:"authorName,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Weeks       int    `json:"weeks"`
	// SourceFileName is the uploaded spreadsheet's name, kept so a coach can
	// tell two imports of the same block apart.
	SourceFileName string `json:"sourceFileName,omitempty"`
	// Visibility is who may read the program: its author alone
	// (ProgramVisibilityPrivate), the club's members or the author's clubmates
	// (ProgramVisibilityClub, the default), or anybody holding the link
	// (ProgramVisibilityPublic). Sharing is always opted into - a program with
	// no stored visibility reads as club-only.
	Visibility string    `json:"visibility"`
	DayCount   int       `json:"dayCount"`
	SetCount   int       `json:"setCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Program visibilities, widest first.
const (
	// ProgramVisibilityPublic is readable by anybody holding the link.
	ProgramVisibilityPublic = "public"
	// ProgramVisibilityClub is readable by the members of the club it belongs
	// to - or, for a program of somebody's own, by the people they train with.
	ProgramVisibilityClub = "club"
	// ProgramVisibilityPrivate is readable by its author alone (and by a
	// superadmin, who moderates everything). This is what an athlete writing a
	// block for themselves gets by default.
	ProgramVisibilityPrivate = "private"
)

// IsValidProgramVisibility reports whether visibility is one this app knows.
func IsValidProgramVisibility(visibility string) bool {
	switch visibility {
	case ProgramVisibilityClub, ProgramVisibilityPublic, ProgramVisibilityPrivate:
		return true
	}
	return false
}

// NormalizeProgramVisibility maps anything unrecognized - including the empty
// string a program written before sharing existed carries - onto club-only, so
// an unknown value can never widen an audience.
func NormalizeProgramVisibility(visibility string) string {
	switch visibility {
	case ProgramVisibilityPublic:
		return ProgramVisibilityPublic
	case ProgramVisibilityPrivate:
		return ProgramVisibilityPrivate
	}
	return ProgramVisibilityClub
}

// ProgramDay is one training session - "WEEK 2 DAY 3" in the source
// spreadsheet.
type ProgramDay struct {
	ID        string `json:"id"`
	ProgramID string `json:"programId"`
	Week      int    `json:"week"`
	Day       int    `json:"day"`
	Title     string `json:"title"`
	Position  int    `json:"position"`
	// Sets is populated by the endpoints that return a whole session.
	Sets      []ProgramSet `json:"sets,omitempty"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// ProgramSet is one prescribed set. It deliberately stores no weight: the load
// is derived per member from their own current 1RM every time it's read (see
// package loadcalc), which is what makes "update your 1RM and the whole
// program recomputes" free.
type ProgramSet struct {
	ID         string `json:"id"`
	ProgramID  string `json:"programId"`
	DayID      string `json:"dayId"`
	ExerciseID string `json:"exerciseId"`
	// ExerciseSlug/ExerciseLabels/Bodyweight denormalize the exercise onto the
	// set so a session renders without a second lookup per row.
	ExerciseSlug   string            `json:"exerciseSlug,omitempty"`
	ExerciseLabels map[string]string `json:"exerciseLabels,omitempty"`
	Bodyweight     bool              `json:"bodyweight"`
	// ExerciseOneRMRef is which competition lift this movement's load resolves
	// against ("bench" for a Larsen press). The client shows it so a member can
	// see - and go and fix - the max a set's weight actually came from.
	ExerciseOneRMRef string `json:"exerciseOneRmRef,omitempty"`
	// WithBelt says this movement is one a lifter wears a belt for, which is
	// what puts a beltless switch on the row (see models.WithBelt).
	WithBelt bool `json:"withBelt"`
	Position int  `json:"position"`
	Reps     int  `json:"reps"`
	// RPE is the prescribed rate of perceived exertion. Nil is the
	// spreadsheet's "?": the coach left it open, and the set falls back to its
	// percentage.
	RPE *float64 `json:"rpe,omitempty"`
	// Percentage is the coach's authored share of the 1RM. It's the load
	// source for a set with no RPE, and is otherwise kept as-authored so the
	// UI can show it next to what the member's own 1RM implies.
	Percentage *float64 `json:"percentage,omitempty"`
	// AbsoluteLoad is a fixed weight in kg, for accessories not tied to a 1RM.
	AbsoluteLoad *float64 `json:"absoluteLoad,omitempty"`
	LoadMode     string   `json:"loadMode"`
	// Part is the optional grouping index the source spreadsheet uses to split
	// a session into blocks (main work, then accessories).
	Part  int    `json:"part,omitempty"`
	Notes string `json:"notes,omitempty"`

	// --- resolved per member, never stored ---

	// Load is the weight this member should lift, computed from their current
	// 1RM. Zero and LoadKnown false when they haven't recorded one yet.
	Load        float64 `json:"load"`
	RoundedLoad float64 `json:"roundedLoad"`
	// ComputedPercentage is the share of the member's 1RM Load represents -
	// for an RPE set, what the coach's authored integer was approximating.
	ComputedPercentage float64 `json:"computedPercentage"`
	LoadKnown          bool    `json:"loadKnown"`
	// OneRM is the max Load was computed from, echoed back so the UI can
	// explain the number and link to editing it.
	OneRM float64 `json:"oneRm,omitempty"`
	// TargetE1RM is the estimated 1RM the set demonstrates if performed exactly
	// as prescribed - equal to OneRM by construction for an RPE set, which is
	// the self-consistency the source spreadsheet lacked.
	TargetE1RM float64 `json:"targetE1rm,omitempty"`
	// Autoregulated says this set's load came from an e1RM the member
	// demonstrated earlier in the same session rather than from the 1RM on
	// file, so the screen can say where the number came from instead of the
	// weight appearing to change on its own.
	Autoregulated bool `json:"autoregulated,omitempty"`
	// Log is this member's feedback on the set, when they've logged it.
	Log *SetLog `json:"log,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ProgramAssignment is a program handed to one member.
type ProgramAssignment struct {
	ID          string `json:"id"`
	ProgramID   string `json:"programId"`
	UserID      string `json:"userId"`
	ProgramName string `json:"programName,omitempty"`
	ClubID      string `json:"clubId,omitempty"`
	ClubName    string `json:"clubName,omitempty"`
	MemberName  string `json:"memberName,omitempty"`
	MemberEmail string `json:"memberEmail,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
	Status      string `json:"status,omitempty"`
	Note        string `json:"note,omitempty"`
	// CompletedSets/TotalSets summarize progress for the assignment list.
	CompletedSets int       `json:"completedSets"`
	TotalSets     int       `json:"totalSets"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Assignment statuses.
const (
	AssignmentStatusActive   = "active"
	AssignmentStatusDone     = "done"
	AssignmentStatusArchived = "archived"
)

// IsValidAssignmentStatus reports whether status is a known assignment status.
func IsValidAssignmentStatus(status string) bool {
	switch status {
	case AssignmentStatusActive, AssignmentStatusDone, AssignmentStatusArchived:
		return true
	}
	return false
}

// SetLog is what the member actually did on a prescribed set: the RPE they
// perceived, and a comment for their coach.
type SetLog struct {
	ID           string   `json:"id"`
	AssignmentID string   `json:"assignmentId"`
	SetID        string   `json:"setId"`
	UserID       string   `json:"userId"`
	ActualReps   *int     `json:"actualReps,omitempty"`
	ActualRPE    *float64 `json:"actualRpe,omitempty"`
	ActualLoad   *float64 `json:"actualLoad,omitempty"`
	// Beltless says the member ran this set without their belt. Only asked
	// about on a movement that would normally be belted; absent means they
	// wore it, which is what a belt movement's default is.
	Beltless    bool       `json:"beltless,omitempty"`
	Comment     string     `json:"comment,omitempty"`
	Done        bool       `json:"done"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	// E1RM is the max this performance estimates, derived from the logged
	// load/reps/RPE - the spreadsheet's e1RM column, computed from what
	// actually happened rather than from what was prescribed.
	E1RM      float64   `json:"e1rm,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SetFeedback is one member's logged set joined with enough of its
// prescription to be read on its own, for the coach's feedback inbox.
type SetFeedback struct {
	SetLog
	MemberName     string            `json:"memberName"`
	MemberHandle   string            `json:"memberHandle,omitempty"`
	MemberPicture  string            `json:"memberPicture,omitempty"`
	ProgramID      string            `json:"programId"`
	ProgramName    string            `json:"programName"`
	Week           int               `json:"week"`
	Day            int               `json:"day"`
	ExerciseSlug   string            `json:"exerciseSlug"`
	Labels         map[string]string `json:"exerciseLabels"`
	PrescribedReps int               `json:"prescribedReps"`
	PrescribedRPE  *float64          `json:"prescribedRpe,omitempty"`
}
