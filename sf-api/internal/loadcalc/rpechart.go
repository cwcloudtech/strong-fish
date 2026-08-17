package loadcalc

import "math"

// The RPE chart: what fraction of a one-rep max a set of N reps at a given RPE
// represents. This is the standard Tuchscherer/RTS table every RPE-based
// powerlifting program is written against, and it's the table the reference
// spreadsheet's Percentage column was hand-copied from - each of its integers
// sits within about a point of the corresponding cell here (see
// TestChartMatchesSpreadsheet).
//
// Rows are reps 1..12; columns are RPE 10 down to 6 in half-point steps.
// Values are percentages of the 1RM.
var rpeChart = [][]float64{
	// RPE:  10    9.5    9     8.5    8     7.5    7     6.5    6
	/*  1 */ {100.0, 97.8, 95.5, 93.9, 92.2, 90.7, 89.2, 87.8, 86.3},
	/*  2 */ {95.5, 93.9, 92.2, 90.7, 89.2, 87.8, 86.3, 85.0, 83.7},
	/*  3 */ {92.2, 90.7, 89.2, 87.8, 86.3, 85.0, 83.7, 82.4, 81.1},
	/*  4 */ {89.2, 87.8, 86.3, 85.0, 83.7, 82.4, 81.1, 80.0, 78.6},
	/*  5 */ {86.3, 85.0, 83.7, 82.4, 81.1, 80.0, 78.6, 77.4, 76.2},
	/*  6 */ {83.7, 82.4, 81.1, 80.0, 78.6, 77.4, 76.2, 75.1, 73.9},
	/*  7 */ {81.1, 80.0, 78.6, 77.4, 76.2, 75.1, 73.9, 72.3, 70.7},
	/*  8 */ {78.6, 77.4, 76.2, 75.1, 73.9, 72.3, 70.7, 69.4, 68.0},
	/*  9 */ {76.2, 75.1, 73.9, 72.3, 70.7, 69.4, 68.0, 66.7, 65.3},
	/* 10 */ {73.9, 72.3, 70.7, 69.4, 68.0, 66.7, 65.3, 64.0, 62.6},
	/* 11 */ {70.7, 69.4, 68.0, 66.7, 65.3, 64.0, 62.6, 61.3, 60.0},
	/* 12 */ {68.0, 66.7, 65.3, 64.0, 62.6, 61.3, 60.0, 58.7, 57.4},
}

const (
	// maxRPE is a set taken to failure - zero reps left in reserve, and the
	// chart's first column.
	maxRPE = 10.0
	// minChartRPE is the chart's last column. Below it the row is continued at
	// the slope of its final half-point step rather than being clamped: the
	// reference program does prescribe RPE 5 work, and a flat clamp would
	// prescribe RPE 6 loads for it.
	minChartRPE = 6.0
	// rpeStep is the spacing between chart columns.
	rpeStep = 0.5
	// minIntensity floors an extrapolated row so a nonsensical prescription
	// (30 reps at RPE 1) can never resolve to a zero or negative load.
	minIntensity = 0.2
)

// Intensity is the fraction of the member's 1RM that reps at rpe corresponds
// to, read off the chart. A nil rpe means the coach didn't prescribe one, which
// reads as taken to failure (RPE 10) - the same way the spreadsheet's e1RM
// column treats a blank RPE.
//
// Values between chart columns are interpolated (a member may log RPE 8.5, and
// coaches do program half points); reps and RPEs past the table's edges
// continue at the slope of its last step.
func Intensity(reps int, rpe *float64) float64 {
	if reps < 1 {
		reps = 1
	}
	effective := maxRPE
	if rpe != nil {
		effective = math.Min(*rpe, maxRPE)
	}

	// Column position: 0 at RPE 10, growing by 1 per half point down.
	column := (maxRPE - effective) / rpeStep

	value := rowValue(reps, column)
	return math.Max(value/100, minIntensity)
}

// rowValue reads one reps-row at a fractional column position, interpolating
// between columns and continuing past the last one at its final slope.
func rowValue(reps int, column float64) float64 {
	row := chartRow(reps)
	last := len(row) - 1

	if column <= 0 {
		return row[0]
	}
	if column >= float64(last) {
		// Past RPE 6: continue at the slope of the 6.5 -> 6 step.
		return row[last] - (column-float64(last))*(row[last-1]-row[last])
	}

	lower := int(column)
	fraction := column - float64(lower)
	return row[lower] + fraction*(row[lower+1]-row[lower])
}

// chartRow returns the chart row for reps, continuing past the table's last
// row at the slope of its final rep step (the table stops at 12; powerlifting
// programs rarely go further, but a coach's accessory block can).
func chartRow(reps int) []float64 {
	if reps <= len(rpeChart) {
		return rpeChart[reps-1]
	}

	last := rpeChart[len(rpeChart)-1]
	previous := rpeChart[len(rpeChart)-2]
	extra := float64(reps - len(rpeChart))

	row := make([]float64, len(last))
	for i := range last {
		row[i] = last[i] - extra*(previous[i]-last[i])
	}
	return row
}
