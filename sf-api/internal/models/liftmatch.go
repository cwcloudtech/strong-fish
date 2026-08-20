package models

import (
	"strings"

	"strong-fish-api/internal/utils"
)

// Which competition lift a movement is loaded off, read from its name.
//
// A member records three maxes. Everything else they are prescribed - a highbar
// squat, a tempo squat, a paused deadlift, a dumbbell bench - is a variation of
// one of those, and the load it should be run at follows that lift's max. When
// the catalog entry does not say which (nobody filled it in, or the importer
// could not work it out from the numbers), the name itself usually does.
//
// # The table
//
// LiftRules below is the whole rule set, and it is meant to be edited: adding a
// movement is adding a line. Each rule is a phrase and the lift it points at.
//
//   - Matching is case- and accent-insensitive, and works on whole words:
//     "RDL", "rdl" and "Rdl 3x8" all match the "rdl" rule, while "cordless"
//     does not. Punctuation and numbers separate words, so "tempo squat 3:3:0"
//     matches "squat".
//   - The most specific phrase wins. "bench" points at the bench press, and
//     "bench pull" - a rowing movement that happens to be done on one - points
//     at nothing, because it is the longer phrase.
//   - A rule may point at nothing on purpose. That is how a movement whose
//     name contains a competition lift but which must not be loaded off it -
//     a goblet squat, a split squat - is kept out.
//   - Ties between phrases of equal length are broken by the order below, so
//     the table reads top to bottom like the decision table it is.
//
// What this is not: it never overrides a max the member actually recorded for
// the movement, nor a reference a coach set on the catalog entry. It is the
// fallback for everything else.
type LiftRule struct {
	// Match is the phrase, written as it would appear in an exercise's name.
	Match string
	// Ref is the competition lift the movement is loaded off: CategorySquat,
	// CategoryBench, CategoryDeadlift - or "" for "this one deliberately has
	// no reference lift".
	Ref string
}

// LiftRules is the decision table. Order matters only between phrases of the
// same length; otherwise the longest match wins wherever it sits.
var LiftRules = []LiftRule{
	// --- movements that name a lift but must not be loaded off it ---
	// Each of these contains "squat", "bench" or "deadlift" and would
	// otherwise be prescribed at a fraction of a two-legged, two-armed max.
	{Match: "goblet squat", Ref: ""},
	{Match: "split squat", Ref: ""},
	{Match: "bulgarian squat", Ref: ""},
	{Match: "hack squat", Ref: ""},
	{Match: "belt squat", Ref: ""},
	{Match: "sissy squat", Ref: ""},
	{Match: "squat jump", Ref: ""},
	{Match: "jump squat", Ref: ""},
	{Match: "bench pull", Ref: ""},
	{Match: "bench row", Ref: ""},
	{Match: "single leg deadlift", Ref: ""},

	// --- squat ---
	{Match: "squat", Ref: CategorySquat},
	{Match: "highbar", Ref: CategorySquat},
	{Match: "high bar", Ref: CategorySquat},
	{Match: "lowbar", Ref: CategorySquat},
	{Match: "low bar", Ref: CategorySquat},
	{Match: "ssb", Ref: CategorySquat},
	{Match: "safety bar", Ref: CategorySquat},
	// A sumo *squat* is a squat; the sumo rule further down is for the pull.
	{Match: "sumo squat", Ref: CategorySquat},
	// French: "flexion de jambes" is the federation's own wording for a squat.
	{Match: "flexion", Ref: CategorySquat},

	// --- bench ---
	{Match: "bench", Ref: CategoryBench},
	{Match: "larsen", Ref: CategoryBench},
	{Match: "spoto", Ref: CategoryBench},
	{Match: "close grip", Ref: CategoryBench},
	{Match: "floor press", Ref: CategoryBench},
	{Match: "incline press", Ref: CategoryBench},
	// French: "développé couché", and the "DC" everybody writes on a program.
	{Match: "developpe couche", Ref: CategoryBench},
	{Match: "dc", Ref: CategoryBench},

	// --- deadlift ---
	{Match: "deadlift", Ref: CategoryDeadlift},
	{Match: "dead lift", Ref: CategoryDeadlift},
	{Match: "rdl", Ref: CategoryDeadlift},
	{Match: "sldl", Ref: CategoryDeadlift},
	{Match: "romanian", Ref: CategoryDeadlift},
	{Match: "stiff leg", Ref: CategoryDeadlift},
	{Match: "rack pull", Ref: CategoryDeadlift},
	{Match: "block pull", Ref: CategoryDeadlift},
	{Match: "deficit pull", Ref: CategoryDeadlift},
	{Match: "sumo", Ref: CategoryDeadlift},
	{Match: "snatch grip pull", Ref: CategoryDeadlift},
	// French: "soulevé de terre", and its "SDT" shorthand.
	{Match: "souleve de terre", Ref: CategoryDeadlift},
	{Match: "sdt", Ref: CategoryDeadlift},
}

// compiledRule is a rule with its phrase already split into words, so matching
// does not re-tokenize the table on every set of every session.
type compiledRule struct {
	words []string
	ref   string
}

var compiledRules = compileRules(LiftRules)

func compileRules(rules []LiftRule) []compiledRule {
	compiled := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		if words := liftWords(rule.Match); len(words) > 0 {
			compiled = append(compiled, compiledRule{words: words, ref: rule.Ref})
		}
	}
	return compiled
}

// MatchOneRMRef reads the competition lift out of an exercise's names - its
// slug, its label in either language, whatever the caller has.
//
// It returns "" when nothing matches, and also when the best match is a rule
// that deliberately points at nothing: both mean "do not load this off a
// competition max", which is the same answer to the caller.
func MatchOneRMRef(names ...string) string {
	best, bestScore := "", 0
	for _, name := range names {
		words := liftWords(name)
		if len(words) == 0 {
			continue
		}
		for _, rule := range compiledRules {
			if !containsWords(words, rule.words) {
				continue
			}
			// Longer phrases are more specific; among equals the table's own
			// order decides, which is why this is a strict comparison.
			if score := len(rule.words)*100 + len(strings.Join(rule.words, "")); score > bestScore {
				best, bestScore = rule.ref, score
			}
		}
	}
	return best
}

// liftWords splits a name into comparable words: lowercased, stripped of
// accents, and broken on anything that is not a letter or a digit. It borrows
// Slugify for that, which is the same normalization the catalog's own slugs go
// through - so "Développé couché 3:1:3" and "developpe-couche" come out as the
// same words.
func liftWords(name string) []string {
	slug := utils.Slugify(name)
	if slug == "" {
		return nil
	}
	return strings.Split(slug, "-")
}

// containsWords reports whether phrase appears in words as consecutive whole
// words. Whole words rather than a substring because the short rules would
// otherwise fire inside longer ones: "dc" sits in "dcline", "sdt" in anything.
func containsWords(words, phrase []string) bool {
	if len(phrase) > len(words) {
		return false
	}
	for start := 0; start+len(phrase) <= len(words); start++ {
		matched := true
		for i, word := range phrase {
			if words[start+i] != word {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
