package nutrition

import (
	"math"
	"testing"

	"strong-fish-api/internal/models"
)

func approx(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.2f, want ~%.2f", label, got, want)
	}
}

// A measured fat-free mass is used as-is for the protein band, and drives a
// Katch-McArdle resting rate, so the protein basis is measured.
func TestTargetsMeasuredFatFreeMass(t *testing.T) {
	got := Targets(Input{WeightKg: 80, FatFreeMassKg: 64, Male: true})
	approx(t, "proteinMin", got.ProteinMinG, 1.6*64, 0.1)
	approx(t, "proteinMax", got.ProteinMaxG, 2.2*64, 0.1)
	if got.ProteinBasis != models.NutritionBasisMeasured {
		t.Errorf("proteinBasis = %q, want measured", got.ProteinBasis)
	}
	// Katch-McArdle resting, lifted by the fallback activity factor since no
	// tracker figure was supplied.
	approx(t, "resting", got.RestingKcal, math.Round(370+21.6*64), 1)
	approx(t, "maintenance", got.MaintenanceKcal, math.Round(370+21.6*64)*DefaultActivityFactor, 2)
}

// A measured fat ratio turns the weight into a fat-free mass.
func TestTargetsFromFatRatio(t *testing.T) {
	got := Targets(Input{WeightKg: 80, FatRatioPct: 20, Male: true})
	approx(t, "ffm", got.FatFreeMassKg, 64, 0.1)
	approx(t, "proteinMax", got.ProteinMaxG, 2.2*64, 0.1)
	if got.ProteinBasis != models.NutritionBasisMeasured {
		t.Errorf("proteinBasis = %q, want measured", got.ProteinBasis)
	}
}

// A measured basal metabolic rate (a Withings scale) plus a tracker's active
// calories gives a fully measured maintenance energy.
func TestTargetsMeasuredEnergyWithActivity(t *testing.T) {
	got := Targets(Input{
		WeightKg: 80, FatFreeMassKg: 64, Male: true,
		MeasuredBMRKcal: 1800, ActiveKcalPerDay: 500, HasActiveKcal: true,
	})
	approx(t, "resting", got.RestingKcal, 1800, 0.5)
	approx(t, "activity", got.ActivityKcal, 500, 0.5)
	approx(t, "maintenance", got.MaintenanceKcal, 2300, 0.5)
	if got.EnergyBasis != models.NutritionBasisMeasured {
		t.Errorf("energyBasis = %q, want measured", got.EnergyBasis)
	}
	if got.Estimated {
		t.Error("fully measured inputs should not be flagged estimated")
	}
}

// With no scale and no height, nothing can be computed and the result asks for
// the height.
func TestTargetsNeedsHeight(t *testing.T) {
	got := Targets(Input{WeightKg: 80, Male: true})
	if !got.NeedsHeight {
		t.Fatal("expected NeedsHeight when no body data and no height")
	}
	if got.HasProtein() || got.HasEnergy() {
		t.Error("no numbers should be produced when height is missing")
	}
}

// With a height but no scale, the fat-free mass is estimated from BMI and
// everything is flagged estimated.
func TestTargetsEstimatedFromBMI(t *testing.T) {
	got := Targets(Input{WeightKg: 80, HeightCm: 180, AgeYears: 30, Male: true})
	if !got.HasProtein() || !got.HasEnergy() {
		t.Fatal("expected protein and energy to be estimated")
	}
	if got.ProteinBasis != models.NutritionBasisEstimated || !got.Estimated {
		t.Errorf("expected estimated basis, got %q estimated=%v", got.ProteinBasis, got.Estimated)
	}
	// Deurenberg body fat ~20.3%, so FFM ~63.7 kg.
	approx(t, "ffm", got.FatFreeMassKg, 63.7, 0.5)
	// Mifflin-St Jeor resting for these inputs is 1780 kcal.
	approx(t, "resting", got.RestingKcal, 1780, 1)
}

func TestBMI(t *testing.T) {
	approx(t, "bmi", BMI(80, 180), 24.69, 0.05)
	if BMI(80, 0) != 0 || BMI(0, 180) != 0 {
		t.Error("BMI with a missing input should be zero")
	}
}
