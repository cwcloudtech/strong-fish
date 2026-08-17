package models

import "time"

// Exercise categories. The three competition lifts double as the reference a
// derived movement's percentage/RPE prescription resolves against.
const (
	CategorySquat     = "squat"
	CategoryBench     = "bench"
	CategoryDeadlift  = "deadlift"
	CategoryAccessory = "accessory"
)

// IsValidCategory reports whether category is a known exercise category.
func IsValidCategory(category string) bool {
	switch category {
	case CategorySquat, CategoryBench, CategoryDeadlift, CategoryAccessory:
		return true
	}
	return false
}

// IsValidOneRMRef reports whether ref names one of the three lifts a
// prescription can be computed against. The empty string is valid and means
// "none" - an accessory loaded in absolute kilos.
func IsValidOneRMRef(ref string) bool {
	switch ref {
	case CategorySquat, CategoryBench, CategoryDeadlift, "":
		return true
	}
	return false
}

// Exercise is one movement in the shared, cross-club catalog: a coach adds
// "larsen press" once and every other coach gets it in their autocomplete.
type Exercise struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	// Aliases are extra normalized names that resolve to this exercise, so a
	// spreadsheet spelling a movement differently imports onto the existing
	// entry instead of forking a duplicate.
	Aliases []string `json:"aliases,omitempty"`
	// Labels is the display name per locale, keyed "en"/"fr". English is the
	// fallback for a locale with no translation.
	Labels   map[string]string `json:"labels"`
	Category string            `json:"category"`
	// OneRMRef names which lift's 1RM a percentage or RPE prescription for this
	// movement is computed against, or "" for a movement loaded in absolute
	// kilos.
	OneRMRef string `json:"oneRmRef,omitempty"`
	// Bodyweight movements (pull-ups, dips) carry reps and RPE but no external
	// load.
	Bodyweight bool `json:"bodyweight"`
	// Main marks the three competition lifts, which every member is prompted
	// to record a 1RM for.
	Main      bool      `json:"main"`
	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Label returns the exercise's name in locale, falling back to English and
// then to the slug.
func (e Exercise) Label(locale string) string {
	if label, ok := e.Labels[locale]; ok && label != "" {
		return label
	}
	if label, ok := e.Labels["en"]; ok && label != "" {
		return label
	}
	return e.Slug
}

// OneRM is one member's current max on one exercise. Every prescribed set is
// computed against it at read time, so revising it recomputes the whole
// program with nothing stored to migrate.
type OneRM struct {
	ID         string            `json:"id"`
	UserID     string            `json:"userId"`
	ExerciseID string            `json:"exerciseId"`
	Slug       string            `json:"slug,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Value      float64           `json:"value"`
	// History keeps every previous value so a member can see their progression
	// and a coach can tell whether a program actually moved the needle.
	History   []OneRMEntry `json:"history,omitempty"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// OneRMEntry is one revision of a member's max.
type OneRMEntry struct {
	Value float64   `json:"value"`
	At    time.Time `json:"at"`
}
