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

	if _, err := messages.Send(ctx, conversationID, bob, MessageFields{
		Content: "Squat felt like RPE 8.",
	}); err != nil {
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

		{"events.ListVisible", func() error { _, err := events.ListVisible(ctx, nil, time.Now(), "", false); return err }},
		{"events.ListPublic", func() error { _, err := events.ListPublic(ctx, time.Time{}); return err }},

		{"ListFeed", func() error { _, _, err := social.ListFeed(ctx, caller, []string{}, 0, 20); return err }},
		{"ListDiscoverFeed/anonymous", func() error { _, _, err := social.ListDiscoverFeed(ctx, "", 0, 20); return err }},
		{"ListDiscoverFeed", func() error { _, _, err := social.ListDiscoverFeed(ctx, caller, 0, 20); return err }},
		// The shared-link path: a stranger with no session opening a post.
		{"FindPublicPost", func() error {
			_, err := social.FindPublicPost(ctx, "00000000-0000-0000-0000-000000000000")
			if err == ErrNotFound {
				return nil
			}
			return err
		}},
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
	if _, err := messages.Send(ctx, conversationID, subject, MessageFields{Content: "hello"}); err != nil {
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

// TestSearchMatchesUsername covers the criterion added on top of name, surname
// and email.
//
// The username is deliberately matched for an anonymized account too, unlike
// the real name and the email: it is the name they chose to be known by, and
// the handle everybody can already see is derived from it. Hiding it would hide
// the only name such an account has.
func TestSearchMatchesUsername(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	users := NewUserStore(pool)
	caller := seedUser(t, pool, "searcher.sql@example.test")

	// Two accounts with usernames: one open, one anonymized. Both public, so
	// the visibility rules are not what is under test here.
	open := seedUser(t, pool, "open.sql@example.test")
	hidden := seedUser(t, pool, "hidden.sql@example.test")
	if _, err := users.merge(ctx, open, map[string]any{
		"name": "Marie", "surname": "Dubois", "username": "marie.d",
		"handle": "marie-d", "profileVisibility": models.ProfileVisibilityPublic,
	}); err != nil {
		t.Fatalf("seeding the open account: %v", err)
	}
	// The username deliberately shares nothing with the real name: one that
	// contained it ("ironthomas") would match a search for "Thomas" quite
	// correctly - through the username, not through the hidden name - and the
	// test would be unable to tell the two apart.
	if _, err := users.merge(ctx, hidden, map[string]any{
		"name": "Thomas", "surname": "Bernard", "username": "barbell42",
		"handle": "barbell42", "anonymous": true,
		"profileVisibility": models.ProfileVisibilityPublic,
	}); err != nil {
		t.Fatalf("seeding the anonymized account: %v", err)
	}

	finds := func(search MemberSearch, wanted string) bool {
		results, _, err := users.SearchMembers(ctx, search, caller, false)
		if err != nil {
			t.Fatalf("SearchMembers(%+v): %v", search, err)
		}
		for _, user := range results {
			if user.ID == wanted {
				return true
			}
		}
		return false
	}

	cases := []struct {
		name   string
		search MemberSearch
		target string
		want   bool
	}{
		// The dedicated criterion, and the free-text box.
		{"username criterion", MemberSearch{Username: "marie.d"}, open, true},
		{"username partially", MemberSearch{Username: "arie."}, open, true},
		{"terms match the username", MemberSearch{Terms: "marie.d"}, open, true},
		// The point of the new clause: the handle is a slug of the username, so
		// somebody searching what its owner actually typed used to find nothing.
		{"a username the handle does not spell", MemberSearch{Terms: "marie.d"}, open, true},
		{"a different username does not match", MemberSearch{Username: "someone-else"}, open, false},

		// An anonymized account is findable by its username - that is its name.
		{"an anonymized username is searchable", MemberSearch{Username: "barbell42"}, hidden, true},
		{"terms reach an anonymized username", MemberSearch{Terms: "barbell"}, hidden, true},
		// But still not by what it hid.
		{"an anonymized real name stays hidden", MemberSearch{Name: "Thomas"}, hidden, false},
		{"an anonymized email stays hidden", MemberSearch{Email: "hidden.sql"}, hidden, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := finds(c.search, c.target); got != c.want {
				t.Errorf("found = %t, want %t", got, c.want)
			}
		})
	}
}

// TestEventColorRoundTrips writes a colour, reads it back, and checks a junk
// one is dropped. The colour lives in the JSONB payload rather than a column,
// so nothing but a real write and read proves it survives the trip - and it
// ends up in a `style` attribute, which is why the junk case matters.
func TestEventColorRoundTrips(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	events := NewEventStore(pool)
	author := seedUser(t, pool, "colour-round-trip@example.com")

	fields := func(color string) EventFields {
		return EventFields{
			Title: "Regional meet", Kind: "competition", Color: color,
			Visibility: models.EventVisibilityPrivate, StartsAt: time.Now().Add(time.Hour),
		}
	}

	created, err := events.Create(ctx, author, fields("#1CB9F7"))
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	t.Cleanup(func() { _ = events.Delete(context.Background(), created.ID) })

	if created.Color != "#1cb9f7" {
		t.Fatalf("created colour = %q, want %q", created.Color, "#1cb9f7")
	}

	found, err := events.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if found.Color != "#1cb9f7" {
		t.Fatalf("colour after a round trip = %q, want %q", found.Color, "#1cb9f7")
	}

	// A colour that isn't one must not reach the stylesheet the UI builds from
	// it, and an event that loses its colour is still an event.
	updated, err := events.Update(ctx, created.ID, fields("red; background: url(x)"))
	if err != nil {
		t.Fatalf("updating: %v", err)
	}
	if updated.Color != "" {
		t.Fatalf("junk colour survived as %q, want it dropped", updated.Color)
	}
}

// TestClearingOptionalEventFields covers a bug the colour work exposed: Update
// merges the new payload into the old one, so every field marked `omitempty`
// silently kept its previous value. Removing an event's location, its link or
// its end time did nothing at all.
//
// It also checks the event is still listed afterwards. An event with no end
// time used to have no `endsAt` key; it now has an empty one, and the listing
// filter reads coalesce(endsAt, startsAt) - which catches a missing key but
// not an empty string, so without a nullif the cleared event would silently
// drop out of every calendar.
func TestClearingOptionalEventFields(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	events := NewEventStore(pool)
	author := seedUser(t, pool, "clearing-fields@example.com")

	full := EventFields{
		Title: "Regional meet", Kind: "competition", Color: "#1cb9f7",
		Visibility: models.EventVisibilityPrivate, Description: "Bring your singlet",
		Location: "Lyon", URL: "https://example.com",
		StartsAt: time.Now().Add(time.Hour), EndsAt: time.Now().Add(3 * time.Hour),
	}
	created, err := events.Create(ctx, author, full)
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	t.Cleanup(func() { _ = events.Delete(context.Background(), created.ID) })

	cleared := full
	cleared.Description, cleared.Location, cleared.URL, cleared.Color = "", "", "", ""
	cleared.EndsAt = time.Time{}

	got, err := events.Update(ctx, created.ID, cleared)
	if err != nil {
		t.Fatalf("updating: %v", err)
	}
	for _, field := range []struct{ name, value string }{
		{"description", got.Description},
		{"location", got.Location},
		{"url", got.URL},
		{"color", got.Color},
	} {
		if field.value != "" {
			t.Errorf("%s = %q, want it cleared", field.name, field.value)
		}
	}
	if !got.EndsAt.IsZero() {
		t.Errorf("endsAt = %v, want it cleared", got.EndsAt)
	}

	listed, err := events.ListVisible(ctx, nil, time.Now(), author, false)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	found := false
	for _, event := range listed {
		if event.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("an event with no end time dropped out of the listing")
	}
}

// TestPostVisibilityMoves covers moving a post between the public feed and a
// club. It asserts on who can *see* the post afterwards rather than on the
// stored label, because the two are separate: every feed query reads
// readability off the club_id column, and the visibility in the payload is
// only what the UI prints. A change that wrote one without the other would
// leave a post labelled private that everybody could still read.
func TestPostVisibilityMoves(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	social := NewSocialStore(pool)
	clubs := NewClubStore(pool)

	author := seedUser(t, pool, "visibility-author@example.com")
	outsider := seedUser(t, pool, "visibility-outsider@example.com")

	club, err := clubs.Create(ctx, author, ClubFields{Name: "Barbell club"})
	if err != nil {
		t.Fatalf("creating a club: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clubs WHERE id = $1`, club.ID)
	})

	post, err := social.CreatePost(ctx, author, PostFields{
		Content: "Squat session", Visibility: models.VisibilityClub, ClubIDs: []string{club.ID},
	})
	if err != nil {
		t.Fatalf("creating a post: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM posts WHERE id = $1`, post.ID)
	})

	// An outsider has no clubs, so a club post is out of reach.
	seenBy := func() bool {
		visible, err := social.CanSeePost(ctx, post.ID, outsider, []string{})
		if err != nil {
			t.Fatalf("checking visibility: %v", err)
		}
		return visible
	}
	if seenBy() {
		t.Fatal("a club post was readable by somebody outside the club")
	}

	moved, err := social.UpdatePost(ctx, post.ID, author, PostFields{
		Content: "Squat session", Visibility: models.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("moving to public: %v", err)
	}
	if moved.Visibility != models.VisibilityPublic {
		t.Fatalf("visibility = %q, want %q", moved.Visibility, models.VisibilityPublic)
	}
	if !seenBy() {
		t.Fatal("a post moved to the public feed is still hidden: the club_id column did not follow")
	}

	// And back: making it club-only again must actually take it away.
	back, err := social.UpdatePost(ctx, post.ID, author, PostFields{
		Content: "Squat session", Visibility: models.VisibilityClub, ClubIDs: []string{club.ID},
	})
	if err != nil {
		t.Fatalf("moving back to the club: %v", err)
	}
	if back.Visibility != models.VisibilityClub {
		t.Fatalf("visibility = %q, want %q", back.Visibility, models.VisibilityClub)
	}
	if seenBy() {
		t.Fatal("a post made club-only again is still readable by an outsider")
	}
}

// TestEventAllDayRoundTrips checks the flag survives a write and a read, and -
// the part that matters - that turning it off again actually turns it off.
// It lives in the JSONB payload, which Update shallow-merges, so a `false`
// that serialized to nothing would leave the event stuck all-day forever.
func TestEventAllDayRoundTrips(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	events := NewEventStore(pool)
	author := seedUser(t, pool, "all-day@example.com")

	start := time.Now().Add(24 * time.Hour)
	created, err := events.Create(ctx, author, EventFields{
		Title: "Training camp", Kind: "training", AllDay: true,
		Visibility: models.EventVisibilityPrivate,
		StartsAt:   start, EndsAt: start.AddDate(0, 0, 3),
	})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	t.Cleanup(func() { _ = events.Delete(context.Background(), created.ID) })

	if !created.AllDay {
		t.Fatal("allDay did not survive the write")
	}
	if !created.WholeDay() {
		t.Fatal("an all-day event must report itself as whole-day")
	}

	timed, err := events.Update(ctx, created.ID, EventFields{
		Title: "Training camp", Kind: "training", AllDay: false,
		Visibility: models.EventVisibilityPrivate,
		StartsAt:   start, EndsAt: start.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("updating: %v", err)
	}
	if timed.AllDay {
		t.Fatal("an event switched back to a timed one is still marked all-day")
	}
}

// TestPostReachesEveryClubItWasSharedWith is the point of the join table: a
// coach who runs two clubs writes one note for both. It also covers the case
// that used to be a leak - deleting a club must take the post out of *that*
// club and leave it in the others, where the old cascading column deleted the
// post outright.
func TestPostReachesEveryClubItWasSharedWith(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	social := NewSocialStore(pool)
	clubs := NewClubStore(pool)

	author := seedUser(t, pool, "two-clubs-author@example.com")
	memberA := seedUser(t, pool, "two-clubs-a@example.com")
	memberB := seedUser(t, pool, "two-clubs-b@example.com")
	outsider := seedUser(t, pool, "two-clubs-outsider@example.com")

	clubA, err := clubs.Create(ctx, author, ClubFields{Name: "Club A"})
	if err != nil {
		t.Fatalf("creating club A: %v", err)
	}
	clubB, err := clubs.Create(ctx, author, ClubFields{Name: "Club B"})
	if err != nil {
		t.Fatalf("creating club B: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM clubs WHERE id = ANY($1)`,
			[]string{clubA.ID, clubB.ID})
	})

	post, err := social.CreatePost(ctx, author, PostFields{
		Content:    "Deload week for everyone",
		Visibility: models.VisibilityClub,
		ClubIDs:    []string{clubA.ID, clubB.ID},
	})
	if err != nil {
		t.Fatalf("creating the post: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM posts WHERE id = $1`, post.ID)
	})

	if len(post.ClubIDs) != 2 || len(post.ClubNames) != 2 {
		t.Fatalf("post carries %d clubs and %d names, want 2 of each", len(post.ClubIDs), len(post.ClubNames))
	}

	seenBy := func(who string, clubIDs []string) bool {
		visible, err := social.CanSeePost(ctx, post.ID, who, clubIDs)
		if err != nil {
			t.Fatalf("checking visibility: %v", err)
		}
		return visible
	}

	if !seenBy(memberA, []string{clubA.ID}) {
		t.Error("a member of the first club cannot see the post")
	}
	if !seenBy(memberB, []string{clubB.ID}) {
		t.Error("a member of the second club cannot see the post")
	}
	if seenBy(outsider, []string{}) {
		t.Error("somebody in neither club can see a club-only post")
	}

	// Deleting one club must not take the post with it.
	if _, err := pool.Exec(ctx, `DELETE FROM clubs WHERE id = $1`, clubA.ID); err != nil {
		t.Fatalf("deleting club A: %v", err)
	}
	if _, err := social.FindPost(ctx, post.ID, author); err != nil {
		t.Fatalf("the post did not survive its first club being deleted: %v", err)
	}
	if !seenBy(memberB, []string{clubB.ID}) {
		t.Error("the surviving club lost sight of the post")
	}
	// And the one whose club is gone must not inherit it.
	if seenBy(memberA, []string{}) {
		t.Error("a member of the deleted club can still see the post")
	}
	// Above all: losing a club must not turn a club-only post public, which is
	// why readability is decided by the label rather than by having no clubs.
	if seenBy(outsider, []string{}) {
		t.Error("a club-only post became public when one of its clubs was deleted")
	}
}

// TestSpecialtyRoundTripsAndClears covers the badge a member picks for
// themselves, both directions.
//
// The second half is the one worth having: every profile field is written into
// one JSONB payload with a shallow merge, so a field the update omits keeps its
// old value rather than being cleared. A member who picked "squat specialist"
// and later changed their mind would have gone on wearing the badge, with the
// picker showing nothing and the profile showing the old answer.
func TestSpecialtyRoundTripsAndClears(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUserStore(pool)
	id := seedUser(t, pool, "specialty@example.com")

	fields := ProfileFields{
		Name: "Marie", Surname: "Dubois",
		ProfileVisibility: models.ProfileVisibilityPublic,
		Specialty:         models.SpecialtyDeadlift,
	}
	saved, err := users.UpdateProfile(ctx, id, fields)
	if err != nil {
		t.Fatalf("saving a specialty: %v", err)
	}
	if saved.Specialty != models.SpecialtyDeadlift {
		t.Fatalf("the update returned %q, want the deadlift badge", saved.Specialty)
	}

	// Read back rather than trusting what the update returned: the point is
	// what landed in the payload.
	reread, err := users.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("re-reading: %v", err)
	}
	if reread.Specialty != models.SpecialtyDeadlift {
		t.Fatalf("stored specialty = %q, want the deadlift badge", reread.Specialty)
	}

	fields.Specialty = ""
	if _, err := users.UpdateProfile(ctx, id, fields); err != nil {
		t.Fatalf("clearing the specialty: %v", err)
	}
	cleared, err := users.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("re-reading after clearing: %v", err)
	}
	if cleared.Specialty != "" {
		t.Errorf("specialty = %q after being cleared; the merge kept the old value", cleared.Specialty)
	}

	// A badge this app does not know must not reach the profile: the clients
	// look the value up in a translation table and would render the key.
	fields.Specialty = "clean-and-jerk"
	if _, err := users.UpdateProfile(ctx, id, fields); err != nil {
		t.Fatalf("saving an unknown specialty: %v", err)
	}
	unknown, err := users.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("re-reading after an unknown specialty: %v", err)
	}
	if unknown.Specialty != "" {
		t.Errorf("specialty = %q, want an unknown badge normalized away", unknown.Specialty)
	}
}

// TestSocialsRoundTripAndClear covers the accounts a member shows on their
// profile, and above all clearing one.
//
// The profile payload is written with a shallow merge, so the whole socials
// object is replaced on every save. That is what makes deleting one account
// work - and it is also what would silently wipe the other four if the update
// ever stopped sending them, so both directions are pinned here.
func TestSocialsRoundTripAndClear(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUserStore(pool)
	id := seedUser(t, pool, "socials@example.com")

	fields := ProfileFields{
		Name: "Marie", Surname: "Dubois",
		ProfileVisibility: models.ProfileVisibilityPublic,
		Socials: models.Socials{
			// As somebody would actually fill it in: two names typed, one
			// address pasted, and a rank read off a page.
			Instagram:            "@marie.lifts",
			Bluesky:              "marie.bsky.social",
			OpenPowerlifting:     "https://www.openpowerlifting.org/u/mariedubois",
			OpenPowerliftingRank: "12th FR -84kg",
		},
	}
	if _, err := users.UpdateProfile(ctx, id, fields); err != nil {
		t.Fatalf("saving: %v", err)
	}

	saved, err := users.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("re-reading: %v", err)
	}
	if saved.Socials.Instagram != "marie.lifts" {
		t.Errorf("instagram = %q, want the at stripped", saved.Socials.Instagram)
	}
	if saved.Socials.OpenPowerlifting != "mariedubois" {
		t.Errorf("openpowerlifting = %q, want the account out of the pasted URL", saved.Socials.OpenPowerlifting)
	}
	if saved.Socials.OpenPowerliftingRank != "12th FR -84kg" {
		t.Errorf("rank = %q", saved.Socials.OpenPowerliftingRank)
	}
	if saved.Socials.TikTok != "" || saved.Socials.X != "" {
		t.Errorf("networks nobody filled in came back as %q / %q", saved.Socials.TikTok, saved.Socials.X)
	}

	// Deleting one account leaves the others alone.
	fields.Socials.Instagram = ""
	if _, err := users.UpdateProfile(ctx, id, fields); err != nil {
		t.Fatalf("clearing one account: %v", err)
	}
	cleared, err := users.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("re-reading after clearing: %v", err)
	}
	if cleared.Socials.Instagram != "" {
		t.Errorf("instagram = %q after being cleared; the merge kept the old value", cleared.Socials.Instagram)
	}
	if cleared.Socials.Bluesky != "marie.bsky.social" {
		t.Errorf("bluesky = %q; clearing one account took another with it", cleared.Socials.Bluesky)
	}
}
