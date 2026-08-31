package strength

import (
	"math"
	"testing"
)

// The anchors below are the formulas evaluated by hand at a round bodyweight,
// which is the only way to check a fitted polynomial without re-deriving it:
// the constants are the thing under test, so a value computed by this package
// could not be evidence about them. They are also cross-checked against what
// the sport reports - a 600kg raw total at 100kg is a strong national lifter,
// and all three coefficients agree on that in their own scale.
const tolerance = 0.6

func closeTo(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %.2f, want %.2f (+/- %.2f)", name, got, want, tolerance)
	}
}

func TestCoefficientsAtKnownBodyweights(t *testing.T) {
	// Men, 100kg, 600kg total.
	closeTo(t, "DOTS(male, 100, 600)", DOTS(Male, 100, 600), 369.3)
	closeTo(t, "Wilks(male, 100, 600)", Wilks(Male, 100, 600), 365.2)
	closeTo(t, "IPFGL(male, raw, 100, 600)", IPFGL(Male, Raw, 100, 600), 75.8)

	// Women, 60kg, 350kg total.
	closeTo(t, "DOTS(female, 60, 350)", DOTS(Female, 60, 350), 388.0)
	closeTo(t, "Wilks(female, 60, 350)", Wilks(Female, 60, 350), 390.2)

	// Equipped lifting is scored on its own curve, and the same total scores
	// lower in a suit than it does without one.
	raw := IPFGL(Male, Raw, 100, 600)
	equipped := IPFGL(Male, Equipped, 100, 600)
	if equipped >= raw {
		t.Errorf("an equipped total should score below the same total raw: raw=%.2f equipped=%.2f", raw, equipped)
	}
}

// TestNothingScoresNothing covers the empty profile, which is most profiles on
// a new instance: no maxes entered is not a score of zero to be ranked, it is
// no score at all.
func TestNothingScoresNothing(t *testing.T) {
	cases := []struct {
		name       string
		bodyweight float64
		total      float64
	}{
		{"no total", 80, 0},
		{"no bodyweight", 0, 400},
		{"neither", 0, 0},
		{"negative", -80, -400},
	}
	for _, c := range cases {
		if got := DOTS(Male, c.bodyweight, c.total); got != 0 {
			t.Errorf("DOTS with %s = %.2f, want 0", c.name, got)
		}
		if got := Wilks(Male, c.bodyweight, c.total); got != 0 {
			t.Errorf("Wilks with %s = %.2f, want 0", c.name, got)
		}
		if got := IPFGL(Male, Raw, c.bodyweight, c.total); got != 0 {
			t.Errorf("IPFGL with %s = %.2f, want 0", c.name, got)
		}
	}
}

// TestHeavierLifterScoresLower is the property every one of these formulas
// exists for: the same total is worth more from a lighter lifter.
func TestHeavierLifterScoresLower(t *testing.T) {
	previous := map[string]float64{}
	for _, bw := range []float64{60, 75, 90, 105, 120} {
		scores := map[string]float64{
			"DOTS":  DOTS(Male, bw, 500),
			"Wilks": Wilks(Male, bw, 500),
			"IPFGL": IPFGL(Male, Raw, bw, 500),
		}
		for name, score := range scores {
			if last, seen := previous[name]; seen && score >= last {
				t.Errorf("%s at %.0fkg = %.2f, not below the %.2f of the lighter lifter", name, bw, score, last)
			}
			previous[name] = score
		}
	}
}

// TestBeyondTheFittedRange covers the bodyweights the polynomials were never
// fitted on. Past them the curve turns over and starts paying heavier lifters
// more, so they are clamped - a 250kg lifter must not out-score a 210kg one on
// the same total.
func TestBeyondTheFittedRange(t *testing.T) {
	if got, want := DOTS(Male, 250, 700), DOTS(Male, 210, 700); got != want {
		t.Errorf("DOTS above the fitted range = %.2f, want it clamped to %.2f", got, want)
	}
	if got, want := Wilks(Female, 200, 400), Wilks(Female, 154.53, 400); got != want {
		t.Errorf("Wilks above the fitted range = %.2f, want it clamped to %.2f", got, want)
	}
}

func TestTiers(t *testing.T) {
	cases := []struct {
		dots float64
		key  string
	}{
		{0, ""},
		{1, "novice"},
		{249, "novice"},
		{250, "intermediate"},
		{349, "intermediate"},
		{350, "advanced"},
		{420, "advanced"},
		{421, "elite"},
		{499, "elite"},
		{500, "worldClass"},
		{900, "worldClass"},
	}
	for _, c := range cases {
		if got := TierFor(c.dots).Key; got != c.key {
			t.Errorf("TierFor(%.0f) = %q, want %q", c.dots, got, c.key)
		}
	}
}

func badgeByKey(badges []Badge, key string) (Badge, bool) {
	for _, badge := range badges {
		if badge.Key == key {
			return badge, true
		}
	}
	return Badge{}, false
}

func TestBadges(t *testing.T) {
	// 80kg lifter: a 2x squat, a bench well under 1.5x, a 2.5x deadlift, and a
	// 500kg total that is both the 500kg and the 1000lb club.
	in := Input{Gender: Male, Division: Raw, BodyweightKg: 80,
		Lifts: Lifts{Squat: 160, Bench: 100, Deadlift: 240}}
	badges := Badges(in, Score(in))

	earned := map[string]bool{
		"multiplier.squat2x":      true,
		"multiplier.deadlift2_5x": true,
		"multiplier.bench1_5x":    false,
		"club.total1000lb":        true,
		"club.total500kg":         true,
		"club.total1500lb":        false,
		"style.deadliftDominant":  true,
		"style.squatDominant":     false,
	}
	for key, want := range earned {
		badge, ok := badgeByKey(badges, key)
		if !ok {
			t.Fatalf("no badge %q in the engine's output", key)
		}
		if badge.Earned != want {
			t.Errorf("%s earned = %v, want %v (value %.2f, target %.2f)", key, badge.Earned, want, badge.Value, badge.Target)
		}
	}

	// A locked badge still says how far along it is.
	if badge, _ := badgeByKey(badges, "multiplier.bench1_5x"); badge.Progress <= 0 || badge.Progress >= 1 {
		t.Errorf("a locked badge's progress = %.2f, want it between 0 and 1", badge.Progress)
	}
}

// TestEmptyProfileEarnsNothing covers a member who has entered no maxes: every
// badge locked, no tier, and no progress bars quietly reading 100%.
func TestEmptyProfileEarnsNothing(t *testing.T) {
	in := Input{Gender: Male, Division: Raw, BodyweightKg: 80}
	result := Evaluate(in, nil)
	if result.Tier.Key != "" {
		t.Errorf("an empty profile has tier %q, want none", result.Tier.Key)
	}
	for _, badge := range result.Badges {
		if badge.Earned {
			t.Errorf("%s earned on an empty profile", badge.Key)
		}
		if badge.Progress != 0 {
			t.Errorf("%s progress = %.2f on an empty profile, want 0", badge.Key, badge.Progress)
		}
	}
}

func TestPercentile(t *testing.T) {
	population := []float64{100, 200, 300, 400, 500}
	cases := []struct {
		score float64
		want  int
	}{
		{50, 0},
		{100, 20},
		{300, 60},
		{500, 100},
		{900, 100},
	}
	for _, c := range cases {
		if got := PercentileOf(c.score, population).Value; got != c.want {
			t.Errorf("PercentileOf(%.0f) = %d, want %d", c.score, got, c.want)
		}
	}
	if got := PercentileOf(300, nil); got.Sample != 0 || got.Value != 0 {
		t.Errorf("with nobody to compare against = %+v, want the zero percentile", got)
	}
	if got := PercentileOf(300, population).Sample; got != 5 {
		t.Errorf("sample = %d, want 5", got)
	}
}
