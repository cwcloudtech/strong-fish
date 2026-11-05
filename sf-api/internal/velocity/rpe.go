package velocity

import (
	"math"
	"sort"
)

// RPEPoint is one past set of the member's: what they rated it, and the mean
// velocity of its last (slowest) rep. The last rep is the effort signal, so it
// is what a felt RPE is paired with.
type RPEPoint struct {
	Rpe      float64
	Velocity float64
}

// PredictRPE estimates a set's RPE from its last rep's velocity, read against
// the member's own history of (felt RPE, last-rep velocity). It returns 0 when
// there is no usable history, or when the velocity falls outside everything the
// member has done - a confident guess with no basis is worse than none.
//
// The method is the standard velocity-based one, and is personal: each RPE's
// velocity band is the interquartile range of the velocities the member has
// produced at that RPE, and a new set's last-rep velocity, widened by ±20%, is
// matched against those bands, giving a range clamped to 5..10. The single
// number returned is the middle of that range, to the nearest half point.
func PredictRPE(history []RPEPoint, lastRepVelocity float64) float64 {
	if lastRepVelocity <= 0 {
		return 0
	}

	byRPE := map[float64][]float64{}
	for _, point := range history {
		if point.Rpe > 0 && point.Velocity > 0 {
			byRPE[point.Rpe] = append(byRPE[point.Rpe], point.Velocity)
		}
	}
	if len(byRPE) == 0 {
		return 0
	}

	type band struct{ rpe, lo, hi float64 }
	bands := make([]band, 0, len(byRPE))
	for rpe, velocities := range byRPE {
		lo, hi := velocityBand(velocities)
		bands = append(bands, band{rpe, lo, hi})
	}
	sort.Slice(bands, func(i, j int) bool { return bands[i].rpe < bands[j].rpe })

	// The window the set's velocity could plausibly belong to.
	low, high := lastRepVelocity*0.8, lastRepVelocity*1.2

	minRPE, maxRPE := 0.0, 0.0
	matched := false
	for _, b := range bands {
		if b.hi >= low && b.lo <= high {
			if !matched {
				minRPE = b.rpe
			}
			maxRPE = b.rpe
			matched = true
		}
	}
	if !matched {
		return 0
	}

	minRPE = math.Max(5, minRPE)
	maxRPE = math.Min(10, maxRPE)
	mid := (minRPE + maxRPE) / 2
	return math.Round(mid*2) / 2
}

// velocityBand is one RPE's velocity range: the interquartile range of the
// velocities seen at it, so a single stray rep does not widen it. One sample is
// a hair-wide band around itself; a degenerate quartile pair collapses the same
// way.
func velocityBand(velocities []float64) (lo, hi float64) {
	sorted := append([]float64(nil), velocities...)
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return sorted[0], sorted[0] + 0.02
	}
	q1 := percentile(sorted, 0.25)
	q3 := percentile(sorted, 0.75)
	if q3 <= q1 {
		mid := (q1 + q3) / 2
		return mid, mid + 0.02
	}
	return q1, q3
}

// percentile is the linear-interpolation percentile of a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	pos := p * float64(n-1)
	i := int(pos)
	if i+1 >= n {
		return sorted[n-1]
	}
	return sorted[i] + (pos-float64(i))*(sorted[i+1]-sorted[i])
}
