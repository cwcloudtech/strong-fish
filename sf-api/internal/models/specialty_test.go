package models

import "testing"

// TestNormalizeSpecialty pins what reaches the database. The badge is stored
// as written, so anything that is not one of the four has to become "no badge"
// here rather than being kept and rendered as a missing translation key.
func TestNormalizeSpecialty(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"a lift", SpecialtySquat, SpecialtySquat},
		{"the totaler", SpecialtyTotal, SpecialtyTotal},
		{"none picked", "", ""},
		{"an account written before this existed", "", ""},
		{"something invented", "clean-and-jerk", ""},
		{"the right word in the wrong case", "Squat", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeSpecialty(tc.in); got != tc.want {
				t.Errorf("NormalizeSpecialty(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSpecialtiesAreAllValid guards the list the clients render against the
// validator the API applies: an option offered in a picker but normalized away
// on save would look like the setting simply did not stick.
func TestSpecialtiesAreAllValid(t *testing.T) {
	if len(Specialties) != 4 {
		t.Fatalf("%d specialties offered, want the three lifts and the totaler", len(Specialties))
	}
	for _, specialty := range Specialties {
		if NormalizeSpecialty(specialty) != specialty {
			t.Errorf("%q is offered but does not survive normalization", specialty)
		}
	}
}
