package models

import (
	"testing"
	"time"
)

// TestCanSeeProfile is the whole access-control rule for a profile in one
// table. It is worth pinning because every widening here is a privacy
// regression that nothing else would catch.
func TestCanSeeProfile(t *testing.T) {
	anonymous := ViewerRelation{}
	stranger := ViewerRelation{}
	clubMate := ViewerRelation{SharesClub: true}
	coach := ViewerRelation{SharesClub: true, ManagesClub: true}
	owner := ViewerRelation{Self: true}
	admin := ViewerRelation{Superadmin: true}

	cases := []struct {
		name       string
		visibility string
		relation   ViewerRelation
		want       bool
	}{
		{"public is readable logged out", ProfileVisibilityPublic, anonymous, true},
		{"public is readable by a stranger", ProfileVisibilityPublic, stranger, true},

		{"clubs hides from anonymous", ProfileVisibilityClubs, anonymous, false},
		{"clubs hides from a stranger", ProfileVisibilityClubs, stranger, false},
		{"clubs shows to a club-mate", ProfileVisibilityClubs, clubMate, true},
		{"clubs shows to a coach", ProfileVisibilityClubs, coach, true},

		{"private hides from anonymous", ProfileVisibilityPrivate, anonymous, false},
		{"private hides from a club-mate", ProfileVisibilityPrivate, clubMate, false},
		{"private shows to a coach", ProfileVisibilityPrivate, coach, true},

		{"the owner always sees themselves", ProfileVisibilityPrivate, owner, true},
		{"a superadmin sees everything", ProfileVisibilityPrivate, admin, true},

		// An unrecognized value must fall to the narrowest level, never the
		// widest: a typo in a payload cannot be allowed to publish somebody.
		{"unknown is treated as private", "everyone-lol", clubMate, false},
		{"empty is treated as private", "", clubMate, false},
		{"unknown still shows to a coach", "everyone-lol", coach, true},
	}

	for _, c := range cases {
		if got := CanSeeProfile(c.visibility, c.relation); got != c.want {
			t.Errorf("%s: CanSeeProfile(%q, %+v) = %t, want %t", c.name, c.visibility, c.relation, got, c.want)
		}
	}
}

func TestNextOccurrence(t *testing.T) {
	born := func(value string) time.Time {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}

	cases := []struct {
		name      string
		birthdate string
		from      string
		want      string
	}{
		{"later this year", "1990-09-12", "2026-03-01", "2026-09-12"},
		{"already passed, so next year", "1990-02-03", "2026-03-01", "2027-02-03"},
		// Today counts: a birthday should not disappear from the calendar on
		// the one day it matters.
		{"today", "1990-03-01", "2026-03-01", "2026-03-01"},
		// The 29th of February rolls to the 1st of March in a common year,
		// rather than the birthday vanishing three years out of four.
		{"leap day in a common year", "2000-02-29", "2026-01-01", "2026-03-01"},
		{"leap day in a leap year", "2000-02-29", "2028-01-01", "2028-02-29"},
	}

	for _, c := range cases {
		got := NextOccurrence(born(c.birthdate), born(c.from))
		if got.Format("2006-01-02") != c.want {
			t.Errorf("%s: NextOccurrence(%s, %s) = %s, want %s",
				c.name, c.birthdate, c.from, got.Format("2006-01-02"), c.want)
		}
	}
}

// TestNormalizeProfileVisibilityNeverWidens is the invariant behind the table
// above, stated on its own: no input may turn into a wider level than it names.
func TestNormalizeProfileVisibilityNeverWidens(t *testing.T) {
	for _, value := range []string{"", "PUBLIC", "public ", "everyone", "club", "friends"} {
		if got := NormalizeProfileVisibility(value); got == ProfileVisibilityPublic && value != ProfileVisibilityPublic {
			t.Errorf("NormalizeProfileVisibility(%q) = %q - an unrecognized value must not publish a profile", value, got)
		}
	}
	if got := NormalizeProfileVisibility(ProfileVisibilityPublic); got != ProfileVisibilityPublic {
		t.Errorf("NormalizeProfileVisibility(%q) = %q", ProfileVisibilityPublic, got)
	}
}
