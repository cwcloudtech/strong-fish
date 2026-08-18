package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"strong-fish-api/internal/models"
)

// These tests execute every query in this package against a real PostgreSQL,
// because a whole class of bug here is invisible to the compiler and to every
// other test: a column that doesn't exist, a syntax error, and above all a
// parameter whose type Postgres infers differently than intended.
//
// That last one shipped. GET /v1/messages returned 500 with "operator does not
// exist: uuid = text" because one query compared $1 to a uuid column in one
// place and to a ::text cast in another; pgx sends a Go string untyped, so
// Postgres picked text for the whole statement and the uuid comparison failed.
// Nothing but running it could have caught that.
//
// They are skipped unless SF_TEST_DATABASE_URL points at a migrated database,
// so `go test ./...` still passes with no infrastructure:
//
//	docker run -d --name sf-test-db -e POSTGRES_USER=strongfish \
//	  -e POSTGRES_PASSWORD=strongfish -e POSTGRES_DB=strongfish \
//	  -p 15432:5432 postgres:16-alpine
//	docker run --rm -v "$PWD/../sf-db:/flyway/sql" flyway/flyway:10-alpine \
//	  -url=jdbc:postgresql://host.docker.internal:15432/strongfish \
//	  -user=strongfish -password=strongfish migrate
//	SF_TEST_DATABASE_URL='postgres://strongfish:strongfish@localhost:15432/strongfish?sslmode=disable' \
//	  go test ./internal/store/
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("SF_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SF_TEST_DATABASE_URL not set; skipping the SQL tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedUser creates an account to run the queries as. Every row it makes is
// removed afterwards, so the tests can run against a database that already has
// data in it without disturbing it.
func seedUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, data) VALUES ($1, '{"role":"confirmed"}'::jsonb) RETURNING id
	`, email).Scan(&id)
	if err != nil {
		t.Fatalf("seeding %s: %v", email, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

// TestMessageQueries covers the conversation and block queries - the ones that
// broke. Each is asserted only to *execute*: the point is the SQL, and the
// behaviour is the handlers' business.
func TestMessageQueries(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	messages := NewMessageStore(pool)
	alice := seedUser(t, pool, "alice.sql@example.test")
	bob := seedUser(t, pool, "bob.sql@example.test")

	conversationID, err := messages.FindOrCreateConversation(ctx, alice, bob)
	if err != nil {
		t.Fatalf("FindOrCreateConversation: %v", err)
	}
	// Deriving it again must return the same row whichever way round the pair
	// is given - that is what the ordered member_a/member_b columns are for.
	again, err := messages.FindOrCreateConversation(ctx, bob, alice)
	if err != nil {
		t.Fatalf("FindOrCreateConversation reversed: %v", err)
	}
	if again != conversationID {
		t.Errorf("a pair produced two conversations: %s and %s", conversationID, again)
	}

	if _, err := messages.Send(ctx, conversationID, bob, "Squat felt like RPE 8."); err != nil {
		t.Fatalf("Send: %v", err)
	}

	conversations, err := messages.ListConversations(ctx, alice)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(conversations) != 1 || conversations[0].Other.ID != bob {
		t.Fatalf("ListConversations = %+v, want one thread with bob", conversations)
	}
	if conversations[0].Unread != 1 {
		t.Errorf("unread = %d, want 1", conversations[0].Unread)
	}

	unread, err := messages.CountUnread(ctx, alice)
	if err != nil {
		t.Fatalf("CountUnread: %v", err)
	}
	if unread != 1 {
		t.Errorf("CountUnread = %d, want 1", unread)
	}

	if _, err := messages.ListMessages(ctx, conversationID, 0); err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if err := messages.MarkRead(ctx, conversationID, alice, time.Now().UTC()); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if _, _, err := messages.ConversationMembers(ctx, conversationID); err != nil {
		t.Fatalf("ConversationMembers: %v", err)
	}

	// Blocking has to take the thread out of both the list and the count -
	// that filtering lives in the same queries.
	if err := messages.Block(ctx, alice, bob); err != nil {
		t.Fatalf("Block: %v", err)
	}
	blocked, err := messages.IsBlockedEither(ctx, bob, alice)
	if err != nil || !blocked {
		t.Fatalf("IsBlockedEither = %t, %v; want true", blocked, err)
	}
	if conversations, err = messages.ListConversations(ctx, alice); err != nil {
		t.Fatalf("ListConversations after block: %v", err)
	} else if len(conversations) != 0 {
		t.Errorf("a blocked thread is still listed: %+v", conversations)
	}
	if _, err := messages.ListBlocks(ctx, alice); err != nil {
		t.Fatalf("ListBlocks: %v", err)
	}
	if _, err := messages.ListBlockedIDs(ctx, alice); err != nil {
		t.Fatalf("ListBlockedIDs: %v", err)
	}
	if err := messages.Unblock(ctx, alice, bob); err != nil {
		t.Fatalf("Unblock: %v", err)
	}
}

// TestReadQueriesExecute runs the rest of the package's read paths, including
// the ones reachable by a logged-out caller - where the "caller id" is an empty
// string or a placeholder uuid, which is its own way to get parameter typing
// wrong.
func TestReadQueriesExecute(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	users := NewUserStore(pool)
	clubs := NewClubStore(pool)
	events := NewEventStore(pool)
	social := NewSocialStore(pool)
	programs := NewProgramStore(pool)
	invitations := NewInvitationStore(pool)

	caller := seedUser(t, pool, "caller.sql@example.test")

	checks := []struct {
		name string
		run  func() error
	}{
		{"SearchMembers/authenticated", func() error {
			_, _, err := users.SearchMembers(ctx, MemberSearch{Terms: "sql"}, caller, false)
			return err
		}},
		// Anonymous: callerID is "", compared against uuid columns through a
		// ::text cast. It must not error.
		{"SearchMembers/anonymous", func() error {
			_, _, err := users.SearchMembers(ctx, MemberSearch{Name: "a"}, "", false)
			return err
		}},
		{"SearchMembers/superadmin", func() error {
			_, _, err := users.SearchMembers(ctx, MemberSearch{Email: "a"}, caller, true)
			return err
		}},
		{"CountByRole", func() error { _, err := users.CountByRole(ctx); return err }},
		{"ListByIDs", func() error { _, err := users.ListByIDs(ctx, []string{caller}); return err }},
		{"ListCoachApplicants", func() error { _, err := users.ListCoachApplicants(ctx); return err }},
		{"CountCoachApplicants", func() error { _, err := users.CountCoachApplicants(ctx); return err }},
		{"ListSuperadminEmails", func() error { _, err := users.ListSuperadminEmails(ctx); return err }},
		{"FindByCalendarFeedToken", func() error {
			_, err := users.FindByCalendarFeedToken(ctx, "nope")
			if err == ErrNotFound {
				return nil
			}
			return err
		}},
		{"RecordConnection", func() error { return users.RecordConnection(ctx, caller, "203.0.113.7", time.Now().UTC()) }},

		{"RelationTo", func() error { _, err := clubs.RelationTo(ctx, caller, caller); return err }},
		{"ListClubMateIDs", func() error { _, err := clubs.ListClubMateIDs(ctx, caller); return err }},
		{"clubs.Count", func() error { _, err := clubs.Count(ctx); return err }},
		{"ListForUser", func() error { _, err := clubs.ListForUser(ctx, caller); return err }},
		{"ListClubIDsForUser", func() error { _, err := clubs.ListClubIDsForUser(ctx, caller); return err }},

		{"events.ListVisible", func() error { _, err := events.ListVisible(ctx, nil, time.Now()); return err }},
		{"events.ListPublic", func() error { _, err := events.ListPublic(ctx, time.Time{}); return err }},

		{"ListFeed", func() error { _, _, err := social.ListFeed(ctx, caller, []string{}, 0, 20); return err }},
		{"ListDiscoverFeed/anonymous", func() error { _, _, err := social.ListDiscoverFeed(ctx, "", 0, 20); return err }},
		{"ListDiscoverFeed", func() error { _, _, err := social.ListDiscoverFeed(ctx, caller, 0, 20); return err }},
		{"ListProfilePosts/anonymous", func() error {
			_, _, err := social.ListProfilePosts(ctx, caller, "", []string{}, 0, 20)
			return err
		}},

		{"programs.Count", func() error { _, err := programs.Count(ctx); return err }},
		{"ListPendingForEmail", func() error {
			_, err := invitations.ListPendingForEmail(ctx, "nobody@example.test")
			return err
		}},
		{"CountPendingForEmail", func() error {
			_, err := invitations.CountPendingForEmail(ctx, "nobody@example.test")
			return err
		}},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err != nil {
				t.Errorf("%v", err)
			}
		})
	}
}

// TestProfileVisibilitySQLMatchesTheModel guards the one rule that is written
// twice: models.CanSeeProfile decides a single profile, and SearchMembers
// re-implements it as SQL so paging stays correct. They have to agree.
func TestProfileVisibilitySQLMatchesTheModel(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	users := NewUserStore(pool)
	stranger := seedUser(t, pool, "stranger.sql@example.test")
	subject := seedUser(t, pool, "subject.sql@example.test")

	found := func(callerID string, superadmin bool) bool {
		results, _, err := users.SearchMembers(ctx,
			MemberSearch{Email: "subject.sql@example.test"}, callerID, superadmin)
		if err != nil {
			t.Fatalf("SearchMembers: %v", err)
		}
		for _, user := range results {
			if user.ID == subject {
				return true
			}
		}
		return false
	}

	for _, visibility := range []string{
		models.ProfileVisibilityPublic,
		models.ProfileVisibilityClubs,
		models.ProfileVisibilityPrivate,
	} {
		if _, err := users.merge(ctx, subject, map[string]any{"profileVisibility": visibility}); err != nil {
			t.Fatalf("setting visibility: %v", err)
		}

		// A stranger shares no club, so the model's answer depends only on the
		// level - and the SQL has to give the same one.
		relation := models.ViewerRelation{}
		want := models.CanSeeProfile(visibility, relation)
		if got := found(stranger, false); got != want {
			t.Errorf("visibility %q: SQL found=%t, CanSeeProfile=%t", visibility, got, want)
		}

		// The owner and a superadmin see it at every level.
		if !found(subject, false) {
			t.Errorf("visibility %q: the owner cannot find themselves", visibility)
		}
		if !found(stranger, true) {
			t.Errorf("visibility %q: a superadmin cannot find it", visibility)
		}
	}
}

// TestAnonymityHoldsAcrossQueries is the test the display_name.go rule exists
// for. A member who anonymizes must be their username *everywhere* a name is
// built - and those names are built in a dozen separate queries, any one of
// which could be missed without anything failing to compile.
func TestAnonymityHoldsAcrossQueries(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	users := NewUserStore(pool)
	messages := NewMessageStore(pool)

	watcher := seedUser(t, pool, "watcher.anon@example.test")
	subject := seedUser(t, pool, "subject.anon@example.test")

	real := ProfileFields{
		Name: "Marie", Surname: "Dubois",
		ProfileVisibility: models.ProfileVisibilityPublic,
	}
	if _, err := users.UpdateProfile(ctx, subject, real); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	// The handle follows the name while there is no username.
	before, err := users.FindByID(ctx, subject)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if before.Handle != "marie-dubois" {
		t.Errorf("handle = %q, want it derived from the name", before.Handle)
	}

	// Setting a username moves the handle to it.
	anonymized := real
	anonymized.Username = "ironfish"
	anonymized.Anonymous = true
	if _, err := users.UpdateProfile(ctx, subject, anonymized); err != nil {
		t.Fatalf("UpdateProfile anonymized: %v", err)
	}
	after, err := users.FindByID(ctx, subject)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if after.Handle != "ironfish" {
		t.Errorf("handle = %q, want the username", after.Handle)
	}
	// The real name is still stored - anonymity hides it, it does not destroy
	// it, and turning the option off has to give it back.
	if after.Name != "Marie" || after.Surname != "Dubois" {
		t.Errorf("the real name was lost: %q %q", after.Name, after.Surname)
	}

	// Every query that names somebody now has to say "ironfish".
	conversationID, err := messages.FindOrCreateConversation(ctx, watcher, subject)
	if err != nil {
		t.Fatalf("FindOrCreateConversation: %v", err)
	}
	if _, err := messages.Send(ctx, conversationID, subject, "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	conversations, err := messages.ListConversations(ctx, watcher)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(conversations) != 1 {
		t.Fatalf("expected one conversation, got %d", len(conversations))
	}
	if got := conversations[0].Other; got.Name != "ironfish" || got.Surname != "" {
		t.Errorf("conversation shows %q %q, want the username alone", got.Name, got.Surname)
	}

	sent, err := messages.ListMessages(ctx, conversationID, 0)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(sent) != 1 || sent[0].Sender.Name != "ironfish" || sent[0].Sender.Surname != "" {
		t.Errorf("message sender shows %+v, want the username alone", sent[0].Sender)
	}

	// Searching the real name must not find them: a search that matches on a
	// hidden name reveals who the username belongs to just as surely as
	// printing it would.
	for _, criteria := range []MemberSearch{
		{Terms: "Marie"},
		{Terms: "Dubois"},
		{Name: "Marie"},
		{Surname: "Dubois"},
		{Terms: "subject.anon@example.test"},
		{Email: "subject.anon@example.test"},
	} {
		results, _, err := users.SearchMembers(ctx, criteria, watcher, false)
		if err != nil {
			t.Fatalf("SearchMembers %+v: %v", criteria, err)
		}
		for _, user := range results {
			if user.ID == subject {
				t.Errorf("SearchMembers %+v found an anonymized member by their real identity", criteria)
			}
		}
	}

	// ...but the username still finds them, or nobody could be reached at all.
	found, _, err := users.SearchMembers(ctx, MemberSearch{Terms: "ironfish"}, watcher, false)
	if err != nil {
		t.Fatalf("SearchMembers by username: %v", err)
	}
	var seen bool
	for _, user := range found {
		if user.ID == subject {
			seen = true
			// The store returns the true record - hiding a name is the job of
			// the projection, so that is what is asserted here.
			if name, surname := user.DisplayName(); name != "ironfish" || surname != "" {
				t.Errorf("DisplayName = %q %q, want the username alone", name, surname)
			}
		}
	}
	if !seen {
		t.Error("an anonymized member cannot be found by their username")
	}

	// The club autocomplete follows the same rule.
	byEmail, err := users.SearchByEmail(ctx, "subject.anon@example.test", 10)
	if err != nil {
		t.Fatalf("SearchByEmail: %v", err)
	}
	for _, user := range byEmail {
		if user.ID == subject {
			t.Error("SearchByEmail found an anonymized member by their address")
		}
	}
}

// TestUsernameIsUnique covers the constraint the handle now depends on.
func TestUsernameIsUnique(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	users := NewUserStore(pool)
	first := seedUser(t, pool, "first.username@example.test")
	second := seedUser(t, pool, "second.username@example.test")

	fields := ProfileFields{Name: "A", Surname: "B", Username: "taken",
		ProfileVisibility: models.ProfileVisibilityPublic}
	if _, err := users.UpdateProfile(ctx, first, fields); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	// Case-insensitively taken: two names that read the same to a person must
	// not both exist.
	for _, candidate := range []string{"taken", "TAKEN", "Taken"} {
		taken, err := users.UsernameTaken(ctx, candidate, second)
		if err != nil {
			t.Fatalf("UsernameTaken(%q): %v", candidate, err)
		}
		if !taken {
			t.Errorf("UsernameTaken(%q) = false, want true", candidate)
		}
	}

	// Its own owner is not blocked by it, or nobody could ever re-save.
	taken, err := users.UsernameTaken(ctx, "taken", first)
	if err != nil {
		t.Fatalf("UsernameTaken for the owner: %v", err)
	}
	if taken {
		t.Error("an account is blocked by its own username")
	}
}
