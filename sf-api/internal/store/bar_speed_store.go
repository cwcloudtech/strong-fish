package store

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/velocity"
)

// BarSpeedStore keeps the bar-speed sets StrongFish captured and computed
// itself, so the panel can read them beside the ones it reads off a connected
// tracker. A recording is one set; its reps are the per-rep velocity chart.
type BarSpeedStore struct {
	pool *pgxpool.Pool
}

func NewBarSpeedStore(pool *pgxpool.Pool) *BarSpeedStore {
	return &BarSpeedStore{pool: pool}
}

// Save writes one captured set - the recording and its reps - in a transaction,
// so a set never lands half-stored with its chart missing.
func (s *BarSpeedStore) Save(ctx context.Context, userID string, set models.BarSet) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	performed := set.PerformedAt
	if performed.IsZero() {
		performed = time.Now().UTC()
	}
	source := set.Source
	if source == "" {
		source = "strongfish"
	}

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO bar_speed_recordings
			(user_id, lift, weight_kg, given_rpe, predicted_rpe, mean_velocity, peak_velocity, source, performed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id::text`,
		userID, set.Lift, set.WeightKg, set.GivenRpe, set.PredictedRpe,
		set.MeanVelocity, set.PeakVelocity, source, performed).Scan(&id); err != nil {
		return err
	}

	for i := range set.RepVelocities {
		peak := 0.0
		if i < len(set.RepPeakVelocities) {
			peak = set.RepPeakVelocities[i]
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO bar_speed_reps (recording_id, rep_order, mean_mps, peak_mps)
			VALUES ($1, $2, $3, $4)`, id, i+1, set.RepVelocities[i], peak); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// LastSets returns the member's most recent set of each lift from StrongFish's
// own recordings, newest first - the same shape the Stance read returns, so the
// two merge cleanly.
func (s *BarSpeedStore) LastSets(ctx context.Context, userID string) ([]models.BarSet, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (lift)
			id::text, lift, weight_kg, given_rpe, predicted_rpe,
			mean_velocity, peak_velocity, source, performed_at
		FROM bar_speed_recordings
		WHERE user_id = $1
		ORDER BY lift, performed_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sets []models.BarSet
	indexByID := map[string]int{}
	for rows.Next() {
		var id string
		var set models.BarSet
		if err := rows.Scan(&id, &set.Lift, &set.WeightKg, &set.GivenRpe, &set.PredictedRpe,
			&set.MeanVelocity, &set.PeakVelocity, &set.Source, &set.PerformedAt); err != nil {
			return nil, err
		}
		indexByID[id] = len(sets)
		sets = append(sets, set)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(sets) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(indexByID))
	for id := range indexByID {
		ids = append(ids, id)
	}
	repRows, err := s.pool.Query(ctx, `
		SELECT recording_id::text, mean_mps, peak_mps
		FROM bar_speed_reps
		WHERE recording_id = ANY($1)
		ORDER BY recording_id, rep_order`, ids)
	if err != nil {
		return nil, err
	}
	defer repRows.Close()
	for repRows.Next() {
		var rid string
		var mean, peak float64
		if err := repRows.Scan(&rid, &mean, &peak); err != nil {
			return nil, err
		}
		if idx, ok := indexByID[rid]; ok {
			sets[idx].RepVelocities = append(sets[idx].RepVelocities, mean)
			sets[idx].RepPeakVelocities = append(sets[idx].RepPeakVelocities, peak)
			sets[idx].Reps = len(sets[idx].RepVelocities)
		}
	}
	if err := repRows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(sets, func(i, j int) bool { return sets[i].PerformedAt.After(sets[j].PerformedAt) })
	return sets, nil
}

// RPEProfile is the member's history of (felt RPE, last-rep velocity) for one
// lift, from the sets they rated - what a predicted RPE is read against. Only
// rated sets count, and each contributes its last (slowest) rep's speed.
func (s *BarSpeedStore) RPEProfile(ctx context.Context, userID, lift string) ([]velocity.RPEPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.given_rpe, last_rep.mean_mps
		FROM bar_speed_recordings r
		JOIN LATERAL (
			SELECT mean_mps FROM bar_speed_reps
			WHERE recording_id = r.id
			ORDER BY rep_order DESC
			LIMIT 1
		) last_rep ON true
		WHERE r.user_id = $1 AND r.lift = $2 AND r.given_rpe > 0`, userID, lift)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []velocity.RPEPoint
	for rows.Next() {
		var point velocity.RPEPoint
		if err := rows.Scan(&point.Rpe, &point.Velocity); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}
