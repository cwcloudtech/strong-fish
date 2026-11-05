package handlers

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"strong-fish-api/internal/models"
)

// synthSamples builds a set's stream from concentric peak velocities, the same
// way the engine's own tests do: each rep is a full down-then-up cycle, so the
// signal starts and ends at rest.
func synthSamples(rate float64, peaks []float64, repDur, gap, lead float64) []analyzeSample {
	dt := 1.0 / rate
	var accel []float64
	add := func(sec float64, f func(u float64) float64) {
		steps := int(math.Round(sec / dt))
		for k := 0; k < steps; k++ {
			accel = append(accel, f(float64(k)/float64(steps)))
		}
	}
	add(lead, func(float64) float64 { return 0 })
	for _, vp := range peaks {
		add(repDur, func(u float64) float64 {
			return -vp * (2 * math.Pi / repDur) * math.Cos(2*math.Pi*u)
		})
		add(gap, func(float64) float64 { return 0 })
	}
	out := make([]analyzeSample, len(accel))
	for i, a := range accel {
		out[i] = analyzeSample{T: float64(i) * dt, AZ: a + 9.80665, QW: 1}
	}
	return out
}

func postAnalyze(payload analyzePayload) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/bar-speed/analyze", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	// No auth context in the test, so the member id is empty and the handler
	// computes without touching the store - a nil store is enough here.
	NewBarSpeedHandler(nil).Analyze(rec, req)
	return rec
}

func TestMergeBarSetsKeepsMostRecentPerLift(t *testing.T) {
	now := time.Now()
	ours := []models.BarSet{
		{Lift: "squat", Source: "strongfish", PerformedAt: now},                      // newer squat
		{Lift: "bench", Source: "strongfish", PerformedAt: now.Add(-48 * time.Hour)}, // older bench
	}
	tracker := []models.BarSet{
		{Lift: "squat", Source: "stance", PerformedAt: now.Add(-24 * time.Hour)}, // older squat
		{Lift: "bench", Source: "stance", PerformedAt: now.Add(-1 * time.Hour)},  // newer bench
		{Lift: "deadlift", Source: "stance", PerformedAt: now.Add(-72 * time.Hour)},
	}

	got := mergeBarSets(ours, tracker)
	if len(got) != 3 {
		t.Fatalf("want 3 lifts, got %d", len(got))
	}
	source := map[string]string{}
	for _, set := range got {
		source[set.Lift] = set.Source
	}
	if source["squat"] != "strongfish" {
		t.Errorf("squat: want the newer strongfish set, got %s", source["squat"])
	}
	if source["bench"] != "stance" {
		t.Errorf("bench: want the newer stance set, got %s", source["bench"])
	}
	if source["deadlift"] != "stance" {
		t.Errorf("deadlift: want the only (stance) set, got %s", source["deadlift"])
	}
	// Newest first.
	for i := 1; i < len(got); i++ {
		if got[i-1].PerformedAt.Before(got[i].PerformedAt) {
			t.Errorf("not sorted newest first at %d", i)
		}
	}
}

func TestBarSpeedAnalyzeComputesSet(t *testing.T) {
	rec := postAnalyze(analyzePayload{
		Movement: "Deadlift", Load: 180, Unit: "kg", SampleRateHz: 200,
		Samples: synthSamples(200, []float64{0.8, 0.7, 0.6}, 1.2, 0.8, 0.5),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var set models.BarSet
	if err := json.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if set.Lift != "deadlift" {
		t.Errorf("lift %q, want deadlift (lowercased)", set.Lift)
	}
	if set.WeightKg != 180 {
		t.Errorf("weight %v, want 180", set.WeightKg)
	}
	if set.Reps != 3 || len(set.RepVelocities) != 3 || len(set.RepPeakVelocities) != 3 {
		t.Fatalf("want 3 reps and 3+3 velocities, got %d (%d/%d)",
			set.Reps, len(set.RepVelocities), len(set.RepPeakVelocities))
	}
	if set.PeakVelocity < 0.6 {
		t.Errorf("set peak %v, want ~0.80", set.PeakVelocity)
	}
}

func TestBarSpeedAnalyzeRejectsBadMovement(t *testing.T) {
	rec := postAnalyze(analyzePayload{
		Movement: "curl", SampleRateHz: 200,
		Samples: synthSamples(200, []float64{0.8}, 1.2, 0.8, 0.5),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for a movement the engine does not read, got %d", rec.Code)
	}
}

func TestBarSpeedAnalyzeRejectsEmpty(t *testing.T) {
	rec := postAnalyze(analyzePayload{Movement: "squat", SampleRateHz: 200})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for no samples, got %d", rec.Code)
	}
}

func TestBarSpeedAnalyzeConvertsPounds(t *testing.T) {
	rec := postAnalyze(analyzePayload{
		Movement: "bench", Load: 100, Unit: "lb", SampleRateHz: 200,
		Samples: synthSamples(200, []float64{0.5}, 1.2, 0.8, 0.5),
	})
	var set models.BarSet
	_ = json.Unmarshal(rec.Body.Bytes(), &set)
	if math.Abs(set.WeightKg-45.4) > 0.2 {
		t.Errorf("100 lb -> %v kg, want ~45.4", set.WeightKg)
	}
}
