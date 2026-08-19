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

type EventStore struct {
	pool *pgxpool.Pool
}

func NewEventStore(pool *pgxpool.Pool) *EventStore {
	return &EventStore{pool: pool}
}

// eventData is the JSONB payload of the events table.
//
// Times are RFC 3339 strings, always normalized to UTC. They are instants
// rather than the floating dates a training day uses, because an event happens
// at a real moment in a real place - and they are stored in one zone because
// the listings compare them as text: two RFC 3339 strings only sort
// chronologically when they share an offset.
//
// Deliberately no `omitempty` on the optional fields. Update merges the new
// payload into the old one (`data || $2::jsonb`), so a key that is omitted is
// a key that keeps its previous value - which made every optional field
// impossible to clear: removing an event's location left the old one on it.
// Writing the empty string overwrites it.
type eventData struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	URL         string `json:"url"`
	Kind        string `json:"kind"`
	Color       string `json:"color"`
	StartsAt    string `json:"startsAt"`
	EndsAt      string `json:"endsAt"`
	Visibility  string `json:"visibility"`
}

var eventSelect = `
	SELECT e.id, e.club_id, e.author_id, e.data, e.created_at, e.updated_at,
	       coalesce(c.data->>'name', ''),
	       coalesce(u.data->>'handle', ''),
	       ` + displayName("u") + `, ` + displaySurname("u") + `,
	       coalesce(u.data->>'picture', '')
	FROM events e
	JOIN users u ON u.id = e.author_id
	LEFT JOIN clubs c ON c.id = e.club_id`

func scanEvent(row pgx.Row) (models.Event, error) {
	var e models.Event
	var raw []byte
	var clubID *string
	if err := row.Scan(&e.ID, &clubID, &e.AuthorID, &raw, &e.CreatedAt, &e.UpdatedAt,
		&e.ClubName, &e.Author.Handle, &e.Author.Name, &e.Author.Surname, &e.Author.Picture); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Event{}, ErrNotFound
		}
		return models.Event{}, err
	}
	if clubID != nil {
		e.ClubID = *clubID
	}
	e.Author.ID = e.AuthorID

	var d eventData
	if err := json.Unmarshal(raw, &d); err != nil {
		return models.Event{}, err
	}
	e.Title = d.Title
	e.Description = d.Description
	e.Location = d.Location
	e.URL = d.URL
	e.Kind = models.NormalizeEventKind(d.Kind)
	e.Color = models.NormalizeHexColor(d.Color)
	e.Visibility = d.Visibility
	if e.Visibility != models.VisibilityPublic && e.Visibility != models.EventVisibilityPrivate {
		e.Visibility = models.VisibilityClub
	}
	e.StartsAt, _ = time.Parse(time.RFC3339, d.StartsAt)
	e.EndsAt, _ = time.Parse(time.RFC3339, d.EndsAt)
	return e, nil
}

func scanEvents(rows pgx.Rows) ([]models.Event, error) {
	defer rows.Close()
	events := []models.Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// EventFields is one event as its author wrote it.
type EventFields struct {
	ClubID      string
	Title       string
	Description string
	Location    string
	URL         string
	Kind        string
	Color       string
	StartsAt    time.Time
	EndsAt      time.Time
	Visibility  string
}

func (f EventFields) payload() eventData {
	data := eventData{
		Title: f.Title, Description: f.Description, Location: f.Location, URL: f.URL,
		Kind:     models.NormalizeEventKind(f.Kind),
		Color:    models.NormalizeHexColor(f.Color),
		StartsAt: f.StartsAt.UTC().Format(time.RFC3339),
	}
	if !f.EndsAt.IsZero() {
		data.EndsAt = f.EndsAt.UTC().Format(time.RFC3339)
	}
	// Anything unrecognized lands on club-only, and a club-less event on
	// private: an event with nowhere to belong must not default to the open
	// calendar.
	switch f.Visibility {
	case models.VisibilityPublic:
		data.Visibility = models.VisibilityPublic
	case models.EventVisibilityPrivate:
		data.Visibility = models.EventVisibilityPrivate
	default:
		data.Visibility = models.VisibilityClub
		if f.ClubID == "" {
			data.Visibility = models.EventVisibilityPrivate
		}
	}
	return data
}

func (s *EventStore) Create(ctx context.Context, authorID string, f EventFields) (models.Event, error) {
	data, err := json.Marshal(f.payload())
	if err != nil {
		return models.Event{}, err
	}

	// An event with no club is the open calendar - a federation meet - so the
	// column is left NULL rather than holding an empty string a UUID column
	// would reject.
	var clubID *string
	if f.ClubID != "" {
		clubID = &f.ClubID
	}

	var id string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO events (club_id, author_id, data) VALUES ($1, $2, $3) RETURNING id
	`, clubID, authorID, data).Scan(&id); err != nil {
		return models.Event{}, err
	}
	return s.FindByID(ctx, id)
}

// Update rewrites an event's payload. The club it belongs to is settled at
// creation: moving one between clubs would change who can see it after the
// fact, the same reason a post's visibility is fixed.
func (s *EventStore) Update(ctx context.Context, id string, f EventFields) (models.Event, error) {
	data, err := json.Marshal(f.payload())
	if err != nil {
		return models.Event{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE events SET data = data || $2::jsonb, updated_at = now() WHERE id = $1
	`, id, data)
	if err != nil {
		return models.Event{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Event{}, ErrNotFound
	}
	return s.FindByID(ctx, id)
}

func (s *EventStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *EventStore) FindByID(ctx context.Context, id string) (models.Event, error) {
	return scanEvent(s.pool.QueryRow(ctx, eventSelect+` WHERE e.id = $1`, id))
}

// ListVisible returns the events one member may see: every public one, plus
// the club-only ones of the clubs they belong to.
//
// clubIDs is passed in rather than joined on club_members so the same query
// serves an anonymous caller (no clubs, public events only) without a second
// code path deciding what "visible" means.
func (s *EventStore) ListVisible(ctx context.Context, clubIDs []string, from time.Time,
	viewerID string, superadmin bool) ([]models.Event, error) {
	if clubIDs == nil {
		clubIDs = []string{}
	}
	// An author always sees their own, whatever its visibility - that is what
	// makes a private event a personal calendar rather than a write-only one.
	//
	// A private one is also visible to the coaches of the clubs its author
	// belongs to, the same rule a private *profile* follows: the people who
	// write your training are the people who should see the meet you entered.
	// The caller id is cast to text so an anonymous "" compares cleanly against
	// a uuid column instead of failing to parse.
	rows, err := s.pool.Query(ctx, eventSelect+`
		WHERE (
		        e.data->>'visibility' = $1
		     OR e.club_id::text = ANY($2)
		     OR e.author_id::text = $4
		     OR $5
		     OR (
		          e.data->>'visibility' = 'private'
		          AND EXISTS (
		                SELECT 1 FROM club_members author
		                JOIN club_members caller
		                  ON caller.club_id = author.club_id AND caller.user_id::text = $4
		                WHERE author.user_id = e.author_id AND caller.role IN ('owner', 'admin')
		              )
		        )
		      )
		  AND ($3 = '' OR coalesce(nullif(e.data->>'endsAt', ''), e.data->>'startsAt') >= $3)
		ORDER BY e.data->>'startsAt'
	`, models.VisibilityPublic, clubIDs, rfc3339OrEmpty(from), viewerID, superadmin)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// ListForClub returns one club's own calendar, public entries included.
func (s *EventStore) ListForClub(ctx context.Context, clubID string, from time.Time) ([]models.Event, error) {
	rows, err := s.pool.Query(ctx, eventSelect+`
		WHERE e.club_id = $1
		  AND ($2 = '' OR coalesce(nullif(e.data->>'endsAt', ''), e.data->>'startsAt') >= $2)
		ORDER BY e.data->>'startsAt'
	`, clubID, rfc3339OrEmpty(from))
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// ListPublic returns the open calendar, for a visitor with no account.
func (s *EventStore) ListPublic(ctx context.Context, from time.Time) ([]models.Event, error) {
	return s.ListVisible(ctx, nil, from, "", false)
}

// rfc3339OrEmpty renders a lower bound for the string comparison the queries
// above do. Stored times are always UTC (see eventData), so comparing the text
// is the same as comparing the instants. A zero time means "no bound".
func rfc3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
