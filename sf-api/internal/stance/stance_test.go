package stance

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMatchLiftReadsFreeText(t *testing.T) {
	cases := map[string]string{
		"Back Squat (comp)": LiftSquat,
		"Paused Bench":      LiftBench,
		"Sumo Deadlift":     LiftDeadlift,
		"dead lift":         LiftDeadlift,
		"Overhead Press":    "",
	}
	for in, want := range cases {
		if got := matchLift(in); got != want {
			t.Errorf("matchLift(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnabledRequiresFirebaseAndSupabase(t *testing.T) {
	full := Config{FirebaseAPIKey: "k", SupabaseProjectRef: "ref", SupabaseAnonKey: "anon"}
	if !full.Enabled() {
		t.Fatal("want enabled with firebase key + supabase ref + anon key")
	}
	if (Config{SupabaseProjectRef: "ref", SupabaseAnonKey: "anon"}).Enabled() {
		t.Error("must be disabled without a firebase api key")
	}
	if (Config{FirebaseAPIKey: "k", SupabaseAnonKey: "anon"}).Enabled() {
		t.Error("must be disabled without a supabase project")
	}
	if full.GoogleEnabled() {
		t.Error("google must be off until a client id/secret are set")
	}
	withGoogle := full
	withGoogle.GoogleClientID = "id"
	if !withGoogle.GoogleEnabled() {
		t.Error("google should be on with a public client id")
	}
}

func TestPairPasswordSignsInAndRefusesBadCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "accounts:signInWithPassword") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var body struct{ Email, Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Password == "right" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"idToken": "id", "refreshToken": "ref", "localId": "uid-1", "expiresIn": "3600",
			})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "INVALID_LOGIN_CREDENTIALS"},
		})
	}))
	defer server.Close()

	c := New(Config{
		FirebaseAPIKey: "k", SupabaseAnonKey: "anon", SupabaseURL: server.URL,
		FirebaseAuthURL: server.URL,
	})

	session, err := c.PairPassword(context.Background(), "a@b.co", "right")
	if err != nil {
		t.Fatalf("good credentials: %v", err)
	}
	if session.FirebaseUID != "uid-1" || session.IDToken != "id" || session.RefreshToken != "ref" {
		t.Fatalf("session not mapped: %+v", session)
	}

	if _, err := c.PairPassword(context.Background(), "a@b.co", "wrong"); !errors.Is(err, ErrRefused) {
		t.Fatalf("bad credentials: want ErrRefused, got %v", err)
	}
}

func TestPairGoogleSignsInWithIdToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "accounts:signInWithIdp") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var body struct{ PostBody string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body.PostBody, "google-id-token") ||
			!strings.Contains(body.PostBody, "providerId=google.com") {
			t.Errorf("idp postBody wrong: %s", body.PostBody)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idToken": "fb-id", "refreshToken": "fb-ref", "localId": "uid-2", "expiresIn": "3600",
		})
	}))
	defer server.Close()

	c := New(Config{
		FirebaseAPIKey: "k", SupabaseAnonKey: "anon", SupabaseURL: server.URL,
		GoogleClientID: "gid", FirebaseAuthURL: server.URL,
	})

	if c.GoogleClientID() != "gid" {
		t.Fatalf("want client id exposed, got %q", c.GoogleClientID())
	}
	session, err := c.PairGoogle(context.Background(), "google-id-token")
	if err != nil {
		t.Fatalf("PairGoogle: %v", err)
	}
	if session.FirebaseUID != "uid-2" || session.IDToken != "fb-id" {
		t.Fatalf("session not mapped: %+v", session)
	}
}

func TestSupabaseURLDerivedFromRefForms(t *testing.T) {
	cases := map[string]string{
		"rszfl":                      "https://rszfl.supabase.co",
		"rszfl.supabase.co":          "https://rszfl.supabase.co",
		"https://rszfl.supabase.co":  "https://rszfl.supabase.co",
		"https://rszfl.supabase.co/": "https://rszfl.supabase.co",
	}
	for ref, want := range cases {
		got := Config{SupabaseProjectRef: ref}.withDefaults().SupabaseURL
		if got != want {
			t.Errorf("ref %q -> %q, want %q", ref, got, want)
		}
	}
}

func TestGoogleClientIDBlankWhenUnconfigured(t *testing.T) {
	c := New(Config{FirebaseAPIKey: "k", SupabaseProjectRef: "ref", SupabaseAnonKey: "anon"})
	if id := c.GoogleClientID(); id != "" {
		t.Fatalf("want blank client id without google configured, got %s", id)
	}
}

func TestLastSetsKeepsTheNewestPerLift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/rest/v1/rpc/get_user_sessions"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			// The RPC is keyed by the Firebase uid directly, exactly as the
			// Stance app passes it - no users-table lookup in between.
			if body["user_id_param"] != "uid-1" {
				t.Errorf("rpc param wrong: %v", body)
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"exercise": "Back Squat", "weight": 100.0, "weight_unit_name": "kg",
					"predicted_rpe_tag": "8", "ended": "2026-01-01T10:00:00Z",
					"repetitions": []map[string]any{{"mean_concentric_velocity": 0.5, "rep_order": 1}}},
				{"exercise": "Comp Squat", "weight": 110.0, "weight_unit_name": "kg",
					"predicted_rpe_tag": "9", "ended": "2026-02-01T10:00:00Z",
					"repetitions": []map[string]any{{"mean_concentric_velocity": 0.4, "rep_order": 1}}},
				{"exercise": "Bench Press", "weight": 80.0, "weight_unit_name": "kg",
					"predicted_rpe_tag": "7", "ended": "2026-01-15T10:00:00Z",
					"repetitions": []map[string]any{{"mean_concentric_velocity": 0.3, "rep_order": 1}}},
				{"exercise": "Overhead Press", "weight": 50.0, "weight_unit_name": "kg",
					"ended": "2026-03-01T10:00:00Z", "repetitions": []map[string]any{}},
			})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := New(Config{FirebaseAPIKey: "k", SupabaseAnonKey: "anon", SupabaseURL: server.URL})

	sets, err := c.LastSets(context.Background(), "uid-1")
	if err != nil {
		t.Fatalf("LastSets: %v", err)
	}
	if len(sets) != 2 {
		t.Fatalf("want 2 lifts (squat, bench), got %d: %+v", len(sets), sets)
	}
	// Newest first: the Feb squat before the Jan bench; and the newer squat wins.
	if sets[0].Lift != LiftSquat || sets[0].WeightKg != 110 || sets[0].PredictedRPE != 9 {
		t.Errorf("squat not the newest: %+v", sets[0])
	}
	if sets[1].Lift != LiftBench {
		t.Errorf("want bench second, got %+v", sets[1])
	}
}
