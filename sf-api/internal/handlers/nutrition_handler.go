package handlers

import (
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"strong-fish-api/internal/devicelink"
	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/nutrition"
	"strong-fish-api/internal/store"
)

// Nutrition and its targets.
//
// The consumed figures come from a food-diary provider (Yazio, MyFitnessPal)
// over one of the named ranges, the same ranges the step chart uses. The
// targets - the ideal protein band and maintenance energy - are computed from
// the member's body rather than read from any provider, so they travel with the
// series but are also offered on their own, for a member who tracks no food but
// still wants to know what to aim for.

// activeCaloriesWindow is how many recent days the maintenance estimate averages
// a tracker's active calories over - a fortnight, long enough to smooth a rest
// day without reaching back to a different training block.
const activeCaloriesWindow = 14

// Nutrition returns a member's logged energy and protein for one of the named
// ranges, with their daily targets.
func (h *DeviceHandler) Nutrition(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	found, ok := h.provider(w, r)
	if !ok {
		return
	}
	reader, ok := found.(devicelink.NutritionReader)
	if !ok {
		writeError(w, http.StatusNotFound, "This provider has no nutrition data", CodeNotFound)
		return
	}
	if !found.Enabled() {
		writeError(w, http.StatusNotImplemented,
			"This integration is not enabled on this server", CodeDeviceUnavailable)
		return
	}

	link, err := h.links.Find(r.Context(), userID, found.Name())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "No account connected", CodeDeviceNotConnected)
			return
		}
		writeStoreError(w, err)
		return
	}

	rangeName := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("range")))
	days, known := stepRanges[rangeName]
	if !known {
		rangeName, days = "week", stepRanges["week"]
	}
	if max := reader.MaxNutritionDays(); days > max {
		days = max
	}

	accessToken, err := h.freshToken(r, found, link)
	if err != nil {
		h.writeProviderError(w, err, found.Name())
		return
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -(days - 1))
	rows, err := reader.DailyNutrition(r.Context(), accessToken, start, end)
	if err != nil {
		h.writeProviderError(w, err, found.Name())
		return
	}

	series := nutritionSeries(found.Name(), rangeName, start, end, rows)
	series.Targets = h.computeTargets(r, userID)
	writeJSON(w, http.StatusOK, series)
}

// NutritionTargets returns only the member's ideal protein band and
// maintenance energy - the "what should I aim for" without a food diary. An
// empty object (not an error) when nothing can be computed yet, so a client can
// decide whether to show the panel; NeedsHeight set when the one thing missing
// is a height to fill in.
func (h *DeviceHandler) NutritionTargets(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	if targets := h.computeTargets(r, userID); targets != nil {
		writeJSON(w, http.StatusOK, *targets)
		return
	}
	writeJSON(w, http.StatusOK, models.NutritionTargets{})
}

// nutritionSeries builds the API series from a provider's daily rows, computing
// the averages over the days that were actually logged.
func nutritionSeries(provider, rangeName string, start, end time.Time,
	rows []devicelink.NutritionDay) models.NutritionSeries {
	series := models.NutritionSeries{
		Provider: provider, Range: rangeName,
		From: start.Format("2006-01-02"), To: end.Format("2006-01-02"),
		Days: make([]models.NutritionDay, 0, len(rows)),
	}
	var totalKcal, totalProtein float64
	for _, row := range rows {
		series.Days = append(series.Days, models.NutritionDay{
			Date: row.Date, Kcal: row.Kcal, ProteinG: row.ProteinG,
		})
		totalKcal += row.Kcal
		totalProtein += row.ProteinG
		series.LoggedDays++
	}
	if series.LoggedDays > 0 {
		series.AvgKcal = math.Round(totalKcal / float64(series.LoggedDays))
		series.AvgProteinG = round1(totalProtein / float64(series.LoggedDays))
	}
	return series
}

// computeTargets works out the member's protein band and maintenance energy, or
// nil when there is nothing to go on. It gathers the body from the profile and
// the connected scale, and the activity from the tracker, then hands the lot to
// the nutrition package, which decides what is measured and what is estimated.
func (h *DeviceHandler) computeTargets(r *http.Request, userID string) *models.NutritionTargets {
	in := h.gatherTargetInput(r, userID)
	if in.WeightKg <= 0 && in.HeightCm <= 0 && in.FatFreeMassKg <= 0 &&
		in.FatRatioPct <= 0 && in.MeasuredBMRKcal <= 0 {
		return nil
	}
	targets := nutrition.Targets(in)
	if !targets.HasProtein() && !targets.HasEnergy() && !targets.NeedsHeight {
		return nil
	}
	return &targets
}

// gatherTargetInput collects everything the targets are computed from: the
// member's profile, their body-composition source (which overrides the profile
// weight with a measured one), and their activity source's active calories.
func (h *DeviceHandler) gatherTargetInput(r *http.Request, userID string) nutrition.Input {
	in := nutrition.Input{Male: true}
	if user, err := h.users.FindByID(r.Context(), userID); err == nil {
		in.WeightKg = user.Bodyweight
		in.HeightCm = user.Height
		// Male is the default an account never asked reads as; only an explicit
		// "female" flips the sex term.
		in.Male = !strings.EqualFold(strings.TrimSpace(user.Gender), "female")
		in.AgeYears = ageFromBirthdate(user.Birthdate)
	}
	if body := h.exportBodyComposition(r, userID); body != nil {
		for _, measure := range body.Measures {
			switch measure.Type {
			case devicelink.MeasureWeight:
				if measure.Value > 0 {
					in.WeightKg = measure.Value // a measured weight beats the profile's
				}
			case devicelink.MeasureFatRatio:
				in.FatRatioPct = measure.Value
			case devicelink.MeasureFatFreeMass:
				in.FatFreeMassKg = measure.Value
			case devicelink.MeasureBasalMetabolicRate:
				in.MeasuredBMRKcal = measure.Value
			}
		}
	}
	if avg, ok := h.activeCaloriesAverage(r, userID); ok {
		in.ActiveKcalPerDay = avg
		in.HasActiveKcal = true
	}
	return in
}

// activeCaloriesAverage is the member's recent daily active calories from the
// activity source, when that source reports them. Best-effort: a tracker that
// cannot answer simply leaves the maintenance estimate to its fallback factor.
func (h *DeviceHandler) activeCaloriesAverage(r *http.Request, userID string) (float64, bool) {
	connected, err := h.links.ListProviders(r.Context(), userID)
	if err != nil {
		return 0, false
	}
	linked := map[string]bool{}
	for _, name := range connected {
		linked[name] = true
	}
	name := devicelink.ActivitySource(h.providers, linked)
	if name == "" {
		return 0, false
	}
	found, ok := devicelink.Find(h.providers, name)
	if !ok {
		return 0, false
	}
	reader, ok := found.(devicelink.ActiveCaloriesReader)
	if !ok || !found.Enabled() {
		return 0, false
	}
	link, err := h.links.Find(r.Context(), userID, name)
	if err != nil {
		return 0, false
	}
	accessToken, err := h.freshToken(r, found, link)
	if err != nil {
		return 0, false
	}
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -(activeCaloriesWindow - 1))
	days, err := reader.DailyActiveCalories(r.Context(), accessToken, start, end)
	if err != nil || len(days) == 0 {
		return 0, false
	}
	var total float64
	for _, day := range days {
		total += day.Kcal
	}
	return total / float64(len(days)), true
}

// exportNutrition is the member's logged nutrition for an export or an AI
// reading, from the one food diary the screen shows too (NutritionSource picks
// it), over the last days. Best-effort and without targets - those are passed
// alongside it - and nil when nothing is logged, so the sheet drops the charts
// rather than drawing empty ones.
func (h *DeviceHandler) exportNutrition(r *http.Request, userID string, days int) *models.NutritionSeries {
	connected, err := h.links.ListProviders(r.Context(), userID)
	if err != nil {
		return nil
	}
	linked := map[string]bool{}
	for _, name := range connected {
		linked[name] = true
	}
	name := devicelink.NutritionSource(h.providers, linked)
	if name == "" {
		return nil
	}
	found, ok := devicelink.Find(h.providers, name)
	if !ok {
		return nil
	}
	reader, ok := found.(devicelink.NutritionReader)
	if !ok {
		return nil
	}
	if max := reader.MaxNutritionDays(); days > max {
		days = max
	}
	link, err := h.links.Find(r.Context(), userID, name)
	if err != nil {
		return nil
	}
	accessToken, err := h.freshToken(r, found, link)
	if err != nil {
		slog.Warn("nutrition export: unavailable", "provider", name, "error", err)
		return nil
	}
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -(days - 1))
	rows, err := reader.DailyNutrition(r.Context(), accessToken, start, end)
	if err != nil {
		slog.Warn("nutrition export: unavailable", "provider", name, "error", err)
		return nil
	}
	if len(rows) == 0 {
		return nil
	}
	series := nutritionSeries(name, "", start, end, rows)
	return &series
}

// ageFromBirthdate is the member's age in whole years from a YYYY-MM-DD
// birthdate, or zero when it is blank or unparseable.
func ageFromBirthdate(birthdate string) float64 {
	birthdate = strings.TrimSpace(birthdate)
	if birthdate == "" {
		return 0
	}
	born, err := time.Parse("2006-01-02", birthdate)
	if err != nil {
		return 0
	}
	now := time.Now().UTC()
	years := now.Year() - born.Year()
	if now.YearDay() < born.YearDay() {
		years--
	}
	if years < 0 || years > 120 {
		return 0
	}
	return float64(years)
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
