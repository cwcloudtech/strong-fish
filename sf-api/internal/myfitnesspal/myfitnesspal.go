// Package myfitnesspal reads a member's logged nutrition from MyFitnessPal -
// the energy and protein their food diary recorded each day - as StrongFish's
// second nutrition source after Yazio.
//
// MyFitnessPal is paired with the member's own personal access token
// (KeyPairer), not an email and password. This is deliberate: MyFitnessPal has
// no open password sign-in - its web login is behind bot protection, and
// defeating that is not something StrongFish does - so the member brings a
// token they obtained themselves, the same "your own credential" model Garmin
// uses for its personal API key. The token is read-only and reads that member's
// own diary; it is never the operator's.
//
// MyFitnessPal has no official public API. This talks to the same v2 mobile
// endpoints the app uses, reverse-engineered, and is therefore PROVISIONAL: the
// endpoint paths, the client-id header and the response shape are isolated in
// the constants below and the parsing is tolerant, so a change at MyFitnessPal
// is a one-file fix rather than a broken feature elsewhere. A token also
// expires and cannot be refreshed without the app's own secret, so a lapsed one
// is reported as "connect again" and the member pastes a fresh token.
package myfitnesspal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"strong-fish-api/internal/devicelink"
)

// Provider is the name this link is stored under (device_links.provider).
const Provider = "myfitnesspal"

// MaxDays bounds a nutrition read. The diary is read a day at a time, so the
// window is capped shorter than the ranged sources, the way Garmin's heart rate
// is - a month is plenty for a trend without a month of requests.
const MaxDays = 31

// tokenTTL is how long a member-supplied token is assumed good for before a
// read is allowed to try a refresh. MyFitnessPal access tokens are short-lived
// and there is no refresh without the app secret, so this only spaces out the
// 401 that eventually tells the member to paste a fresh token.
const tokenTTL = time.Hour

// Provisional MyFitnessPal endpoints and headers. Isolated here because they
// are reverse-engineered and may move.
const (
	diaryPath = "/v2/diary"
	// mfpClientID is the public client id the v2 API expects in a header; it
	// identifies the app, not the member. Overridable for a stand-in in tests.
	defaultClientID = "mfp-main-js"
)

// Config is what an operator may override - nothing is required, each member
// brings their own token. The URL and client id default to MyFitnessPal's own.
type Config struct {
	APIBaseURL string
	ClientID   string
	HTTPClient *http.Client
}

// Enabled is always true: MyFitnessPal needs no operator credentials, so it is
// always offered and a member can connect with their own token.
func (c Config) Enabled() bool { return true }

// WithDefaults fills in MyFitnessPal's own endpoint and client id for anything
// left blank.
func (c Config) WithDefaults() Config {
	if c.APIBaseURL == "" {
		c.APIBaseURL = "https://api.myfitnesspal.com"
	}
	c.APIBaseURL = strings.TrimRight(c.APIBaseURL, "/")
	if c.ClientID == "" {
		c.ClientID = defaultClientID
	}
	return c
}

// Client talks to MyFitnessPal for one deployment.
type Client struct {
	cfg  Config
	http *http.Client
}

// New builds a Client from a Config.
func New(cfg Config) *Client {
	cfg = cfg.WithDefaults()
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{cfg: cfg, http: httpClient}
}

func (c *Client) Name() string  { return Provider }
func (c *Client) Enabled() bool { return c.cfg.Enabled() }

// --- pairing (a personal access token, per member) ---

var errNotOAuth = fmt.Errorf("myfitnesspal: paired with a personal token, not an authorization redirect")

func (c *Client) AuthorizationURL(string, string) string { return "" }

func (c *Client) Exchange(context.Context, string, string, string) (devicelink.Token, error) {
	return devicelink.Token{}, errNotOAuth
}

// PairKey takes the member's personal MyFitnessPal access token, confirms it
// can read, and keeps it. There is no separate refresh credential: the token is
// the credential, stored as both, and a read that is refused asks the member to
// paste a fresh one.
func (c *Client) PairKey(ctx context.Context, token string) (devicelink.Token, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return devicelink.Token{}, devicelink.ErrRefused
	}
	// Prove it reads before storing it, so the member learns at connect time
	// rather than at the first empty chart.
	if _, err := c.DailyNutrition(ctx, token, time.Now().AddDate(0, 0, -1), time.Now()); err != nil {
		if err == devicelink.ErrReauthorize {
			return devicelink.Token{}, devicelink.ErrRefused
		}
		return devicelink.Token{}, err
	}
	return devicelink.Token{
		AccessToken:  token,
		RefreshToken: token,
		ExpiresAt:    time.Now().Add(tokenTTL),
	}, nil
}

// Refresh cannot renew a member-supplied token without the app secret, so it
// hands the stored one back with a fresh assumed lifetime. A token that has
// truly lapsed is caught by the 401 on the next read, which asks the member to
// reconnect.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (devicelink.Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return devicelink.Token{}, devicelink.ErrReauthorize
	}
	return devicelink.Token{
		AccessToken:  refreshToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(tokenTTL),
	}, nil
}

// CheckAccount has nothing to confirm: PairKey already proved the token reads.
func (c *Client) CheckAccount(context.Context, string) (string, error) { return "", nil }

// --- nutrition ---

// MaxNutritionDays is the widest window the diary is read over.
func (c *Client) MaxNutritionDays() int { return MaxDays }

// diaryResponse is the v2 diary for one day: a list of items, of which the food
// entries carry the nutrition. Parsed tolerantly - a shape that drifts should
// return a thinner day, not an error.
type diaryResponse struct {
	Items []struct {
		Type                string `json:"type"`
		NutritionalContents struct {
			// Energy is sometimes a bare number and sometimes an object with a
			// value and a unit; RawMessage lets energyValue read either.
			Energy  json.RawMessage `json:"energy"`
			Protein float64         `json:"protein"`
		} `json:"nutritional_contents"`
	} `json:"items"`
}

// DailyNutrition returns one entry per day in [start, end] that has food
// logged, summing the food entries' energy and protein - MyFitnessPal reports
// per entry, so the day's total is summed here.
func (c *Client) DailyNutrition(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.NutritionDay, error) {
	start = truncate(start)
	end = truncate(end)
	if end.Before(start) {
		return nil, nil
	}
	if earliest := end.AddDate(0, 0, -(MaxDays - 1)); start.Before(earliest) {
		start = earliest
	}

	days := make([]devicelink.NutritionDay, 0)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		var parsed diaryResponse
		q := url.Values{
			"username":   {"me"},
			"entry_date": {date},
			"fields[]":   {"nutritional_contents"},
		}
		if err := c.get(ctx, accessToken, diaryPath, q, &parsed); err != nil {
			return nil, err
		}
		var kcal, protein float64
		logged := false
		for _, item := range parsed.Items {
			if item.Type != "food_entry" && item.Type != "diary_meal" {
				continue
			}
			kcal += energyValue(item.NutritionalContents.Energy)
			protein += item.NutritionalContents.Protein
			logged = true
		}
		if !logged || (kcal <= 0 && protein <= 0) {
			continue
		}
		days = append(days, devicelink.NutritionDay{Date: date, Kcal: kcal, ProteinG: protein})
	}
	return days, nil
}

// energyValue reads the energy, which the API sends either as a bare number of
// kilocalories or as {"value": n, "unit": "calories"}.
func energyValue(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var obj struct {
		Value float64 `json:"value"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Value
	}
	return 0
}

// --- transport ---

type apiError struct {
	status int
	body   string
}

func (e *apiError) Error() string {
	if e.body != "" {
		return fmt.Sprintf("myfitnesspal: api returned status %d: %s", e.status, e.body)
	}
	return fmt.Sprintf("myfitnesspal: api returned status %d", e.status)
}

func (c *Client) get(ctx context.Context, accessToken, path string, q url.Values, out any) error {
	endpoint := c.cfg.APIBaseURL + path
	if len(q) > 0 {
		endpoint += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("mfp-client-id", c.cfg.ClientID)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return devicelink.ErrReauthorize
		}
		return &apiError{status: resp.StatusCode, body: snippet(data)}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("myfitnesspal: decoding %s: %w", path, err)
		}
	}
	return nil
}

func truncate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
