package models

import "time"

// Profile visibility levels, from widest to narrowest.
const (
	// ProfileVisibilityPublic is readable by anybody, signed in or not - which
	// is what makes a shared profile link work.
	ProfileVisibilityPublic = "public"
	// ProfileVisibilityClubs is readable by the members of the clubs its owner
	// belongs to. Unlike "public" this cannot be evaluated for an anonymous
	// visitor: knowing whether somebody shares a club with the owner requires
	// knowing who they are, so a logged-out reader is refused.
	ProfileVisibilityClubs = "clubs"
	// ProfileVisibilityPrivate is readable by a superadmin, and by the owner or
	// an admin of a club its owner belongs to - their coach, in other words -
	// and by nobody else.
	ProfileVisibilityPrivate = "private"
)

// IsValidProfileVisibility reports whether visibility is one this app knows.
func IsValidProfileVisibility(visibility string) bool {
	switch visibility {
	case ProfileVisibilityPublic, ProfileVisibilityClubs, ProfileVisibilityPrivate:
		return true
	}
	return false
}

// NormalizeProfileVisibility maps anything unrecognized - including the empty
// string an account written before this existed carries - onto the narrowest
// level. An unknown value must never widen an audience.
func NormalizeProfileVisibility(visibility string) string {
	if IsValidProfileVisibility(visibility) {
		return visibility
	}
	return ProfileVisibilityPrivate
}

// ViewerRelation is what one caller is to one profile's owner, which is all the
// visibility rules need to know. It's resolved in a single query (see
// ClubStore.RelationTo) rather than by walking both people's club lists.
type ViewerRelation struct {
	// Self is true when the caller is the profile's owner. Nobody is ever
	// hidden from themselves.
	Self bool
	// Superadmin can read every profile: they moderate them.
	Superadmin bool
	// SharesClub is true when caller and owner belong to at least one club in
	// common.
	SharesClub bool
	// ManagesClub is true when the caller owns or administers a club the
	// profile's owner belongs to - their coach.
	ManagesClub bool
}

// CanSeeProfile applies the visibility rules.
func CanSeeProfile(visibility string, relation ViewerRelation) bool {
	if relation.Self || relation.Superadmin {
		return true
	}
	switch NormalizeProfileVisibility(visibility) {
	case ProfileVisibilityPublic:
		return true
	case ProfileVisibilityClubs:
		return relation.SharesClub
	default:
		return relation.ManagesClub
	}
}

// Birthday is the calendar entry a filled-in birthdate produces. It is derived
// on read rather than stored as a row in the events table: it has no author, it
// cannot be edited, and it has to disappear the moment its owner clears the
// date or narrows their profile.
type Birthday struct {
	UserID  string
	Handle  string
	Name    string
	Surname string
	// Date is the birthdate itself; Next is the occurrence being shown.
	Date time.Time
	Next time.Time
}

// NextOccurrence returns the first anniversary of birthdate falling on or after
// from.
//
// The 29th of February is deliberately left to Go's own date normalization,
// which rolls it to the 1st of March in a common year - the alternative is
// hiding the birthday entirely in three years out of four.
func NextOccurrence(birthdate, from time.Time) time.Time {
	year := from.Year()
	next := time.Date(year, birthdate.Month(), birthdate.Day(), 0, 0, 0, 0, time.UTC)
	if next.Before(time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)) {
		next = time.Date(year+1, birthdate.Month(), birthdate.Day(), 0, 0, 0, 0, time.UTC)
	}
	return next
}
