package stance

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// supabaseURLTemplate builds a project's API base URL from its ref: every
// Supabase project answers at https://<ref>.supabase.co.
const supabaseURLTemplate = "https://%s.supabase.co"

// Config is what an operator sets to switch the integration on. Empty leaves
// it off (see Enabled), which is how a deployment with no Stance arrangement
// behaves: the provider is listed as unavailable and nothing here runs.
//
// Every value has a home in ~/.stance.env. The URLs and the table/bucket
// names are overridable so this one implementation can be pointed at a
// stand-in while developing, or corrected without a release if Stance's schema
// differs - the defaults below are Stance's documented shapes, not a contract.
type Config struct {
	// SupabaseProjectRef names the Stance project; its API base URL is built
	// from it with supabaseURLTemplate. SupabaseAnonKey is the project's
	// client key. The ref and the key both empty means the feature is off.
	SupabaseProjectRef string
	SupabaseAnonKey    string
	// SupabaseURL overrides the base URL built from SupabaseProjectRef. Blank
	// in normal use; set directly to point at a stand-in while developing, or
	// in tests.
	SupabaseURL string
	// PreprocessURL is the recompute service for squat and bench. Blank
	// leaves Submit without a conclusion for those lifts rather than failing.
	PreprocessURL string
	// SetsTable is the PostgREST relation the last sets are read from, and
	// SetsSelect the columns/embeds to pull. Overridable in case Stance's
	// relation differs from the default.
	SetsTable  string
	SetsSelect string
	// AppURL is where the "record a set" button sends the member: the Stance
	// app on the same phone, which owns the encoder and does the capture.
	// Blank defaults to the app's store entry, which brings the installed app
	// forward; a deep link can replace it without a client release.
	AppURL string
	// HTTPClient is injected by tests; nil uses a default with a sane
	// timeout.
	HTTPClient *http.Client
}

// Enabled reports whether there is enough to attempt a call.
func (c Config) Enabled() bool {
	return (strings.TrimSpace(c.SupabaseProjectRef) != "" || strings.TrimSpace(c.SupabaseURL) != "") &&
		strings.TrimSpace(c.SupabaseAnonKey) != ""
}

// withDefaults fills in Stance's documented shapes for anything left blank.
func (c Config) withDefaults() Config {
	c.SupabaseURL = strings.TrimRight(strings.TrimSpace(c.SupabaseURL), "/")
	if c.SupabaseURL == "" && strings.TrimSpace(c.SupabaseProjectRef) != "" {
		c.SupabaseURL = fmt.Sprintf(supabaseURLTemplate, strings.TrimSpace(c.SupabaseProjectRef))
	}
	c.PreprocessURL = strings.TrimRight(strings.TrimSpace(c.PreprocessURL), "/")
	if c.SetsTable == "" {
		c.SetsTable = "full_sets"
	}
	if c.AppURL == "" {
		c.AppURL = "https://play.google.com/store/apps/details?id=com.stancehome.Stance_Fitness"
	}
	if c.SetsSelect == "" {
		// The set, plus its reps embedded, in Stance's own field names. A
		// relation that names its columns differently is corrected here, not
		// in the caller.
		c.SetsSelect = "id,exercise_raw,weight_kg,weight_unit,reps,rpe,predicted_rpe," +
			"mean_concentric_velocity,peak_concentric_velocity,set_ended_at_epoch_ms," +
			"repetitions(mean_concentric_velocity,rep_index)"
	}
	return c
}

// supabaseClient is the one implementation of Client, talking to Stance's
// Supabase backend. Everything specific to it - the auth grant shapes, the
// PostgREST query, the recompute call - lives here so the Client interface
// stays about bar speed and nothing else.
type supabaseClient struct {
	cfg  Config
	http *http.Client
}

// New builds the Supabase-backed client. A disabled config still returns a
// client; its Enabled reports false and the handlers never call the rest.
func New(cfg Config) Client {
	cfg = cfg.withDefaults()
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &supabaseClient{cfg: cfg, http: httpClient}
}

func (c *supabaseClient) Enabled() bool     { return c.cfg.Enabled() }
func (c *supabaseClient) RecordURL() string { return c.cfg.AppURL }

// --- auth (GoTrue) ---

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	User         struct {
		ID string `json:"id"`
	} `json:"user"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Message          string `json:"msg"`
}

func (c *supabaseClient) Pair(ctx context.Context, email, password string) (Session, error) {
	return c.token(ctx, "password", map[string]string{
		"email": strings.TrimSpace(email), "password": password,
	})
}

func (c *supabaseClient) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	return c.token(ctx, "refresh_token", map[string]string{"refresh_token": refreshToken})
}

// token runs one GoTrue grant. The access token is short-lived; ExpiresAt is
// computed from expires_in so the caller renews before a read rather than
// after a 401.
func (c *supabaseClient) token(ctx context.Context, grant string, body map[string]string) (Session, error) {
	endpoint := c.cfg.SupabaseURL + "/auth/v1/token?grant_type=" + grant

	var parsed tokenResponse
	status, err := c.call(ctx, http.MethodPost, endpoint, "", body, &parsed)
	if err != nil {
		return Session{}, err
	}
	if status == http.StatusBadRequest || status == http.StatusUnauthorized {
		// GoTrue says why in the body. A bad password on pairing and a dead
		// refresh token are both "connect again" to the member, but the
		// provider's words go in the log.
		return Session{}, fmt.Errorf("%w: %s", ErrRefused, firstNonEmpty(parsed.ErrorDescription, parsed.Message, parsed.Error))
	}
	if status/100 != 2 || parsed.AccessToken == "" {
		return Session{}, fmt.Errorf("stance: auth failed (status %d)", status)
	}

	session := Session{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		UserID:       parsed.User.ID,
	}
	if parsed.ExpiresIn > 0 {
		session.ExpiresAt = time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second)
	}
	return session, nil
}

// --- reads (PostgREST) ---

// setRow is one row of the sets relation, in Stance's field names. Tolerant
// of a column being absent: the exact schema is not pinned, and a missing
// velocity is a zero, not a parse failure.
type setRow struct {
	Exercise     string  `json:"exercise_raw"`
	WeightKg     float64 `json:"weight_kg"`
	Reps         int     `json:"reps"`
	RPE          float64 `json:"rpe"`
	PredictedRPE float64 `json:"predicted_rpe"`
	MeanVelocity float64 `json:"mean_concentric_velocity"`
	PeakVelocity float64 `json:"peak_concentric_velocity"`
	EndedAtMs    int64   `json:"set_ended_at_epoch_ms"`
	Repetitions  []struct {
		MeanVelocity float64 `json:"mean_concentric_velocity"`
		RepIndex     int     `json:"rep_index"`
	} `json:"repetitions"`
}

func (c *supabaseClient) LastSets(ctx context.Context, accessToken, userID string) ([]Set, error) {
	// Newest first; the first row seen for each lift is its last set. A plain
	// descending order over the member's rows, filtered server-side, so one
	// request covers all three lifts.
	query := url.Values{}
	query.Set("select", c.cfg.SetsSelect)
	query.Set("user_id", "eq."+userID)
	query.Set("order", "set_ended_at_epoch_ms.desc")
	query.Set("limit", "200")
	endpoint := c.cfg.SupabaseURL + "/rest/v1/" + c.cfg.SetsTable + "?" + query.Encode()

	var rows []setRow
	status, err := c.call(ctx, http.MethodGet, endpoint, accessToken, nil, &rows)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized {
		return nil, ErrReauthorize
	}
	if status/100 != 2 {
		return nil, fmt.Errorf("stance: reading sets failed (status %d)", status)
	}

	seen := map[string]bool{}
	sets := []Set{}
	for _, row := range rows {
		lift := matchLift(row.Exercise)
		if lift == "" || seen[lift] {
			continue
		}
		seen[lift] = true
		sets = append(sets, row.toSet(lift))
		if len(seen) == len(Lifts) {
			break
		}
	}
	return sets, nil
}

func (r setRow) toSet(lift string) Set {
	set := Set{
		Lift: lift, WeightKg: r.WeightKg, Reps: r.Reps,
		GivenRPE: r.RPE, PredictedRPE: r.PredictedRPE,
		MeanVelocity: r.MeanVelocity, PeakVelocity: r.PeakVelocity,
	}
	if r.EndedAtMs > 0 {
		set.PerformedAt = time.UnixMilli(r.EndedAtMs).UTC()
	}
	for _, rep := range r.Repetitions {
		set.RepVelocities = append(set.RepVelocities, rep.MeanVelocity)
	}
	// A rep count the row did not carry is taken from the reps it did.
	if set.Reps == 0 {
		set.Reps = len(set.RepVelocities)
	}
	return set
}

// --- submit (recompute service) ---

type preprocessResponse struct {
	Reps []struct {
		MeanVelocity float64 `json:"mean_concentric_velocity"`
		PeakVelocity float64 `json:"peak_concentric_velocity"`
	} `json:"reps"`
	PredictedRPE float64 `json:"predicted_rpe"`
}

func (c *supabaseClient) Submit(ctx context.Context, accessToken, userID string, in Submission) (Set, error) {
	set := Set{
		Lift: matchLift(in.Lift), WeightKg: in.WeightKg, Reps: in.Reps,
		GivenRPE: in.GivenRPE, PerformedAt: time.Now().UTC(),
	}

	// Only squat and bench are recomputed server-side; deadlift is computed
	// on the phone and arrives already concluded, so a submission with no
	// recompute endpoint keeps whatever the phone sent rather than failing.
	if c.cfg.PreprocessURL == "" {
		return set, nil
	}

	body := map[string]any{
		"user_id": userID, "exercise": in.Lift, "weight_kg": in.WeightKg,
		"reps": in.Reps, "rpe": in.GivenRPE,
		// The recording travels base64-encoded; the service decodes it.
		"recording": base64.StdEncoding.EncodeToString(in.Samples),
	}
	var parsed preprocessResponse
	status, err := c.call(ctx, http.MethodPost, c.cfg.PreprocessURL+"/", accessToken, body, &parsed)
	if err != nil {
		return Set{}, err
	}
	if status/100 != 2 {
		return Set{}, fmt.Errorf("stance: preprocess failed (status %d)", status)
	}

	set.PredictedRPE = parsed.PredictedRPE
	var sum, peak float64
	for _, rep := range parsed.Reps {
		set.RepVelocities = append(set.RepVelocities, rep.MeanVelocity)
		sum += rep.MeanVelocity
		if rep.PeakVelocity > peak {
			peak = rep.PeakVelocity
		}
	}
	if n := len(parsed.Reps); n > 0 {
		set.MeanVelocity = sum / float64(n)
		set.PeakVelocity = peak
		if set.Reps == 0 {
			set.Reps = n
		}
	}
	return set, nil
}

// --- transport ---

// call makes one request to the backend. accessToken blank sends only the
// anon key (the auth grants); set, and it goes as the bearer alongside it, as
// every PostgREST/recompute call needs.
func (c *supabaseClient) call(ctx context.Context, method, endpoint, accessToken string,
	body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("apikey", c.cfg.SupabaseAnonKey)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	} else {
		// The auth endpoints accept the anon key as the bearer too.
		req.Header.Set("Authorization", "Bearer "+c.cfg.SupabaseAnonKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if out != nil && len(bytes.TrimSpace(payload)) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			// A 2xx that would not parse is a shape we did not expect; a
			// non-2xx that would not parse is already an error the status
			// carries, so only the former is worth surfacing.
			if resp.StatusCode/100 == 2 {
				return resp.StatusCode, fmt.Errorf("stance: unexpected response: %w", err)
			}
		}
	}
	return resp.StatusCode, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "unknown error"
}
