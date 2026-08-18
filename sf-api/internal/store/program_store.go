package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
)

type ProgramStore struct {
	pool *pgxpool.Pool
}

func NewProgramStore(pool *pgxpool.Pool) *ProgramStore {
	return &ProgramStore{pool: pool}
}

// programData is the JSONB payload of the programs table. The week count is
// deliberately absent: it's derived from the sessions (see programSelect), so a
// program built one session at a time can't drift out of step with a stored
// count.
type programData struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	SourceFileName string `json:"sourceFileName,omitempty"`
	// Visibility is absent on every program written before sharing existed,
	// which is exactly why the zero value has to mean "club only" - see
	// models.NormalizeProgramVisibility.
	Visibility string `json:"visibility,omitempty"`
}

// dayData is the JSONB payload of the program_days table.
type dayData struct {
	Week     int    `json:"week"`
	Day      int    `json:"day"`
	Title    string `json:"title"`
	Position int    `json:"position"`
}

// setData is the JSONB payload of the program_sets table. No load is stored:
// see models.ProgramSet and package loadcalc.
type setData struct {
	Position     int      `json:"position"`
	Reps         int      `json:"reps"`
	RPE          *float64 `json:"rpe,omitempty"`
	Percentage   *float64 `json:"percentage,omitempty"`
	AbsoluteLoad *float64 `json:"absoluteLoad,omitempty"`
	LoadMode     string   `json:"loadMode"`
	Part         int      `json:"part,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

const programSelect = `
	SELECT p.id, p.club_id, p.author_id, p.data, p.created_at, p.updated_at,
	       coalesce(u.data->>'name', '') || ' ' || coalesce(u.data->>'surname', ''),
	       (SELECT count(*) FROM program_days WHERE program_id = p.id),
	       (SELECT count(*) FROM program_sets WHERE program_id = p.id),
	       (SELECT coalesce(max((data->>'week')::int), 0) FROM program_days WHERE program_id = p.id),
	       coalesce(c.data->>'name', '')
	FROM programs p
	JOIN users u ON u.id = p.author_id
	JOIN clubs c ON c.id = p.club_id`

func scanProgram(row pgx.Row) (models.Program, error) {
	var p models.Program
	var raw []byte
	if err := row.Scan(&p.ID, &p.ClubID, &p.AuthorID, &raw, &p.CreatedAt, &p.UpdatedAt,
		&p.AuthorName, &p.DayCount, &p.SetCount, &p.Weeks, &p.ClubName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Program{}, ErrNotFound
		}
		return models.Program{}, err
	}
	var d programData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.Program{}, err
	}
	p.Name = d.Name
	p.Description = d.Description
	p.SourceFileName = d.SourceFileName
	p.Visibility = models.NormalizeProgramVisibility(d.Visibility)
	return p, nil
}

func scanPrograms(rows pgx.Rows) ([]models.Program, error) {
	defer rows.Close()
	programs := []models.Program{}
	for rows.Next() {
		p, err := scanProgram(rows)
		if err != nil {
			return nil, err
		}
		programs = append(programs, p)
	}
	return programs, rows.Err()
}

// NewProgram is a whole parsed program, written in one transaction so a failed
// import never leaves a half-populated block behind.
type NewProgram struct {
	ClubID         string
	AuthorID       string
	Name           string
	Description    string
	SourceFileName string
	Visibility     string
	Days           []NewDay
}

type NewDay struct {
	Week     int
	Day      int
	Title    string
	Position int
	Sets     []NewSet
}

type NewSet struct {
	ExerciseID   string
	Position     int
	Reps         int
	RPE          *float64
	Percentage   *float64
	AbsoluteLoad *float64
	LoadMode     string
	Part         int
	Notes        string
}

// Create writes a program with all its days and sets.
func (s *ProgramStore) Create(ctx context.Context, p NewProgram) (models.Program, error) {
	data, err := json.Marshal(programData{
		Name: p.Name, Description: p.Description, SourceFileName: p.SourceFileName,
		Visibility: models.NormalizeProgramVisibility(p.Visibility),
	})
	if err != nil {
		return models.Program{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Program{}, err
	}
	defer tx.Rollback(ctx)

	var programID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO programs (club_id, author_id, data) VALUES ($1, $2, $3) RETURNING id
	`, p.ClubID, p.AuthorID, data).Scan(&programID); err != nil {
		return models.Program{}, err
	}

	for _, day := range p.Days {
		dayJSON, err := json.Marshal(dayData{Week: day.Week, Day: day.Day, Title: day.Title, Position: day.Position})
		if err != nil {
			return models.Program{}, err
		}

		var dayID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO program_days (program_id, data) VALUES ($1, $2) RETURNING id
		`, programID, dayJSON).Scan(&dayID); err != nil {
			return models.Program{}, err
		}

		for _, set := range day.Sets {
			setJSON, err := json.Marshal(setData{
				Position: set.Position, Reps: set.Reps, RPE: set.RPE, Percentage: set.Percentage,
				AbsoluteLoad: set.AbsoluteLoad, LoadMode: set.LoadMode, Part: set.Part, Notes: set.Notes,
			})
			if err != nil {
				return models.Program{}, err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO program_sets (program_id, day_id, exercise_id, data) VALUES ($1, $2, $3, $4)
			`, programID, dayID, set.ExerciseID, setJSON); err != nil {
				return models.Program{}, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Program{}, err
	}
	return s.FindByID(ctx, programID)
}

func (s *ProgramStore) FindByID(ctx context.Context, id string) (models.Program, error) {
	return scanProgram(s.pool.QueryRow(ctx, programSelect+` WHERE p.id = $1`, id))
}

// ListForClub returns a club's programs, newest first.
// FindPublicByID returns a program only when it has been shared publicly.
// The predicate lives in the query rather than in a caller-side check, so the
// unauthenticated path cannot read a private program even if it forgets to
// look at Visibility.
func (s *ProgramStore) FindPublicByID(ctx context.Context, id string) (models.Program, error) {
	return scanProgram(s.pool.QueryRow(ctx,
		programSelect+` WHERE p.id = $1 AND p.data->>'visibility' = $2`, id, models.ProgramVisibilityPublic))
}

func (s *ProgramStore) ListForClub(ctx context.Context, clubID string) ([]models.Program, error) {
	rows, err := s.pool.Query(ctx, programSelect+` WHERE p.club_id = $1 ORDER BY p.created_at DESC`, clubID)
	if err != nil {
		return nil, err
	}
	return scanPrograms(rows)
}

// UpdateMeta renames a program or changes its description; the sessions
// themselves are edited set by set.
func (s *ProgramStore) UpdateMeta(ctx context.Context, id, name, description, visibility string) (models.Program, error) {
	patch, err := json.Marshal(map[string]any{
		"name": name, "description": description,
		"visibility": models.NormalizeProgramVisibility(visibility),
	})
	if err != nil {
		return models.Program{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE programs SET data = data || $2::jsonb, updated_at = now() WHERE id = $1
	`, id, patch)
	if err != nil {
		return models.Program{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Program{}, ErrNotFound
	}
	return s.FindByID(ctx, id)
}

func (s *ProgramStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM programs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- days and sets ---

const daySelect = `SELECT id, program_id, data, created_at, updated_at FROM program_days`

func scanDay(row pgx.Row) (models.ProgramDay, error) {
	var d models.ProgramDay
	var raw []byte
	if err := row.Scan(&d.ID, &d.ProgramID, &raw, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ProgramDay{}, ErrNotFound
		}
		return models.ProgramDay{}, err
	}
	var payload dayData
	if err := json.Unmarshal(raw, &payload); err != nil {
		return models.ProgramDay{}, err
	}
	d.Week = payload.Week
	d.Day = payload.Day
	d.Title = payload.Title
	d.Position = payload.Position
	return d, nil
}

// ListDays returns a program's sessions in order.
func (s *ProgramStore) ListDays(ctx context.Context, programID string) ([]models.ProgramDay, error) {
	rows, err := s.pool.Query(ctx, daySelect+`
		WHERE program_id = $1
		ORDER BY (data->>'week')::int, (data->>'day')::int, (data->>'position')::int
	`, programID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	days := []models.ProgramDay{}
	for rows.Next() {
		d, err := scanDay(rows)
		if err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, rows.Err()
}

func (s *ProgramStore) FindDay(ctx context.Context, dayID string) (models.ProgramDay, error) {
	return scanDay(s.pool.QueryRow(ctx, daySelect+` WHERE id = $1`, dayID))
}

// AddDay appends a session to a program, for a coach building one by hand
// rather than importing it.
func (s *ProgramStore) AddDay(ctx context.Context, programID string, f NewDay) (models.ProgramDay, error) {
	data, err := json.Marshal(dayData{Week: f.Week, Day: f.Day, Title: f.Title, Position: f.Position})
	if err != nil {
		return models.ProgramDay{}, err
	}
	var dayID string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO program_days (program_id, data) VALUES ($1, $2) RETURNING id
	`, programID, data).Scan(&dayID); err != nil {
		return models.ProgramDay{}, err
	}
	return s.FindDay(ctx, dayID)
}

// UpdateDay renumbers or renames a session.
func (s *ProgramStore) UpdateDay(ctx context.Context, dayID string, f NewDay) (models.ProgramDay, error) {
	patch, err := json.Marshal(dayData{Week: f.Week, Day: f.Day, Title: f.Title, Position: f.Position})
	if err != nil {
		return models.ProgramDay{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE program_days SET data = data || $2::jsonb, updated_at = now() WHERE id = $1
	`, dayID, patch)
	if err != nil {
		return models.ProgramDay{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.ProgramDay{}, ErrNotFound
	}
	return s.FindDay(ctx, dayID)
}

// DeleteDay removes a session; its sets go with it by cascade.
func (s *ProgramStore) DeleteDay(ctx context.Context, dayID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM program_days WHERE id = $1`, dayID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// NextDayNumber suggests where a new session goes: the next day of the last
// week, so repeatedly adding sessions fills a week before starting the next.
// Returns (1, 1) for an empty program.
func (s *ProgramStore) NextDayNumber(ctx context.Context, programID string) (week, day, position int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT coalesce(max((data->>'week')::int), 1),
		       coalesce(max((data->>'position')::int) + 1, 0)
		FROM program_days WHERE program_id = $1
	`, programID).Scan(&week, &position)
	if err != nil {
		return 0, 0, 0, err
	}

	err = s.pool.QueryRow(ctx, `
		SELECT coalesce(max((data->>'day')::int) + 1, 1)
		FROM program_days WHERE program_id = $1 AND (data->>'week')::int = $2
	`, programID, week).Scan(&day)
	if err != nil {
		return 0, 0, 0, err
	}
	return week, day, position, nil
}

// setSelect joins the exercise so a session renders without a lookup per row.
const setSelect = `
	SELECT ps.id, ps.program_id, ps.day_id, ps.exercise_id,
	       coalesce(e.data->>'slug', ''), coalesce(e.data->'labels', '{}'::jsonb),
	       coalesce((e.data->>'bodyweight')::boolean, false),
	       coalesce(e.data->>'oneRmRef', ''), coalesce(e.data->>'category', ''),
	       ps.data, ps.created_at, ps.updated_at
	FROM program_sets ps
	JOIN exercises e ON e.id = ps.exercise_id`

func scanSet(row pgx.Row) (models.ProgramSet, error) {
	var s models.ProgramSet
	var raw, labels []byte
	// category is selected for ordering/diagnostics but isn't part of a set.
	var category string
	if err := row.Scan(&s.ID, &s.ProgramID, &s.DayID, &s.ExerciseID,
		&s.ExerciseSlug, &labels, &s.Bodyweight, &s.ExerciseOneRMRef, &category,
		&raw, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ProgramSet{}, ErrNotFound
		}
		return models.ProgramSet{}, err
	}
	var d setData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.ProgramSet{}, err
	}
	if err := json.Unmarshal(labels, &s.ExerciseLabels); err != nil {
		return models.ProgramSet{}, err
	}
	s.Position = d.Position
	s.Reps = d.Reps
	s.RPE = d.RPE
	s.Percentage = d.Percentage
	s.AbsoluteLoad = d.AbsoluteLoad
	s.LoadMode = d.LoadMode
	s.Part = d.Part
	s.Notes = d.Notes
	return s, nil
}

func scanSets(rows pgx.Rows) ([]models.ProgramSet, error) {
	defer rows.Close()
	sets := []models.ProgramSet{}
	for rows.Next() {
		s, err := scanSet(rows)
		if err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}

// ListSetsForProgram returns every set in a program, ordered by session then
// position.
func (s *ProgramStore) ListSetsForProgram(ctx context.Context, programID string) ([]models.ProgramSet, error) {
	rows, err := s.pool.Query(ctx, setSelect+`
		JOIN program_days pd ON pd.id = ps.day_id
		WHERE ps.program_id = $1
		ORDER BY (pd.data->>'week')::int, (pd.data->>'day')::int, (ps.data->>'position')::int
	`, programID)
	if err != nil {
		return nil, err
	}
	return scanSets(rows)
}

// ListSetsForDay returns one session's sets in order.
func (s *ProgramStore) ListSetsForDay(ctx context.Context, dayID string) ([]models.ProgramSet, error) {
	rows, err := s.pool.Query(ctx, setSelect+`
		WHERE ps.day_id = $1 ORDER BY (ps.data->>'position')::int
	`, dayID)
	if err != nil {
		return nil, err
	}
	return scanSets(rows)
}

func (s *ProgramStore) FindSet(ctx context.Context, setID string) (models.ProgramSet, error) {
	return scanSet(s.pool.QueryRow(ctx, setSelect+` WHERE ps.id = $1`, setID))
}

// UpdateSet edits one prescribed set - a coach adjusting a session after the
// import.
func (s *ProgramStore) UpdateSet(ctx context.Context, setID, exerciseID string, f NewSet) (models.ProgramSet, error) {
	patch, err := json.Marshal(setData{
		Position: f.Position, Reps: f.Reps, RPE: f.RPE, Percentage: f.Percentage,
		AbsoluteLoad: f.AbsoluteLoad, LoadMode: f.LoadMode, Part: f.Part, Notes: f.Notes,
	})
	if err != nil {
		return models.ProgramSet{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE program_sets SET exercise_id = $2, data = $3, updated_at = now() WHERE id = $1
	`, setID, exerciseID, patch)
	if err != nil {
		return models.ProgramSet{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.ProgramSet{}, ErrNotFound
	}
	return s.FindSet(ctx, setID)
}

// AddSet appends a set to a session.
func (s *ProgramStore) AddSet(ctx context.Context, programID, dayID, exerciseID string, f NewSet) (models.ProgramSet, error) {
	payload, err := json.Marshal(setData{
		Position: f.Position, Reps: f.Reps, RPE: f.RPE, Percentage: f.Percentage,
		AbsoluteLoad: f.AbsoluteLoad, LoadMode: f.LoadMode, Part: f.Part, Notes: f.Notes,
	})
	if err != nil {
		return models.ProgramSet{}, err
	}
	var setID string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO program_sets (program_id, day_id, exercise_id, data)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, programID, dayID, exerciseID, payload).Scan(&setID); err != nil {
		return models.ProgramSet{}, err
	}
	return s.FindSet(ctx, setID)
}

func (s *ProgramStore) DeleteSet(ctx context.Context, setID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM program_sets WHERE id = $1`, setID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// NextSetPosition returns the position to append a new set at in a session.
func (s *ProgramStore) NextSetPosition(ctx context.Context, dayID string) (int, error) {
	var next *int
	if err := s.pool.QueryRow(ctx, `
		SELECT max((data->>'position')::int) + 1 FROM program_sets WHERE day_id = $1
	`, dayID).Scan(&next); err != nil {
		return 0, err
	}
	if next == nil {
		return 0, nil
	}
	return *next, nil
}

// --- assignments ---

// assignmentData is the JSONB payload of the program_assignments table.
type assignmentData struct {
	StartDate string `json:"startDate,omitempty"`
	Status    string `json:"status,omitempty"`
	Note      string `json:"note,omitempty"`
}

const assignmentSelect = `
	SELECT a.id, a.program_id, a.user_id, a.data, a.created_at, a.updated_at,
	       coalesce(p.data->>'name', ''), p.club_id, coalesce(c.data->>'name', ''),
	       coalesce(u.data->>'name', '') || ' ' || coalesce(u.data->>'surname', ''), u.email,
	       (SELECT count(*) FROM program_sets WHERE program_id = a.program_id),
	       (SELECT count(*) FROM set_logs WHERE assignment_id = a.id AND (data->>'done')::boolean IS TRUE)
	FROM program_assignments a
	JOIN programs p ON p.id = a.program_id
	JOIN clubs c ON c.id = p.club_id
	JOIN users u ON u.id = a.user_id`

func scanAssignment(row pgx.Row) (models.ProgramAssignment, error) {
	var a models.ProgramAssignment
	var raw []byte
	if err := row.Scan(&a.ID, &a.ProgramID, &a.UserID, &raw, &a.CreatedAt, &a.UpdatedAt,
		&a.ProgramName, &a.ClubID, &a.ClubName, &a.MemberName, &a.MemberEmail,
		&a.TotalSets, &a.CompletedSets); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ProgramAssignment{}, ErrNotFound
		}
		return models.ProgramAssignment{}, err
	}
	var d assignmentData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.ProgramAssignment{}, err
	}
	a.StartDate = d.StartDate
	a.Status = d.Status
	a.Note = d.Note
	return a, nil
}

func scanAssignments(rows pgx.Rows) ([]models.ProgramAssignment, error) {
	defer rows.Close()
	assignments := []models.ProgramAssignment{}
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

// Assign hands a program to a member. Re-assigning an existing pairing updates
// it rather than failing, so a coach can restart a block without first
// unassigning it.
func (s *ProgramStore) Assign(ctx context.Context, programID, userID, startDate, note string) (models.ProgramAssignment, error) {
	data, err := json.Marshal(assignmentData{
		StartDate: startDate, Status: models.AssignmentStatusActive, Note: note,
	})
	if err != nil {
		return models.ProgramAssignment{}, err
	}
	var id string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO program_assignments (program_id, user_id, data) VALUES ($1, $2, $3)
		ON CONFLICT (program_id, user_id) DO UPDATE SET data = program_assignments.data || $3::jsonb, updated_at = now()
		RETURNING id
	`, programID, userID, data).Scan(&id); err != nil {
		return models.ProgramAssignment{}, err
	}
	return s.FindAssignment(ctx, id)
}

func (s *ProgramStore) FindAssignment(ctx context.Context, id string) (models.ProgramAssignment, error) {
	return scanAssignment(s.pool.QueryRow(ctx, assignmentSelect+` WHERE a.id = $1`, id))
}

// FindAssignmentFor returns a member's assignment of one program.
func (s *ProgramStore) FindAssignmentFor(ctx context.Context, programID, userID string) (models.ProgramAssignment, error) {
	return scanAssignment(s.pool.QueryRow(ctx, assignmentSelect+`
		WHERE a.program_id = $1 AND a.user_id = $2`, programID, userID))
}

// ListAssignmentsForUser returns the programs a member has been given.
func (s *ProgramStore) ListAssignmentsForUser(ctx context.Context, userID string) ([]models.ProgramAssignment, error) {
	rows, err := s.pool.Query(ctx, assignmentSelect+` WHERE a.user_id = $1 ORDER BY a.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	return scanAssignments(rows)
}

// ListAssignmentsForProgram returns everyone running a program, for its coach.
func (s *ProgramStore) ListAssignmentsForProgram(ctx context.Context, programID string) ([]models.ProgramAssignment, error) {
	rows, err := s.pool.Query(ctx, assignmentSelect+` WHERE a.program_id = $1 ORDER BY u.email`, programID)
	if err != nil {
		return nil, err
	}
	return scanAssignments(rows)
}

// SetAssignmentStatus marks a block active, done or archived.
func (s *ProgramStore) SetAssignmentStatus(ctx context.Context, id, status string) (models.ProgramAssignment, error) {
	patch, err := json.Marshal(map[string]any{"status": status})
	if err != nil {
		return models.ProgramAssignment{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE program_assignments SET data = data || $2::jsonb, updated_at = now() WHERE id = $1
	`, id, patch)
	if err != nil {
		return models.ProgramAssignment{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.ProgramAssignment{}, ErrNotFound
	}
	return s.FindAssignment(ctx, id)
}

func (s *ProgramStore) Unassign(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM program_assignments WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- set logs ---

// setLogData is the JSONB payload of the set_logs table.
type setLogData struct {
	ActualReps  *int       `json:"actualReps,omitempty"`
	ActualRPE   *float64   `json:"actualRpe,omitempty"`
	ActualLoad  *float64   `json:"actualLoad,omitempty"`
	Comment     string     `json:"comment,omitempty"`
	Done        bool       `json:"done"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

const setLogSelect = `SELECT id, assignment_id, set_id, user_id, data, created_at, updated_at FROM set_logs`

func scanSetLog(row pgx.Row) (models.SetLog, error) {
	var l models.SetLog
	var raw []byte
	if err := row.Scan(&l.ID, &l.AssignmentID, &l.SetID, &l.UserID, &raw, &l.CreatedAt, &l.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.SetLog{}, ErrNotFound
		}
		return models.SetLog{}, err
	}
	var d setLogData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.SetLog{}, err
	}
	l.ActualReps = d.ActualReps
	l.ActualRPE = d.ActualRPE
	l.ActualLoad = d.ActualLoad
	l.Comment = d.Comment
	l.Done = d.Done
	l.CompletedAt = d.CompletedAt
	return l, nil
}

// SetLogFields is one member's feedback on one prescribed set.
type SetLogFields struct {
	ActualReps *int
	ActualRPE  *float64
	ActualLoad *float64
	Comment    string
	Done       bool
}

// LogSet records (or replaces) what a member did on a set.
func (s *ProgramStore) LogSet(ctx context.Context, assignmentID, setID, userID string, f SetLogFields) (models.SetLog, error) {
	payload := setLogData{
		ActualReps: f.ActualReps, ActualRPE: f.ActualRPE, ActualLoad: f.ActualLoad,
		Comment: f.Comment, Done: f.Done,
	}
	if f.Done {
		now := time.Now().UTC()
		payload.CompletedAt = &now
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return models.SetLog{}, err
	}

	var id string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO set_logs (assignment_id, set_id, user_id, data) VALUES ($1, $2, $3, $4)
		ON CONFLICT (assignment_id, set_id) DO UPDATE SET data = $4, updated_at = now()
		RETURNING id
	`, assignmentID, setID, userID, data).Scan(&id); err != nil {
		return models.SetLog{}, err
	}
	return scanSetLog(s.pool.QueryRow(ctx, setLogSelect+` WHERE id = $1`, id))
}

// DeleteSetLog clears a member's feedback on a set.
func (s *ProgramStore) DeleteSetLog(ctx context.Context, assignmentID, setID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM set_logs WHERE assignment_id = $1 AND set_id = $2`, assignmentID, setID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListLogsForAssignment returns a member's logs keyed by set id, for rendering
// a whole program with its feedback in one pass.
func (s *ProgramStore) ListLogsForAssignment(ctx context.Context, assignmentID string) (map[string]models.SetLog, error) {
	rows, err := s.pool.Query(ctx, setLogSelect+` WHERE assignment_id = $1`, assignmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := map[string]models.SetLog{}
	for rows.Next() {
		l, err := scanSetLog(rows)
		if err != nil {
			return nil, err
		}
		logs[l.SetID] = l
	}
	return logs, rows.Err()
}

// ListFeedbackForClub is the coach's inbox: every set one of their members left
// a comment or a perceived RPE on, newest first.
func (s *ProgramStore) ListFeedbackForClub(ctx context.Context, clubID string, page, size int) ([]models.SetFeedback, int, error) {
	limit, offset := clampPage(page, size, 100)

	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM set_logs sl
		JOIN program_assignments a ON a.id = sl.assignment_id
		JOIN programs p ON p.id = a.program_id
		WHERE p.club_id = $1
		  AND (coalesce(sl.data->>'comment', '') <> '' OR sl.data->>'actualRpe' IS NOT NULL)
	`, clubID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT sl.id, sl.assignment_id, sl.set_id, sl.user_id, sl.data, sl.created_at, sl.updated_at,
		       coalesce(u.data->>'name', '') || ' ' || coalesce(u.data->>'surname', ''),
		       coalesce(u.data->>'handle', ''), coalesce(u.data->>'picture', ''),
		       p.id, coalesce(p.data->>'name', ''),
		       coalesce((pd.data->>'week')::int, 0), coalesce((pd.data->>'day')::int, 0),
		       coalesce(e.data->>'slug', ''), coalesce(e.data->'labels', '{}'::jsonb),
		       coalesce((ps.data->>'reps')::int, 0), ps.data->>'rpe'
		FROM set_logs sl
		JOIN program_assignments a ON a.id = sl.assignment_id
		JOIN programs p ON p.id = a.program_id
		JOIN program_sets ps ON ps.id = sl.set_id
		JOIN program_days pd ON pd.id = ps.day_id
		JOIN exercises e ON e.id = ps.exercise_id
		JOIN users u ON u.id = sl.user_id
		WHERE p.club_id = $1
		  AND (coalesce(sl.data->>'comment', '') <> '' OR sl.data->>'actualRpe' IS NOT NULL)
		ORDER BY sl.updated_at DESC
		LIMIT $2 OFFSET $3
	`, clubID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	feedback := []models.SetFeedback{}
	for rows.Next() {
		var f models.SetFeedback
		var raw, labels []byte
		var prescribedRPE *string
		if err := rows.Scan(&f.ID, &f.AssignmentID, &f.SetID, &f.UserID, &raw, &f.CreatedAt, &f.UpdatedAt,
			&f.MemberName, &f.MemberHandle, &f.MemberPicture, &f.ProgramID, &f.ProgramName,
			&f.Week, &f.Day, &f.ExerciseSlug, &labels, &f.PrescribedReps, &prescribedRPE); err != nil {
			return nil, 0, err
		}
		var d setLogData
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(labels, &f.Labels); err != nil {
			return nil, 0, err
		}
		f.ActualReps = d.ActualReps
		f.ActualRPE = d.ActualRPE
		f.ActualLoad = d.ActualLoad
		f.Comment = d.Comment
		f.Done = d.Done
		f.CompletedAt = d.CompletedAt
		if prescribedRPE != nil {
			if parsed, err := parseFloat(*prescribedRPE); err == nil {
				f.PrescribedRPE = &parsed
			}
		}
		feedback = append(feedback, f)
	}
	return feedback, total, rows.Err()
}
