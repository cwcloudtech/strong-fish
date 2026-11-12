// Package garmin reads steps, heart rate and body composition from Garmin
// Connect's Personal API, the third activity source after Google Health and
// Withings.
//
// Each member connects their own Garmin account by entering a personal API key
// they generate in Garmin Connect - there is no OAuth consent. The key is
// exchanged for a 24-hour access token (a JWT, returned as plain text) and kept
// as the refresh credential so the token can be renewed; it reads that member's
// own data. So Garmin is paired per member, like an account sign-in, not
// configured once by the operator.
//
// The data comes from Garmin Connect's own service endpoints, keyed by calendar
// date. Steps over a range are one request; heart rate is one request per day,
// so its view is capped shorter.
package garmin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"strong-fish-api/internal/devicelink"
)

// Provider is the name this link is stored under (device_links.provider).
const Provider = "garmin"

// MaxStepsDays is the widest steps view - steps come in one ranged request, so
// this can be as wide as the others. MaxHeartRateDays is shorter because heart
// rate is read a day at a time.
const (
	MaxStepsDays     = 90
	MaxHeartRateDays = 14
	recentBodyWindow = 30
)

// tokenTTL is the fallback lifetime of a Personal API access token when its own
// expiry cannot be read (Garmin: 24 hours). A margin is taken off so a token
// about to lapse is refreshed before a read rather than after it fails.
const tokenTTL = 24*time.Hour - 10*time.Minute

// Config is what an operator may override. Nothing is required - each member
// brings their own personal API key - and the URLs default to Garmin's; they
// are overridable for a stand-in in tests, or should Garmin move the Personal
// API while it is still rolling out.
type Config struct {
	// TokenURL exchanges a member's API key for an access token.
	TokenURL string
	// APIBaseURL is the host the Connect service endpoints are read from.
	APIBaseURL string
	HTTPClient *http.Client
}

// Enabled is always true: Garmin needs no operator credentials - each member
// brings their own key - so it is always offered.
func (c Config) Enabled() bool { return true }

// WithDefaults fills in Garmin's own endpoints for anything left blank.
func (c Config) WithDefaults() Config {
	if c.TokenURL == "" {
		c.TokenURL = "https://personal.connectapi.garmin.com/personalapitoken-service/personal-api/personal-api-key/token"
	}
	if c.APIBaseURL == "" {
		c.APIBaseURL = "https://personal.connectapi.garmin.com"
	}
	c.APIBaseURL = strings.TrimRight(c.APIBaseURL, "/")
	return c
}

// Client talks to Garmin for one deployment.
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

// --- pairing (a personal API key, per member) ---

// errNotOAuth is returned by the consent methods: Garmin is paired with a
// personal API key, not an authorization redirect.
var errNotOAuth = errors.New("garmin: paired with a personal API key, not an authorization redirect")

func (c *Client) AuthorizationURL(string, string) string { return "" }

func (c *Client) Exchange(context.Context, string, string, string) (devicelink.Token, error) {
	return devicelink.Token{}, errNotOAuth
}

// PairKey exchanges a member's personal API key for an access token, keeping
// the key as the refresh token so the 24-hour token can be renewed.
func (c *Client) PairKey(ctx context.Context, apiKey string) (devicelink.Token, error) {
	return c.obtainTokenWith(ctx, apiKey)
}

// Refresh obtains a new access token with the member's key, which is stored as
// the refresh token (the Personal API has no refresh token of its own).
func (c *Client) Refresh(ctx context.Context, refreshToken string) (devicelink.Token, error) {
	return c.obtainTokenWith(ctx, refreshToken)
}

// obtainTokenWith exchanges one personal API key for an access token. The token
// endpoint returns the token as plain text (a JWT), so the request must not ask
// for JSON - that is answered with a 406. The key is kept as the refresh token.
func (c *Client) obtainTokenWith(ctx context.Context, apiKey string) (devicelink.Token, error) {
	if strings.TrimSpace(apiKey) == "" {
		return devicelink.Token{}, devicelink.ErrReauthorize
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, nil)
	if err != nil {
		return devicelink.Token{}, err
	}
	req.Header.Set("Authorization", "ApiKey "+apiKey)
	req.Header.Set("Accept", "*/*")

	resp, err := c.http.Do(req)
	if err != nil {
		return devicelink.Token{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return devicelink.Token{}, err
	}
	if resp.StatusCode/100 != 2 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return devicelink.Token{}, devicelink.ErrReauthorize
		}
		return devicelink.Token{}, &apiError{status: resp.StatusCode, body: snippet(data)}
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return devicelink.Token{}, fmt.Errorf("garmin: token response was empty")
	}
	expires := jwtExpiry(token)
	if expires.IsZero() {
		expires = time.Now().Add(tokenTTL)
	} else {
		expires = expires.Add(-10 * time.Minute)
	}
	return devicelink.Token{
		AccessToken:  token,
		RefreshToken: apiKey,
		ExpiresAt:    expires,
	}, nil
}

// jwtExpiry reads the exp claim from a JWT, or the zero time when it cannot.
func jwtExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}

// CheckAccount has nothing to confirm: the member's key was already proven
// usable by the token exchange at pairing.
func (c *Client) CheckAccount(ctx context.Context, accessToken string) (string, error) {
	return "", nil
}

// --- activity ---

func (c *Client) MaxRangeDays() int     { return MaxStepsDays }
func (c *Client) MaxHeartRateDays() int { return MaxHeartRateDays }

// DailySteps returns one entry per day Garmin reports over [start, end], from
// the Connect daily-steps stats endpoint (one request for the whole range).
func (c *Client) DailySteps(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.DayCount, error) {
	path := fmt.Sprintf("/usersummary-service/stats/steps/daily/%s/%s",
		start.Format("2006-01-02"), end.Format("2006-01-02"))

	// Tolerant of the two shapes this stat is returned in - a flat totalSteps,
	// or one nested under "values" - since the account used to confirm it had
	// no data to pin the shape down.
	var rows []struct {
		CalendarDate string `json:"calendarDate"`
		TotalSteps   *int64 `json:"totalSteps"`
		Values       struct {
			TotalSteps *int64 `json:"totalSteps"`
		} `json:"values"`
	}
	if err := c.get(ctx, accessToken, path, nil, &rows); err != nil {
		return nil, err
	}
	days := []devicelink.DayCount{}
	for _, r := range rows {
		if r.CalendarDate == "" {
			continue
		}
		steps := int64(0)
		switch {
		case r.TotalSteps != nil:
			steps = *r.TotalSteps
		case r.Values.TotalSteps != nil:
			steps = *r.Values.TotalSteps
		}
		days = append(days, devicelink.DayCount{Date: r.CalendarDate, Steps: steps})
	}
	return days, nil
}

// dailySummary is the Connect daily summary, which carries the step total, the
// day's heart-rate figures, and the active calories burned.
type dailySummary struct {
	CalendarDate       string   `json:"calendarDate"`
	TotalSteps         *int64   `json:"totalSteps"`
	RestingHeartRate   *float64 `json:"restingHeartRate"`
	MinHeartRate       *float64 `json:"minHeartRate"`
	MaxHeartRate       *float64 `json:"maxHeartRate"`
	ActiveKilocalories *float64 `json:"activeKilocalories"`
}

// DailyHeartRate returns one entry per day in [start, end] that has heart-rate
// data, read a day at a time from the Connect daily summary.
func (c *Client) DailyHeartRate(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.DayHeart, error) {
	days := []devicelink.DayHeart{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		var s dailySummary
		if err := c.get(ctx, accessToken, "/usersummary-service/usersummary/daily",
			url.Values{"calendarDate": {date}}, &s); err != nil {
			return nil, err
		}
		if s.MaxHeartRate == nil && s.RestingHeartRate == nil {
			continue
		}
		max := deref(s.MaxHeartRate)
		min := deref(s.MinHeartRate)
		if min == 0 {
			min = deref(s.RestingHeartRate)
		}
		days = append(days, devicelink.DayHeart{
			Date:  date,
			Avg:   deref(s.RestingHeartRate),
			Max:   max,
			Min:   min,
			Zone:  zoneFor(max),
			Known: true,
		})
	}
	return days, nil
}

// DailyActiveCalories returns the active calories burned on each day in
// [start, end] the summary has a figure for, read a day at a time from the same
// Connect daily summary as the heart rate.
func (c *Client) DailyActiveCalories(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.DayCalories, error) {
	days := []devicelink.DayCalories{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		var s dailySummary
		if err := c.get(ctx, accessToken, "/usersummary-service/usersummary/daily",
			url.Values{"calendarDate": {date}}, &s); err != nil {
			return nil, err
		}
		if s.ActiveKilocalories == nil || *s.ActiveKilocalories <= 0 {
			continue
		}
		days = append(days, devicelink.DayCalories{Date: date, Kcal: *s.ActiveKilocalories})
	}
	return days, nil
}

func deref(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// zoneFor colours a day by its peak heart rate. Garmin's summary gives no zone,
// so these are fixed bands - enough to tell a hard day from an easy one.
func zoneFor(maxHR float64) string {
	switch {
	case maxHR >= 170:
		return devicelink.ZonePeak
	case maxHR >= 140:
		return devicelink.ZoneVigorous
	case maxHR >= 110:
		return devicelink.ZoneModerate
	default:
		return devicelink.ZoneLight
	}
}

// --- body composition ---

// weightValues are the measures a Garmin Index scale records. Weights are in
// grams.
type weightValues struct {
	Weight      *float64 `json:"weight"`
	BMI         *float64 `json:"bmi"`
	BodyFat     *float64 `json:"bodyFat"`
	BodyWater   *float64 `json:"bodyWater"`
	BoneMass    *float64 `json:"boneMass"`
	MuscleMass  *float64 `json:"muscleMass"`
	VisceralFat *float64 `json:"visceralFat"`
}

// LatestBodyComposition returns the most recent value of each measure a Garmin
// Index scale has recorded in the recent window.
func (c *Client) LatestBodyComposition(ctx context.Context, accessToken string) ([]devicelink.BodyMeasure, error) {
	now := time.Now()
	path := fmt.Sprintf("/weight-service/weight/range/%s/%s",
		now.AddDate(0, 0, -recentBodyWindow).Format("2006-01-02"), now.Format("2006-01-02"))

	var resp struct {
		DailyWeightSummaries []struct {
			SummaryDate  string        `json:"summaryDate"`
			LatestWeight *weightValues `json:"latestWeight"`
			weightValues
		} `json:"dailyWeightSummaries"`
		TotalAverage weightValues `json:"totalAverage"`
	}
	if err := c.get(ctx, accessToken, path, nil, &resp); err != nil {
		return nil, err
	}

	// The latest day with a reading; the range average only as a last resort.
	var values *weightValues
	var at time.Time
	for i := range resp.DailyWeightSummaries {
		d := resp.DailyWeightSummaries[i]
		v := d.LatestWeight
		if v == nil {
			v = &d.weightValues
		}
		if v.Weight == nil {
			continue
		}
		if when, err := time.Parse("2006-01-02", d.SummaryDate); err == nil && when.After(at) {
			at, values = when, v
		} else if values == nil {
			values = v
		}
	}
	if values == nil {
		if resp.TotalAverage.Weight == nil {
			return []devicelink.BodyMeasure{}, nil
		}
		values = &resp.TotalAverage
	}
	if at.IsZero() {
		at = now
	}

	measures := []devicelink.BodyMeasure{}
	add := func(v *float64, name, unit string, scale float64) {
		if v != nil && *v > 0 {
			measures = append(measures, devicelink.BodyMeasure{
				Type: name, Value: round1(*v * scale), Unit: unit, TakenAt: at,
			})
		}
	}
	add(values.Weight, devicelink.MeasureWeight, "kg", 0.001)
	add(values.BodyFat, devicelink.MeasureFatRatio, "%", 1)
	add(values.MuscleMass, devicelink.MeasureMuscleMass, "kg", 0.001)
	add(values.BoneMass, devicelink.MeasureBoneMass, "kg", 0.001)
	add(values.BodyWater, devicelink.MeasureHydration, "%", 1)
	add(values.BMI, devicelink.MeasureBMI, "kg/m2", 1)
	return measures, nil
}

func round1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }

// --- sleep ---

// garminSleep is one night's summary from the sleep service, tolerant of the
// two shapes it arrives in - the fields at the top level, or nested under
// dailySleepDTO. sleepTimeSeconds is Garmin's total time asleep (deep + light +
// REM, with awake excluded); when it is absent the stage seconds are summed.
type garminSleep struct {
	CalendarDate      string   `json:"calendarDate"`
	SleepTimeSeconds  *float64 `json:"sleepTimeSeconds"`
	DeepSleepSeconds  *float64 `json:"deepSleepSeconds"`
	LightSleepSeconds *float64 `json:"lightSleepSeconds"`
	RemSleepSeconds   *float64 `json:"remSleepSeconds"`
}

// garminSleepResponse reads the night whether it is at the top level or under
// dailySleepDTO, so one unmarshal covers both shapes.
type garminSleepResponse struct {
	garminSleep
	DailySleepDTO *garminSleep `json:"dailySleepDTO"`
}

// asleepMinutes is the night's minutes asleep: the total Garmin gave, or the
// sum of the stages when it gave none. Zero when the night has nothing.
func (s garminSleep) asleepMinutes() int {
	if s.SleepTimeSeconds != nil && *s.SleepTimeSeconds > 0 {
		return int(*s.SleepTimeSeconds / 60)
	}
	seconds := deref(s.DeepSleepSeconds) + deref(s.LightSleepSeconds) + deref(s.RemSleepSeconds)
	if seconds <= 0 {
		return 0
	}
	return int(seconds / 60)
}

// SleepDays returns one entry per night Garmin has sleep for over [start, end],
// read a day at a time from the Connect sleep service's daily sleep endpoint
// (Garmin keys a night by the morning woken).
//
// Per the fitness page's rule, a night the service has nothing for - or answers
// 404 for - is skipped rather than failing the week; only an expired grant stops
// the read. A run that reads no night but met an error returns the last one, so
// a wrong path or an outage reaches the troubleshoot console instead of looking
// like an empty week. The date query is sent under both of Garmin's spellings
// (calendarDate and date) so the one the service reads is always present.
func (c *Client) SleepDays(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.SleepDay, error) {
	days := []devicelink.SleepDay{}
	var lastErr error
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		var resp garminSleepResponse
		err := c.get(ctx, accessToken, "/sleep-service/sleep/dailySleep",
			url.Values{"calendarDate": {date}, "date": {date}, "nonSleepBufferMinutes": {"60"}}, &resp)
		if err != nil {
			if errors.Is(err, devicelink.ErrReauthorize) {
				return nil, err
			}
			lastErr = err
			continue
		}
		night := resp.garminSleep
		if resp.DailySleepDTO != nil {
			night = *resp.DailySleepDTO
		}
		minutes := night.asleepMinutes()
		if minutes <= 0 {
			continue
		}
		key := date
		if night.CalendarDate != "" {
			key = night.CalendarDate
		}
		days = append(days, devicelink.SleepDay{Date: key, AsleepMin: minutes})
	}
	if len(days) == 0 && lastErr != nil {
		return days, lastErr
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
		return fmt.Sprintf("garmin: api returned status %d: %s", e.status, e.body)
	}
	return fmt.Sprintf("garmin: api returned status %d", e.status)
}

func isUnauthorized(err error) bool {
	if e, ok := err.(*apiError); ok {
		return e.status == http.StatusUnauthorized || e.status == http.StatusForbidden
	}
	return false
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
	if err := c.do(req, out); err != nil {
		if isUnauthorized(err) {
			return devicelink.ErrReauthorize
		}
		return err
	}
	return nil
}

func (c *Client) do(req *http.Request, out any) error {
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
		return &apiError{status: resp.StatusCode, body: snippet(data)}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("garmin: decoding %s: %w", req.URL.Path, err)
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
