package store

import (
	"context"
	"testing"
	"time"

	"strong-fish-api/internal/models"
)

// waitForAudit polls for a specific audit row, because Record writes it
// fire-and-forget in a goroutine: the write lands shortly after the call
// returns, not during it.
func waitForAudit(t *testing.T, audit *AuditStore, action, subjectID string) models.AuditEntry {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		entries, _, err := audit.List(context.Background(), 0, 10, action, subjectID)
		if err != nil {
			t.Fatalf("List while waiting for %s/%s: %v", action, subjectID, err)
		}
		if len(entries) > 0 {
			return entries[0]
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("audit row %s/%s never appeared", action, subjectID)
	return models.AuditEntry{}
}

// TestAuditQueries exercises the audit store against real PostgreSQL. The risk
// it guards is the one this whole file exists for: Record passes $1 both as the
// user_id column and inside a `WHERE id = $1::uuid` subquery, and marshals a map
// into `$5::jsonb` - exactly the untyped-parameter shape that has broken before.
// It also asserts the design's two load-bearing properties: a blank actor
// becomes NULL rather than a rejected empty uuid, and the actor's email is
// snapshotted so a since-deleted account's events still name who acted.
func TestAuditQueries(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	audit := NewAuditStore(pool)
	actor := seedUser(t, pool, "audit.actor@example.test")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM audit_log WHERE subject_id LIKE 'auditsql-%'`)
	})

	// An AI call by a real account: user id kept, email snapshotted, detail round-trips.
	audit.Record(ctx, actor, models.AuditActionAICall, models.AuditSubjectAIConnection, "auditsql-conn-1",
		map[string]any{"provider": "openai", "model": "gpt-4o", "input": 10, "output": 20, "cached": 5})
	// An anonymous event: a blank actor must store NULL, not fail on the uuid column.
	audit.Record(ctx, "", models.AuditActionIPBanned, models.AuditSubjectIP, "auditsql-ip-1",
		map[string]any{"ip": "203.0.113.9", "reason": "abuse"})

	call := waitForAudit(t, audit, models.AuditActionAICall, "auditsql-conn-1")
	if call.UserId != actor {
		t.Errorf("AI-call event lost its actor: got %q, want %q", call.UserId, actor)
	}
	if call.UserEmail != "audit.actor@example.test" {
		t.Errorf("AI-call event did not resolve the actor email: got %q", call.UserEmail)
	}
	if call.SubjectType != models.AuditSubjectAIConnection {
		t.Errorf("subject type: got %q", call.SubjectType)
	}
	if call.Detail["provider"] != "openai" {
		t.Errorf("detail did not round-trip: %#v", call.Detail)
	}
	// JSON numbers decode as float64; the magnitude is what matters here.
	if got, ok := call.Detail["input"].(float64); !ok || got != 10 {
		t.Errorf("detail input: got %#v", call.Detail["input"])
	}

	anon := waitForAudit(t, audit, models.AuditActionIPBanned, "auditsql-ip-1")
	if anon.UserId != "" {
		t.Errorf("anonymous event kept a user id: %q", anon.UserId)
	}
	if anon.UserEmail != "" {
		t.Errorf("anonymous event resolved an email from nowhere: %q", anon.UserEmail)
	}

	// The subject filter matches a subject_type (the console's dropdown)...
	byType, _, err := audit.List(ctx, 0, 50, "", models.AuditSubjectAIConnection)
	if err != nil {
		t.Fatalf("List by subject type: %v", err)
	}
	for _, e := range byType {
		if e.SubjectType != models.AuditSubjectAIConnection {
			t.Errorf("subject-type filter returned a %q event", e.SubjectType)
		}
	}
	// ...and a concrete subject_id.
	byID, _, err := audit.List(ctx, 0, 50, "", "auditsql-conn-1")
	if err != nil {
		t.Fatalf("List by subject id: %v", err)
	}
	if len(byID) != 1 || byID[0].SubjectId != "auditsql-conn-1" {
		t.Errorf("subject-id filter returned %d rows", len(byID))
	}

	// The action filter is exact.
	byAction, _, err := audit.List(ctx, 0, 50, models.AuditActionAICall, "")
	if err != nil {
		t.Fatalf("List by action: %v", err)
	}
	for _, e := range byAction {
		if e.Action != models.AuditActionAICall {
			t.Errorf("action filter returned a %q event", e.Action)
		}
	}

	// hasNext is inferred by fetching one past the page: with two of our rows,
	// a page of one must report another page.
	page, hasNext, err := audit.List(ctx, 0, 1, "", "auditsql-ip-1")
	if err != nil {
		t.Fatalf("List paged: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("page size 1 returned %d rows", len(page))
	}
	if hasNext {
		t.Errorf("a single matching row should not report a next page")
	}

	// The snapshot is why the email is copied in: delete the actor, and the
	// AI-call event must still name them though its user_id is now NULL.
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, actor); err != nil {
		t.Fatalf("deleting actor: %v", err)
	}
	afterDelete, _, err := audit.List(ctx, 0, 10, models.AuditActionAICall, "auditsql-conn-1")
	if err != nil {
		t.Fatalf("List after actor deletion: %v", err)
	}
	if len(afterDelete) != 1 {
		t.Fatalf("event vanished with its actor: %d rows", len(afterDelete))
	}
	if afterDelete[0].UserId != "" {
		t.Errorf("user_id should be NULL after the actor is deleted, got %q", afterDelete[0].UserId)
	}
	if afterDelete[0].UserEmail != "audit.actor@example.test" {
		t.Errorf("the snapshot email was lost on deletion: got %q", afterDelete[0].UserEmail)
	}
}
