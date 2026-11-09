// Package healthsheet is what a member's health data looks like as a table:
// which columns it has, and what goes in each cell.
//
// The same reason programsheet exists. The export is produced in two formats -
// a picture to paste into a conversation, a PDF to keep or print - and if each
// renderer decided for itself what a cell meant, the two would drift. So the
// renderers own layout and nothing else: they are handed these rows.
package healthsheet

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programsheet"
)

// The colours of the app's own screens, so a member reads the export the way
// they read the page: four walking bands for steps, and the provider's four
// heart rate zones from blue to red.
var (
	stepsLow  = color.RGBA{220, 38, 38, 255}
	stepsFair = color.RGBA{217, 119, 6, 255}
	stepsGood = color.RGBA{101, 195, 122, 255}
	stepsHigh = color.RGBA{22, 163, 74, 255}

	zoneColors = map[string]color.RGBA{
		devicelink.ZoneLight:    {59, 130, 246, 255},
		devicelink.ZoneModerate: {234, 179, 8, 255},
		devicelink.ZoneVigorous: {249, 115, 22, 255},
		devicelink.ZonePeak:     {220, 38, 38, 255},
	}
)

// StepTone is the band a day's count falls in - the same thresholds as the
// app's bars.
func StepTone(steps int64) color.RGBA {
	switch {
	case steps < 6000:
		return stepsLow
	case steps < 10000:
		return stepsFair
	case steps < 15000:
		return stepsGood
	}
	return stepsHigh
}

// Sections is the whole export: steps, then heart rate when there is any,
// then the latest body composition when a scale is connected.
func Sections(steps models.StepSeries, heart *models.HeartSeries,
	body *models.BodyComposition, locale string) []programsheet.Section {
	sections := []programsheet.Section{StepsSection(steps, locale)}
	if heart != nil {
		sections = append(sections, HeartSection(*heart, locale))
	}
	if body != nil && len(body.Measures) > 0 {
		sections = append(sections, BodySection(*body, locale))
	}
	return sections
}

// AllSections is every part of the one fitness export, in reading order and
// each only when there is something behind it: steps and heart rate (when an
// activity source is connected), the nutrition the member logged and their
// daily targets (when a food app is connected, or their body is known), the
// latest body composition (when a scale is), and the bar-speed charts (when a
// tracker is). Unlike Sections it leaves steps out entirely when there are
// none, rather than drawing an empty chart - this one sheet is assembled from
// whatever the member has, not from one provider that must exist.
//
// Nutrition sits just under the heart rate and before the bar speed: the diary
// charts read like the step chart they follow, and the daily targets are the
// number an athlete and coach read against what was eaten to decide cut, bulk
// or maintain.
func AllSections(steps models.StepSeries, heart *models.HeartSeries,
	nutrition *models.NutritionSeries, targets *models.NutritionTargets,
	body *models.BodyComposition, barSets []models.BarSet, locale string) []programsheet.Section {
	var sections []programsheet.Section
	if len(steps.Days) > 0 {
		sections = append(sections, StepsSection(steps, locale))
		if heart != nil {
			sections = append(sections, HeartSection(*heart, locale))
		}
	}
	sections = append(sections, NutritionSections(nutrition, targets, locale)...)
	if body != nil && len(body.Measures) > 0 {
		sections = append(sections, BodySection(*body, locale))
	}
	sections = append(sections, BarSpeedSections(barSets, locale)...)
	return sections
}

// StepsSection is a histogram of the range, a bar a day coloured by its band.
func StepsSection(steps models.StepSeries, locale string) programsheet.Section {
	chart := &programsheet.Chart{
		Legend: []programsheet.LegendEntry{
			{Label: programsheet.Heading(locale, "stepsLow"), Color: stepsLow},
			{Label: programsheet.Heading(locale, "stepsFair"), Color: stepsFair},
			{Label: programsheet.Heading(locale, "stepsGood"), Color: stepsGood},
			{Label: programsheet.Heading(locale, "stepsHigh"), Color: stepsHigh},
		},
	}
	for _, day := range steps.Days {
		chart.Bars = append(chart.Bars, programsheet.Bar{
			Label:   shortDate(day.Date),
			Value:   float64(day.Steps),
			Caption: formatInt(day.Steps),
			Color:   StepTone(day.Steps),
		})
	}
	return programsheet.Section{Heading: programsheet.Heading(locale, "stepsChart"), Chart: chart}
}

// HeartSection is a histogram of the days with heart rate: each bar as tall as
// the day's highest reading, captioned with its average, coloured by the
// hardest zone reached. A day the watch recorded nothing is a flat sliver -
// nobody's heart beat zero times, and a bar there would be read as one.
func HeartSection(heart models.HeartSeries, locale string) programsheet.Section {
	chart := &programsheet.Chart{}
	for _, zone := range []string{devicelink.ZoneLight, devicelink.ZoneModerate,
		devicelink.ZoneVigorous, devicelink.ZonePeak} {
		chart.Legend = append(chart.Legend,
			programsheet.LegendEntry{Label: ZoneLabel(locale, zone), Color: zoneColors[zone]})
	}
	for _, day := range heart.Days {
		bar := programsheet.Bar{Label: shortDate(day.Date)}
		if day.Known {
			bar.Value = float64(day.Max)
			if bar.Value == 0 {
				bar.Value = float64(day.Avg)
			}
			bar.Caption = strconv.Itoa(day.Avg)
			bar.Color = zoneColors[day.Zone]
		}
		chart.Bars = append(chart.Bars, bar)
	}
	return programsheet.Section{Heading: programsheet.Heading(locale, "heartChart"), Chart: chart}
}

// BodySection is the latest reading of each measure, as a table: one value per
// measure is a list, not a series, and a bar chart of kilograms beside
// percentages beside kilocalories would compare things that do not compare.
func BodySection(body models.BodyComposition, locale string) programsheet.Section {
	rows := make([]programsheet.Line, 0, len(body.Measures))
	for _, measure := range body.Measures {
		value := strconv.FormatFloat(measure.Value, 'f', -1, 64)
		if unit := displayUnit(measure.Unit); unit != "" {
			value += " " + unit
		}
		rows = append(rows, programsheet.Line{Cells: []string{
			programsheet.Heading(locale, "bc."+measure.Type),
			value,
			measure.TakenAt.Format("2006-01-02"),
		}})
	}
	return programsheet.Section{
		Heading: programsheet.Heading(locale, "bodyComposition"),
		Columns: []programsheet.Column{
			{Header: programsheet.Heading(locale, "measure"), Weight: 1.8},
			{Header: programsheet.Heading(locale, "value"), Weight: 1.2},
			{Header: programsheet.Heading(locale, "measuredOn"), Weight: 1.2},
		},
		Rows: rows,
	}
}

// Nutrition bar colours. Protein reads against its target - amber when the day
// fell short of the minimum dose, green when it met it. Energy reads against
// maintenance - blue for a deficit (a cut), green for maintenance, amber for a
// surplus (a bulk) - so the cut/bulk/maintain shape is visible without reading
// a single number. A neutral navy stands in when there is no target to judge
// against.
var (
	nutShort   = color.RGBA{217, 119, 6, 255}  // amber - under the protein target
	nutMet     = color.RGBA{22, 163, 74, 255}   // green - protein met, or maintenance
	nutDeficit = color.RGBA{59, 130, 246, 255}  // blue - an energy deficit
	nutSurplus = color.RGBA{217, 119, 6, 255}   // amber - an energy surplus
	nutNeutral = color.RGBA{14, 94, 155, 255}   // navy - no target to judge against
)

// NutritionSections is the member's logged nutrition and their daily targets:
// a protein-per-day histogram and an energy-per-day histogram when a food app
// is connected, then a small table of the ideal protein band and maintenance
// energy when their body is known enough to work them out. Returned as its own
// slice so the export drops the lot when there is neither.
func NutritionSections(series *models.NutritionSeries, targets *models.NutritionTargets,
	locale string) []programsheet.Section {
	var sections []programsheet.Section
	if series != nil && len(series.Days) > 0 {
		sections = append(sections, proteinSection(*series, targets, locale))
		sections = append(sections, energySection(*series, targets, locale))
	}
	if targets != nil && (targets.HasProtein() || targets.HasEnergy() || targets.NeedsHeight) {
		if section := targetsSection(*targets, locale); section != nil {
			sections = append(sections, *section)
		}
	}
	return sections
}

// proteinSection is a bar a day of protein eaten, each coloured by whether it
// met the minimum of the target band.
func proteinSection(series models.NutritionSeries, targets *models.NutritionTargets,
	locale string) programsheet.Section {
	chart := &programsheet.Chart{}
	min := 0.0
	if targets != nil {
		min = targets.ProteinMinG
	}
	for _, day := range series.Days {
		colour := nutNeutral
		if min > 0 {
			if day.ProteinG >= min {
				colour = nutMet
			} else {
				colour = nutShort
			}
		}
		chart.Bars = append(chart.Bars, programsheet.Bar{
			Label:   shortDate(day.Date),
			Value:   day.ProteinG,
			Caption: strconv.Itoa(int(day.ProteinG + 0.5)),
			Color:   colour,
		})
	}
	return programsheet.Section{Heading: programsheet.Heading(locale, "proteinChart"), Chart: chart}
}

// energySection is a bar a day of energy eaten, each coloured by where it fell
// against maintenance - a deficit, maintenance, or a surplus.
func energySection(series models.NutritionSeries, targets *models.NutritionTargets,
	locale string) programsheet.Section {
	chart := &programsheet.Chart{}
	maintenance := 0.0
	if targets != nil {
		maintenance = targets.MaintenanceKcal
	}
	for _, day := range series.Days {
		colour := nutNeutral
		if maintenance > 0 {
			switch {
			case day.Kcal < maintenance*0.92:
				colour = nutDeficit
			case day.Kcal > maintenance*1.08:
				colour = nutSurplus
			default:
				colour = nutMet
			}
		}
		chart.Bars = append(chart.Bars, programsheet.Bar{
			Label:   shortDate(day.Date),
			Value:   day.Kcal,
			Caption: formatInt(int64(day.Kcal + 0.5)),
			Color:   colour,
		})
	}
	return programsheet.Section{Heading: programsheet.Heading(locale, "kcalChart"), Chart: chart}
}

// targetsSection is the ideal protein band and maintenance energy as a small
// table, or - when the height is missing and nothing could be computed - a
// single line asking for it. A trailing note flags the whole thing as an
// estimate when any input was guessed.
func targetsSection(targets models.NutritionTargets, locale string) *programsheet.Section {
	if targets.NeedsHeight {
		return &programsheet.Section{
			Columns: []programsheet.Column{{Header: programsheet.Heading(locale, "nutritionTargets"), Weight: 1}},
			Rows:    []programsheet.Line{{Cells: []string{programsheet.Heading(locale, "nt.needHeight")}}},
		}
	}
	rows := []programsheet.Line{}
	if targets.HasProtein() {
		// The rule first, then the kilograms it works out to: "1.6-2.2 g x 64 kg
		// FFM = 102-141 g", so the member sees where the band comes from.
		rows = append(rows, programsheet.Line{Cells: []string{
			programsheet.Heading(locale, "nt.idealProtein"),
			fmt.Sprintf(programsheet.Heading(locale, "nt.proteinFormula"),
				trimFloat(targets.ProteinPerKgMin), trimFloat(targets.ProteinPerKgMax),
				strconv.Itoa(int(targets.FatFreeMassKg+0.5)),
				strconv.Itoa(int(targets.ProteinMinG+0.5)), strconv.Itoa(int(targets.ProteinMaxG+0.5))),
		}})
	}
	if targets.HasEnergy() {
		rows = append(rows, programsheet.Line{Cells: []string{
			programsheet.Heading(locale, "nt.maintenanceEnergy"),
			fmt.Sprintf(programsheet.Heading(locale, "nt.energy"), formatInt(int64(targets.MaintenanceKcal+0.5))),
		}})
	}
	if len(rows) == 0 {
		return nil
	}
	if targets.Estimated {
		rows = append(rows, programsheet.Line{Cells: []string{programsheet.Heading(locale, "nt.estimate"), ""}})
	}
	return &programsheet.Section{
		Heading: programsheet.Heading(locale, "nutritionTargets"),
		Columns: []programsheet.Column{
			{Header: programsheet.Heading(locale, "measure"), Weight: 1.8},
			{Header: programsheet.Heading(locale, "value"), Weight: 1.2},
		},
		Rows: rows,
	}
}

// displayUnit is the unit as printed. Squared metres are spelt out by the API
// so a client is never handed a character it cannot encode.
func displayUnit(unit string) string {
	if unit == "kg/m2" {
		return "kg/m²"
	}
	return unit
}

// shortDate is the label under a bar: day and month, which is all the room a
// bar of a ninety-day range has.
func shortDate(date string) string {
	if len(date) == len("2006-01-02") {
		return date[8:10] + "/" + date[5:7]
	}
	return date
}

// Title is what the sheet is called: whose data, over what.
func Title(memberName string, locale string) string {
	name := strings.TrimSpace(memberName)
	if name == "" {
		return strings.TrimSuffix(programsheet.Heading(locale, "healthFor"), " - %s")
	}
	return fmt.Sprintf(programsheet.Heading(locale, "healthFor"), name)
}

// Subtitle says what the numbers add up to, in a sentence - the thing somebody
// repeats when they forward the picture.
func Subtitle(steps models.StepSeries, heart *models.HeartSeries, locale string) string {
	parts := []string{
		fmt.Sprintf("%s - %s", steps.From, steps.To),
		fmt.Sprintf(programsheet.Heading(locale, "stepsAvg"), formatInt(steps.DailyAverage)),
	}

	switch {
	case heart == nil || heart.AverageBPM == 0:
		parts = append(parts, programsheet.Heading(locale, "bpmNone"))
	default:
		// Said explicitly, because the heart rate covers fewer days than the
		// steps whenever the range is a wide one - a reader comparing the two
		// columns deserves to know why one stops early.
		parts = append(parts, fmt.Sprintf(programsheet.Heading(locale, "bpmRange"),
			len(heart.Days), heart.AverageBPM, heart.PeakBPM))
	}
	return strings.Join(parts, "  -  ")
}

// ZoneLabel names one of the provider's four bands.
func ZoneLabel(locale, zone string) string {
	switch zone {
	case devicelink.ZoneLight:
		return programsheet.Heading(locale, "zoneLight")
	case devicelink.ZoneModerate:
		return programsheet.Heading(locale, "zoneModerate")
	case devicelink.ZoneVigorous:
		return programsheet.Heading(locale, "zoneVigorous")
	case devicelink.ZonePeak:
		return programsheet.Heading(locale, "zonePeak")
	}
	return programsheet.Heading(locale, "zoneNone")
}

// formatInt groups thousands with a thin space, which reads the same in both
// languages the sheet is produced in.
func formatInt[T int | int64](value T) string {
	digits := strconv.FormatInt(int64(value), 10)
	var out strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
	}
	return out.String()
}

// One colour per lift, so the three charts read apart at a glance and match
// the lift colours the apps use elsewhere: squat yellow, bench red, deadlift
// green. A velocity has no good/bad bands the way steps do, so the colour
// names the lift rather than judging the number.
var (
	liftSquatColor    = color.RGBA{234, 179, 8, 255} // yellow
	liftBenchColor    = color.RGBA{220, 38, 38, 255} // red
	liftDeadliftColor = color.RGBA{22, 163, 74, 255} // green
	liftOtherColor    = color.RGBA{14, 94, 155, 255} // fallback navy
)

// liftBarColor is the chart colour for one lift.
func liftBarColor(lift string) color.RGBA {
	switch lift {
	case devicelink.LiftSquat:
		return liftSquatColor
	case devicelink.LiftBench:
		return liftBenchColor
	case devicelink.LiftDeadlift:
		return liftDeadliftColor
	}
	return liftOtherColor
}

// BarSpeedSections is one chart per tracked lift: a bar per rep, its height
// the rep's mean concentric velocity, under a heading that carries the set's
// weight, reps and the two RPEs (what the member said, what the bar said).
//
// Returned as its own slice so the export appends it after steps and heart
// rate, and skips it whole when the member tracks no bar speed.
func BarSpeedSections(sets []models.BarSet, locale string) []programsheet.Section {
	sections := make([]programsheet.Section, 0, len(sets))
	for _, set := range sets {
		if len(set.RepVelocities) == 0 {
			continue
		}
		colour := liftBarColor(set.Lift)
		chart := &programsheet.Chart{}
		for i, velocity := range set.RepVelocities {
			chart.Bars = append(chart.Bars, programsheet.Bar{
				Label:   fmt.Sprintf(programsheet.Heading(locale, "repN"), i+1),
				Value:   velocity,
				Caption: formatVelocity(velocity),
				Color:   colour,
			})
		}
		sections = append(sections, programsheet.Section{
			Heading: barSpeedHeading(set, locale),
			Chart:   chart,
		})
	}
	return sections
}

// barSpeedHeading says what the set was, so the chart reads without a legend:
// the lift, the load and reps, the given vs predicted RPE when both are there
// (the whole point of the tracker is the gap between them), and the peak speed
// and velocity loss the bars alone cannot show.
func barSpeedHeading(set models.BarSet, locale string) string {
	parts := []string{LiftLabel(locale, set.Lift)}
	if set.WeightKg > 0 && set.Reps > 0 {
		parts = append(parts, fmt.Sprintf(programsheet.Heading(locale, "barSpeedLoad"),
			trimFloat(set.WeightKg), set.Reps))
	}
	if set.GivenRpe > 0 && set.PredictedRpe > 0 {
		parts = append(parts, fmt.Sprintf(programsheet.Heading(locale, "barSpeedRpePair"),
			trimFloat(set.GivenRpe), trimFloat(set.PredictedRpe)))
	} else if set.PredictedRpe > 0 {
		parts = append(parts, fmt.Sprintf(programsheet.Heading(locale, "barSpeedRpeReal"),
			trimFloat(set.PredictedRpe)))
	}
	if set.PeakVelocity > 0 {
		parts = append(parts, fmt.Sprintf(programsheet.Heading(locale, "barSpeedPeak"),
			formatVelocity(set.PeakVelocity)))
	}
	if loss := VelocityLoss(set.RepVelocities); loss > 0 {
		parts = append(parts, fmt.Sprintf(programsheet.Heading(locale, "barSpeedLoss"),
			strconv.Itoa(int(loss+0.5))))
	}
	// The descriptive tags last, so a reader - and the AI - knows what the set
	// was: a top set's speed is the effort signal, a warm-up's is not, and a
	// tempo/paused variation is slow on purpose, not from fatigue.
	switch models.NormalizeSetType(set.SetType) {
	case models.SetTypeTopset:
		parts = append(parts, programsheet.Heading(locale, "barSpeedSetTypeTopset"))
	case models.SetTypeBackoff:
		parts = append(parts, programsheet.Heading(locale, "barSpeedSetTypeBackoff"))
	default:
		parts = append(parts, programsheet.Heading(locale, "barSpeedSetTypeWarmup"))
	}
	if set.TempoVariation {
		parts = append(parts, programsheet.Heading(locale, "barSpeedTempo"))
	}
	return strings.Join(parts, "  -  ")
}

// VelocityLoss is how much the bar slowed across the set, as a percentage of
// the fastest rep - the standard fatigue reading a velocity tracker gives. 0
// when there is nothing to compare (one rep, or no slowing).
func VelocityLoss(repVelocities []float64) float64 {
	if len(repVelocities) < 2 {
		return 0
	}
	best, last := repVelocities[0], repVelocities[0]
	for _, v := range repVelocities {
		if v > best {
			best = v
		}
		last = v
	}
	if best <= 0 || last >= best {
		return 0
	}
	return (best - last) / best * 100
}

// LiftLabel names one of the three lifts in the reader's language.
func LiftLabel(locale, lift string) string {
	switch lift {
	case devicelink.LiftSquat:
		return programsheet.Heading(locale, "liftSquat")
	case devicelink.LiftBench:
		return programsheet.Heading(locale, "liftBench")
	case devicelink.LiftDeadlift:
		return programsheet.Heading(locale, "liftDeadlift")
	}
	return lift
}

// formatVelocity is a bar's caption: two decimals and the unit, "0.42 m/s".
func formatVelocity(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".") + " m/s"
}

// trimFloat writes a measurement without a pointless decimal.
func trimFloat(value float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
}
