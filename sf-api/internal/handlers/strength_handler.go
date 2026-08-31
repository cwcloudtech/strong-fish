package handlers

import (
	"net/http"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/strength"
	"strong-fish-api/internal/utils"
)

// StrengthHandler scores powerlifting totals: the calculator anybody can open,
// and the tier and badges a profile wears.
//
// The three coefficients live in one place (internal/strength) and are computed
// here rather than in each client, for the same reason the loads are: two
// implementations of a fitted polynomial are two answers to the same question,
// and the one on the phone would be the one nobody noticed had drifted.
type StrengthHandler struct {
	exercises *store.ExerciseStore
	users     *store.UserStore
}

func NewStrengthHandler(exercises *store.ExerciseStore, users *store.UserStore) *StrengthHandler {
	return &StrengthHandler{exercises: exercises, users: users}
}

// scorePayload is what the calculator sends. Weights are in kilograms: the
// clients convert pounds before sending, so no formula here has to ask which
// unit it was handed.
type scorePayload struct {
	Gender     string  `json:"gender"`
	Division   string  `json:"division"`
	Bodyweight float64 `json:"bodyweight"`
	Squat      float64 `json:"squat"`
	Bench      float64 `json:"bench"`
	Deadlift   float64 `json:"deadlift"`
}

// Score runs the calculator.
//
// Open to anybody, signed in or not: a lifter working out what their meet total
// is worth has no reason to have an account first, and it is the page most
// likely to bring them one.
func (h *StrengthHandler) Score(w http.ResponseWriter, r *http.Request) {
	var p scorePayload
	if !decodeJSON(w, r, &p) {
		return
	}

	input := strength.Input{
		Gender:       strength.NormalizeGender(p.Gender),
		Division:     strength.NormalizeDivision(p.Division),
		BodyweightKg: p.Bodyweight,
		Lifts:        strength.Lifts{Squat: p.Squat, Bench: p.Bench, Deadlift: p.Deadlift},
	}
	writeJSON(w, http.StatusOK, strength.Evaluate(input, h.population(r)))
}

// Defaults pre-fills the calculator for whoever is asking: their gender and
// bodyweight from their profile, their three maxes from the ones they recorded.
// A logged-out caller gets an empty form, which is the honest answer.
func (h *StrengthHandler) Defaults(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	if utils.IsBlank(userID) {
		writeJSON(w, http.StatusOK, scorePayload{Gender: string(strength.Male), Division: string(strength.Raw)})
		return
	}

	lifter, err := h.exercises.LifterFor(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, scorePayload{
		Gender:     string(strength.NormalizeGender(lifter.Gender)),
		Division:   string(strength.Raw),
		Bodyweight: lifter.Bodyweight,
		Squat:      lifter.Squat,
		Bench:      lifter.Bench,
		Deadlift:   lifter.Deadlift,
	})
}

// population is every complete lifter on this deployment, scored, for the
// percentile bar. A failure here is not worth refusing a calculation over: the
// scores are the answer, and the percentile is context.
func (h *StrengthHandler) population(r *http.Request) []float64 {
	lifters, err := h.exercises.ListLifters(r.Context())
	if err != nil {
		return nil
	}
	scores := make([]float64, 0, len(lifters))
	for _, lifter := range lifters {
		dots := strength.DOTS(strength.NormalizeGender(lifter.Gender), lifter.Bodyweight,
			lifter.Squat+lifter.Bench+lifter.Deadlift)
		if dots > 0 {
			scores = append(scores, dots)
		}
	}
	return scores
}

// ForUser is the strength a profile wears: the same evaluation, run on the
// member's own recorded maxes rather than on typed-in numbers.
//
// Returned as part of the profile rather than fetched separately, so a badge
// never renders a beat behind the maxes it was computed from.
func (h *StrengthHandler) ForUser(r *http.Request, userID string) *strength.Result {
	lifter, err := h.exercises.LifterFor(r.Context(), userID)
	if err != nil {
		return nil
	}
	// Nothing to say about a member who has not weighed in or has no maxes:
	// an empty result would draw a profile full of locked badges and a tier
	// they were never measured for.
	if lifter.Bodyweight <= 0 || lifter.Squat+lifter.Bench+lifter.Deadlift <= 0 {
		return nil
	}

	result := strength.Evaluate(strength.Input{
		Gender:       strength.NormalizeGender(lifter.Gender),
		Division:     strength.Raw,
		BodyweightKg: lifter.Bodyweight,
		Lifts:        strength.Lifts{Squat: lifter.Squat, Bench: lifter.Bench, Deadlift: lifter.Deadlift},
	}, h.population(r))
	return &result
}
