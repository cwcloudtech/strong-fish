package models

// NutritionDay is one day's consumed nutrition, as the clients render it: the
// energy eaten and the protein in it.
type NutritionDay struct {
	Date string `json:"date"`
	// Kcal is the energy consumed that day; ProteinG the protein in grams. A
	// day with nothing logged is absent from the series, not a row of zeroes -
	// a blank day in a food diary is a day the member did not log, not a day
	// they ate nothing.
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"proteinG"`
}

// NutritionSeries is a range of days of consumed nutrition, with the averages
// over the days that were actually logged.
//
// The averages are computed on the server, not in each client, for the same
// reason the step average is: the web app and the phone would otherwise each
// decide what to do about an unlogged day, and disagree about the same week.
type NutritionSeries struct {
	Provider string         `json:"provider"`
	Range    string         `json:"range"`
	From     string         `json:"from"`
	To       string         `json:"to"`
	Days     []NutritionDay `json:"days"`
	// AvgKcal and AvgProteinG are the means over the days with anything logged.
	// An unlogged day does not drag them down - see NutritionDay.
	AvgKcal     float64 `json:"avgKcal"`
	AvgProteinG float64 `json:"avgProteinG"`
	// LoggedDays is how many days of the range had an entry, so a client can
	// say "averaged over 5 of 7 days" rather than implying the whole week.
	LoggedDays int `json:"loggedDays"`
	// Targets is the member's ideal protein band and daily energy, when they
	// can be computed. Carried here so the one nutrition read answers both "what
	// did I eat" and "what should I aim for"; nil when neither can be worked out.
	Targets *NutritionTargets `json:"targets,omitempty"`
}

// Where a target figure was derived from, least certain last.
const (
	// NutritionBasisMeasured is a figure built from a real body-composition
	// reading (a Withings/Garmin scale): fat-free mass or a measured BMR.
	NutritionBasisMeasured = "measured"
	// NutritionBasisEstimated is a figure guessed from height, weight, age and
	// sex when no scale is connected - a BMI-derived body fat, a formula BMR.
	// It carries a warning in the clients because it is an estimate.
	NutritionBasisEstimated = "estimated"
)

// NutritionTargets is the member's ideal daily protein band and energy, with
// how each was worked out so a client can warn when a number is an estimate.
//
// The protein band is grams of protein per kilogram of fat-free mass, the dose
// a lifter is advised to eat; the energy is what it takes to hold weight, which
// a member reads against what they actually ate to decide whether they are in a
// cut, a bulk, or maintaining.
type NutritionTargets struct {
	// ProteinMinG and ProteinMaxG are the daily protein band in grams, the
	// per-kilogram constants (see internal/nutrition) times the fat-free mass.
	ProteinMinG float64 `json:"proteinMinG,omitempty"`
	ProteinMaxG float64 `json:"proteinMaxG,omitempty"`
	// FatFreeMassKg is the fat-free mass the band was computed from, shown so
	// the member sees what the dose is per.
	FatFreeMassKg float64 `json:"fatFreeMassKg,omitempty"`
	// ProteinBasis is NutritionBasisMeasured or NutritionBasisEstimated.
	ProteinBasis string `json:"proteinBasis,omitempty"`
	// MaintenanceKcal is the estimated daily energy to maintain weight: resting
	// expenditure plus the active calories a tracker reported.
	MaintenanceKcal float64 `json:"maintenanceKcal,omitempty"`
	// RestingKcal and ActivityKcal are the two parts of it, kept apart so a
	// member sees how much of the target is simply being alive and how much is
	// what they moved.
	RestingKcal  float64 `json:"restingKcal,omitempty"`
	ActivityKcal float64 `json:"activityKcal,omitempty"`
	// EnergyBasis is NutritionBasisMeasured or NutritionBasisEstimated.
	EnergyBasis string `json:"energyBasis,omitempty"`
	// Estimated is true when any input was guessed rather than measured, so the
	// clients can show one plain warning that the figures are approximate.
	Estimated bool `json:"estimated,omitempty"`
	// NeedsHeight is true when nothing could be computed because the member's
	// height is not filled in: a BMI needs a height, and without a scale the
	// height is the one thing standing between them and these numbers. The
	// clients turn it into a prompt to fill the profile rather than an error.
	NeedsHeight bool `json:"needsHeight,omitempty"`
}

// HasProtein reports whether a protein band was computed.
func (t NutritionTargets) HasProtein() bool { return t.ProteinMaxG > 0 }

// HasEnergy reports whether a maintenance energy was computed.
func (t NutritionTargets) HasEnergy() bool { return t.MaintenanceKcal > 0 }
