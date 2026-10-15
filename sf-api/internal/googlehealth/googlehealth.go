// Package googlehealth reads a member's daily step counts from the Google
// Health API - the successor to the Fitbit Web API, which Google switches off
// on 30 October 2026 and whose data now arrives here instead.
//
// A member links their Fitbit, Pixel Watch or phone to their Google account in
// the Google Health app; this package asks that account, with the member's
// consent, for what it already counted. Nothing is written back: the scope
// requested is read-only, and strong-fish has no business adding to somebody's
// health record.
//
// The OAuth flow is the ordinary Google authorization code exchange, kept here
// rather than in internal/oidc because it is not a login: it never resolves an
// identity into a local account, it asks for offline access so the link
// outlives the browser tab, and its tokens are stored per member instead of
// being spent once.
package googlehealth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"strong-fish-api/internal/devicelink"
)

// Provider is the name this link is stored under (see the health_links table).
const Provider = "google-health"

// Scope is the one permission asked for: the family that carries steps, read
// only. Asking for more would be asking a member to hand over sleep and
// weight to an app that shows neither.
const Scope = "https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly"

// MaxRollupDays is the longest window the API accepts in one steps roll-up,
// and so the longest view there is. The widest the app offers is exactly this,
// which is why nothing here pages: a request that would need two is a range
// the product does not have.
const MaxRollupDays = 90

// Config is what an operator sets. The three URLs are configurable so the flow
// can be pointed at a stand-in during development; in production they are left
// at their defaults.
type Config struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	APIBaseURL   string
}

// Enabled reports whether an operator has configured the integration. Without
// it the endpoints stay registered but answer that the feature is off, which
// is a clearer thing for a client to render than a 404.
func (c Config) Enabled() bool {
	return strings.TrimSpace(c.ClientID) != "" && strings.TrimSpace(c.ClientSecret) != ""
}

// WithDefaults fills in Google's own endpoints for anything left blank.
func (c Config) WithDefaults() Config {
	if c.AuthURL == "" {
		c.AuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if c.TokenURL == "" {
		c.TokenURL = "https://oauth2.googleapis.com/token"
	}
	if c.APIBaseURL == "" {
		c.APIBaseURL = "https://health.googleapis.com"
	}
	c.APIBaseURL = strings.TrimRight(c.APIBaseURL, "/")
	return c
}

// Client talks to Google on behalf of one deployment.
type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	return &Client{cfg: cfg.WithDefaults(), http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Enabled() bool { return c.cfg.Enabled() }

// Name is the {provider} path segment and the stored provider value.
func (c *Client) Name() string { return Provider }

// MaxRangeDays is Google's own cap on a steps roll-up, and therefore the
// widest view the app can offer.
func (c *Client) MaxRangeDays() int { return MaxRollupDays }

// CheckAccount confirms the Google account that just consented has a Google
// Health profile behind it, and returns the id it is known by there.
func (c *Client) CheckAccount(ctx context.Context, accessToken string) (string, error) {
	identity, err := c.CheckIdentity(ctx, accessToken)
	if err != nil {
		return "", err
	}
	return identity.HealthUserID, nil
}

// AuthorizationURL is where the member's browser goes to consent.
//
// access_type=offline is what produces a refresh token, without which the link
// would stop working within the hour and the member would be reconnecting
// every time they opened the page. prompt=consent is what makes Google issue
// that refresh token again on a second authorization - it only sends one the
// first time otherwise, so a member who disconnects and reconnects would
// silently end up with a link that cannot refresh.
func (c *Client) AuthorizationURL(redirectURI, state string) string {
	q := make(url.Values)
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", Scope)
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("include_granted_scopes", "true")
	return c.cfg.AuthURL + "?" + q.Encode()
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// Exchange turns the code from the consent redirect into a grant.
func (c *Client) Exchange(ctx context.Context, code, redirectURI string) (devicelink.Token, error) {
	return c.token(ctx, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	})
}

// Refresh renews an expired access token. Google does not send the refresh
// token back, so the caller keeps the one it already has.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (devicelink.Token, error) {
	token, err := c.token(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return devicelink.Token{}, err
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}

func (c *Client) token(ctx context.Context, form url.Values) (devicelink.Token, error) {
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return devicelink.Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return devicelink.Token{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return devicelink.Token{}, err
	}
	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return devicelink.Token{}, fmt.Errorf("googlehealth: token response unreadable: %w", err)
	}
	if resp.StatusCode != http.StatusOK || parsed.AccessToken == "" {
		// invalid_grant is what a revoked or already-spent grant looks like,
		// and it is the one failure the member can fix, by connecting again.
		if parsed.Error == "invalid_grant" {
			return devicelink.Token{}, devicelink.ErrReauthorize
		}
		return devicelink.Token{}, fmt.Errorf("googlehealth: token exchange failed: %s %s", parsed.Error, parsed.ErrorDesc)
	}

	return devicelink.Token{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		// A minute short, so a token that is about to lapse is refreshed
		// before a request rather than after it fails.
		ExpiresAt: time.Now().Add(time.Duration(parsed.ExpiresIn)*time.Second - time.Minute),
		Scope:     parsed.Scope,
	}, nil
}

// Identity is who Google Health knows this member as.
type Identity struct {
	HealthUserID string `json:"healthUserId"`
	LegacyUserID string `json:"legacyUserId"`
}

// CheckIdentity confirms the Google account that just consented actually has a
// Google Health profile behind it.
//
// Worth doing at the moment of linking rather than at the first read: consent
// can succeed against an account that has never opened the Google Health app,
// and the difference between "connected, no data yet" and "this will never
// work until you link Fitbit" is the whole of what the member needs to be
// told.
func (c *Client) CheckIdentity(ctx context.Context, accessToken string) (Identity, error) {
	var identity Identity
	err := c.call(ctx, http.MethodGet, "/v4/users/me/identity", accessToken, nil, &identity)
	return identity, err
}

type civilDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type civilDateTime struct {
	Date civilDate `json:"date"`
}

type rollupRequest struct {
	Range struct {
		Start civilDateTime `json:"start"`
		End   civilDateTime `json:"end"`
	} `json:"range"`
	WindowSizeDays int `json:"windowSizeDays"`
}

type rollupResponse struct {
	RollupDataPoints []struct {
		CivilStartTime civilDateTime `json:"civilStartTime"`
		Steps          *struct {
			CountSum string `json:"countSum"`
		} `json:"steps"`
	} `json:"rollupDataPoints"`
	NextPageToken string `json:"nextPageToken"`
}

// DailySteps returns one entry per day from start up to and including end, at
// most MaxRollupDays of them.
//
// Days the provider has nothing for come back as zero rather than as a gap: a
// day with no steps and a day the watch was in a drawer look the same from
// here, and a chart with holes in it reads as a bug.
func (c *Client) DailySteps(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.DayCount, error) {
	start = truncateDay(start)
	end = truncateDay(end)
	if end.Before(start) {
		return nil, nil
	}
	// Clamped rather than refused: the only caller asks for a range the
	// product offers, and a request that somehow asks for more should come
	// back shorter instead of failing.
	if earliest := end.AddDate(0, 0, -(MaxRollupDays - 1)); start.Before(earliest) {
		start = earliest
	}

	var body rollupRequest
	body.Range.Start = civilDateTime{Date: dateOf(start)}
	// The end of the range is exclusive, so the last day asked for is the day
	// after it.
	body.Range.End = civilDateTime{Date: dateOf(end.AddDate(0, 0, 1))}
	body.WindowSizeDays = 1

	var parsed rollupResponse
	if err := c.call(ctx, http.MethodPost,
		"/v4/users/me/dataTypes/steps/dataPoints:dailyRollUp", accessToken, body, &parsed); err != nil {
		return nil, err
	}

	counts := map[string]int64{}
	for _, point := range parsed.RollupDataPoints {
		if point.Steps == nil {
			continue
		}
		// An int64 arrives as a string in Google's JSON mapping.
		value, err := strconv.ParseInt(point.Steps.CountSum, 10, 64)
		if err != nil {
			continue
		}
		date := point.CivilStartTime.Date
		counts[fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day)] += value
	}

	days := []devicelink.DayCount{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		days = append(days, devicelink.DayCount{Date: key, Steps: counts[key]})
	}
	return days, nil
}

type apiError struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (c *Client) call(ctx context.Context, method, path, accessToken string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.APIBaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		var failure apiError
		_ = json.Unmarshal(raw, &failure)
		switch {
		case strings.Contains(failure.Error.Status, "ACCOUNT_NOT_LINKED"),
			strings.Contains(failure.Error.Message, "ACCOUNT_NOT_LINKED"):
			return devicelink.ErrNotLinked
		case resp.StatusCode == http.StatusUnauthorized:
			return devicelink.ErrReauthorize
		}
		return fmt.Errorf("googlehealth: %s %s: %d %s", method, path, resp.StatusCode, failure.Error.Message)
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func dateOf(t time.Time) civilDate {
	return civilDate{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}
}
