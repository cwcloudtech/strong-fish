package store

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
)

// AuditStore is the administrative audit trail: a write-mostly log the
// superadmin console reads newest-first. Every auditable event - an AI call, a
// sign-in, a role change, a ban, a deletion - is one row, written off the
// request that caused it so the record can never slow or fail the action.
type AuditStore struct {
	pool *pgxpool.Pool
}

func NewAuditStore(pool *pgxpool.Pool) *AuditStore {
	return &AuditStore{pool: pool}
}

// auditMaxPageSize caps one page of the trail. A page is requested with a size
// (default auditDefaultPageSize) and is clamped to this.
const (
	auditDefaultPageSize = 50
	auditMaxPageSize     = 200
)

// Record writes one audit row and returns nothing: an audit write is
// administrative data that must never slow or fail the action it records, so it
// is fire-and-forget. The work runs on a detached context (context.WithoutCancel)
// in a goroutine, so the member's request can return - and its context be
// cancelled - without losing or blocking on the write; any failure is logged
// and swallowed.
//
// userID may be blank (an anonymous event, or an actor since gone): it is then
// stored as NULL. When set, the actor's email is snapshotted into the row at
// write time, so the event still names who did it after the account is deleted.
// detail is the event's own payload, marshalled into data->'detail'.
func (s *AuditStore) Record(ctx context.Context, userID, action, subjectType, subjectID string, detail map[string]any) {
	ctx = context.WithoutCancel(ctx)

	raw, err := json.Marshal(detail)
	if err != nil || len(raw) == 0 {
		// A payload that will not marshal must not cost the event: log the row
		// with an empty detail rather than drop it.
		if err != nil {
			slog.Error("audit: failed to marshal detail", "action", action, "error", err)
		}
		raw = []byte("{}")
	}

	// A blank id becomes a typed NULL rather than an empty string the uuid
	// column would reject; the email subquery then simply finds nobody.
	var uid any
	if userID != "" {
		uid = userID
	}

	go func() {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO audit_log (user_id, action, subject_type, subject_id, data)
			SELECT $1::uuid, $2, $3, $4,
			       jsonb_build_object(
			         'actorEmail', (SELECT email FROM users WHERE id = $1::uuid),
			         'detail', $5::jsonb
			       )
		`, uid, action, subjectType, subjectID, string(raw))
		if err != nil {
			slog.Error("audit: failed to record event", "action", action,
				"subjectType", subjectType, "subjectId", subjectID, "error", err)
		}
	}()
}

// List returns one page of the trail, newest-first. page is 0-based; size
// defaults to auditDefaultPageSize and is capped at auditMaxPageSize. action is
// an optional exact-action filter; subject is an optional filter that matches
// either a subject_type (the console's type dropdown - "every AI call") or a
// specific subject_id (one connection, one address). The second return is
// hasNext, inferred by fetching one row past the page rather than with a second
// count query.
func (s *AuditStore) List(ctx context.Context, page, size int, action, subject string) ([]models.AuditEntry, bool, error) {
	if size <= 0 {
		size = auditDefaultPageSize
	}
	if size > auditMaxPageSize {
		size = auditMaxPageSize
	}
	if page < 0 {
		page = 0
	}

	rows, err := s.pool.Query(ctx, `
		SELECT a.id,
		       a.user_id,
		       COALESCE(u.email, a.data->>'actorEmail', '') AS email,
		       a.action,
		       a.subject_type,
		       a.subject_id,
		       COALESCE(a.data->'detail', '{}'::jsonb) AS detail,
		       a.created_at
		FROM audit_log a
		LEFT JOIN users u ON u.id = a.user_id
		WHERE ($1 = '' OR a.action = $1)
		  AND ($2 = '' OR a.subject_id = $2 OR a.subject_type = $2)
		ORDER BY a.created_at DESC
		LIMIT $3 OFFSET $4
	`, action, subject, size+1, page*size)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	entries := []models.AuditEntry{}
	for rows.Next() {
		var e models.AuditEntry
		var userID *string
		var detail []byte
		if err := rows.Scan(&e.Id, &userID, &e.UserEmail, &e.Action,
			&e.SubjectType, &e.SubjectId, &detail, &e.CreatedAt); err != nil {
			return nil, false, err
		}
		if userID != nil {
			e.UserId = *userID
		}
		if len(detail) > 0 {
			_ = json.Unmarshal(detail, &e.Detail)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	// A full page plus one means there is another page; trim the probe row off.
	hasNext := len(entries) > size
	if hasNext {
		entries = entries[:size]
	}
	return entries, hasNext, nil
}
