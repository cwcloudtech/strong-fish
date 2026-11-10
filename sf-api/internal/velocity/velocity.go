// Package velocity turns a raw accelerometer stream from one set into the
// numbers a bar-speed panel shows: a rep count, each rep's mean and peak
// concentric velocity, and how much the bar slowed across the set.
//
// It is StrongFish's own engine, built from classical velocity-based-training
// signal processing - estimate the bar's vertical acceleration, integrate it to
// velocity, find the concentric phases, read each one's speed - so it depends on
// no vendor's model and runs on any device that can post a stream of samples: a
// phone over Bluetooth, or a 5-euro board over HTTP. The movement is carried
// through so later tuning can differ per lift, but the method is the same for
// all three.
//
// The numbers are not meant to match any one tracker to the decimal; they are a
// consistent, independent reading of a real set.
package velocity

import "math"

// Sample is one IMU tick: raw linear acceleration on three body axes in m/s²
// (gravity still in it), and an optional orientation quaternion. T is seconds
// from the start of the set; it is advisory, the sample rate drives the maths.
type Sample struct {
	T              float64
	AX, AY, AZ     float64
	QW, QX, QY, QZ float64
}

// Capture is one set's stream and what it was.
type Capture struct {
	// Movement is "squat", "bench" or "deadlift"; carried for per-lift tuning,
	// not yet used to change the maths.
	Movement     string
	Samples      []Sample
	SampleRateHz float64
}

// Rep is one detected repetition's concentric velocity, in m/s.
type Rep struct {
	MeanMps  float64
	PeakMps  float64
	StartSec float64
	EndSec   float64
}

// Result is the computed set: the reps in order, the set's mean and peak speed,
// the velocity loss from the fastest rep to the last as a percentage, and a
// confidence in the reading between 0 and 1.
type Result struct {
	Reps         []Rep
	MeanVelocity float64
	PeakVelocity float64
	VelocityLoss float64
	// Confidence is how regular the detected reps were - a rough quality score,
	// low when the segmentation looks erratic (a noisy or badly-mounted sensor),
	// so a client can warn rather than present a shaky reading as fact.
	Confidence float64
}

// profile is the per-movement tuning of rep detection. The three lifts move
// differently - a deadlift pull is long and slow, a bench press short and
// quick - so each gets its own phase threshold and rep-duration bounds; an
// unknown movement falls back to the middle.
type profile struct {
	phaseThresholdFraction float64
	minRepDurationSec      float64
	maxRepDurationSec      float64
}

func movementProfile(movement string) profile {
	switch movement {
	case "deadlift":
		return profile{phaseThresholdFraction: 0.15, minRepDurationSec: 0.30, maxRepDurationSec: 6.0}
	case "bench":
		return profile{phaseThresholdFraction: 0.18, minRepDurationSec: 0.15, maxRepDurationSec: 4.0}
	case "squat":
		return profile{phaseThresholdFraction: 0.15, minRepDurationSec: 0.25, maxRepDurationSec: 5.0}
	}
	return profile{phaseThresholdFraction: phaseThresholdFraction, minRepDurationSec: 0.20, maxRepDurationSec: 6.0}
}

const (
	gravity = 9.80665

	// Two concentric phases this close together are the same rep, not two: the
	// bar dipped below the threshold only briefly - a grind's momentary stall at
	// the sticking point, or a patch of noise - and never actually came back
	// down. A real next rep is separated by the bar being lowered, which keeps it
	// under the threshold far longer than this. Keying the split off the gap's
	// length is what a controlled bench needs: its descent never crossed a
	// velocity floor cleanly, so reading the gap beats reading a reversal out of
	// a drifting integrated velocity. The per-rep duration bounds are per-movement
	// (see profile).
	minGapSec = 0.4

	// The velocity is smoothed over this window before reps are cut out of it,
	// so a single noisy sample does not open or close a phase.
	smoothWindowSec = 0.1
	// The time constant of the gravity estimate when there is no orientation to
	// rotate by: slow enough to hold still through a rep, quick enough to follow
	// the bar being re-racked at a new angle.
	gravityTauSec = 0.5

	// A concentric phase is where the smoothed velocity rises above this
	// fraction of the set's peak - relative, so a fast bench and a grinding
	// deadlift are each measured against their own top speed.
	phaseThresholdFraction = 0.15
	phaseThresholdFloor    = 0.05

	// For a BRIEF gap only, the tie-breaker between "same rep" and "next rep":
	// the bar either stalled (a sticking point, velocity stays positive) or was
	// dropped between touch-and-go reps (velocity goes clearly negative). A long
	// gap is a new rep regardless, so this never has to read a slow, drift-prone
	// descent - only a fast one, which is far too negative to be noise.
	reversalFloor = 0.1

	// When a span is longer than a single rep can be, it is re-cut at valleys
	// below this fraction of its own peak: the dips between reps the first pass
	// did not separate, either because the drop was too brief to be a gap or
	// because drift never let the bar fall below the phase threshold. Half the
	// peak sits well under a rep's own speed and well above the dip between two.
	valleyFraction = 0.5
)

// Analyze reads one set's stream into its reps and velocities. An empty or
// unreadable stream yields a zero Result rather than an error: a set the sensor
// could not make sense of is one with no reps, not a failed request.
func Analyze(c Capture) Result {
	n := len(c.Samples)
	if n < 8 || c.SampleRateHz <= 0 {
		return Result{}
	}
	dt := 1.0 / c.SampleRateHz

	accel := verticalAccel(c.Samples, dt)
	removeBias(accel)
	vel := integrate(accel, dt)
	smooth := uniformSmooth(vel, int(math.Round(smoothWindowSec/dt)))

	reps := detectReps(vel, smooth, dt, movementProfile(c.Movement))
	if len(reps) == 0 {
		return Result{}
	}

	result := Result{Reps: reps}
	var meanSum float64
	best := reps[0].MeanMps
	for _, rep := range reps {
		meanSum += rep.MeanMps
		if rep.PeakMps > result.PeakVelocity {
			result.PeakVelocity = rep.PeakMps
		}
		if rep.MeanMps > best {
			best = rep.MeanMps
		}
	}
	result.MeanVelocity = meanSum / float64(len(reps))

	last := reps[len(reps)-1].MeanMps
	if best > 0 && last < best {
		result.VelocityLoss = (best - last) / best * 100
	}
	result.Confidence = confidence(reps)
	return result
}

// confidence scores how regular the detected reps were, 0 to 1: consistent rep
// durations read as a clean capture, erratic ones as a noisy or badly-mounted
// sensor. A single rep has nothing to corroborate it, so it gets a middling
// score rather than a confident one.
func confidence(reps []Rep) float64 {
	if len(reps) == 0 {
		return 0
	}
	if len(reps) == 1 {
		return 0.7
	}
	durations := make([]float64, len(reps))
	var sum float64
	for i, rep := range reps {
		durations[i] = rep.EndSec - rep.StartSec
		sum += durations[i]
	}
	mean := sum / float64(len(durations))
	if mean <= 0 {
		return 0.5
	}
	var variance float64
	for _, d := range durations {
		variance += (d - mean) * (d - mean)
	}
	cv := math.Sqrt(variance/float64(len(durations))) / mean
	score := 1 - cv
	score = math.Max(0, math.Min(1, score))
	return math.Round(score*100) / 100
}

// verticalAccel reduces each tick to one number: the bar's acceleration along
// the vertical, with gravity removed, in m/s². With an orientation quaternion it
// rotates the body reading into the world frame and takes the vertical; without
// one it tracks gravity as a slow average and projects the linear acceleration
// onto it, which needs no fixed sensor mounting.
func verticalAccel(samples []Sample, dt float64) []float64 {
	out := make([]float64, len(samples))
	if hasOrientation(samples) {
		for i, s := range samples {
			out[i] = worldZ(s) - gravity
		}
		return out
	}

	alpha := dt / (gravityTauSec + dt)
	gx, gy, gz := samples[0].AX, samples[0].AY, samples[0].AZ
	for i, s := range samples {
		gx += alpha * (s.AX - gx)
		gy += alpha * (s.AY - gy)
		gz += alpha * (s.AZ - gz)
		norm := math.Sqrt(gx*gx + gy*gy + gz*gz)
		if norm < 1e-6 {
			out[i] = 0
			continue
		}
		// Linear acceleration (gravity removed) projected onto the gravity
		// direction: the component that moves the bar up and down.
		lx, ly, lz := s.AX-gx, s.AY-gy, s.AZ-gz
		out[i] = (lx*gx + ly*gy + lz*gz) / norm
	}
	return out
}

func hasOrientation(samples []Sample) bool {
	for _, s := range samples {
		if s.QW != 0 || s.QX != 0 || s.QY != 0 || s.QZ != 0 {
			return true
		}
	}
	return false
}

// worldZ rotates the body-frame acceleration into the world frame by the
// sample's quaternion and returns the vertical component (third row of the
// rotation matrix dotted with the reading), still including gravity.
func worldZ(s Sample) float64 {
	w, x, y, z := s.QW, s.QX, s.QY, s.QZ
	return 2*(x*z-w*y)*s.AX + 2*(y*z+w*x)*s.AY + (1-2*(x*x+y*y))*s.AZ
}

// integrate turns acceleration into velocity by the trapezoidal rule.
func integrate(accel []float64, dt float64) []float64 {
	vel := make([]float64, len(accel))
	for i := 1; i < len(accel); i++ {
		vel[i] = vel[i-1] + (accel[i]+accel[i-1])*0.5*dt
	}
	return vel
}

// removeBias subtracts the average acceleration, in place. A constant sensor
// bias would integrate into a velocity ramping across the whole set; a real set
// starts and ends at rest, so its true vertical acceleration averages to zero
// and whatever is left in the mean is that bias. Taking it out keeps the
// integrated velocity from drifting, without touching the shape of a rep (unlike
// detrending the velocity itself, which mistakes a rep's down-then-up for a
// trend and bends the baseline).
func removeBias(accel []float64) {
	if len(accel) == 0 {
		return
	}
	var sum float64
	for _, a := range accel {
		sum += a
	}
	mean := sum / float64(len(accel))
	for i := range accel {
		accel[i] -= mean
	}
}

// detectReps cuts the velocity into concentric phases - stretches where the
// smoothed velocity is clearly positive - and reads each one's mean and peak off
// the un-smoothed velocity. Phases too short to be a rep are dropped; phases
// barely apart are one rep, not two.
func detectReps(vel, smooth []float64, dt float64, prof profile) []Rep {
	peak := 0.0
	for _, v := range smooth {
		if v > peak {
			peak = v
		}
	}
	if peak <= 0 {
		return nil
	}
	threshold := math.Max(phaseThresholdFloor, prof.phaseThresholdFraction*peak)
	minLen := int(math.Round(prof.minRepDurationSec / dt))
	maxLen := int(math.Round(prof.maxRepDurationSec / dt))
	minGap := int(math.Round(minGapSec / dt))

	type span struct{ start, end int }
	var spans []span
	i := 0
	for i < len(smooth) {
		if smooth[i] <= threshold {
			i++
			continue
		}
		start := i
		for i < len(smooth) && smooth[i] > threshold {
			i++
		}
		end := i // exclusive
		// Two concentric phases are the same rep only when the bar neither paused
		// long nor came down between them. A LONG gap is the bar being lowered -
		// a new rep - whatever the velocity did, which is drift-immune and is what
		// keeps a slow, controlled bench from merging into one phase (the earlier
		// "0 reps"). A BRIEF gap is ambiguous, so the tie-breaker is whether the
		// bar actually went down: a sticking-point stall stays positive and is the
		// same rep, a fast drop between touch-and-go reps goes clearly negative and
		// is the next one. No lift-off removal - a stray un-rack is corrected on the
		// review screen, not guessed at here.
		if len(spans) > 0 {
			prevEnd := spans[len(spans)-1].end
			brief := start-prevEnd < minGap
			cameDown := minValue(smooth, prevEnd, start) < -reversalFloor
			if brief && !cameDown {
				spans[len(spans)-1].end = end
				continue
			}
		}
		spans = append(spans, span{start, end})
	}

	var reps []Rep
	for _, s := range spans {
		reps = append(reps, repsFromSpan(vel, smooth, s.start, s.end, dt, minLen, maxLen, minGap)...)
	}
	return reps
}

// repsFromSpan turns one above-threshold span into reps. A span no longer than
// a single rep is one rep. A longer one - touch-and-go reps whose drop was too
// brief to separate, or a set whose drift never let the bar dip below the phase
// threshold between reps - is re-cut at its own deeper valleys (below half its
// peak), so a whole set is never dropped as "0 reps". If even that finds
// nothing, the span still counts once rather than vanishing.
func repsFromSpan(vel, smooth []float64, start, end int, dt float64, minLen, maxLen, minGap int) []Rep {
	if end-start < minLen {
		return nil
	}
	if end-start <= maxLen {
		return []Rep{makeRep(vel, start, end, dt)}
	}

	var peak float64
	for j := start; j < end; j++ {
		if smooth[j] > peak {
			peak = smooth[j]
		}
	}
	level := valleyFraction * peak

	var subs [][2]int
	i := start
	for i < end {
		if smooth[i] <= level {
			i++
			continue
		}
		s := i
		for i < end && smooth[i] > level {
			i++
		}
		// Glue a brief dip below the level back on: it is a wobble inside a rep,
		// not the valley between two.
		if len(subs) > 0 && s-subs[len(subs)-1][1] < minGap {
			subs[len(subs)-1][1] = i
			continue
		}
		subs = append(subs, [2]int{s, i})
	}

	var reps []Rep
	for _, sp := range subs {
		if sp[1]-sp[0] >= minLen {
			reps = append(reps, makeRep(vel, sp[0], sp[1], dt))
		}
	}
	if len(reps) == 0 {
		reps = append(reps, makeRep(vel, start, end, dt))
	}
	return reps
}

// makeRep reads one span's concentric speed: its mean and peak velocity, and
// when it ran.
func makeRep(vel []float64, start, end int, dt float64) Rep {
	var sum, max float64
	for j := start; j < end; j++ {
		sum += vel[j]
		if vel[j] > max {
			max = vel[j]
		}
	}
	count := float64(end - start)
	return Rep{
		MeanMps:  sum / count,
		PeakMps:  max,
		StartSec: float64(start) * dt,
		EndSec:   float64(end) * dt,
	}
}

// minValue is the smallest value in data over [lo, hi); +Inf for an empty range,
// so two phases with no samples between them read as "no reversal" and merge.
func minValue(data []float64, lo, hi int) float64 {
	min := math.Inf(1)
	for i := lo; i < hi && i < len(data); i++ {
		if data[i] < min {
			min = data[i]
		}
	}
	return min
}

// uniformSmooth is a centred moving average of odd window length, for taking the
// jitter out of the velocity before phases are cut from it.
func uniformSmooth(data []float64, window int) []float64 {
	if window < 3 {
		return append([]float64(nil), data...)
	}
	if window%2 == 0 {
		window++
	}
	return movingAverage(data, window)
}

// movingAverage is a centred average over window samples, clamped at the ends so
// the output is the same length as the input.
func movingAverage(data []float64, window int) []float64 {
	out := make([]float64, len(data))
	if window < 1 {
		copy(out, data)
		return out
	}
	half := window / 2
	for i := range data {
		lo := i - half
		if lo < 0 {
			lo = 0
		}
		hi := i + half
		if hi >= len(data) {
			hi = len(data) - 1
		}
		var sum float64
		for j := lo; j <= hi; j++ {
			sum += data[j]
		}
		out[i] = sum / float64(hi-lo+1)
	}
	return out
}
