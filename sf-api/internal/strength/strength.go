// Package strength scores a powerlifting total.
//
// Three coefficients, because the sport has not settled on one: DOTS is what
// most federations and lifters quote today, Wilks (1994) is what two decades of
// records were set under and is still asked for, and IPF GL is what the IPF
// ranks its own meets by. They disagree, deliberately - each was fitted to a
// different population - so all three are reported rather than one being picked
// on the members' behalf.
//
// Everything here works in kilograms. The clients convert pounds at the edge,
// because a formula that has to ask which unit it was handed is a formula that
// will one day be handed the wrong one.
package strength

import "math"

// Gender selects the coefficients. Powerlifting's formulas are fitted separately
// for men and women; there is no unisex variant of any of the three.
type Gender string

const (
	Male   Gender = "male"
	Female Gender = "female"
)

// NormalizeGender reads a gender off a profile, defaulting to male - which is what
// the profile itself defaults to.
func NormalizeGender(value string) Gender {
	if Gender(value) == Female {
		return Female
	}
	return Male
}

// Division separates lifting in a shirt and suit from lifting without.
// Only IPF GL distinguishes them; DOTS and Wilks were fitted on raw lifting
// alone and are reported unchanged for both.
type Division string

const (
	Raw      Division = "raw"
	Equipped Division = "equipped"
)

// NormalizeDivision defaults to raw, which is what most members lift in.
func NormalizeDivision(value string) Division {
	if Division(value) == Equipped {
		return Equipped
	}
	return Raw
}

// Lifts is one lifter's three competition maxes, in kilograms.
type Lifts struct {
	Squat    float64 `json:"squat"`
	Bench    float64 `json:"bench"`
	Deadlift float64 `json:"deadlift"`
}

// Total is what the three add up to - the number every coefficient scores.
func (l Lifts) Total() float64 { return l.Squat + l.Bench + l.Deadlift }

// Input is everything a score needs.
type Input struct {
	Gender       Gender   `json:"gender"`
	Division     Division `json:"division"`
	BodyweightKg float64  `json:"bodyweight"`
	Lifts        Lifts    `json:"lifts"`
}

// dotsCoefficients are the 2019 polynomial's terms, lowest order first.
//
// DOTS replaced Wilks in most federations because Wilks scored the lightest
// and heaviest lifters unfairly against the middle; the polynomial is fitted to
// the same shape of data with that corrected.
var dotsCoefficients = map[Gender][]float64{
	Male:   {-307.75076, 24.0900756, -0.1918759221, 0.0007391293, -0.000001093},
	Female: {-57.96288, 13.6175032, -0.1126655495, 0.0005158568, -0.0000010706},
}

// dotsRange is where the polynomial was fitted. Outside it the curve turns and
// starts handing out scores that are not merely inaccurate but wrong in the
// other direction, so the bodyweight is clamped rather than extrapolated.
var dotsRange = map[Gender][2]float64{
	Male:   {40, 210},
	Female: {40, 150},
}

// DOTS scores a total for a lifter of this bodyweight. Zero for a total or a
// bodyweight that is not there yet, which is what an empty profile looks like.
func DOTS(gender Gender, bodyweightKg, totalKg float64) float64 {
	if totalKg <= 0 || bodyweightKg <= 0 {
		return 0
	}
	gender = NormalizeGender(string(gender))
	bounds := dotsRange[gender]
	bw := clamp(bodyweightKg, bounds[0], bounds[1])
	return round2(totalKg * 500 / polynomial(dotsCoefficients[gender], bw))
}

// wilksCoefficients are the original 1994 terms, lowest order first.
var wilksCoefficients = map[Gender][]float64{
	Male: {-216.0475144, 16.2606339, -0.002388645, -0.00113732, 0.00000701863, -0.00000001291},
	Female: {594.31747775582, -27.23842536447, 0.82112226871, -0.00930733913,
		0.00004731582, -0.00000009054},
}

var wilksRange = map[Gender][2]float64{
	Male:   {40, 201.9},
	Female: {26.51, 154.53},
}

// Wilks scores a total under the 1994 formula - the one every record set
// before 2020 was ranked by, which is why it is still quoted.
func Wilks(gender Gender, bodyweightKg, totalKg float64) float64 {
	if totalKg <= 0 || bodyweightKg <= 0 {
		return 0
	}
	gender = NormalizeGender(string(gender))
	bounds := wilksRange[gender]
	bw := clamp(bodyweightKg, bounds[0], bounds[1])
	return round2(totalKg * 500 / polynomial(wilksCoefficients[gender], bw))
}

// glParameters are the IPF's 2020 fit: a saturating curve rather than a
// polynomial, one set per gender and division. These are the full-power numbers;
// the IPF publishes separate ones for single-lift meets, which this app has no
// notion of.
type glParameters struct{ a, b, c float64 }

var glByDivision = map[Division]map[Gender]glParameters{
	Raw: {
		Male:   {1199.72839, 1025.18162, 0.00921},
		Female: {610.32796, 1045.59282, 0.03048},
	},
	Equipped: {
		Male:   {1236.25115, 1449.21864, 0.01644},
		Female: {758.63878, 949.31382, 0.02435},
	},
}

// IPFGL scores a total the way the IPF ranks its meets. Unlike the other two
// it knows the difference between raw and equipped lifting.
func IPFGL(gender Gender, division Division, bodyweightKg, totalKg float64) float64 {
	if totalKg <= 0 || bodyweightKg <= 0 {
		return 0
	}
	p := glByDivision[NormalizeDivision(string(division))][NormalizeGender(string(gender))]
	denominator := p.a - p.b*math.Exp(-p.c*bodyweightKg)
	if denominator <= 0 {
		// Only reachable for a bodyweight far below anything the fit covers,
		// where the curve has not yet crossed zero. No score is a better
		// answer than a negative one.
		return 0
	}
	return round2(totalKg * 100 / denominator)
}

// Scores is the three coefficients side by side.
type Scores struct {
	DOTS  float64 `json:"dots"`
	Wilks float64 `json:"wilks"`
	IPFGL float64 `json:"ipfGl"`
}

// Score computes all three at once, which is how they are always shown.
func Score(in Input) Scores {
	total := in.Lifts.Total()
	return Scores{
		DOTS:  DOTS(in.Gender, in.BodyweightKg, total),
		Wilks: Wilks(in.Gender, in.BodyweightKg, total),
		IPFGL: IPFGL(in.Gender, in.Division, in.BodyweightKg, total),
	}
}

// polynomial evaluates coefficients[0] + coefficients[1]*x + ... by Horner's
// method, which keeps the fifth-order terms from losing precision.
func polynomial(coefficients []float64, x float64) float64 {
	result := 0.0
	for i := len(coefficients) - 1; i >= 0; i-- {
		result = result*x + coefficients[i]
	}
	return result
}

func clamp(value, low, high float64) float64 {
	return math.Min(math.Max(value, low), high)
}

// round2 keeps two decimals: a coefficient is quoted to two everywhere the
// sport writes one down, and the extra digits are noise from the fit.
func round2(value float64) float64 { return math.Round(value*100) / 100 }
