package strength

import "math"

// Tier is where a DOTS score puts a lifter, and what that rung is called.
//
// The boundaries are the ones the sport talks in - 500 DOTS is the number
// people mean by "world class" - and the names are this app's own. A tier is
// deliberately coarse: it is a thing to be proud of and to aim past, not a
// ranking to defend by a decimal.
type Tier struct {
	// Key is stable and translatable; Label is the English name it carries in
	// the JSON so a client with no dictionary still shows something.
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Min   float64 `json:"min"`
	// Max is the top of the band, or 0 for the open-ended one.
	Max float64 `json:"max,omitempty"`
}

// Tiers, lowest first. A score below the first tier's Min has no tier at all,
// which is what an empty profile has.
var Tiers = []Tier{
	{Key: "novice", Label: "Iron Recruit", Min: 0, Max: 249.99},
	{Key: "intermediate", Label: "Platform Contender", Min: 250, Max: 349.99},
	{Key: "advanced", Label: "National Caliber", Min: 350, Max: 420.99},
	{Key: "elite", Label: "Master Lifter", Min: 421, Max: 499.99},
	{Key: "worldClass", Label: "Titan", Min: 500},
}

// TierFor is the band a DOTS score falls in, or the zero Tier when there is no
// score yet - a member who has entered no maxes is not a novice, they are
// simply unmeasured.
func TierFor(dots float64) Tier {
	if dots <= 0 {
		return Tier{}
	}
	found := Tiers[0]
	for _, tier := range Tiers {
		if dots >= tier.Min {
			found = tier
		}
	}
	return found
}

// Badge is one achievement, earned or not.
//
// Unearned badges are returned too, with how far along they are: a target you
// can see is worth more than one that appears from nowhere the day you hit it.
type Badge struct {
	// Key names the badge to the clients, which translate it and pick its
	// icon: "multiplier.squat2x", "club.total500kg", "dots.400", ...
	Key string `json:"key"`
	// Kind groups badges on the profile: multiplier, club, dots, style.
	Kind   string `json:"kind"`
	Earned bool   `json:"earned"`
	// Progress is 0..1 towards Target, for the ones that are a number to reach.
	// A style badge has no progress - it is a shape, not a distance - and
	// leaves this at 0.
	Progress float64 `json:"progress"`
	// Target and Value are what the badge asks for and where the lifter is, in
	// whatever unit the badge is about (kg for a club, a coefficient for DOTS,
	// a ratio for a multiplier). Both are for the client to render, never to
	// recompute the verdict from.
	Target float64 `json:"target,omitempty"`
	Value  float64 `json:"value,omitempty"`
}

// badgeRule is one row of the engine: what it is called, what it asks of a
// lifter, and how to read where they are against it.
//
// Adding an achievement is adding a row. The rules take the whole input and
// the scores, so a rule can ask about a ratio, a total, a coefficient, or the
// shape of somebody's three lifts without the engine knowing which.
type badgeRule struct {
	key    string
	kind   string
	target float64
	// value is where the lifter stands, in the badge's own unit.
	value func(Input, Scores) float64
	// earned defaults to value >= target; a rule that is not a threshold - the
	// specialization badges - provides its own.
	earned func(Input, Scores) bool
}

// poundsPerKg converts the clubs that are named in pounds. The lifting is done
// in kilograms everywhere in this app; only the club's name is imperial.
const poundsPerKg = 0.45359237

var badgeRules = []badgeRule{
	// --- what somebody lifts against what they weigh ---
	{key: "multiplier.squat2x", kind: "multiplier", target: 2,
		value: func(in Input, _ Scores) float64 { return ratio(in.Lifts.Squat, in.BodyweightKg) }},
	{key: "multiplier.bench1_5x", kind: "multiplier", target: 1.5,
		value: func(in Input, _ Scores) float64 { return ratio(in.Lifts.Bench, in.BodyweightKg) }},
	{key: "multiplier.deadlift2_5x", kind: "multiplier", target: 2.5,
		value: func(in Input, _ Scores) float64 { return ratio(in.Lifts.Deadlift, in.BodyweightKg) }},
	{key: "multiplier.total10x", kind: "multiplier", target: 10,
		value: func(in Input, _ Scores) float64 { return ratio(in.Lifts.Total(), in.BodyweightKg) }},

	// --- the clubs, which are just a total with a name ---
	{key: "club.total1000lb", kind: "club", target: 1000 * poundsPerKg,
		value: func(in Input, _ Scores) float64 { return in.Lifts.Total() }},
	{key: "club.total500kg", kind: "club", target: 500,
		value: func(in Input, _ Scores) float64 { return in.Lifts.Total() }},
	{key: "club.total1500lb", kind: "club", target: 1500 * poundsPerKg,
		value: func(in Input, _ Scores) float64 { return in.Lifts.Total() }},

	// --- the coefficient clubs ---
	{key: "dots.300", kind: "dots", target: 300,
		value: func(_ Input, s Scores) float64 { return s.DOTS }},
	{key: "dots.400", kind: "dots", target: 400,
		value: func(_ Input, s Scores) float64 { return s.DOTS }},
	{key: "dots.500", kind: "dots", target: 500,
		value: func(_ Input, s Scores) float64 { return s.DOTS }},

	// --- the shape of the three lifts, rather than their size ---
	//
	// These are not a distance to close, so they carry no progress: a lifter
	// either has this shape today or does not, and a bar creeping towards
	// "your bench is poor" would be a strange thing to show anybody.
	{key: "style.povertyBench", kind: "style",
		earned: func(in Input, _ Scores) bool {
			return complete(in) && in.Lifts.Bench*4 < in.Lifts.Squat+in.Lifts.Deadlift
		}},
	{key: "style.squatDominant", kind: "style",
		earned: func(in Input, _ Scores) bool {
			return complete(in) && in.Lifts.Squat > in.Lifts.Deadlift
		}},
	{key: "style.deadliftDominant", kind: "style",
		earned: func(in Input, _ Scores) bool {
			return complete(in) && in.Lifts.Deadlift >= in.Lifts.Squat*1.25
		}},
}

// Badges runs every rule over one lifter's numbers.
//
// Every badge comes back, earned or not: the client decides whether to show the
// locked ones, and a profile that only ever received what it had already won
// could not draw a target.
func Badges(in Input, scores Scores) []Badge {
	badges := make([]Badge, 0, len(badgeRules))
	for _, rule := range badgeRules {
		badge := Badge{Key: rule.key, Kind: rule.kind, Target: round2(rule.target)}
		if rule.value != nil {
			badge.Value = round2(rule.value(in, scores))
			if rule.target > 0 {
				badge.Progress = round2(clamp(badge.Value/rule.target, 0, 1))
			}
		}
		switch {
		case rule.earned != nil:
			badge.Earned = rule.earned(in, scores)
		case rule.value != nil:
			badge.Earned = rule.value(in, scores) >= rule.target
		}
		badges = append(badges, badge)
	}
	return badges
}

// complete reports whether all three lifts are on record, which the shape
// badges need: two lifts out of three say nothing about a lifter's balance.
func complete(in Input) bool {
	return in.Lifts.Squat > 0 && in.Lifts.Bench > 0 && in.Lifts.Deadlift > 0
}

func ratio(lift, bodyweight float64) float64 {
	if lift <= 0 || bodyweight <= 0 {
		return 0
	}
	return lift / bodyweight
}

// Percentile is where a score sits among the members of this deployment.
//
// Computed from the population rather than from a published curve: a club's
// members are who its lifters actually compare themselves to, and a curve
// fitted on international meet data would tell a beginners' gym that everybody
// in it is bottom decile. Sample is carried alongside so a client can say what
// the number is out of - a percentile among four people is a fact about four
// people.
type Percentile struct {
	Value  int `json:"value"`
	Sample int `json:"sample"`
}

// PercentileOf ranks score against others, as the share of them it is at least
// as high as. An empty population has no percentile to give.
func PercentileOf(score float64, population []float64) Percentile {
	if score <= 0 || len(population) == 0 {
		return Percentile{}
	}
	below := 0
	for _, other := range population {
		if score >= other {
			below++
		}
	}
	return Percentile{
		Value:  int(math.Round(float64(below) / float64(len(population)) * 100)),
		Sample: len(population),
	}
}

// Result is everything the calculator and the profile show.
type Result struct {
	// TotalKg is the three lifts added up, in kilograms, whatever unit the
	// caller typed them in.
	TotalKg    float64    `json:"total"`
	Scores     Scores     `json:"scores"`
	Tier       Tier       `json:"tier"`
	Badges     []Badge    `json:"badges"`
	Percentile Percentile `json:"percentile,omitempty"`
}

// Evaluate scores a lifter and works out what they have earned.
func Evaluate(in Input, population []float64) Result {
	scores := Score(in)
	return Result{
		TotalKg:    round2(in.Lifts.Total()),
		Scores:     scores,
		Tier:       TierFor(scores.DOTS),
		Badges:     Badges(in, scores),
		Percentile: PercentileOf(scores.DOTS, population),
	}
}
