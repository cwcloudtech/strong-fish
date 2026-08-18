package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

type ExerciseStore struct {
	pool *pgxpool.Pool
}

func NewExerciseStore(pool *pgxpool.Pool) *ExerciseStore {
	return &ExerciseStore{pool: pool}
}

// exerciseData is the JSONB payload of the exercises table.
type exerciseData struct {
	Slug       string            `json:"slug"`
	Aliases    []string          `json:"aliases,omitempty"`
	Labels     map[string]string `json:"labels"`
	Category   string            `json:"category"`
	OneRMRef   string            `json:"oneRmRef,omitempty"`
	Bodyweight bool              `json:"bodyweight"`
	Main       bool              `json:"main,omitempty"`
	CreatedBy  string            `json:"createdBy,omitempty"`
}

func scanExercise(row pgx.Row) (models.Exercise, error) {
	var e models.Exercise
	var raw []byte
	if err := row.Scan(&e.ID, &raw, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Exercise{}, ErrNotFound
		}
		return models.Exercise{}, err
	}
	var d exerciseData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.Exercise{}, err
	}
	e.Slug = d.Slug
	e.Aliases = d.Aliases
	e.Labels = d.Labels
	e.Category = d.Category
	e.OneRMRef = d.OneRMRef
	e.Bodyweight = d.Bodyweight
	e.Main = d.Main
	e.CreatedBy = d.CreatedBy
	return e, nil
}

func scanExercises(rows pgx.Rows) ([]models.Exercise, error) {
	defer rows.Close()
	exercises := []models.Exercise{}
	for rows.Next() {
		e, err := scanExercise(rows)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, e)
	}
	return exercises, rows.Err()
}

// ExerciseFields are the properties set when adding a movement to the shared
// catalog or editing one.
type ExerciseFields struct {
	Slug       string
	Aliases    []string
	Labels     map[string]string
	Category   string
	OneRMRef   string
	Bodyweight bool
	// Main flags a competition movement ("mouvement de compétition"): the lifts
	// every member is prompted to record a 1RM for, and that a derived movement's
	// percentage/RPE prescription resolves against. Only a superadmin sets it.
	Main      bool
	CreatedBy string
}

// ExerciseUsage counts what deleting an exercise would take with it. The
// foreign key from program_sets cascades, so removing a movement silently drops
// every set prescribing it - a superadmin is shown these numbers first.
type ExerciseUsage struct {
	Programs int `json:"programs"`
	Sets     int `json:"sets"`
	// OneRMs is how many members have recorded a max for this movement; those
	// rows cascade too.
	OneRMs int `json:"oneRms"`
}

// Create adds a movement to the catalog, visible to every coach's autocomplete
// from then on.
func (s *ExerciseStore) Create(ctx context.Context, f ExerciseFields) (models.Exercise, error) {
	data, err := json.Marshal(exerciseData{
		Slug: f.Slug, Aliases: f.Aliases, Labels: f.Labels, Category: f.Category,
		OneRMRef: f.OneRMRef, Bodyweight: f.Bodyweight, Main: f.Main, CreatedBy: f.CreatedBy,
	})
	if err != nil {
		return models.Exercise{}, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO exercises (data) VALUES ($1)
		RETURNING id, data, created_at, updated_at
	`, data)
	exercise, err := scanExercise(row)
	if isUniqueViolation(err) {
		return models.Exercise{}, ErrDuplicateExercise
	}
	return exercise, err
}

// Update edits a catalog entry. The slug is immutable: programs and 1RMs
// reference the exercise by id, but importers and autocomplete resolve by slug,
// so renaming it would strand an existing spreadsheet's rows.
func (s *ExerciseStore) Update(ctx context.Context, id string, f ExerciseFields) (models.Exercise, error) {
	patch, err := json.Marshal(map[string]any{
		"aliases":    f.Aliases,
		"labels":     f.Labels,
		"category":   f.Category,
		"oneRmRef":   f.OneRMRef,
		"bodyweight": f.Bodyweight,
		"main":       f.Main,
	})
	if err != nil {
		return models.Exercise{}, err
	}
	return scanExercise(s.pool.QueryRow(ctx, `
		UPDATE exercises SET data = data || $2::jsonb, updated_at = now()
		WHERE id = $1
		RETURNING id, data, created_at, updated_at
	`, id, patch))
}

// Usage counts what deleting an exercise would take with it, so the superadmin
// is warned before the cascade rather than after.
func (s *ExerciseStore) Usage(ctx context.Context, id string) (ExerciseUsage, error) {
	var usage ExerciseUsage
	err := s.pool.QueryRow(ctx, `
		SELECT (SELECT count(DISTINCT program_id) FROM program_sets WHERE exercise_id = $1),
		       (SELECT count(*) FROM program_sets WHERE exercise_id = $1),
		       (SELECT count(*) FROM one_rms WHERE exercise_id = $1)
	`, id).Scan(&usage.Programs, &usage.Sets, &usage.OneRMs)
	return usage, err
}

// Delete removes a catalog entry along with every set prescribing it and every
// max recorded against it.
//
// The schema's foreign key from program_sets is RESTRICT, which would refuse
// this outright, so the sets are removed explicitly in the same transaction -
// the cascade is deliberate (a superadmin who confirmed the impact reported by
// Usage) rather than something a stray delete could trigger.
func (s *ExerciseStore) Delete(ctx context.Context, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM program_sets WHERE exercise_id = $1`, id); err != nil {
		return err
	}

	// Sessions left with no sets at all are dropped too: an empty session is
	// noise in a program rather than a meaningful rest day.
	if _, err := tx.Exec(ctx, `
		DELETE FROM program_days pd
		WHERE NOT EXISTS (SELECT 1 FROM program_sets WHERE day_id = pd.id)
	`); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *ExerciseStore) FindByID(ctx context.Context, id string) (models.Exercise, error) {
	return scanExercise(s.pool.QueryRow(ctx, `
		SELECT id, data, created_at, updated_at FROM exercises WHERE id = $1
	`, id))
}

// FindBySlug resolves a normalized name to a catalog entry, matching aliases as
// well as the canonical slug - so a spreadsheet spelling a movement differently
// (or with the reference file's "Dumbbel" typo) lands on the existing entry
// instead of forking a duplicate.
func (s *ExerciseStore) FindBySlug(ctx context.Context, slug string) (models.Exercise, error) {
	return scanExercise(s.pool.QueryRow(ctx, `
		SELECT id, data, created_at, updated_at FROM exercises
		WHERE data->>'slug' = $1 OR data->'aliases' ? $1
		LIMIT 1
	`, slug))
}

// List returns the whole catalog, optionally filtered by a search query that
// matches the slug or any locale's label.
func (s *ExerciseStore) List(ctx context.Context, query string) ([]models.Exercise, error) {
	if utils.IsBlank(query) {
		rows, err := s.pool.Query(ctx, `
			SELECT id, data, created_at, updated_at FROM exercises
			ORDER BY data->>'category', data->>'slug'
		`)
		if err != nil {
			return nil, err
		}
		return scanExercises(rows)
	}

	pattern := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := s.pool.Query(ctx, `
		SELECT id, data, created_at, updated_at FROM exercises
		WHERE data->>'slug' LIKE $1
		   OR lower(coalesce(data->'labels'->>'en', '')) LIKE $1
		   OR lower(coalesce(data->'labels'->>'fr', '')) LIKE $1
		ORDER BY data->>'category', data->>'slug'
	`, pattern)
	if err != nil {
		return nil, err
	}
	return scanExercises(rows)
}

// ListByIDs loads several exercises at once, keyed by id - used when resolving a
// whole session's sets without a query per row.
func (s *ExerciseStore) ListByIDs(ctx context.Context, ids []string) (map[string]models.Exercise, error) {
	if len(ids) == 0 {
		return map[string]models.Exercise{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, data, created_at, updated_at FROM exercises WHERE id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	exercises, err := scanExercises(rows)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]models.Exercise, len(exercises))
	for _, e := range exercises {
		byID[e.ID] = e
	}
	return byID, nil
}

// FindMainLifts returns the three competition lifts keyed by category, which is
// how a derived movement's 1RM reference resolves to a concrete exercise (a
// Larsen press's oneRmRef is "bench", and the member's bench 1RM is what loads
// it).
func (s *ExerciseStore) FindMainLifts(ctx context.Context) (map[string]models.Exercise, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, data, created_at, updated_at FROM exercises
		WHERE (data->>'main')::boolean IS TRUE
	`)
	if err != nil {
		return nil, err
	}
	exercises, err := scanExercises(rows)
	if err != nil {
		return nil, err
	}

	byCategory := make(map[string]models.Exercise, len(exercises))
	for _, e := range exercises {
		byCategory[e.Category] = e
	}
	return byCategory, nil
}

// --- one-rep maxes ---

type OneRMStore struct {
	pool *pgxpool.Pool
}

func NewOneRMStore(pool *pgxpool.Pool) *OneRMStore {
	return &OneRMStore{pool: pool}
}

// oneRMData is the JSONB payload of the one_rms table.
type oneRMData struct {
	Value   float64             `json:"value"`
	History []models.OneRMEntry `json:"history,omitempty"`
}

const oneRMSelect = `
	SELECT o.id, o.user_id, o.exercise_id, coalesce(e.data->>'slug', ''),
	       coalesce(e.data->'labels', '{}'::jsonb), o.data, o.created_at, o.updated_at
	FROM one_rms o
	JOIN exercises e ON e.id = o.exercise_id`

func scanOneRM(row pgx.Row) (models.OneRM, error) {
	var m models.OneRM
	var raw, labels []byte
	if err := row.Scan(&m.ID, &m.UserID, &m.ExerciseID, &m.Slug, &labels, &raw, &m.CreatedAt, &m.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.OneRM{}, ErrNotFound
		}
		return models.OneRM{}, err
	}
	var d oneRMData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.OneRM{}, err
	}
	if err := json.Unmarshal(labels, &m.Labels); err != nil {
		return models.OneRM{}, err
	}
	m.Value = d.Value
	m.History = d.History
	return m, nil
}

// maxHistoryEntries caps how many past values one 1RM keeps. A member updating
// their max daily for years shouldn't grow the row without bound; the recent
// ones are what a progression chart needs.
const maxHistoryEntries = 200

// Set records a member's current max for an exercise, appending the previous
// value to the history. Members revise these as often as they like, and every
// prescribed set is computed against the current one at read time - so this is
// the whole of "recalculate my program".
func (s *OneRMStore) Set(ctx context.Context, userID, exerciseID string, value float64) (models.OneRM, error) {
	existing, err := s.Find(ctx, userID, exerciseID)
	history := []models.OneRMEntry{}
	switch {
	case err == nil:
		history = existing.History
		// Re-saving the same number isn't a revision; don't pad the history
		// with it.
		if existing.Value != value {
			history = append(history, models.OneRMEntry{Value: existing.Value, At: existing.UpdatedAt})
		}
	case !errors.Is(err, ErrNotFound):
		return models.OneRM{}, err
	}
	if len(history) > maxHistoryEntries {
		history = history[len(history)-maxHistoryEntries:]
	}

	data, err := json.Marshal(oneRMData{Value: value, History: history})
	if err != nil {
		return models.OneRM{}, err
	}

	var id string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO one_rms (user_id, exercise_id, data) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, exercise_id)
		DO UPDATE SET data = $3, updated_at = now()
		RETURNING id
	`, userID, exerciseID, data).Scan(&id); err != nil {
		return models.OneRM{}, err
	}
	return s.Find(ctx, userID, exerciseID)
}

func (s *OneRMStore) Find(ctx context.Context, userID, exerciseID string) (models.OneRM, error) {
	return scanOneRM(s.pool.QueryRow(ctx, oneRMSelect+`
		WHERE o.user_id = $1 AND o.exercise_id = $2`, userID, exerciseID))
}

// ListForUser returns every max a member has recorded.
func (s *OneRMStore) ListForUser(ctx context.Context, userID string) ([]models.OneRM, error) {
	rows, err := s.pool.Query(ctx, oneRMSelect+`
		WHERE o.user_id = $1
		ORDER BY e.data->>'category', e.data->>'slug'`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	maxes := []models.OneRM{}
	for rows.Next() {
		m, err := scanOneRM(rows)
		if err != nil {
			return nil, err
		}
		maxes = append(maxes, m)
	}
	return maxes, rows.Err()
}

// MapForUser returns a member's maxes keyed by exercise id, which is what
// resolving a whole session's loads needs.
func (s *OneRMStore) MapForUser(ctx context.Context, userID string) (map[string]float64, error) {
	maxes, err := s.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	byExercise := make(map[string]float64, len(maxes))
	for _, m := range maxes {
		byExercise[m.ExerciseID] = m.Value
	}
	return byExercise, nil
}

// Delete removes a recorded max, putting the member's affected sets back to
// "unknown load" rather than leaving a stale number behind.
func (s *OneRMStore) Delete(ctx context.Context, userID, exerciseID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM one_rms WHERE user_id = $1 AND exercise_id = $2`, userID, exerciseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListBests returns the member's maxes on the three competition lifts, for
// their public profile, plus the powerlifting total.
func (s *OneRMStore) ListBests(ctx context.Context, userID string) ([]models.ProfileBest, float64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.exercise_id, coalesce(e.data->>'slug', ''),
		       coalesce(e.data->'labels', '{}'::jsonb),
		       coalesce((o.data->>'value')::float, 0), o.updated_at
		FROM one_rms o
		JOIN exercises e ON e.id = o.exercise_id
		WHERE o.user_id = $1 AND (e.data->>'main')::boolean IS TRUE
		ORDER BY CASE e.data->>'category'
		           WHEN 'squat' THEN 0 WHEN 'bench' THEN 1 WHEN 'deadlift' THEN 2 ELSE 3 END
	`, userID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	bests := []models.ProfileBest{}
	total := 0.0
	for rows.Next() {
		var b models.ProfileBest
		var labels []byte
		var updatedAt time.Time
		if err := rows.Scan(&b.ExerciseID, &b.Slug, &labels, &b.Value, &updatedAt); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(labels, &b.Labels); err != nil {
			return nil, 0, err
		}
		b.UpdatedAt = updatedAt
		bests = append(bests, b)
		total += b.Value
	}
	return bests, total, rows.Err()
}
