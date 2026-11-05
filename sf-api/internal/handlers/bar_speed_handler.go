package handlers

import (
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/velocity"
)

// BarSpeedHandler computes, stores and reads bar-speed sets.
//
// Device-agnostic on purpose: whatever captured the signal - a phone over
// Bluetooth, a cheap board over HTTP - posts the same stream of samples and the
// movement, and gets back the set the way the panel renders it. The maths is
// StrongFish's own (internal/velocity), so this depends on no outside service.
//
// A captured set is kept in StrongFish's own store, and the read merges it with
// what a connected tracker reports, so the panel shows the most recent set of
// each lift whichever produced it.
type BarSpeedHandler struct {
	store *store.BarSpeedStore
	// trackerSets reads the member's last set of each lift from a connected
	// bar-speed tracker (Stance), tagged by source, or nil. Injected so this
	// handler need not know about device links.
	trackerSets func(*http.Request, string) []models.BarSet
}

func NewBarSpeedHandler(barSpeed *store.BarSpeedStore) *BarSpeedHandler {
	return &BarSpeedHandler{store: barSpeed}
}

// WithTracker wires in the connected-tracker read the consolidated list merges
// with our own recordings.
func (h *BarSpeedHandler) WithTracker(f func(*http.Request, string) []models.BarSet) *BarSpeedHandler {
	h.trackerSets = f
	return h
}

// The movements the engine reads, lowercased. Anything else is refused rather
// than charted on a scale that does not fit it.
var barSpeedMovements = map[string]bool{
	"squat": true, "bench": true, "deadlift": true,
}

// maxBarSpeedSamples caps one set's stream: at 200 Hz this is ten minutes, far
// more than a set, and it keeps a stuck or hostile client from posting an
// unbounded body.
const maxBarSpeedSamples = 120_000

type analyzeSample struct {
	T  float64 `json:"t"`
	AX float64 `json:"ax"`
	AY float64 `json:"ay"`
	AZ float64 `json:"az"`
	QW float64 `json:"qw,omitempty"`
	QX float64 `json:"qx,omitempty"`
	QY float64 `json:"qy,omitempty"`
	QZ float64 `json:"qz,omitempty"`
}

type analyzePayload struct {
	Movement     string          `json:"movement"`
	Load         float64         `json:"load"`
	Unit         string          `json:"unit"`
	SampleRateHz float64         `json:"sampleRateHz"`
	Samples      []analyzeSample `json:"samples"`
	// GivenRpe is what the member rated the set, 0 when they did not. It is kept
	// and, over time, is what the predicted RPE of later sets is read against.
	GivenRpe float64 `json:"givenRpe"`
}

// List is the member's last set of each lift, consolidated across sources: our
// own recordings and whatever a connected tracker reports, keeping the most
// recent of each lift. Newest first, each tagged by its source.
func (h *BarSpeedHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	ours, err := h.store.LastSets(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	var tracker []models.BarSet
	if h.trackerSets != nil {
		tracker = h.trackerSets(r, userID)
	}
	writeJSON(w, http.StatusOK, mergeBarSets(ours, tracker))
}

// mergeBarSets keeps, for each lift, the most recent set across every source,
// newest first. A tie goes to the first source given.
func mergeBarSets(sources ...[]models.BarSet) []models.BarSet {
	best := map[string]models.BarSet{}
	for _, source := range sources {
		for _, set := range source {
			if cur, ok := best[set.Lift]; !ok || set.PerformedAt.After(cur.PerformedAt) {
				best[set.Lift] = set
			}
		}
	}
	out := make([]models.BarSet, 0, len(best))
	for _, set := range best {
		out = append(out, set)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PerformedAt.After(out[j].PerformedAt) })
	return out
}

// Analyze turns a raw stream into a computed set, and keeps it: the member's own
// capture is stored so it shows in the panel beside any tracker's sets.
func (h *BarSpeedHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p analyzePayload
	if !decodeJSON(w, r, &p) {
		return
	}

	movement := strings.ToLower(strings.TrimSpace(p.Movement))
	if !barSpeedMovements[movement] {
		writeError(w, http.StatusBadRequest,
			"movement must be squat, bench or deadlift", CodeInvalidRequestBody)
		return
	}
	if p.SampleRateHz <= 0 {
		writeError(w, http.StatusBadRequest, "sampleRateHz is required", CodeAllFieldsRequired)
		return
	}
	if len(p.Samples) == 0 {
		writeError(w, http.StatusBadRequest, "samples are required", CodeAllFieldsRequired)
		return
	}
	if len(p.Samples) > maxBarSpeedSamples {
		writeError(w, http.StatusBadRequest, "too many samples", CodeInvalidRequestBody)
		return
	}

	samples := make([]velocity.Sample, len(p.Samples))
	for i, s := range p.Samples {
		samples[i] = velocity.Sample{
			T: s.T, AX: s.AX, AY: s.AY, AZ: s.AZ,
			QW: s.QW, QX: s.QX, QY: s.QY, QZ: s.QZ,
		}
	}

	result := velocity.Analyze(velocity.Capture{
		Movement: movement, Samples: samples, SampleRateHz: p.SampleRateHz,
	})
	set := barSpeedSet(movement, barSpeedKg(p.Load, p.Unit), result)
	set.GivenRpe = p.GivenRpe

	// Keep a real set (one the engine found reps in) for the authenticated
	// member. Best-effort: a store that is briefly down must not cost the
	// member the reading they just captured.
	if userID != "" && len(result.Reps) > 0 {
		// Predict the RPE from the member's own history *before* storing this
		// set, so the profile it is read against does not yet include it. 0
		// until they have rated enough past sets to build one.
		lastRep := result.Reps[len(result.Reps)-1].MeanMps
		if profile, err := h.store.RPEProfile(r.Context(), userID, movement); err == nil {
			set.PredictedRpe = velocity.PredictRPE(profile, lastRep)
		}
		if err := h.store.Save(r.Context(), userID, set); err != nil {
			slog.Error("bar speed: capture not stored", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, set)
}

// barSpeedKg normalises the load to kilograms; a pound unit is converted, and
// anything else is taken as already in kilograms.
func barSpeedKg(load float64, unit string) float64 {
	u := strings.ToLower(strings.TrimSpace(unit))
	if strings.HasPrefix(u, "lb") || strings.HasPrefix(u, "pound") {
		return math.Round(load*0.45359237*10) / 10
	}
	return load
}

// barSpeedSet maps the engine's result onto the BarSet the clients already
// render, filling the per-rep series and the set-level speeds. Velocity loss is
// derived by the clients from the per-rep series, as it is for a Stance set.
func barSpeedSet(movement string, loadKg float64, result velocity.Result) models.BarSet {
	set := models.BarSet{
		Lift:         movement,
		Source:       "strongfish",
		WeightKg:     loadKg,
		Reps:         len(result.Reps),
		MeanVelocity: result.MeanVelocity,
		PeakVelocity: result.PeakVelocity,
		Confidence:   result.Confidence,
		PerformedAt:  time.Now().UTC(),
	}
	for _, rep := range result.Reps {
		set.RepVelocities = append(set.RepVelocities, rep.MeanMps)
		set.RepPeakVelocities = append(set.RepPeakVelocities, rep.PeakMps)
	}
	return set
}
