// Package geo turns a bare IP address into what is publicly known about where
// it is - a country, a city, the network behind it. It is the enrichment the
// sessions table stores alongside each address, so the IP administration screen
// can say "this is a residential line in Lyon" rather than only "1.2.3.4".
//
// It calls out to two public services and keeps nothing of its own: ip-api.com
// first (keyless, plain HTTP, generous free tier) and ipinfo.io as a fallback
// (HTTPS, optional token). Both failing is not an error the caller should care
// about - an address with no geo is still a perfectly good address - so Enrich
// returns a nil map rather than failing the request that triggered it.
package geo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxBody caps how much of a provider's answer is read: a geo lookup is a small
// JSON object, and a provider having a bad day must not stream megabytes into a
// best-effort background call.
const maxBody = 64 << 10

// Geo is a reusable enricher. It is safe for concurrent use - the only state is
// an http.Client and an optional token, both read-only after construction.
type Geo struct {
	ipinfoToken string
	http        *http.Client
}

// New builds an enricher. The ipinfo token may be empty: ipinfo answers without
// one at low volume, and ip-api - tried first - needs no key at all.
func New(ipinfoToken string) *Geo {
	return &Geo{ipinfoToken: ipinfoToken, http: &http.Client{Timeout: 5 * time.Second}}
}

// Enrich resolves what is known about an address, trying ip-api first and
// ipinfo second. A nil map (both sources silent) is a valid answer, not a
// failure: the caller stores whatever comes back and moves on.
func (g *Geo) Enrich(ctx context.Context, ip string) (map[string]string, error) {
	if info := g.ipapi(ctx, ip); info != nil {
		return info, nil
	}
	if info := g.ipinfo(ctx, ip); info != nil {
		return info, nil
	}
	return nil, nil
}

// ipapi queries ip-api.com. It asks for exactly the fields that are stored and
// treats anything but status:"success" (a reserved range, a rate-limit) as "no
// answer" by returning nil.
func (g *Geo) ipapi(ctx context.Context, ip string) map[string]string {
	url := "http://ip-api.com/json/" + ip +
		"?fields=status,country,countryCode,region,regionName,city,zip,lat,lon,timezone,isp,org,as,query"
	var r struct {
		Status      string  `json:"status"`
		Country     string  `json:"country"`
		CountryCode string  `json:"countryCode"`
		RegionName  string  `json:"regionName"`
		City        string  `json:"city"`
		Zip         string  `json:"zip"`
		Lat         float64 `json:"lat"`
		Lon         float64 `json:"lon"`
		Timezone    string  `json:"timezone"`
		ISP         string  `json:"isp"`
		Org         string  `json:"org"`
		AS          string  `json:"as"`
	}
	if !g.getJSON(ctx, url, &r) || r.Status != "success" {
		return nil
	}
	return nonEmpty(map[string]string{
		"source":      "ip-api.com",
		"country":     r.Country,
		"countryCode": r.CountryCode,
		"region":      r.RegionName,
		"city":        r.City,
		"zip":         r.Zip,
		"timezone":    r.Timezone,
		"isp":         r.ISP,
		"org":         r.Org,
		"asn":         r.AS,
		"lat":         ftoa(r.Lat),
		"lon":         ftoa(r.Lon),
	})
}

// ipinfo queries ipinfo.io. It rejects the "bogon" answer the service gives for
// private/reserved ranges, so a lookup of a LAN address stores nothing rather
// than a placeholder.
func (g *Geo) ipinfo(ctx context.Context, ip string) map[string]string {
	url := "https://ipinfo.io/" + ip + "/json"
	if g.ipinfoToken != "" {
		url += "?token=" + g.ipinfoToken
	}
	var r struct {
		Country  string `json:"country"`
		Region   string `json:"region"`
		City     string `json:"city"`
		Postal   string `json:"postal"`
		Timezone string `json:"timezone"`
		Org      string `json:"org"`
		Loc      string `json:"loc"`
		Bogon    bool   `json:"bogon"`
	}
	if !g.getJSON(ctx, url, &r) || r.Bogon || r.Country == "" {
		return nil
	}
	info := map[string]string{
		"source":      "ipinfo.io",
		"country":     r.Country,
		"countryCode": r.Country,
		"region":      r.Region,
		"city":        r.City,
		"zip":         r.Postal,
		"timezone":    r.Timezone,
		"org":         r.Org,
	}
	// ipinfo packs the coordinates as "lat,lng"; split them so the stored shape
	// matches ip-api's separate lat/lon.
	if lat, lon, ok := strings.Cut(r.Loc, ","); ok {
		info["lat"] = strings.TrimSpace(lat)
		info["lon"] = strings.TrimSpace(lon)
	}
	return nonEmpty(info)
}

// getJSON runs one GET and decodes it, returning false on any trouble - a
// non-200, a transport error, a body that will not parse. It never surfaces the
// error: a failed lookup is a nil enrichment, which the caller already handles.
func (g *Geo) getJSON(ctx context.Context, url string, out any) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return false
	}
	return json.Unmarshal(body, out) == nil
}

// nonEmpty drops blank values so the stored object carries only what was
// actually resolved - an address with no known city has no "city" key rather
// than an empty one.
func nonEmpty(m map[string]string) map[string]string {
	for k, v := range m {
		if strings.TrimSpace(v) == "" {
			delete(m, k)
		}
	}
	return m
}

// ftoa renders a coordinate without a trailing ".000000", and treats an exact
// zero as "not known" - 0,0 is in the ocean, never a real geolocation.
func ftoa(f float64) string {
	if f == 0 {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
