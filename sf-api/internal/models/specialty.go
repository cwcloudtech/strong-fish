package models

// A lifter's specialty: the badge they wear on their own profile, picked by
// them and by nobody else.
//
// It is a claim, not a computation. Working it out from the three bests would
// be easy and wrong: a member who has not entered their maxes would carry no
// badge at all, and one coming back from a squat injury would be relabelled by
// the app the week their numbers dip. What a lifter calls themselves is theirs
// to say - including saying nothing, which is the empty string.
const (
	SpecialtyDeadlift = "deadlift"
	SpecialtySquat    = "squat"
	SpecialtyBench    = "bench"
	// SpecialtyTotal is the balanced totaler: no single lift carries them.
	SpecialtyTotal = "total"
)

// Specialties are the badges a member may pick, in the order they are offered.
// The three lifts come in competition order, and the totaler last as the
// "none of the three in particular" answer.
var Specialties = []string{SpecialtySquat, SpecialtyBench, SpecialtyDeadlift, SpecialtyTotal}

// IsValidSpecialty reports whether specialty is one this app knows.
func IsValidSpecialty(specialty string) bool {
	for _, known := range Specialties {
		if specialty == known {
			return true
		}
	}
	return false
}

// NormalizeSpecialty maps anything unrecognized onto none at all - which is
// what an account written before this existed carries, and what somebody
// clearing their badge sends. Unlike the visibility levels there is no privacy
// at stake here, so the fallback is simply "unsaid" rather than the narrowest
// option.
func NormalizeSpecialty(specialty string) string {
	if IsValidSpecialty(specialty) {
		return specialty
	}
	return ""
}
