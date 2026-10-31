package healthsheet

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/programpdf"
	"strong-fish-api/internal/programpng"
)

// sample is a fortnight of steps and heart rate, one day without the watch,
// and a scale's latest readings.
func sample(days int) (models.StepSeries, *models.HeartSeries, *models.BodyComposition) {
	end := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	steps := models.StepSeries{}
	heart := &models.HeartSeries{AverageBPM: 68, PeakBPM: 171}
	zones := []string{devicelink.ZoneLight, devicelink.ZoneModerate, devicelink.ZoneVigorous, devicelink.ZonePeak}
	for i := days - 1; i >= 0; i-- {
		date := end.AddDate(0, 0, -i).Format("2006-01-02")
		count := int64((i*3779)%17000 + 1500)
		steps.Days = append(steps.Days, models.StepDay{Date: date, Steps: count})
		steps.Total += count
		day := models.HeartDay{Date: date, Avg: 60 + i%15, Max: 120 + (i*7)%55, Zone: zones[i%4], Known: i%6 != 3}
		if i < 14 {
			heart.Days = append(heart.Days, day)
		}
	}
	steps.From, steps.To = steps.Days[0].Date, steps.Days[len(steps.Days)-1].Date
	steps.DailyAverage = steps.Total / int64(len(steps.Days))
	body := &models.BodyComposition{Provider: "withings", Measures: []models.BodyMeasure{
		{Type: devicelink.MeasureWeight, Value: 82.4, Unit: "kg", TakenAt: end},
		{Type: devicelink.MeasureFatRatio, Value: 17.9, Unit: "%", TakenAt: end, Adjusted: ptr(22.9)},
		{Type: devicelink.MeasureMuscleMass, Value: 64.2, Unit: "kg", TakenAt: end, Adjusted: ptr(60.1)},
		{Type: devicelink.MeasureBMI, Value: 24.6, Unit: "kg/m2", TakenAt: end},
	}}
	return steps, heart, body
}

// TestExportRenders draws both formats for a short and a long range. Set
// HEALTHSHEET_OUT to a directory to keep the files and look at them.
func TestExportRenders(t *testing.T) {
	out := os.Getenv("HEALTHSHEET_OUT")
	if out == "" {
		out = t.TempDir()
	}
	for _, days := range []int{7, 90} {
		steps, heart, body := sample(days)
		sections := Sections(steps, heart, body, "en")
		if len(sections) != 3 || sections[0].Chart == nil || sections[1].Chart == nil || sections[2].Chart != nil {
			t.Fatalf("unexpected sections: %+v", sections)
		}
		if len(sections[0].Chart.Bars) != days {
			t.Fatalf("expected %d step bars, got %d", days, len(sections[0].Chart.Bars))
		}
		title := Title("Jo Doe", "en")
		subtitle := Subtitle(steps, heart, "en")

		image, err := programpng.RenderTables(title, subtitle, sections, "https://example.org")
		if err != nil || !bytes.HasPrefix(image, []byte("\x89PNG")) {
			t.Fatalf("png: %v", err)
		}
		document, err := programpdf.RenderTables(title, subtitle, sections, "https://example.org")
		if err != nil || !bytes.HasPrefix(document, []byte("%PDF")) {
			t.Fatalf("pdf: %v", err)
		}
		_ = os.WriteFile(filepath.Join(out, fmt.Sprintf("health-%d.png", days)), image, 0o644)
		_ = os.WriteFile(filepath.Join(out, fmt.Sprintf("health-%d.pdf", days)), document, 0o644)
	}
}

func ptr(value float64) *float64 { return &value }

func TestBodySectionShowsTheAdjustedValue(t *testing.T) {
	_, _, body := sample(7)
	rows := BodySection(*body, "en").Rows
	if got := rows[0].Cells[1]; got != "82.4 kg" {
		t.Errorf("weight: got %q", got)
	}
	if got := rows[1].Cells[1]; got != "17.9 % (~ 22.9 %)" {
		t.Errorf("fat ratio: got %q", got)
	}
}

func TestStepTone(t *testing.T) {
	for steps, want := range map[int64]any{5999: stepsLow, 6000: stepsFair, 10000: stepsGood, 15000: stepsHigh} {
		if StepTone(steps) != want {
			t.Errorf("%d: wrong band", steps)
		}
	}
}
