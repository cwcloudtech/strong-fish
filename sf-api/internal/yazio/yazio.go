// Package yazio reads a member's logged nutrition from Yazio - the energy and
// protein their food diary recorded each day - as the first of StrongFish's
// nutrition sources.
//
// Each member connects their own Yazio account with the email and password they
// sign in to the app with (AccountPairer). Those are exchanged once, at the
// app's own public OAuth token endpoint, for an access and refresh token kept
// per member like any other device link; the password itself is never stored.
// There is no operator credential - the client id and secret below are the
// Yazio app's own public ones, the same values the mobile app ships - so the
// integration is always offered.
//
// Yazio has no official public API. This talks to the same mobile endpoints the
// app uses, reverse-engineered, so it may change without notice; the endpoints
// and the token response shape are isolated here so a move is a one-file fix.
package yazio

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
const Provider = "yazio"

// MaxDays is the widest nutrition window - the daily-nutrients endpoint takes
// a start and end, and ninety days matches the other sources' ceiling.
const MaxDays = 90

// Yazio's own public mobile-app OAuth client credentials. Public by design:
// they identify the app, not a member, and the member's own email and password
// are what actually authenticate. Overridable only for a stand-in in tests.
const (
	defaultClientID     = "1_4hiybetvfksgw40o0sog4s884kwc840wwso8go4k8c04goo4c"
	defaultClientSecret = "6rok2m65xuskgkgogw40wkkk8sw0osg84s8cggsc4woos4s8o"
)

// tokenTTL is the fallback lifetime when the token response carries no
// expires_in. A margin is taken off so a token about to lapse is refreshed
// before a read rather than after it fails.
const tokenTTL = time.Hour - 5*time.Minute

const (
	loginPath     = "/v20/oauth/token"
	nutrientsPath = "/v20/user/consumed-items/nutrients-daily"
)

// Config is what an operator may override - nothing is required, each member
// brings their own account. The URL and credentials default to Yazio's own and
// are overridable for a stand-in in tests.
type Config struct {
	APIBaseURL   string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

// Enabled is always true: Yazio needs no operator credentials, so it is always
// offered and a member can connect their own account.
func (c Config) Enabled() bool { return true }

// WithDefaults fills in Yazio's own endpoint and public credentials for
// anything left blank.
func (c Config) WithDefaults() Config {
	if c.APIBaseURL == "" {
		c.APIBaseURL = "https://yzapi.yazio.com"
	}
	c.APIBaseURL = strings.TrimRight(c.APIBaseURL, "/")
	if c.ClientID == "" {
		c.ClientID = defaultClientID
	}
	if c.ClientSecret == "" {
		c.ClientSecret = defaultClientSecret
	}
	return c
}

// Client talks to Yazio for one deployment.
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

// --- pairing (an email and password, per member) ---

var errNotOAuth = fmt.Errorf("yazio: paired with an account, not an authorization redirect")

func (c *Client) AuthorizationURL(string, string) string { return "" }

func (c *Client) Exchange(context.Context, string, string, string) (devicelink.Token, error) {
	return devicelink.Token{}, errNotOAuth
}

// CheckAccount has nothing to confirm: the member's credentials were already
// proven usable by the token exchange at pairing.
func (c *Client) CheckAccount(context.Context, string) (string, error) { return "", nil }

// PairAccount signs in with the member's Yazio email and password.
func (c *Client) PairAccount(ctx context.Context, email, password string) (devicelink.Token, error) {
	return c.token(ctx, url.Values{
		"grant_type": {"password"},
		"username":   {email},
		"password":   {password},
	})
}

// Refresh renews an expired access token with the stored refresh token.
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

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// token exchanges a grant at Yazio's OAuth endpoint. A refused password at
// pairing is ErrRefused (the member's to fix); a refused refresh token is
// ErrReauthorize (connect again).
func (c *Client) token(ctx context.Context, form url.Values) (devicelink.Token, error) {
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIBaseURL+loginPath,
		strings.NewReader(form.Encode()))
	if err != nil {
		return devicelink.Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return devicelink.Token{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return devicelink.Token{}, err
	}

	var parsed tokenResponse
	_ = json.Unmarshal(data, &parsed)
	if resp.StatusCode/100 != 2 || parsed.AccessToken == "" {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest {
			// A password grant turned down is a bad email or password; a refresh
			// grant turned down is a lapsed link to reconnect.
			if form.Get("grant_type") == "password" {
				return devicelink.Token{}, devicelink.ErrRefused
			}
			return devicelink.Token{}, devicelink.ErrReauthorize
		}
		return devicelink.Token{}, &apiError{status: resp.StatusCode, body: snippet(data)}
	}

	expires := time.Now().Add(tokenTTL)
	if parsed.ExpiresIn > 0 {
		expires = time.Now().Add(time.Duration(parsed.ExpiresIn)*time.Second - 5*time.Minute)
	}
	return devicelink.Token{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		ExpiresAt:    expires,
	}, nil
}

// --- nutrition ---

// MaxNutritionDays is the widest window the daily-nutrients view is offered over.
func (c *Client) MaxNutritionDays() int { return MaxDays }

// dailyNutrients is one day of the nutrients-daily endpoint: the figures are
// already aggregated per day by Yazio, energy in kcal and protein in grams.
type dailyNutrients struct {
	Date    string  `json:"date"`
	Energy  float64 `json:"energy"`
	Protein float64 `json:"protein"`
}

// DailyNutrition returns one entry per day in [start, end] that has anything
// logged, from Yazio's own per-day aggregate so nothing is summed here.
func (c *Client) DailyNutrition(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.NutritionDay, error) {
	q := url.Values{
		"start": {start.Format("2006-01-02")},
		"end":   {end.Format("2006-01-02")},
	}
	var rows []dailyNutrients
	if err := c.get(ctx, accessToken, nutrientsPath, q, &rows); err != nil {
		return nil, err
	}
	days := make([]devicelink.NutritionDay, 0, len(rows))
	for _, row := range rows {
		date := row.Date
		if len(date) >= 10 {
			date = date[:10]
		}
		if date == "" || (row.Energy <= 0 && row.Protein <= 0) {
			continue
		}
		days = append(days, devicelink.NutritionDay{
			Date: date, Kcal: row.Energy, ProteinG: row.Protein,
		})
	}
	return days, nil
}

// --- transport ---

type apiError struct {
	status int
	body   string
}

func (e *apiError) Error() string {
	if e.body != "" {
		return fmt.Sprintf("yazio: api returned status %d: %s", e.status, e.body)
	}
	return fmt.Sprintf("yazio: api returned status %d", e.status)
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
			return fmt.Errorf("yazio: decoding %s: %w", path, err)
		}
	}
	return nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
