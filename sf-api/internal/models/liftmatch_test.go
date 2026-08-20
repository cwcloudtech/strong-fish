package models

import "testing"

// TestMatchOneRMRef is the decision table read back as behaviour. Every line
// here is a weight somebody would be told to put on a bar, so the cases that
// must come back with *no* lift matter as much as the ones that match.
func TestMatchOneRMRef(t *testing.T) {
	cases := []struct {
		name, want string
	}{
		// The examples the feature was asked for.
		{"Highbar squat", CategorySquat},
		{"Tempo squat 3:3:0", CategorySquat},
		{"Paused deadlift", CategoryDeadlift},
		{"Dumbbel bench", CategoryBench},
		{"RDL", CategoryDeadlift},

		// Case and accents are not part of a name.
		{"HIGHBAR SQUAT", CategorySquat},
		{"rdl", CategoryDeadlift},
		{"Développé couché prise serrée", CategoryBench},
		{"SDT jambes tendues", CategoryDeadlift},

		// Variations that carry the lift's name.
		{"Close grip bench press", CategoryBench},
		{"Larsen press", CategoryBench},
		{"Front squat", CategorySquat},
		{"SSB box squat", CategorySquat},
		{"Deficit deadlift", CategoryDeadlift},
		{"Rack pull", CategoryDeadlift},
		{"Sumo deadlift", CategoryDeadlift},

		// A longer phrase wins over the lift's bare name.
		{"Goblet squat", ""},
		{"Bulgarian split squat", ""},
		{"Bench pull", ""},
		{"Single leg deadlift", ""},
		{"Sumo squat", CategorySquat},

		// Whole words only: a rule must not fire inside another word.
		{"Cordless machine", ""},
		{"Abduction", ""},

		// Nothing to go on.
		{"Lateral raises", ""},
		{"Hammer curl", ""},
		{"", ""},
		{"3:1:3", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchOneRMRef(tc.name); got != tc.want {
				t.Errorf("MatchOneRMRef(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestMatchOneRMRefAcrossNames covers a caller handing over everything it knows
// about one movement: the slug and both labels. A match on any of them is a
// match, which is what makes an English catalog entry resolve a French program.
func TestMatchOneRMRefAcrossNames(t *testing.T) {
	if got := MatchOneRMRef("mouvement-1", "", "Soulevé de terre avec pause"); got != CategoryDeadlift {
		t.Errorf("matching across names = %q, want the deadlift", got)
	}
	// The most specific answer wins wherever it was found: the slug says
	// "squat", the label says it is a split squat, and a split squat is not
	// loaded off the back squat.
	if got := MatchOneRMRef("split-squat", "Split squat"); got != "" {
		t.Errorf("split squat = %q, want no reference lift", got)
	}
}

// TestLiftRulesAreValid keeps the table honest: a rule pointing at something
// that is not a competition lift would be stored as an exercise's reference
// and quietly resolve to no load at all.
func TestLiftRulesAreValid(t *testing.T) {
	for _, rule := range LiftRules {
		if !IsValidOneRMRef(rule.Ref) {
			t.Errorf("rule %q points at %q, which is not a lift", rule.Match, rule.Ref)
		}
		if len(liftWords(rule.Match)) == 0 {
			t.Errorf("rule %q has no words to match on", rule.Match)
		}
	}
}
