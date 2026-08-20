package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

type ExerciseHandler struct {
	exercises *store.ExerciseStore
	oneRMs    *store.OneRMStore
	users     *store.UserStore
}

func NewExerciseHandler(exercises *store.ExerciseStore, oneRMs *store.OneRMStore, users *store.UserStore) *ExerciseHandler {
	return &ExerciseHandler{exercises: exercises, oneRMs: oneRMs, users: users}
}

// List returns the shared catalog, optionally filtered by ?q= - this is what
// backs the autocomplete a coach uses when filling a program, so every coach
// sees every other coach's movements.
func (h *ExerciseHandler) List(w http.ResponseWriter, r *http.Request) {
	exercises, err := h.exercises.List(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exercises)
}

type exercisePayload struct {
	// Name is the movement as the coach types it; the lookup slug is derived
	// from it rather than asked for.
	Name       string            `json:"name"`
	Labels     map[string]string `json:"labels"`
	Aliases    []string          `json:"aliases"`
	Category   string            `json:"category"`
	OneRMRef   string            `json:"oneRmRef"`
	Bodyweight bool              `json:"bodyweight"`
	// Main flags the movement as a competition lift. Only a superadmin may set
	// it, so it's read off the payload but ignored for anyone else (see
	// ExerciseHandler.mainFlag).
	Main bool `json:"main"`
}

// resolveLabels fills in the en/fr labels, defaulting either to the typed name
// so a coach in a hurry doesn't have to translate before saving.
func (p exercisePayload) resolveLabels() map[string]string {
	labels := map[string]string{}
	for locale, label := range p.Labels {
		if utils.IsNotBlank(label) {
			labels[locale] = label
		}
	}
	if utils.IsBlank(labels["en"]) {
		labels["en"] = utils.If(utils.IsNotBlank(labels["fr"]), labels["fr"], p.Name)
	}
	if utils.IsBlank(labels["fr"]) {
		labels["fr"] = labels["en"]
	}
	return labels
}

// normalizeAliases slugifies the alternative names a coach listed, dropping
// blanks and the canonical slug itself.
func normalizeAliases(aliases []string, slug string) []string {
	normalized := []string{}
	seen := map[string]bool{slug: true}
	for _, alias := range aliases {
		if candidate := utils.Slugify(alias); utils.IsNotBlank(candidate) && !seen[candidate] {
			seen[candidate] = true
			normalized = append(normalized, candidate)
		}
	}
	return normalized
}

// Create adds a movement to the catalog. Any coach may; it's shared globally by
// design, so "larsen press" is only ever named once.
func (h *ExerciseHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())

	var p exercisePayload
	if !decodeJSON(w, r, &p) {
		return
	}

	name := utils.If(utils.IsNotBlank(p.Name), p.Name, p.Labels["en"])
	slug := utils.Slugify(name)
	if utils.IsBlank(slug) {
		writeError(w, http.StatusBadRequest, "Please add a name", CodeNameRequired)
		return
	}
	if !models.IsValidCategory(p.Category) {
		writeError(w, http.StatusBadRequest, "Invalid category", CodeInvalidCategory)
		return
	}
	if !models.IsValidOneRMRef(p.OneRMRef) {
		writeError(w, http.StatusBadRequest, "Invalid 1RM reference", CodeInvalidCategory)
		return
	}

	// An existing slug, alias or name means the movement is already in the
	// catalog; report it rather than creating a near-duplicate. Matched
	// case-insensitively, so "Highbar Squat" and "highbar squat" are one
	// movement (see ExerciseStore.FindByName).
	if _, err := h.exercises.FindByName(r.Context(), name); err == nil {
		writeError(w, http.StatusBadRequest, "An exercise with this name already exists", CodeDuplicateExercise)
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeStoreError(w, err)
		return
	}

	labels := p.resolveLabels()
	labels["en"] = utils.If(utils.IsNotBlank(labels["en"]), labels["en"], name)

	exercise, err := h.exercises.Create(r.Context(), store.ExerciseFields{
		Slug: slug, Aliases: normalizeAliases(p.Aliases, slug), Labels: labels,
		Category: p.Category, OneRMRef: p.OneRMRef, Bodyweight: p.Bodyweight,
		Main: h.mainFlag(r, p.Main, false), CreatedBy: userID,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, exercise)
}

// Update edits a catalog entry's labels and classification. The slug stays put:
// importers and autocomplete resolve by it, so renaming would strand rows in
// spreadsheets already written against it.
func (h *ExerciseHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "exerciseId")

	var p exercisePayload
	if !decodeJSON(w, r, &p) {
		return
	}
	if !models.IsValidCategory(p.Category) {
		writeError(w, http.StatusBadRequest, "Invalid category", CodeInvalidCategory)
		return
	}
	if !models.IsValidOneRMRef(p.OneRMRef) {
		writeError(w, http.StatusBadRequest, "Invalid 1RM reference", CodeInvalidCategory)
		return
	}

	existing, err := h.exercises.FindByID(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	exercise, err := h.exercises.Update(r.Context(), id, store.ExerciseFields{
		Aliases: normalizeAliases(p.Aliases, existing.Slug), Labels: p.resolveLabels(),
		Category: p.Category, OneRMRef: p.OneRMRef, Bodyweight: p.Bodyweight,
		Main: h.mainFlag(r, p.Main, existing.Main),
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exercise)
}

// Usage reports what deleting an exercise would take with it, so the superadmin
// confirms an informed cascade rather than discovering it afterwards.
func (h *ExerciseHandler) Usage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "exerciseId")

	if _, err := h.exercises.FindByID(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	usage, err := h.exercises.Usage(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, usage)
}

// Delete removes a catalog entry along with every set prescribing it and every
// max recorded against it. The client is expected to have shown the caller
// Usage first; this endpoint carries out the decision rather than second-
// guessing it.
func (h *ExerciseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "exerciseId")
	if err := h.exercises.Delete(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

// mainFlag resolves the competition-movement flag: a superadmin sets it, and
// anyone else leaves it at whatever it already was. Coaches can add movements
// to the shared catalog, but which lifts count as competition movements is an
// instance-wide decision - a member's 1RM prompts and every derived movement's
// load resolve against them.
func (h *ExerciseHandler) mainFlag(r *http.Request, requested, current bool) bool {
	userID, _ := middleware.UserIDFromContext(r.Context())
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil || user.Role != models.GlobalRoleSuperadmin {
		return current
	}
	return requested
}

// --- one-rep maxes ---

// ListOneRMs returns the caller's recorded maxes.
func (h *ExerciseHandler) ListOneRMs(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	maxes, err := h.oneRMs.ListForUser(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, maxes)
}

type oneRMPayload struct {
	Value float64 `json:"value"`
}

// SetOneRM records the caller's current max for an exercise. Members may revise
// this as often as they like: nothing derived is stored, so every set of every
// program they're running recomputes on the next read.
func (h *ExerciseHandler) SetOneRM(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	exerciseID := chi.URLParam(r, "exerciseId")

	var p oneRMPayload
	if !decodeJSON(w, r, &p) {
		return
	}
	// A zero or negative max would scale a whole program down to nothing;
	// clearing one is done by deleting it instead.
	if p.Value <= 0 {
		writeError(w, http.StatusBadRequest, "A 1RM must be greater than zero", CodeInvalidOneRM)
		return
	}

	if _, err := h.exercises.FindByID(r.Context(), exerciseID); err != nil {
		writeStoreError(w, err)
		return
	}

	oneRM, err := h.oneRMs.Set(r.Context(), userID, exerciseID, p.Value)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, oneRM)
}

// DeleteOneRM clears a recorded max, putting the member's affected sets back to
// "unknown load".
func (h *ExerciseHandler) DeleteOneRM(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	exerciseID := chi.URLParam(r, "exerciseId")

	if err := h.oneRMs.Delete(r.Context(), userID, exerciseID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"exerciseId": exerciseID})
}
