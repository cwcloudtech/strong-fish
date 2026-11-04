package stance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LastSets reads the member's sets from Supabase and returns the most recent
// of each lift, newest first. The sessions RPC is keyed by the Firebase uid
// directly - that is what the Stance app passes as user_id_param (its own
// client names the argument firebaseId), and the RPC joins to the member
// server-side. Reads carry the anon key only - there is no per-member Supabase
// token.
func (c *client) LastSets(ctx context.Context, firebaseUID string) ([]Set, error) {
	rows, err := c.fetchSets(ctx, firebaseUID)
	if err != nil {
		return nil, err
	}

	// Keep the newest set of each of the three lifts.
	newest := map[string]Set{}
	for _, row := range rows {
		lift := matchLift(row.Exercise)
		if lift == "" {
			continue
		}
		set := row.toSet(lift)
		if cur, ok := newest[lift]; !ok || set.PerformedAt.After(cur.PerformedAt) {
			newest[lift] = set
		}
	}
	out := make([]Set, 0, len(newest))
	for _, set := range newest {
		out = append(out, set)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PerformedAt.After(out[j].PerformedAt) })
	return out, nil
}

// fetchSets calls the sessions RPC for one member and returns the flattened
// sets. The user_id_param is the member's Firebase uid - the RPC resolves it
// to the sets server-side, exactly as the Stance app's own client does.
func (c *client) fetchSets(ctx context.Context, firebaseUID string) ([]fullSet, error) {
	endpoint := c.cfg.SupabaseURL + "/rest/v1/rpc/" + c.cfg.SessionsRPC
	body := map[string]any{"user_id_param": firebaseUID}

	// The RPC returns the user's sets, each with its reps embedded. Some
	// deployments wrap them in sessions; both shapes are accepted.
	status, data, err := c.supabaseCall(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}
	if status/100 != 2 {
		return nil, fmt.Errorf("stance: sessions read failed (status %d: %s)", status, snippet(data))
	}
	return flattenSets(data), nil
}

// flattenSets reads the RPC payload whether it is a flat array of sets or an
// array of sessions each carrying a "sets" array.
func flattenSets(raw json.RawMessage) []fullSet {
	var flat []fullSet
	if err := json.Unmarshal(raw, &flat); err == nil {
		// If these look like sets (have reps or an exercise), take them.
		for _, s := range flat {
			if s.Exercise != "" || len(s.Repetitions) > 0 {
				return flat
			}
		}
	}
	var sessions []struct {
		Sets []fullSet `json:"sets"`
	}
	if err := json.Unmarshal(raw, &sessions); err == nil {
		var out []fullSet
		for _, s := range sessions {
			out = append(out, s.Sets...)
		}
		if len(out) > 0 {
			return out
		}
	}
	return flat
}

// fullSet is one set as the sessions RPC returns it, in Stance's field names.
// Only the fields StrongFish charts are modelled; the rest are ignored.
type fullSet struct {
	ID           string    `json:"id"`
	Exercise     string    `json:"exercise"`
	Weight       float64   `json:"weight"`
	WeightUnit   string    `json:"weight_unit_name"`
	PredictedRPE string    `json:"predicted_rpe_tag"`
	GivenRPE     string    `json:"rpe"`
	SetTag       string    `json:"tag"`
	Started      string    `json:"started"`
	Ended        string    `json:"ended"`
	Repetitions  []fullRep `json:"repetitions"`
}

type fullRep struct {
	MeanConcentricVelocity float64 `json:"mean_concentric_velocity"`
	PeakConcentricVelocity float64 `json:"peak_concentric_velocity"`
	RepOrder               int     `json:"rep_order"`
}

func (r fullSet) toSet(lift string) Set {
	reps := append([]fullRep(nil), r.Repetitions...)
	sort.Slice(reps, func(i, j int) bool { return reps[i].RepOrder < reps[j].RepOrder })

	var velocities, peakVelocities []float64
	var sum, peak float64
	for _, rep := range reps {
		velocities = append(velocities, rep.MeanConcentricVelocity)
		peakVelocities = append(peakVelocities, rep.PeakConcentricVelocity)
		sum += rep.MeanConcentricVelocity
		if rep.PeakConcentricVelocity > peak {
			peak = rep.PeakConcentricVelocity
		}
	}
	var mean float64
	if len(reps) > 0 {
		mean = sum / float64(len(reps))
	}

	return Set{
		Lift:              lift,
		WeightKg:          toKg(r.Weight, r.WeightUnit),
		Reps:              len(reps),
		GivenRPE:          parseRPE(firstNonEmpty(r.GivenRPE, r.SetTag)),
		PredictedRPE:      parseRPE(r.PredictedRPE),
		MeanVelocity:      mean,
		PeakVelocity:      peak,
		RepVelocities:     velocities,
		RepPeakVelocities: peakVelocities,
		PerformedAt:       firstTime(r.Ended, r.Started),
	}
}

// toKg normalises a weight to kilograms. A pound unit is converted; anything
// else is taken as already in kilograms.
func toKg(weight float64, unit string) float64 {
	u := strings.ToLower(strings.TrimSpace(unit))
	if strings.HasPrefix(u, "lb") || strings.HasPrefix(u, "pound") {
		return math.Round(weight*0.45359237*10) / 10
	}
	return weight
}

// parseRPE pulls the first number out of a tag such as "8.5" or "RPE 8". 0
// when there is none.
func parseRPE(tag string) float64 {
	var b strings.Builder
	for _, r := range tag {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			break
		}
	}
	if b.Len() == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return 0
	}
	return v
}

// firstTime parses the first timestamp that parses, or now as a last resort so
// a set is never dropped for a missing date.
func firstTime(values ...string) time.Time {
	for _, v := range values {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(v)); err == nil {
			return t
		}
	}
	return time.Now()
}

// supabaseCall makes one PostgREST request, authorised with the anon key as
// both the apikey and the bearer (there is no per-member token for reads). It
// returns the raw body so a non-2xx can be reported with PostgREST's own
// error message.
func (c *client) supabaseCall(ctx context.Context, method, endpoint string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("apikey", c.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+c.cfg.SupabaseAnonKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}

// snippet trims a response body to something loggable.
func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
