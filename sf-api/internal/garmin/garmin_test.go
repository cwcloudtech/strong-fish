package garmin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"strong-fish-api/internal/devicelink"
)

func TestPairKeyObtainsPlainTextToken(t *testing.T) {
	// The token endpoint returns the JWT as text/plain; asking for JSON is a
	// 406, so the request must not. The member's key authenticates it and is
	// kept as the refresh token.
	jwt := "hdr." + base64.RawURLEncoding.EncodeToString([]byte(`{"exp":4102444800}`)) + ".sig"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "ApiKey member-key" {
			t.Errorf("token auth header wrong: %q", r.Header.Get("Authorization"))
		}
		if a := r.Header.Get("Accept"); a != "" && a != "*/*" {
			t.Errorf("token Accept must not force JSON, got %q", a)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(jwt))
	}))
	defer server.Close()

	c := New(Config{TokenURL: server.URL})

	// Not an OAuth provider: no authorize URL.
	if c.AuthorizationURL("https://app/cb", "s") != "" {
		t.Fatal("garmin should expose no authorize URL")
	}

	token, err := c.PairKey(context.Background(), "member-key")
	if err != nil {
		t.Fatalf("PairKey: %v", err)
	}
	if token.AccessToken != jwt || token.RefreshToken != "member-key" {
		t.Fatalf("token not mapped (key kept as refresh): %+v", token)
	}
	if !token.ExpiresAt.After(time.Now()) {
		t.Fatalf("token should carry the JWT's expiry: %+v", token)
	}

	// Refresh re-exchanges the stored key.
	if _, err := c.Refresh(context.Background(), "member-key"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
}

func dataStub(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/stats/steps/daily/"):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"calendarDate": "2026-01-01", "totalSteps": 1000},
				{"calendarDate": "2026-01-02", "totalSteps": 2000},
			})
		case strings.HasSuffix(r.URL.Path, "/usersummary/daily"):
			date := r.URL.Query().Get("calendarDate")
			out := map[string]any{"calendarDate": date}
			if date == "2026-01-02" {
				out["maxHeartRate"] = 175
				out["minHeartRate"] = 48
				out["restingHeartRate"] = 52
			}
			_ = json.NewEncoder(w).Encode(out)
		case strings.Contains(r.URL.Path, "/weight-service/weight/range/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"dailyWeightSummaries": []map[string]any{
					{"summaryDate": "2026-01-02", "latestWeight": map[string]any{
						"weight": 82000, "bmi": 24.1, "bodyFat": 18.5, "muscleMass": 36000}},
				},
				"totalAverage": map[string]any{},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
}

func TestDailyStepsAndHeartRate(t *testing.T) {
	server := dataStub(t)
	defer server.Close()
	c := New(Config{APIBaseURL: server.URL})
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	steps, err := c.DailySteps(context.Background(), "at", start, end)
	if err != nil {
		t.Fatalf("DailySteps: %v", err)
	}
	if len(steps) != 2 || steps[0].Steps != 1000 || steps[1].Steps != 2000 {
		t.Fatalf("steps wrong: %+v", steps)
	}

	hearts, err := c.DailyHeartRate(context.Background(), "at", start, end)
	if err != nil {
		t.Fatalf("DailyHeartRate: %v", err)
	}
	// Only Jan 2 has heart data.
	if len(hearts) != 1 || hearts[0].Date != "2026-01-02" || hearts[0].Zone != devicelink.ZonePeak {
		t.Fatalf("heart wrong: %+v", hearts)
	}
}

func TestLatestBodyCompositionFromWeightRange(t *testing.T) {
	server := dataStub(t)
	defer server.Close()
	c := New(Config{APIBaseURL: server.URL})

	measures, err := c.LatestBodyComposition(context.Background(), "at")
	if err != nil {
		t.Fatalf("LatestBodyComposition: %v", err)
	}
	got := map[string]devicelink.BodyMeasure{}
	for _, m := range measures {
		got[m.Type] = m
	}
	if w := got[devicelink.MeasureWeight]; w.Value != 82 || w.Unit != "kg" {
		t.Errorf("weight not converted from grams: %+v", w)
	}
	if f := got[devicelink.MeasureFatRatio]; f.Value != 18.5 {
		t.Errorf("fat ratio wrong: %+v", f)
	}
	if m := got[devicelink.MeasureMuscleMass]; m.Value != 36 {
		t.Errorf("muscle mass not converted: %+v", m)
	}
}

// Sleep arrives in two shapes - the fields at the top level, or nested under
// dailySleepDTO - and the total is either sleepTimeSeconds or only the stage
// seconds. This pins both, that the minutes are the seconds over 60, and that a
// 404 for one night is skipped (the member just did not wear the watch) rather
// than failing the whole week.
func TestSleepDaysReadsBothShapesAndSkips404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/sleep-service/sleep/dailySleep") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		switch r.URL.Query().Get("calendarDate") {
		case "2026-01-01": // top-level total: 7h = 25200s = 420 min
			_ = json.NewEncoder(w).Encode(map[string]any{
				"calendarDate": "2026-01-01", "sleepTimeSeconds": 25200})
		case "2026-01-02": // nested, no total: stage seconds summed = 6h = 360 min
			_ = json.NewEncoder(w).Encode(map[string]any{"dailySleepDTO": map[string]any{
				"calendarDate":      "2026-01-02",
				"deepSleepSeconds":  7200,
				"lightSleepSeconds": 10800,
				"remSleepSeconds":   3600,
			}})
		default: // no night: the service 404s, which must not fail the week
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := New(Config{APIBaseURL: server.URL})

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	days, err := c.SleepDays(context.Background(), "at", start, end)
	if err != nil {
		t.Fatalf("SleepDays: %v", err)
	}
	if len(days) != 2 {
		t.Fatalf("want 2 nights (the 404 skipped), got %+v", days)
	}
	if days[0].Date != "2026-01-01" || days[0].AsleepMin != 420 {
		t.Errorf("night 1 (top-level total) wrong: %+v", days[0])
	}
	if days[1].Date != "2026-01-02" || days[1].AsleepMin != 360 {
		t.Errorf("night 2 (nested, stage sum) wrong: %+v", days[1])
	}
}

// When no night can be read at all (e.g. the path is wrong and every day 404s),
// the last error is returned so it reaches the troubleshoot console, rather than
// looking like an empty week.
func TestSleepDaysSurfacesErrorWhenNothingRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	c := New(Config{APIBaseURL: server.URL})

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	days, err := c.SleepDays(context.Background(), "at", start, end)
	if err == nil {
		t.Fatal("want the read error surfaced when no night came back")
	}
	if len(days) != 0 {
		t.Fatalf("want no nights, got %+v", days)
	}
}
