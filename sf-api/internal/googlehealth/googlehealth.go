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

// Scope is what consent asks for, read only.
//
// Three families, because Google splits them where the screen does not: steps
// and the heart rate zones earned by training are "activity and fitness", the
// beats per minute behind them are a "health metric", and sleep is its own
// family again. Showing a BPM coloured by its zone needs one of the first two;
// the week of nights needs the third.
//
// Each is a scope Google publishes, so it is safe to ask for - an unrecognised
// scope is what breaks a consent screen, not an extra real one. A member who
// linked before sleep was added has to reconnect to grant it (prompt=consent
// re-asks), which is what a MISSING_OAUTH_SCOPE on the sleep read tells them.
const Scope = "https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly " +
	"https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly " +
	"https://www.googleapis.com/auth/googlehealth.sleep.readonly"

// MaxRollupDays is the longest window the API accepts in one steps roll-up,
// and so the longest view there is. The widest the app offers is exactly this,
// which is why nothing here pages: a request that would need two is a range
// the product does not have.
const MaxRollupDays = 90

// MaxHeartRollupDays is the same thing for heart rate, where Google's limit is
// far shorter - fourteen days, against ninety for steps. It is not a detail
// that can be papered over: asking for a month of beats per minute is refused
// outright, which is why the longer views show steps alone.
const MaxHeartRollupDays = 14

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

// MaxHeartRateDays is the same for heart rate, which Google caps far shorter.
func (c *Client) MaxHeartRateDays() int { return MaxHeartRollupDays }

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
func (c *Client) Exchange(ctx context.Context, code, redirectURI string, _ string) (devicelink.Token, error) {
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
		// Not JSON at all: a proxy's error page, usually. The snippet is what
		// tells the operator that, instead of leaving them with "unreadable".
		return devicelink.Token{}, fmt.Errorf("googlehealth: token response unreadable (http %d): %s",
			resp.StatusCode, snippet(body))
	}
	if resp.StatusCode != http.StatusOK || parsed.AccessToken == "" {
		// invalid_grant is what a revoked or already-spent grant looks like,
		// and it is the one failure the member can fix, by connecting again.
		if parsed.Error == "invalid_grant" {
			return devicelink.Token{}, devicelink.ErrReauthorize
		}
		// Google's own error code and description, verbatim: "invalid_client"
		// and "redirect_uri_mismatch" are the two that actually happen, and
		// each points at a different box in the Cloud console. Anything
		// unrecognized carries the body so nothing is lost in translation.
		detail := strings.TrimSpace(parsed.Error + " " + parsed.ErrorDesc)
		if detail == "" {
			detail = snippet(body)
		}
		return devicelink.Token{}, fmt.Errorf("googlehealth: token exchange refused (http %d): %s",
			resp.StatusCode, detail)
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

type heartRollupResponse struct {
	RollupDataPoints []struct {
		CivilStartTime civilDateTime `json:"civilStartTime"`
		HeartRate      *struct {
			BeatsPerMinuteAvg float64 `json:"beatsPerMinuteAvg"`
			BeatsPerMinuteMax float64 `json:"beatsPerMinuteMax"`
			BeatsPerMinuteMin float64 `json:"beatsPerMinuteMin"`
		} `json:"heartRate"`
		TimeInHeartRateZone *struct {
			TimeInHeartRateZones []struct {
				HeartRateZone string `json:"heartRateZone"`
				Duration      string `json:"duration"`
			} `json:"timeInHeartRateZones"`
		} `json:"timeInHeartRateZone"`
	} `json:"rollupDataPoints"`
}

// zoneRank orders the provider's bands so "the hardest one reached" is a
// comparison rather than a special case per zone.
var zoneRank = map[string]int{
	"LIGHT":    1,
	"MODERATE": 2,
	"VIGOROUS": 3,
	"PEAK":     4,
}

var zoneNames = map[string]string{
	"LIGHT":    devicelink.ZoneLight,
	"MODERATE": devicelink.ZoneModerate,
	"VIGOROUS": devicelink.ZoneVigorous,
	"PEAK":     devicelink.ZonePeak,
}

// DailyHeartRate returns one entry per day from start up to and including end.
//
// Two reads, because Google keeps the beats and the zones in different data
// types - and in different scopes. The zones are what colour the number: they
// are the provider's own bands, computed against the member's own maximum
// heart rate, which is not something to re-derive here from a birthday.
func (c *Client) DailyHeartRate(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.DayHeart, error) {
	start = truncateDay(start)
	end = truncateDay(end)
	if end.Before(start) {
		return nil, nil
	}
	if earliest := end.AddDate(0, 0, -(MaxHeartRollupDays - 1)); start.Before(earliest) {
		start = earliest
	}

	days := map[string]*devicelink.DayHeart{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		days[key] = &devicelink.DayHeart{Date: key, Zone: devicelink.ZoneLight}
	}

	for _, dataType := range []string{"heart-rate", "time-in-heart-rate-zone"} {
		var body rollupRequest
		body.Range.Start = civilDateTime{Date: dateOf(start)}
		body.Range.End = civilDateTime{Date: dateOf(end.AddDate(0, 0, 1))}
		body.WindowSizeDays = 1

		var parsed heartRollupResponse
		if err := c.call(ctx, http.MethodPost,
			"/v4/users/me/dataTypes/"+dataType+"/dataPoints:dailyRollUp", accessToken, body, &parsed); err != nil {
			return nil, err
		}

		for _, point := range parsed.RollupDataPoints {
			date := point.CivilStartTime.Date
			day, ok := days[fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day)]
			if !ok {
				continue
			}
			if rate := point.HeartRate; rate != nil {
				day.Avg, day.Max, day.Min = rate.BeatsPerMinuteAvg, rate.BeatsPerMinuteMax, rate.BeatsPerMinuteMin
				day.Known = day.Avg > 0 || day.Max > 0
			}
			if zones := point.TimeInHeartRateZone; zones != nil {
				for _, zone := range zones.TimeInHeartRateZones {
					// A zone listed with no time in it is not a zone reached.
					if zone.Duration == "" || strings.HasPrefix(zone.Duration, "0s") {
						continue
					}
					if name, known := zoneNames[zone.HeartRateZone]; known &&
						zoneRank[zone.HeartRateZone] > zoneRank[strings.ToUpper(day.Zone)] {
						day.Zone = name
					}
				}
			}
		}
	}

	ordered := []devicelink.DayHeart{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		ordered = append(ordered, *days[day.Format("2006-01-02")])
	}
	return ordered, nil
}

// caloriesRollupResponse is the active-energy roll-up. PROVISIONAL: the exact
// data-type name and field are reverse-engineered from the activity family, so
// the parse is tolerant and the caller treats a miss as "no active calories",
// not a failure.
type caloriesRollupResponse struct {
	RollupDataPoints []struct {
		CivilStartTime       civilDateTime `json:"civilStartTime"`
		ActiveCaloriesBurned *struct {
			EnergySum float64 `json:"energySum"`
		} `json:"activeCaloriesBurned"`
	} `json:"rollupDataPoints"`
}

// DailyActiveCalories returns the active calories burned on each day from start
// up to and including end that Google Health has a figure for.
//
// PROVISIONAL (see caloriesRollupResponse): the active-energy data type is not
// one this app had an account to confirm against, so a shape it does not
// recognize comes back as no days rather than an error.
func (c *Client) DailyActiveCalories(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.DayCalories, error) {
	start = truncateDay(start)
	end = truncateDay(end)
	if end.Before(start) {
		return nil, nil
	}
	if earliest := end.AddDate(0, 0, -(MaxRollupDays - 1)); start.Before(earliest) {
		start = earliest
	}

	var body rollupRequest
	body.Range.Start = civilDateTime{Date: dateOf(start)}
	body.Range.End = civilDateTime{Date: dateOf(end.AddDate(0, 0, 1))}
	body.WindowSizeDays = 1

	var parsed caloriesRollupResponse
	if err := c.call(ctx, http.MethodPost,
		"/v4/users/me/dataTypes/active-calories-burned/dataPoints:dailyRollUp", accessToken, body, &parsed); err != nil {
		return nil, err
	}

	days := []devicelink.DayCalories{}
	for _, point := range parsed.RollupDataPoints {
		if point.ActiveCaloriesBurned == nil || point.ActiveCaloriesBurned.EnergySum <= 0 {
			continue
		}
		date := point.CivilStartTime.Date
		days = append(days, devicelink.DayCalories{
			Date: fmt.Sprintf("%04d-%02d-%02d", date.Year, date.Month, date.Day),
			Kcal: point.ActiveCaloriesBurned.EnergySum,
		})
	}
	return days, nil
}

// sleepListResponse is a page of sleep sessions, each a night with a summary and
// the per-stage run. Sleep is a Session record type: it is read with the list
// method (GET), not a roll-up. The night's total is summary.minutesAsleep -
// which Google defines as the asleep stages (LIGHT, REM, DEEP), awake excluded -
// and the stages are a fallback when a list response carries no summary.
type sleepListResponse struct {
	DataPoints []struct {
		Sleep *struct {
			Interval struct {
				EndTime      string        `json:"endTime"`
				CivilEndTime civilDateTime `json:"civilEndTime"`
			} `json:"interval"`
			Stages []struct {
				StartTime string `json:"startTime"`
				EndTime   string `json:"endTime"`
				Type      string `json:"type"`
			} `json:"stages"`
			Summary *struct {
				// Minutes, as a JSON string of an integer.
				MinutesAsleep string `json:"minutesAsleep"`
			} `json:"summary"`
		} `json:"sleep"`
	} `json:"dataPoints"`
}

// SleepDays reads the member's nights over [start, end] from Google Health
// (which also carries Fitbit sleep synced into it), one entry per morning woken
// with the minutes actually slept.
//
// Sleep is read with the list method and filtered by the civil end time (the
// member's local morning), because sleep is a session type with no daily
// roll-up. A night's total is summary.minutesAsleep; the night is keyed by the
// morning it ends, and when a day has more than one session the longest - the
// main sleep, not a nap - is the one kept.
func (c *Client) SleepDays(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.SleepDay, error) {
	start = truncateDay(start)
	end = truncateDay(end)
	if end.Before(start) {
		return nil, nil
	}

	// A night ending on or after the first morning asked for. The filter is on
	// civil (local) end time, the same clock the mornings are counted in.
	filter := fmt.Sprintf(`sleep.interval.civil_end_time >= "%s"`, start.Format("2006-01-02"))
	path := "/v4/users/me/dataTypes/sleep/dataPoints?pageSize=100&filter=" + url.QueryEscape(filter)

	var parsed sleepListResponse
	if err := c.call(ctx, http.MethodGet, path, accessToken, nil, &parsed); err != nil {
		return nil, err
	}

	byDate := map[string]int{}
	for _, point := range parsed.DataPoints {
		if point.Sleep == nil {
			continue
		}
		minutes := 0
		if point.Sleep.Summary != nil {
			if parsedMinutes, err := strconv.Atoi(strings.TrimSpace(point.Sleep.Summary.MinutesAsleep)); err == nil {
				minutes = parsedMinutes
			}
		}
		if minutes <= 0 {
			// No summary on this list response: sum the asleep stages instead.
			minutes = asleepStageMinutes(point.Sleep.Stages)
		}
		if minutes <= 0 {
			continue
		}
		date := civilDateKey(point.Sleep.Interval.CivilEndTime.Date)
		if date == "" {
			// No civil end time: fall back to the UTC end timestamp's date.
			if when, e := time.Parse(time.RFC3339, point.Sleep.Interval.EndTime); e == nil {
				date = when.Format("2006-01-02")
			}
		}
		if date == "" {
			continue
		}
		// The longest session a day has is its main sleep; a shorter nap on the
		// same date does not add to it.
		if minutes > byDate[date] {
			byDate[date] = minutes
		}
	}

	days := []devicelink.SleepDay{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if minutes, ok := byDate[key]; ok {
			days = append(days, devicelink.SleepDay{Date: key, AsleepMin: minutes})
		}
	}
	return days, nil
}

// civilDateKey formats a civil date as YYYY-MM-DD, or blank when it is unset.
func civilDateKey(d civilDate) string {
	if d.Year == 0 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// asleepStageMinutes sums the minutes of the stages that count as asleep - every
// stage but AWAKE, which is how Google defines the night's asleep total. The
// fallback for a list response that carries the stages but no summary.
func asleepStageMinutes(stages []struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Type      string `json:"type"`
}) int {
	total := 0
	for _, stage := range stages {
		if stage.Type == "AWAKE" || stage.Type == "SLEEP_STAGE_TYPE_UNSPECIFIED" {
			continue
		}
		start, err1 := time.Parse(time.RFC3339, stage.StartTime)
		end, err2 := time.Parse(time.RFC3339, stage.EndTime)
		if err1 != nil || err2 != nil || !end.After(start) {
			continue
		}
		total += int(end.Sub(start).Minutes())
	}
	return total
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
		case strings.Contains(failure.Error.Status, "MISSING_OAUTH_SCOPE"),
			strings.Contains(failure.Error.Message, "MISSING_OAUTH_SCOPE"):
			// A link made before this app asked for a scope it now needs.
			// The member has to consent again, which is what ErrReauthorize
			// already tells them to do.
			return devicelink.ErrReauthorize
		}
		// SERVICE_DISABLED here means the Google Health API is not enabled on
		// the Cloud project - a different fix from anything to do with the
		// OAuth client, and invisible unless the status is carried through.
		detail := strings.TrimSpace(failure.Error.Status + " " + failure.Error.Message)
		if detail == "" {
			detail = snippet(raw)
		}
		return fmt.Errorf("googlehealth: %s %s refused (http %d): %s", method, path, resp.StatusCode, detail)
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// snippet is as much of an unexpected response as is worth putting in a log
// line: enough to recognize an HTML error page or an unfamiliar error code,
// not so much that a log file fills with somebody's markup.
func snippet(body []byte) string {
	const limit = 300
	text := strings.TrimSpace(string(body))
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func dateOf(t time.Time) civilDate {
	return civilDate{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}
}
