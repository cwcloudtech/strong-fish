// Package loadcalc turns a coach's prescription (reps, RPE, percentage) plus
// one member's own 1RM into the weight that member should actually put on the
// bar, and back again from what they lifted into an estimated 1RM.
//
// # What was wrong with the spreadsheet
//
// Powerlifting programs are written in RPE: "3 reps @ RPE 8" means three reps
// leaving two in reserve. How much of a one-rep max that represents is read off
// the standard RPE chart (see rpechart.go).
//
// The reference spreadsheet (ai-gen/assets/program.xlsx) doesn't compute its
// loads that way. Every row carries a Percentage column the coach typed in by
// hand from the chart, and the load is percentage/100 * 1RM. Two things go
// wrong with that:
//
//  1. The typed percentages disagree with each other. Across the five weeks,
//     eight distinct (reps, RPE) pairs are given two different percentages -
//     5 reps @ RPE 8 is 82% in one session and 78% in another, 3 reps @ RPE 8
//     is 87% in one and 81% in another. Whichever is right, both can't be.
//  2. They're frozen against whatever 1RM the author had in mind. A percentage
//     doesn't follow the member who actually runs the program, and it doesn't
//     move when they get stronger.
//
// Load fixes both by reading the chart directly against the member's own
// current 1RM:
//
//	load = 1RM * Intensity(reps, RPE)
//
// which is self-consistent (E1RM(Load(m)) == m exactly, so a set performed as
// prescribed estimates back to the max it was prescribed from - the
// spreadsheet's own e1RM column drifts by up to 3kg instead), and recomputes
// for free whenever the member updates their max, since no derived weight is
// ever stored.
//
// A percentage prescription stays supported for the sets a coach deliberately
// left without an RPE (the spreadsheet writes those as "?") - it's still
// applied to the member's own 1RM rather than the author's. Accessories keep an
// absolute weight, or none at all.
package loadcalc

import "math"

// Load modes.
const (
	// ModeRPE derives the load from reps at an RPE against the member's 1RM.
	ModeRPE = "rpe"
	// ModePercentage takes a fixed share of the member's 1RM, for sets the
	// coach prescribed without an RPE.
	ModePercentage = "percentage"
	// ModeAbsolute is a fixed weight in kilos, independent of any 1RM.
	ModeAbsolute = "absolute"
	// ModeBodyweight is a movement loaded by the athlete's own body (pull-ups,
	// dips): reps and RPE still apply, there's just no number to put on a bar.
	ModeBodyweight = "bodyweight"
)

// IsValidMode reports whether mode is one of the four load modes.
func IsValidMode(mode string) bool {
	switch mode {
	case ModeRPE, ModePercentage, ModeAbsolute, ModeBodyweight:
		return true
	}
	return false
}

// Prescription is one set as the coach wrote it. Reps is always meaningful;
// which of RPE/Percentage/AbsoluteLoad carries the load depends on Mode, and
// nil means the coach didn't write one.
type Prescription struct {
	Mode         string
	Reps         int
	RPE          *float64
	Percentage   *float64
	AbsoluteLoad *float64
}

// Result is what one member should do for a Prescription, given their 1RM.
type Result struct {
	// Load is the exact computed weight in kg, rounded to one decimal. Zero for
	// bodyweight sets, and for any prescription whose 1RM the member hasn't
	// entered yet (Known is then false).
	Load float64
	// RoundedLoad is Load snapped to a loadable increment (see
	// RoundToIncrement) - what actually goes on the bar.
	RoundedLoad float64
	// Percentage is the share of the member's 1RM that Load represents. For a
	// ModeRPE set this is the chart value the coach's hand-typed integer was
	// approximating; it's 0 when no 1RM is involved.
	Percentage float64
	// Known is false when the load can't be computed because the member has no
	// 1RM recorded for the exercise yet. The UI shows the spreadsheet's "?" in
	// that case and prompts them to enter one.
	Known bool
}

// Load resolves p against oneRM (the member's current max for the exercise, or
// nil when they haven't recorded one) into the weight they should lift.
func Load(p Prescription, oneRM *float64, increment float64) Result {
	switch p.Mode {
	case ModeBodyweight:
		// Nothing to compute: a pull-up is a pull-up. Reported as known so the
		// UI shows the set as ready rather than prompting for a 1RM.
		return Result{Known: true}

	case ModeAbsolute:
		if p.AbsoluteLoad == nil {
			return Result{}
		}
		return resolved(*p.AbsoluteLoad, 0, increment)

	case ModePercentage:
		if p.Percentage == nil || oneRM == nil || *oneRM <= 0 {
			return Result{}
		}
		return resolved(*p.Percentage/100**oneRM, *p.Percentage, increment)

	case ModeRPE:
		if oneRM == nil || *oneRM <= 0 {
			return Result{}
		}
		ratio := Intensity(p.Reps, p.RPE)
		return resolved(*oneRM*ratio, ratio*100, increment)
	}
	return Result{}
}

// resolved packages a computed weight into a Result.
func resolved(load, percentage, increment float64) Result {
	rounded := round1(load)
	return Result{
		Load:        rounded,
		RoundedLoad: RoundToIncrement(rounded, increment),
		Percentage:  round1(percentage),
		Known:       true,
	}
}

// E1RM estimates the one-rep max a completed set demonstrates - the
// spreadsheet's e1RM column, computed from what the member actually did rather
// than from what was prescribed. It's the exact inverse of Load's RPE branch,
// so a set performed as prescribed estimates back to the 1RM it came from.
//
// Returns 0 (unknown) for a set with no load, which is how the spreadsheet
// renders its bodyweight rows.
func E1RM(load float64, reps int, rpe *float64) float64 {
	if load <= 0 || reps <= 0 {
		return 0
	}
	return round1(load / Intensity(reps, rpe))
}

// RoundToIncrement snaps a weight to the nearest loadable step - 2.5kg by
// default, one small plate per side on a 20kg bar. A non-positive increment
// leaves the weight untouched, for gyms with fractional plates.
func RoundToIncrement(load, increment float64) float64 {
	if increment <= 0 {
		return round1(load)
	}
	return round1(math.Round(load/increment) * increment)
}

// round1 matches the source spreadsheet's ROUND(...,1).
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
