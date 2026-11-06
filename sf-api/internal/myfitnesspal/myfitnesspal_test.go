package myfitnesspal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"strong-fish-api/internal/devicelink"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(Config{APIBaseURL: server.URL, HTTPClient: server.Client()})
}

// A day's food entries are summed into one total, reading the energy in both
// the bare-number and the {value} shapes.
func TestDailyNutritionSums(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != diaryPath {
			t.Errorf("path = %q, want %q", r.URL.Path, diaryPath)
		}
		if r.Header.Get("mfp-client-id") == "" {
			t.Error("missing mfp-client-id header")
		}
		w.Write([]byte(`{"items":[
			{"type":"food_entry","nutritional_contents":{"energy":{"value":500,"unit":"calories"},"protein":40}},
			{"type":"food_entry","nutritional_contents":{"energy":300,"protein":20}},
			{"type":"exercise_entry","nutritional_contents":{"energy":999,"protein":999}}
		]}`))
	})
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	days, err := client.DailyNutrition(context.Background(), "tok", day, day)
	if err != nil {
		t.Fatalf("DailyNutrition: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}
	// 500 + 300 kcal and 40 + 20 g; the exercise entry is ignored.
	if days[0].Kcal != 800 || days[0].ProteinG != 60 {
		t.Errorf("day = %+v, want kcal 800 protein 60", days[0])
	}
}

// A day with nothing logged is dropped rather than returned as zero.
func TestDailyNutritionSkipsEmptyDays(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[]}`))
	})
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	days, err := client.DailyNutrition(context.Background(), "tok", day, day.AddDate(0, 0, 2))
	if err != nil {
		t.Fatalf("DailyNutrition: %v", err)
	}
	if len(days) != 0 {
		t.Fatalf("got %d days, want 0", len(days))
	}
}

func TestPairKeyRefusedWhenUnauthorized(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := client.PairKey(context.Background(), "bad-token")
	if err != devicelink.ErrRefused {
		t.Fatalf("err = %v, want ErrRefused", err)
	}
}

func TestPairKeyEmpty(t *testing.T) {
	client := New(Config{})
	if _, err := client.PairKey(context.Background(), "   "); err != devicelink.ErrRefused {
		t.Fatalf("err = %v, want ErrRefused for blank token", err)
	}
}

func TestEnabledAndCapability(t *testing.T) {
	client := New(Config{})
	if !client.Enabled() {
		t.Error("myfitnesspal should always be enabled")
	}
	if _, ok := any(client).(devicelink.NutritionReader); !ok {
		t.Error("client should be a NutritionReader")
	}
	if _, ok := any(client).(devicelink.KeyPairer); !ok {
		t.Error("client should be a KeyPairer")
	}
}
