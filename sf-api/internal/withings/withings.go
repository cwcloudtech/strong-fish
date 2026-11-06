// Package withings reads a member's body composition from their Withings
// scale - weight, fat, muscle, water, bone, whatever their model measures -
// and the steps and heart rate their Withings watch counted.
//
// The flow is Withings' own OAuth 2.0 authorization code grant, the "app to
// app" integration of their public API: the member consents on Withings' site,
// the callback exchanges the code, and the grant is stored per member like any
// other device link. Read only: nothing is ever written back to somebody's
// scale history.
//
// Two things set Withings apart from Google and are handled here rather than
// leaking into the handlers: every answer is HTTP 200 with the real outcome in
// a "status" field of the body, and every refresh rotates the refresh token, so
// the one sent back has to replace the one stored.
package withings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"strong-fish-api/internal/devicelink"
)

// Provider is the name this link is stored under (see the device_links table).
const Provider = "withings"

// Scope is what consent asks for: the scale's measures and the watch's daily
// activity, and nothing about the account itself.
const Scope = "user.metrics,user.activity"

// MaxActivityDays is the widest view offered from Withings, steps and heart
// rate alike: unlike Google it keeps heart rate as a daily summary next to the
// steps, so there is no shorter cap for one of them.
const MaxActivityDays = 90

// Config is what an operator sets. The two URLs are configurable so the flow
// can be pointed at a stand-in during development; in production they are left
// at their defaults.
type Config struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	APIBaseURL   string
}

// Enabled reports whether an operator has configured the integration.
func (c Config) Enabled() bool {
	return strings.TrimSpace(c.ClientID) != "" && strings.TrimSpace(c.ClientSecret) != ""
}

// WithDefaults fills in Withings' own endpoints for anything left blank.
func (c Config) WithDefaults() Config {
	if c.AuthURL == "" {
		c.AuthURL = "https://account.withings.com/oauth2_user/authorize2"
	}
	if c.APIBaseURL == "" {
		c.APIBaseURL = "https://wbsapi.withings.net"
	}
	c.APIBaseURL = strings.TrimRight(c.APIBaseURL, "/")
	return c
}

// Client talks to Withings on behalf of one deployment.
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

// AuthorizationURL is where the member's browser goes to consent.
//
// No equivalent of Google's access_type=offline: Withings always issues a
// refresh token with the code exchange.
func (c *Client) AuthorizationURL(redirectURI, state string) string {
	q := make(url.Values)
	q.Set("response_type", "code")
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", Scope)
	q.Set("state", state)
	return c.cfg.AuthURL + "?" + q.Encode()
}

// Exchange turns the code from the consent redirect into a grant.
func (c *Client) Exchange(ctx context.Context, code, redirectURI string, _ string) (devicelink.Token, error) {
	return c.token(ctx, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	})
}

// Refresh renews an expired access token. Withings rotates the refresh token
// on every call - the old one stops working once the new one is issued - so
// the token returned here is the one to store.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (devicelink.Token, error) {
	token, err := c.token(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		// A refresh token that is refused is one that was revoked, already
		// rotated away, or older than a year: in every case the member has to
		// connect again, and retrying will not change that.
		if _, refused := err.(*apiError); refused {
			return devicelink.Token{}, devicelink.ErrReauthorize
		}
		return devicelink.Token{}, err
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}

type tokenBody struct {
	// A number in the documentation and a string in some answers; read as
	// either, since all it is used for is a support lookup.
	UserID       json.RawMessage `json:"userid"`
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	ExpiresIn    int             `json:"expires_in"`
	Scope        string          `json:"scope"`
}

func (c *Client) token(ctx context.Context, form url.Values) (devicelink.Token, error) {
	form.Set("action", "requesttoken")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)

	var parsed tokenBody
	if err := c.call(ctx, "/v2/oauth2", "", form, &parsed); err != nil {
		return devicelink.Token{}, err
	}
	if parsed.AccessToken == "" {
		return devicelink.Token{}, fmt.Errorf("withings: token response carried no access token")
	}

	return devicelink.Token{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		// A minute short, so a token that is about to lapse is refreshed
		// before a request rather than after it fails.
		ExpiresAt: time.Now().Add(time.Duration(parsed.ExpiresIn)*time.Second - time.Minute),
		Scope:     parsed.Scope,
		UserID:    strings.Trim(string(parsed.UserID), `"`),
	}, nil
}

// CheckAccount confirms the fresh grant can read measures at all. The id the
// member is known by already came with the token, so it answers blank and the
// caller keeps that one.
//
// A Withings account with no scale is not a failure here: it reads as no
// measures yet, which is what the panel says.
func (c *Client) CheckAccount(ctx context.Context, accessToken string) (string, error) {
	now := time.Now()
	_, err := c.measures(ctx, accessToken, []int{typeWeight}, now.AddDate(0, 0, -1), now)
	return "", err
}

// MaxRangeDays is the widest steps view.
func (c *Client) MaxRangeDays() int { return MaxActivityDays }

// MaxHeartRateDays is the same window: the heart rate comes from the same
// daily summary as the steps.
func (c *Client) MaxHeartRateDays() int { return MaxActivityDays }

type activityBody struct {
	Activities []struct {
		Date      string  `json:"date"`
		Steps     int64   `json:"steps"`
		HRAverage float64 `json:"hr_average"`
		HRMin     float64 `json:"hr_min"`
		HRMax     float64 `json:"hr_max"`
		// Seconds spent in each of Withings' four zones, lightest first.
		HRZone0 int `json:"hr_zone_0"`
		HRZone1 int `json:"hr_zone_1"`
		HRZone2 int `json:"hr_zone_2"`
		HRZone3 int `json:"hr_zone_3"`
	} `json:"activities"`
	More   bool `json:"more"`
	Offset int  `json:"offset"`
}

// activityFields is every daily summary field read: one call answers both the
// steps and the heart rate.
const activityFields = "steps,hr_average,hr_min,hr_max,hr_zone_0,hr_zone_1,hr_zone_2,hr_zone_3"

// activity reads the daily summaries from start to end inclusive, keyed by
// date. A day the watch reported nothing for is absent.
func (c *Client) activity(ctx context.Context, accessToken string, start, end time.Time) (
	map[string]devicelink.DayHeart, map[string]int64, error) {
	hearts := map[string]devicelink.DayHeart{}
	steps := map[string]int64{}
	offset := 0
	for page := 0; page < maxPages; page++ {
		form := url.Values{
			"action":       {"getactivity"},
			"startdateymd": {start.Format("2006-01-02")},
			"enddateymd":   {end.Format("2006-01-02")},
			"data_fields":  {activityFields},
		}
		if offset > 0 {
			form.Set("offset", strconv.Itoa(offset))
		}

		var parsed activityBody
		if err := c.call(ctx, "/v2/measure", accessToken, form, &parsed); err != nil {
			return nil, nil, err
		}
		for _, day := range parsed.Activities {
			// Several devices can report the same day - a watch and the phone
			// app - and the busiest is the one that was actually worn.
			if day.Steps > steps[day.Date] {
				steps[day.Date] = day.Steps
			}
			if day.HRAverage <= 0 {
				continue
			}
			zone := devicelink.ZoneLight
			switch {
			case day.HRZone3 > 0:
				zone = devicelink.ZonePeak
			case day.HRZone2 > 0:
				zone = devicelink.ZoneVigorous
			case day.HRZone1 > 0:
				zone = devicelink.ZoneModerate
			}
			hearts[day.Date] = devicelink.DayHeart{
				Date: day.Date, Avg: day.HRAverage, Max: day.HRMax, Min: day.HRMin,
				Zone: zone, Known: true,
			}
		}
		if !parsed.More || parsed.Offset == 0 {
			break
		}
		offset = parsed.Offset
	}
	return hearts, steps, nil
}

// DailySteps returns one entry per day from start up to and including end.
// Days with nothing reported come back as zero, as with every provider.
func (c *Client) DailySteps(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.DayCount, error) {
	start, end, ok := clampDays(start, end)
	if !ok {
		return nil, nil
	}
	_, counts, err := c.activity(ctx, accessToken, start, end)
	if err != nil {
		return nil, err
	}
	days := []devicelink.DayCount{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		days = append(days, devicelink.DayCount{Date: key, Steps: counts[key]})
	}
	return days, nil
}

// DailyActiveCalories returns the active calories burned on each day over the
// span, from the same daily summary as steps and heart rate: Withings reports
// an active "calories" alongside the steps, so no extra call is needed.
func (c *Client) DailyActiveCalories(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.DayCalories, error) {
	start, end, ok := clampDays(start, end)
	if !ok {
		return nil, nil
	}
	calories, err := c.activeCalories(ctx, accessToken, start, end)
	if err != nil {
		return nil, err
	}
	days := []devicelink.DayCalories{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if kcal, ok := calories[key]; ok && kcal > 0 {
			days = append(days, devicelink.DayCalories{Date: key, Kcal: kcal})
		}
	}
	return days, nil
}

// activeCalories reads the daily active calories, keyed by date. A separate
// call from activity() so the busy steps/heart path is untouched; the busiest
// device wins the day, as with steps.
func (c *Client) activeCalories(ctx context.Context, accessToken string, start, end time.Time) (map[string]float64, error) {
	calories := map[string]float64{}
	offset := 0
	for page := 0; page < maxPages; page++ {
		form := url.Values{
			"action":       {"getactivity"},
			"startdateymd": {start.Format("2006-01-02")},
			"enddateymd":   {end.Format("2006-01-02")},
			"data_fields":  {"calories"},
		}
		if offset > 0 {
			form.Set("offset", strconv.Itoa(offset))
		}
		var parsed struct {
			Activities []struct {
				Date     string  `json:"date"`
				Calories float64 `json:"calories"`
			} `json:"activities"`
			More   bool `json:"more"`
			Offset int  `json:"offset"`
		}
		if err := c.call(ctx, "/v2/measure", accessToken, form, &parsed); err != nil {
			return nil, err
		}
		for _, day := range parsed.Activities {
			if day.Calories > calories[day.Date] {
				calories[day.Date] = day.Calories
			}
		}
		if !parsed.More || parsed.Offset == 0 {
			break
		}
		offset = parsed.Offset
	}
	return calories, nil
}

// DailyHeartRate returns one entry per day over the same span. Withings' four
// zones are mapped in order onto the four the app colours, and a day's zone is
// the hardest one it spent any time in.
func (c *Client) DailyHeartRate(ctx context.Context, accessToken string, start, end time.Time) ([]devicelink.DayHeart, error) {
	start, end, ok := clampDays(start, end)
	if !ok {
		return nil, nil
	}
	hearts, _, err := c.activity(ctx, accessToken, start, end)
	if err != nil {
		return nil, err
	}
	days := []devicelink.DayHeart{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if found, ok := hearts[key]; ok {
			days = append(days, found)
		} else {
			days = append(days, devicelink.DayHeart{Date: key, Zone: devicelink.ZoneLight})
		}
	}
	return days, nil
}

// clampDays truncates both ends to whole days and keeps the span within
// MaxActivityDays, shortening rather than refusing, as Google's client does.
func clampDays(start, end time.Time) (time.Time, time.Time, bool) {
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	if end.Before(start) {
		return start, end, false
	}
	if earliest := end.AddDate(0, 0, -(MaxActivityDays - 1)); start.Before(earliest) {
		start = earliest
	}
	return start, end, true
}

// Withings' numeric measure types, the ones a body composition scale takes.
const (
	typeWeight             = 1
	typeHeight             = 4
	typeFatFreeMass        = 5
	typeFatRatio           = 6
	typeFatMass            = 8
	typeMuscleMass         = 76
	typeHydration          = 77
	typeBoneMass           = 88
	typeVisceralFat        = 170
	typeBasalMetabolicRate = 226
)

// measureKind is how one Withings type reaches the clients, in the order the
// panel lists them: weight first, since every scale takes it and it is what a
// member looks for, then what it is made of.
var measureKinds = []struct {
	code int
	name string
	unit string
}{
	{typeWeight, devicelink.MeasureWeight, "kg"},
	{typeFatRatio, devicelink.MeasureFatRatio, "%"},
	{typeFatMass, devicelink.MeasureFatMass, "kg"},
	{typeFatFreeMass, devicelink.MeasureFatFreeMass, "kg"},
	{typeMuscleMass, devicelink.MeasureMuscleMass, "kg"},
	{typeHydration, devicelink.MeasureHydration, "kg"},
	{typeBoneMass, devicelink.MeasureBoneMass, "kg"},
	{typeVisceralFat, devicelink.MeasureVisceralFat, ""},
	{typeBasalMetabolicRate, devicelink.MeasureBasalMetabolicRate, "kcal"},
}

// recentWindow is how far back the first read reaches. A year is long enough
// that anybody who still steps on their scale has every measure it takes in
// it, and short enough that a daily weigher's history is a few pages rather
// than a few thousand groups.
const recentWindow = 365

// maxPages bounds one read. Withings pages its answers; a history that would
// need more than this is one where the latest of everything was found long
// before the last page.
const maxPages = 10

// reading is the latest value seen of one type.
type reading struct {
	value float64
	at    time.Time
}

// LatestBodyComposition returns the most recent value of every measure the
// member's scale has taken.
//
// The last year first, and the whole history only when that year is empty: a
// member who has not weighed in since still gets their last reading, dated, and
// everybody else is spared a read of years of daily weigh-ins.
//
// Height is read on its own, over all time, because it is not something a
// scale measures: it was typed in once, often years ago, and is only wanted
// here to turn the latest weight into a BMI.
func (c *Client) LatestBodyComposition(ctx context.Context, accessToken string) ([]devicelink.BodyMeasure, error) {
	types := make([]int, len(measureKinds))
	for i, kind := range measureKinds {
		types[i] = kind.code
	}

	now := time.Now()
	latest, err := c.measures(ctx, accessToken, types, now.AddDate(0, 0, -recentWindow), now)
	if err != nil {
		return nil, err
	}
	if len(latest) == 0 {
		if latest, err = c.measures(ctx, accessToken, types, time.Time{}, now); err != nil {
			return nil, err
		}
	}

	results := []devicelink.BodyMeasure{}
	for _, kind := range measureKinds {
		if found, ok := latest[kind.code]; ok {
			results = append(results, devicelink.BodyMeasure{
				Type: kind.name, Value: found.value, Unit: kind.unit, TakenAt: found.at,
			})
		}
	}

	// Without a height there is no BMI, which is a missing line rather than a
	// failed panel.
	if weight, ok := latest[typeWeight]; ok {
		heights, err := c.measures(ctx, accessToken, []int{typeHeight}, time.Time{}, now)
		if err == nil {
			if height, ok := heights[typeHeight]; ok && height.value > 0 {
				results = append(results, devicelink.BodyMeasure{
					Type:    devicelink.MeasureBMI,
					Value:   math.Round(weight.value/(height.value*height.value)*10) / 10,
					Unit:    "kg/m2",
					TakenAt: weight.at,
				})
			}
		}
	}
	return results, nil
}

type measureBody struct {
	MeasureGroups []struct {
		Date     int64 `json:"date"`
		Category int   `json:"category"`
		Measures []struct {
			Value int64 `json:"value"`
			Type  int   `json:"type"`
			Unit  int   `json:"unit"`
		} `json:"measures"`
	} `json:"measuregrps"`
	More   int `json:"more"`
	Offset int `json:"offset"`
}

// measures reads every group of the given types between since and until, and
// keeps the latest value of each type. A zero since means all of history.
func (c *Client) measures(ctx context.Context, accessToken string, types []int,
	since, until time.Time) (map[int]reading, error) {
	codes := make([]string, len(types))
	for i, code := range types {
		codes[i] = strconv.Itoa(code)
	}

	latest := map[int]reading{}
	offset := 0
	for page := 0; page < maxPages; page++ {
		form := url.Values{
			"action":    {"getmeas"},
			"meastypes": {strings.Join(codes, ",")},
			// 1 is a real measurement; 2 would be the goals the member set.
			"category": {"1"},
			"enddate":  {strconv.FormatInt(until.Unix(), 10)},
		}
		if !since.IsZero() {
			form.Set("startdate", strconv.FormatInt(since.Unix(), 10))
		}
		if offset > 0 {
			form.Set("offset", strconv.Itoa(offset))
		}

		var parsed measureBody
		if err := c.call(ctx, "/measure", accessToken, form, &parsed); err != nil {
			return nil, err
		}

		for _, group := range parsed.MeasureGroups {
			if group.Category != 1 {
				continue
			}
			at := time.Unix(group.Date, 0).UTC()
			for _, measure := range group.Measures {
				if current, seen := latest[measure.Type]; seen && !at.After(current.at) {
					continue
				}
				// The value is an integer and a power of ten: 72450 and -3 is
				// 72.45 kg. Rounded back to the precision it was sent with, so
				// floating point does not print 72.45000000000001.
				value := float64(measure.Value) * math.Pow10(measure.Unit)
				if measure.Unit < 0 {
					scale := math.Pow10(-measure.Unit)
					value = math.Round(value*scale) / scale
				}
				latest[measure.Type] = reading{value: value, at: at}
			}
		}

		if parsed.More == 0 || parsed.Offset == 0 {
			break
		}
		offset = parsed.Offset
	}
	return latest, nil
}

// apiError is Withings answering with a non-zero status: the request reached
// them and was refused, as opposed to never arriving.
type apiError struct {
	Path   string
	Status int
	Detail string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("withings: %s refused (status %d): %s", e.Path, e.Status, e.Detail)
}

// Statuses that mean the access token itself is no good: missing, expired,
// revoked, or issued for a scope that does not cover the call.
var authStatuses = map[int]bool{100: true, 101: true, 102: true, 200: true, 401: true}

type envelope struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
	Error  string          `json:"error"`
}

func (c *Client) call(ctx context.Context, path, accessToken string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIBaseURL+path,
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
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

	var parsed envelope
	if err := json.Unmarshal(raw, &parsed); err != nil {
		// Not JSON at all: a proxy's error page, usually.
		return fmt.Errorf("withings: %s unreadable (http %d): %s", path, resp.StatusCode, snippet(raw))
	}
	if parsed.Status != 0 {
		// The token endpoint answers a bad client_id or redirect URI with its
		// own status too; those go to the log with Withings' wording, while
		// a dead access token on a read is what the member can fix.
		if accessToken != "" && authStatuses[parsed.Status] {
			return devicelink.ErrReauthorize
		}
		detail := parsed.Error
		if detail == "" {
			detail = snippet(raw)
		}
		return &apiError{Path: path, Status: parsed.Status, Detail: detail}
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(parsed.Body, out)
}

// snippet is as much of an unexpected response as is worth putting in a log
// line.
func snippet(body []byte) string {
	const limit = 300
	text := strings.TrimSpace(string(body))
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}
