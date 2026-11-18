// Package notify announces events to a Discord channel through an incoming
// webhook. It is the outward half of the IP feature: a sign-in, a sign-out, a
// finished session or a new post, each posted with who did it and - resolved on
// the spot from the address - where they did it from.
//
// It posts to the webhook's Slack-compatible endpoint
// (https://discord.com/api/webhooks/{token}/slack), the same shape the CI
// deploy notification uses, so one DISCORD_TOKEN drives both. Everything here
// is best-effort: a webhook that is slow or down must never be felt by the
// member whose action triggered the message, so the caller runs Notify on its
// own goroutine and Notify itself never returns an error to surface.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// webhookTpl turns a DISCORD_TOKEN into the Slack-compatible webhook URL. The
// token is the "{id}/{token}" segment Discord issues; the /slack suffix is what
// makes the endpoint accept a Slack attachment payload.
const webhookTpl = "https://discord.com/api/webhooks/%s/slack"

// disabledToken is the placeholder a deployment that has not set a real webhook
// carries (matching the API's default), treated as "no notifier".
const disabledToken = "changeit"

// Enricher resolves what is known about an address. It matches geo.Geo.Enrich
// and middleware.Enricher structurally, so main wires the same function here
// without this package importing either.
type Enricher func(ctx context.Context, ip string) (map[string]string, error)

// Discord posts to one incoming webhook's Slack endpoint.
type Discord struct {
	url    string
	enrich Enricher
	http   *http.Client
}

// NewDiscord builds a notifier from a DISCORD_TOKEN. It returns nil for a blank
// or placeholder token so main can leave the handlers' notifier unset - the
// no-op case - rather than wire a sender with nowhere to send.
func NewDiscord(token string, enrich Enricher) *Discord {
	token = strings.TrimSpace(token)
	if token == "" || token == disabledToken {
		return nil
	}
	return &Discord{
		url:    fmt.Sprintf(webhookTpl, token),
		enrich: enrich,
		http:   &http.Client{Timeout: 5 * time.Second},
	}
}

// slackMessage is the Slack-compatible payload the /slack endpoint accepts.
type slackMessage struct {
	Username    string            `json:"username,omitempty"`
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color  string       `json:"color,omitempty"`
	Title  string       `json:"title"`
	Fields []slackField `json:"fields,omitempty"`
	TS     int64        `json:"ts,omitempty"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Notify resolves the address's geolocation and posts the event. Synchronous by
// design - the caller owns the goroutine - and silent on failure: a missed chat
// message is not worth more than a warning, let alone an error the member sees.
func (d *Discord) Notify(ctx context.Context, event, member, email, ip string) {
	var geo map[string]string
	if d.enrich != nil && ip != "" {
		if info, err := d.enrich(ctx, ip); err == nil {
			geo = info
		}
	}

	payload := slackMessage{
		Username: "StrongFish",
		Attachments: []slackAttachment{{
			Color:  colorFor(event),
			Title:  event,
			TS:     time.Now().UTC().Unix(),
			Fields: fieldsFor(member, email, ip, geo),
		}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.http.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "discord notification failed", "event", event, "error", err)
		return
	}
	_ = resp.Body.Close()
}

// fieldsFor lays out the member, the address, and whatever geolocation came
// back, dropping anything blank so a lookup that found nothing leaves no empty
// fields behind.
func fieldsFor(member, email, ip string, geo map[string]string) []slackField {
	who := member
	if email != "" {
		who = fmt.Sprintf("%s (%s)", member, email)
	}
	fields := []slackField{{Title: "Member", Value: orDash(who), Short: false}}
	fields = append(fields, slackField{Title: "IP", Value: orDash(ip), Short: true})

	if loc := location(geo); loc != "" {
		fields = append(fields, slackField{Title: "Location", Value: loc, Short: true})
	}
	if net := network(geo); net != "" {
		fields = append(fields, slackField{Title: "Network", Value: net, Short: true})
	}
	return fields
}

// location reads "City, Region, Country" out of the geo map, using only the
// parts that resolved.
func location(geo map[string]string) string {
	if geo == nil {
		return ""
	}
	parts := []string{}
	for _, key := range []string{"city", "region", "country"} {
		if v := strings.TrimSpace(geo[key]); v != "" {
			parts = append(parts, v)
		}
	}
	loc := strings.Join(parts, ", ")
	if code := strings.TrimSpace(geo["countryCode"]); code != "" && loc != "" {
		loc += " (" + code + ")"
	}
	return loc
}

// network names the operator behind the address - the ISP, or the organisation
// and its AS number when that is all that resolved.
func network(geo map[string]string) string {
	if geo == nil {
		return ""
	}
	if isp := strings.TrimSpace(geo["isp"]); isp != "" {
		return isp
	}
	org := strings.TrimSpace(geo["org"])
	asn := strings.TrimSpace(geo["asn"])
	switch {
	case org != "" && asn != "":
		return org + " · " + asn
	case org != "":
		return org
	default:
		return asn
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// colorFor tints the attachment by what happened: green for arriving, grey for
// leaving, blue for activity - a glanceable cue in a busy channel.
func colorFor(event string) string {
	switch {
	case strings.Contains(event, "Sign-in"), strings.Contains(event, "Sign-up"):
		return "#2ecc71"
	case strings.Contains(event, "Sign-out"):
		return "#95a5a6"
	default:
		return "#3498db"
	}
}
