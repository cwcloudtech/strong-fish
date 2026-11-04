package withings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"strong-fish-api/internal/devicelink"
)

func stub(t *testing.T, handler func(action string, w http.ResponseWriter, r *http.Request)) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		handler(r.PostForm.Get("action"), w, r)
	}))
	t.Cleanup(server.Close)
	return New(Config{ClientID: "id", ClientSecret: "secret", APIBaseURL: server.URL})
}

func TestExchangeReadsTheUserID(t *testing.T) {
	client := stub(t, func(action string, w http.ResponseWriter, r *http.Request) {
		if action != "requesttoken" || r.PostForm.Get("client_secret") != "secret" {
			t.Errorf("unexpected token request: %v", r.PostForm)
		}
		fmt.Fprint(w, `{"status":0,"body":{"userid":363,"access_token":"a","refresh_token":"r","expires_in":10800,"scope":"user.metrics"}}`)
	})
	token, err := client.Exchange(context.Background(), "code", "http://cb", "state")
	if err != nil {
		t.Fatal(err)
	}
	if token.UserID != "363" || token.AccessToken != "a" || token.RefreshToken != "r" {
		t.Fatalf("unexpected token: %+v", token)
	}
}

func TestRefusedRefreshAsksToReconnect(t *testing.T) {
	client := stub(t, func(_ string, w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"status":503,"error":"Invalid Params: invalid refresh_token"}`)
	})
	if _, err := client.Refresh(context.Background(), "old"); !errors.Is(err, devicelink.ErrReauthorize) {
		t.Fatalf("expected ErrReauthorize, got %v", err)
	}
}

func TestLatestBodyCompositionKeepsTheNewestOfEach(t *testing.T) {
	older := time.Date(2026, 10, 1, 7, 0, 0, 0, time.UTC).Unix()
	newer := time.Date(2026, 10, 30, 7, 0, 0, 0, time.UTC).Unix()
	client := stub(t, func(action string, w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing bearer token")
		}
		if r.PostForm.Get("meastypes") == "4" {
			fmt.Fprintf(w, `{"status":0,"body":{"measuregrps":[{"date":%d,"category":1,"measures":[{"value":180,"type":4,"unit":-2}]}]}}`, older)
			return
		}
		fmt.Fprintf(w, `{"status":0,"body":{"measuregrps":[
			{"date":%d,"category":1,"measures":[{"value":72450,"type":1,"unit":-3},{"value":215,"type":6,"unit":-1}]},
			{"date":%d,"category":1,"measures":[{"value":74000,"type":1,"unit":-3},{"value":5800,"type":76,"unit":-2}]},
			{"date":%d,"category":2,"measures":[{"value":70000,"type":1,"unit":-3}]}
		],"more":0,"offset":0}}`, newer, older, newer)
	})

	measures, err := client.LatestBodyComposition(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, measure := range measures {
		got[measure.Type] = measure.Value
	}
	want := map[string]float64{
		devicelink.MeasureWeight:     72.45,
		devicelink.MeasureFatRatio:   21.5,
		devicelink.MeasureMuscleMass: 58,
		devicelink.MeasureBMI:        22.4,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s: got %v, want %v", name, got[name], value)
		}
	}
	if measures[0].Type != devicelink.MeasureWeight {
		t.Errorf("weight should come first, got %s", measures[0].Type)
	}
}

func TestDailyActivity(t *testing.T) {
	client := stub(t, func(action string, w http.ResponseWriter, _ *http.Request) {
		if action != "getactivity" {
			t.Errorf("unexpected action %q", action)
		}
		fmt.Fprint(w, `{"status":0,"body":{"activities":[
			{"date":"2026-10-30","steps":8200,"hr_average":71,"hr_min":52,"hr_max":160,"hr_zone_0":600,"hr_zone_1":300,"hr_zone_2":120,"hr_zone_3":0},
			{"date":"2026-10-31","steps":1200}
		],"more":false,"offset":0}}`)
	})
	start := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	steps, err := client.DailySteps(context.Background(), "token", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 || steps[0].Steps != 0 || steps[1].Steps != 8200 || steps[2].Steps != 1200 {
		t.Fatalf("unexpected steps: %+v", steps)
	}

	hearts, err := client.DailyHeartRate(context.Background(), "token", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(hearts) != 3 || hearts[0].Known || !hearts[1].Known || hearts[2].Known {
		t.Fatalf("unexpected heart days: %+v", hearts)
	}
	if hearts[1].Zone != devicelink.ZoneVigorous || hearts[1].Avg != 71 {
		t.Fatalf("unexpected heart day: %+v", hearts[1])
	}
}

func TestExpiredTokenAsksToReconnect(t *testing.T) {
	client := stub(t, func(_ string, w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"status":401,"error":"invalid_token"}`)
	})
	if _, err := client.LatestBodyComposition(context.Background(), "token"); !errors.Is(err, devicelink.ErrReauthorize) {
		t.Fatalf("expected ErrReauthorize, got %v", err)
	}
}

func TestActivitySourcePrefersRegistrationOrder(t *testing.T) {
	google := fakeActivity{name: "google-health"}
	scale := New(Config{ClientID: "id", ClientSecret: "secret"})
	providers := []devicelink.Provider{google, scale}

	if got := devicelink.ActivitySource(providers, map[string]bool{"google-health": true, "withings": true}); got != "google-health" {
		t.Errorf("both connected: got %q", got)
	}
	if got := devicelink.ActivitySource(providers, map[string]bool{"withings": true}); got != "withings" {
		t.Errorf("withings alone: got %q", got)
	}
}

type fakeActivity struct{ name string }

func (f fakeActivity) Name() string                           { return f.name }
func (f fakeActivity) Enabled() bool                          { return true }
func (f fakeActivity) AuthorizationURL(string, string) string { return "" }
func (f fakeActivity) Exchange(context.Context, string, string, string) (devicelink.Token, error) {
	return devicelink.Token{}, nil
}
func (f fakeActivity) Refresh(context.Context, string) (devicelink.Token, error) {
	return devicelink.Token{}, nil
}
func (f fakeActivity) CheckAccount(context.Context, string) (string, error) { return "", nil }
func (f fakeActivity) MaxRangeDays() int                                    { return 90 }
func (f fakeActivity) MaxHeartRateDays() int                                { return 14 }
func (f fakeActivity) DailySteps(context.Context, string, time.Time, time.Time) ([]devicelink.DayCount, error) {
	return nil, nil
}
func (f fakeActivity) DailyHeartRate(context.Context, string, time.Time, time.Time) ([]devicelink.DayHeart, error) {
	return nil, nil
}
