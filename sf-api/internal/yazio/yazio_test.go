package yazio

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

func TestPairAccount(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != loginPath {
			t.Errorf("path = %q, want %q", r.URL.Path, loginPath)
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "password" || r.Form.Get("username") != "a@b.c" {
			t.Errorf("unexpected form: %v", r.Form)
		}
		w.Write([]byte(`{"access_token":"tok","refresh_token":"ref","expires_in":3600}`))
	})
	token, err := client.PairAccount(context.Background(), "a@b.c", "pw")
	if err != nil {
		t.Fatalf("PairAccount: %v", err)
	}
	if token.AccessToken != "tok" || token.RefreshToken != "ref" {
		t.Errorf("token = %+v", token)
	}
	if token.ExpiresAt.Before(time.Now()) {
		t.Error("token should not already be expired")
	}
}

func TestPairAccountRefused(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	})
	_, err := client.PairAccount(context.Background(), "a@b.c", "wrong")
	if err != devicelink.ErrRefused {
		t.Fatalf("err = %v, want ErrRefused", err)
	}
}

func TestDailyNutrition(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != nutrientsPath {
			t.Errorf("path = %q, want %q", r.URL.Path, nutrientsPath)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`[
			{"date":"2026-01-01","energy":2100.5,"protein":150.2},
			{"date":"2026-01-02","energy":0,"protein":0},
			{"date":"2026-01-03","energy":1800,"protein":120}
		]`))
	})
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	days, err := client.DailyNutrition(context.Background(), "tok", start, start.AddDate(0, 0, 2))
	if err != nil {
		t.Fatalf("DailyNutrition: %v", err)
	}
	// The all-zero day is dropped as "nothing logged".
	if len(days) != 2 {
		t.Fatalf("got %d days, want 2: %+v", len(days), days)
	}
	if days[0].Date != "2026-01-01" || days[0].Kcal != 2100.5 || days[0].ProteinG != 150.2 {
		t.Errorf("day0 = %+v", days[0])
	}
}

func TestDailyNutritionReauthorize(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := client.DailyNutrition(context.Background(), "tok", time.Now().AddDate(0, 0, -1), time.Now())
	if err != devicelink.ErrReauthorize {
		t.Fatalf("err = %v, want ErrReauthorize", err)
	}
}

func TestEnabledAndCapability(t *testing.T) {
	client := New(Config{})
	if !client.Enabled() {
		t.Error("yazio should always be enabled")
	}
	if _, ok := any(client).(devicelink.NutritionReader); !ok {
		t.Error("client should be a NutritionReader")
	}
	if _, ok := any(client).(devicelink.AccountPairer); !ok {
		t.Error("client should be an AccountPairer")
	}
}
