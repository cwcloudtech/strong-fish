// Package nutrition turns what a member's body is - weight, fat, height, age,
// sex - and what their trackers measured into the two targets a lifter eats
// against: a daily protein band and a daily energy to maintain weight.
//
// The numbers are only ever as good as the inputs. A connected scale gives a
// real fat-free mass and a measured resting metabolism; without one, both are
// estimated from height and weight by published formulas, and every figure
// that was guessed is flagged so the clients can say so. The point is not a
// clinical prescription - it is to let an athlete and their coach see, at a
// glance, whether what they ate sits above, below, or at what it takes to hold
// their weight, which is the whole of the cut/bulk/maintain decision.
package nutrition

import (
	"math"

	"strong-fish-api/internal/models"
)

// The protein band, in grams of protein per kilogram of fat-free mass.
//
// Constants rather than literals because they are a medical judgement that
// gets revised: the range advised for a training lifter is 1.6-2.2 g/kg of
// lean mass in the current literature, and when that moves it moves here, in
// one place, not scattered through the handlers.
const (
	ProteinPerKgFatFreeMassMin = 1.6
	ProteinPerKgFatFreeMassMax = 2.2
)

// DefaultActivityFactor multiplies resting energy into a maintenance estimate
// when no tracker reported active calories.
//
// 1.35 is between "sedentary" (1.2) and "lightly active" (1.375): a floor for
// somebody who lifts but whose watch is not telling us what else they did. It
// is only used as a fallback, and the result is flagged estimated - a real
// active-calorie figure from a tracker replaces it.
const DefaultActivityFactor = 1.35

// defaultAgeYears stands in when a member has not filled a birthdate. The age
// terms in the formulas move the answer only slightly, so a sensible default
// keeps the estimate usable rather than refusing it, and the estimate is
// flagged anyway.
const defaultAgeYears = 30.0

// Input is everything the targets are computed from, gathered by the handler
// from the member's profile, their body-composition source and their activity
// source. A field that is not known is left zero.
type Input struct {
	// WeightKg is the member's weight - from a connected scale when there is
	// one, otherwise the bodyweight on their profile.
	WeightKg float64
	// HeightCm is the member's height from their profile. Zero means it is not
	// filled in, which is what gates the estimated path (see Targets).
	HeightCm float64
	// AgeYears is derived from the profile birthdate, or zero when unknown.
	AgeYears float64
	// Male selects the sex term in the formulas. StrongFish stores a gender
	// with "male" as the default, which is what an account never asked reads as.
	Male bool

	// FatFreeMassKg, FatRatioPct and MeasuredBMRKcal come from a connected
	// scale, zero when absent. A measured fat-free mass is used directly; a fat
	// ratio turns the weight into one; a measured basal metabolic rate is the
	// resting energy without a formula.
	FatFreeMassKg   float64
	FatRatioPct     float64
	MeasuredBMRKcal float64

	// ActiveKcalPerDay is the average daily active calories a tracker reported,
	// and HasActiveKcal says whether that figure is real. Without it the
	// maintenance energy falls back to DefaultActivityFactor.
	ActiveKcalPerDay float64
	HasActiveKcal    bool
}

// BMI is the body mass index from a weight in kilograms and a height in
// centimetres, or zero when either is missing.
func BMI(weightKg, heightCm float64) float64 {
	if weightKg <= 0 || heightCm <= 0 {
		return 0
	}
	m := heightCm / 100
	return weightKg / (m * m)
}

// bodyFatFromBMI estimates body fat percentage from BMI, age and sex by the
// Deurenberg equation - the standard way to get a fat ratio when there is no
// scale to measure one. Male is 1 in the sex term, female 0.
func bodyFatFromBMI(bmi, ageYears float64, male bool) float64 {
	sex := 0.0
	if male {
		sex = 1
	}
	bf := 1.20*bmi + 0.23*ageYears - 10.8*sex - 5.4
	return clamp(bf, 3, 60)
}

// mifflinBMR is the Mifflin-St Jeor resting metabolic rate, the current
// first-choice estimate when no scale measured one.
func mifflinBMR(weightKg, heightCm, ageYears float64, male bool) float64 {
	bmr := 10*weightKg + 6.25*heightCm - 5*ageYears
	if male {
		return bmr + 5
	}
	return bmr - 161
}

// katchMcArdleBMR is the Katch-McArdle resting metabolic rate, which depends
// only on fat-free mass - the right estimate when a scale gave a real lean
// mass, since it needs no height.
func katchMcArdleBMR(fatFreeMassKg float64) float64 {
	return 370 + 21.6*fatFreeMassKg
}

// Targets computes the member's protein band and maintenance energy from the
// inputs, filling in how each was derived and flagging estimates.
//
// The order is "measured beats estimated" throughout: a real lean mass or
// resting rate from a scale is used as-is; only when there is none does the
// height-and-weight estimate kick in, and then every derived number is marked
// estimated. When the estimate is the only path and the height is missing, the
// result carries NeedsHeight and nothing else, so the clients prompt for the
// one field that unlocks the rest.
func Targets(in Input) models.NutritionTargets {
	var t models.NutritionTargets

	// A missing birthdate falls back to a default age. The age terms move the
	// answer only slightly, and where they are used at all (the BMI-estimate
	// path) the result is already flagged estimated for being BMI-derived, so
	// the default needs no separate warning.
	age := in.AgeYears
	if age <= 0 {
		age = defaultAgeYears
	}

	// --- fat-free mass, for the protein band ---
	ffm, ffmMeasured := fatFreeMass(in)
	if ffm <= 0 {
		// The estimate path is the only one left and it needs a height. Say so
		// and stop: a protein band off a guessed body with no height, and an
		// energy off nothing, would be numbers with no backing.
		if in.HeightCm <= 0 {
			return models.NutritionTargets{NeedsHeight: true}
		}
		if bmi := BMI(in.WeightKg, in.HeightCm); bmi > 0 {
			bf := bodyFatFromBMI(bmi, age, in.Male)
			ffm = in.WeightKg * (1 - bf/100)
		}
	}
	if ffm > 0 {
		t.FatFreeMassKg = round1(ffm)
		t.ProteinMinG = round1(ProteinPerKgFatFreeMassMin * ffm)
		t.ProteinMaxG = round1(ProteinPerKgFatFreeMassMax * ffm)
		if ffmMeasured {
			t.ProteinBasis = models.NutritionBasisMeasured
		} else {
			t.ProteinBasis = models.NutritionBasisEstimated
			t.Estimated = true
		}
	}

	// --- resting energy, then maintenance ---
	resting, restingMeasured := restingEnergy(in, ffm, ffmMeasured, age)
	if resting > 0 {
		t.RestingKcal = math.Round(resting)
		active := in.ActiveKcalPerDay
		energyEstimated := !restingMeasured
		if in.HasActiveKcal && active > 0 {
			t.ActivityKcal = math.Round(active)
		} else {
			// No tracker figure: lift the resting rate by a light-activity
			// factor so the target is a day's energy, not just lying still.
			active = resting * (DefaultActivityFactor - 1)
			t.ActivityKcal = math.Round(active)
			energyEstimated = true
		}
		t.MaintenanceKcal = t.RestingKcal + t.ActivityKcal
		if energyEstimated {
			t.EnergyBasis = models.NutritionBasisEstimated
			t.Estimated = true
		} else {
			t.EnergyBasis = models.NutritionBasisMeasured
		}
	}

	return t
}

// fatFreeMass returns the member's fat-free mass and whether it was measured
// (true) or has to be estimated (false, meaning the caller must derive it from
// BMI). A measured lean mass is used directly; a measured fat ratio turns the
// weight into one.
func fatFreeMass(in Input) (ffm float64, measured bool) {
	switch {
	case in.FatFreeMassKg > 0:
		return in.FatFreeMassKg, true
	case in.FatRatioPct > 0 && in.WeightKg > 0:
		return in.WeightKg * (1 - in.FatRatioPct/100), true
	default:
		return 0, false
	}
}

// restingEnergy returns the resting metabolic rate and whether it was measured.
// A scale's own figure wins; then Katch-McArdle off a measured lean mass; then
// Mifflin-St Jeor off height, weight, age and sex; then Katch-McArdle off even
// an estimated lean mass as a last resort.
func restingEnergy(in Input, ffm float64, ffmMeasured bool, age float64) (bmr float64, measured bool) {
	switch {
	case in.MeasuredBMRKcal > 0:
		return in.MeasuredBMRKcal, true
	case ffm > 0 && ffmMeasured:
		return katchMcArdleBMR(ffm), true
	case in.HeightCm > 0 && in.WeightKg > 0:
		return mifflinBMR(in.WeightKg, in.HeightCm, age, in.Male), false
	case ffm > 0:
		return katchMcArdleBMR(ffm), false
	default:
		return 0, false
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
