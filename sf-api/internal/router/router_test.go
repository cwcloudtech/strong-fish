package router

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/handlers"
)

// newTestRouter builds the real route tree with zero-valued handlers. Nothing
// here dispatches into a handler body - these tests only ask "does this path
// resolve to something", which is exactly the class of bug that motivated them:
// registering a subrouter at a path that already had a leaf handler silently
// makes the leaf unreachable rather than failing at startup.
func newTestRouter() http.Handler {
	return New(Handlers{
		User:       &handlers.UserHandler{},
		MFA:        &handlers.MFAHandler{},
		OIDC:       &handlers.OIDCHandler{},
		Club:       &handlers.ClubHandler{},
		Exercise:   &handlers.ExerciseHandler{},
		Program:    &handlers.ProgramHandler{},
		Training:   &handlers.TrainingHandler{},
		Social:     &handlers.SocialHandler{},
		Profile:    &handlers.ProfileHandler{},
		Admin:      &handlers.AdminHandler{},
		Config:     &handlers.ConfigHandler{},
		Contact:    &handlers.ContactHandler{},
		ApiKey:     &handlers.ApiKeyHandler{},
		Media:      &handlers.MediaHandler{},
		Event:      &handlers.EventHandler{},
		Calendar:   &handlers.CalendarHandler{},
		Search:     &handlers.SearchHandler{},
		Invitation: &handlers.InvitationHandler{},
		Message:    &handlers.MessageHandler{},
	}, nil, nil, Options{
		JWTSecret: "test",
		// A stub is enough: the tests assert the endpoint is registered, and
		// serving Prometheus output would need a live meter provider.
		MetricsHandler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	})
}

// routes lists every endpoint the API is meant to expose. A missing entry here
// is a route nobody is checking; an entry that doesn't resolve is a routing bug.
var routes = []struct{ method, path string }{
	// The API's own documentation, served at the root.
	{"GET", "/"},
	{"GET", "/openapi.json"},

	{"GET", "/v1/health"},
	{"GET", "/v1/manifest"},
	{"GET", "/v1/config"},
	{"GET", "/v1/assets/logo.png"},
	{"POST", "/v1/contact"},
	{"GET", "/v1/mobile-app"},
	{"GET", "/v1/metrics"},
	{"GET", "/v1/public/programs/prog-1"},
	{"GET", "/v1/media/st-1/obj-1"},
	{"GET", "/v1/public/programs/prog-1/export.pdf"},
	{"GET", "/v1/public/programs/prog-1/export.xlsx"},
	{"GET", "/v1/public/posts/post-1"},
	{"GET", "/v1/calendar/tok-1.ics"},

	{"GET", "/v1/oidc"},
	{"GET", "/v1/oidc/callback"},
	{"GET", "/v1/oidc/google/login"},
	{"GET", "/v1/oidc/google/callback"},

	{"GET", "/v1/user/confirmation"},
	{"POST", "/v1/users"},
	{"POST", "/v1/users/login"},
	{"POST", "/v1/users/forgot-password"},
	{"POST", "/v1/users/reset-password"},
	{"POST", "/v1/users/login/mfa/totp"},
	{"POST", "/v1/users/login/mfa/webauthn/begin"},
	{"POST", "/v1/users/login/mfa/webauthn/finish"},
	{"GET", "/v1/users/me"},
	{"PUT", "/v1/users/me"},
	{"PUT", "/v1/users/me/picture"},
	{"GET", "/v1/users/search"},
	{"GET", "/v1/users/me/mfa"},
	{"POST", "/v1/users/me/mfa/totp/setup"},
	{"POST", "/v1/users/me/mfa/totp/confirm"},
	{"DELETE", "/v1/users/me/mfa/totp"},
	{"POST", "/v1/users/me/mfa/webauthn/begin"},
	{"POST", "/v1/users/me/mfa/webauthn/finish"},
	{"DELETE", "/v1/users/me/mfa/webauthn/cred-1"},
	{"GET", "/v1/users/me/api-keys"},
	{"POST", "/v1/users/me/api-keys"},
	{"DELETE", "/v1/users/me/api-keys/key-1"},
	{"POST", "/v1/users/me/config/file"},
	{"POST", "/v1/users/me/config/qr"},
	{"GET", "/v1/users/me/invitations"},
	{"POST", "/v1/users/me/invitations/inv-1/accept"},
	{"POST", "/v1/users/me/invitations/inv-1/decline"},
	{"GET", "/v1/users/me/storage"},
	{"GET", "/v1/users/me/storage/shares"},
	{"POST", "/v1/users/me/storage/shares"},
	{"DELETE", "/v1/users/me/storage/shares/user-1"},
	{"GET", "/v1/storages"},
	{"GET", "/v1/media/st-1/obj-1/link"},
	{"PUT", "/v1/users/me/storage"},
	{"DELETE", "/v1/users/me/storage"},
	{"GET", "/v1/users/me/calendar-feed"},
	{"POST", "/v1/users/me/calendar-feed/enable"},
	{"POST", "/v1/users/me/calendar-feed/disable"},
	{"POST", "/v1/users/me/calendar-feed/regenerate"},

	{"GET", "/v1/profiles/ada"},
	{"GET", "/v1/profiles/ada/posts"},
	{"GET", "/v1/profiles/ada/follows"},
	{"POST", "/v1/profiles/ada/follow"},
	{"DELETE", "/v1/profiles/ada/follow"},

	{"GET", "/v1/exercises"},
	{"POST", "/v1/exercises"},
	{"PUT", "/v1/exercises/ex-1"},
	{"GET", "/v1/exercises/ex-1/usage"},
	{"DELETE", "/v1/exercises/ex-1"},

	{"GET", "/v1/one-rms"},
	{"PUT", "/v1/one-rms/ex-1"},
	{"DELETE", "/v1/one-rms/ex-1"},

	{"GET", "/v1/clubs"},
	{"POST", "/v1/clubs"},
	{"GET", "/v1/clubs/club-1"},
	{"PUT", "/v1/clubs/club-1"},
	{"DELETE", "/v1/clubs/club-1"},
	{"POST", "/v1/clubs/club-1/transfer"},
	{"GET", "/v1/clubs/club-1/feed"},
	{"GET", "/v1/clubs/club-1/feedback"},
	{"GET", "/v1/clubs/club-1/invitations"},
	{"POST", "/v1/clubs/club-1/invitations"},
	{"DELETE", "/v1/clubs/club-1/invitations/inv-1"},
	{"GET", "/v1/clubs/club-1/members"},
	{"POST", "/v1/clubs/club-1/members"},
	{"DELETE", "/v1/clubs/club-1/members/me"},
	{"PUT", "/v1/clubs/club-1/members/user-1"},
	{"DELETE", "/v1/clubs/club-1/members/user-1"},

	// The pair that regressed: a leaf handler and a subrouter on the same path.
	{"GET", "/v1/clubs/club-1/programs"},
	{"GET", "/v1/programs"},
	{"POST", "/v1/programs"},
	{"POST", "/v1/programs/import"},
	{"GET", "/v1/programs/prog-1"},
	{"PUT", "/v1/programs/prog-1"},
	{"DELETE", "/v1/programs/prog-1"},
	{"GET", "/v1/programs/prog-1/export.pdf"},
	{"GET", "/v1/programs/prog-1/export.xlsx"},
	{"POST", "/v1/programs/prog-1/copy"},
	{"POST", "/v1/programs/prog-1/days"},
	{"PUT", "/v1/programs/prog-1/days/day-1"},
	{"DELETE", "/v1/programs/prog-1/days/day-1"},
	{"POST", "/v1/programs/prog-1/days/day-1/sets"},
	{"PUT", "/v1/programs/prog-1/sets/set-1"},
	{"DELETE", "/v1/programs/prog-1/sets/set-1"},
	{"GET", "/v1/programs/prog-1/assignments"},
	{"POST", "/v1/programs/prog-1/assignments"},
	{"DELETE", "/v1/programs/prog-1/assignments/a-1"},
	{"GET", "/v1/clubs/club-1/programs/prog-1/export.pdf"},
	{"GET", "/v1/clubs/club-1/programs/prog-1/export.xlsx"},
	{"POST", "/v1/clubs/club-1/programs/prog-1/copy"},
	{"POST", "/v1/clubs/club-1/programs"},
	{"POST", "/v1/clubs/club-1/programs/import"},
	{"GET", "/v1/clubs/club-1/programs/prog-1"},
	{"PUT", "/v1/clubs/club-1/programs/prog-1"},
	{"DELETE", "/v1/clubs/club-1/programs/prog-1"},
	{"POST", "/v1/clubs/club-1/programs/prog-1/days"},
	{"PUT", "/v1/clubs/club-1/programs/prog-1/days/day-1"},
	{"DELETE", "/v1/clubs/club-1/programs/prog-1/days/day-1"},
	{"POST", "/v1/clubs/club-1/programs/prog-1/days/day-1/sets"},
	{"PUT", "/v1/clubs/club-1/programs/prog-1/sets/set-1"},
	{"DELETE", "/v1/clubs/club-1/programs/prog-1/sets/set-1"},
	{"GET", "/v1/clubs/club-1/programs/prog-1/assignments"},
	{"POST", "/v1/clubs/club-1/programs/prog-1/assignments"},
	{"DELETE", "/v1/clubs/club-1/programs/prog-1/assignments/as-1"},

	{"GET", "/v1/training"},
	{"GET", "/v1/training/as-1"},
	{"PUT", "/v1/training/as-1/status"},
	{"PUT", "/v1/training/as-1/sets/set-1/log"},
	{"DELETE", "/v1/training/as-1/sets/set-1/log"},
	{"PUT", "/v1/training/as-1/days/day-1/log"},
	{"GET", "/v1/training/as-1/export.pdf"},
	{"GET", "/v1/training/as-1/export.xlsx"},

	{"GET", "/v1/posts"},
	{"GET", "/v1/posts/discover"},
	{"POST", "/v1/posts"},
	{"GET", "/v1/posts/post-1"},
	{"PUT", "/v1/posts/post-1"},
	{"DELETE", "/v1/posts/post-1"},
	{"POST", "/v1/posts/post-1/like"},
	{"DELETE", "/v1/posts/post-1/like"},
	{"GET", "/v1/posts/post-1/comments"},
	{"POST", "/v1/posts/post-1/comments"},
	{"PUT", "/v1/posts/post-1/comments/c-1"},
	{"DELETE", "/v1/posts/post-1/comments/c-1"},

	{"GET", "/v1/messages"},
	{"GET", "/v1/messages/unread"},
	{"GET", "/v1/messages/with/user-1"},
	{"POST", "/v1/messages/with/user-1"},
	{"DELETE", "/v1/messages/msg-1"},

	{"GET", "/v1/blocks"},
	{"POST", "/v1/blocks/user-1"},
	{"DELETE", "/v1/blocks/user-1"},

	{"POST", "/v1/media/videos"},
	{"POST", "/v1/media/audio"},

	{"GET", "/v1/events"},
	{"POST", "/v1/events"},
	{"POST", "/v1/events/import"},
	{"GET", "/v1/events/ev-1"},
	{"GET", "/v1/search/members"},
	{"PUT", "/v1/events/ev-1"},
	{"DELETE", "/v1/events/ev-1"},

	{"POST", "/v1/reports"},

	{"GET", "/v1/admin/stats"},
	{"GET", "/v1/admin/clubs"},
	{"GET", "/v1/admin/reports"},
	{"GET", "/v1/admin/coach-requests"},
	{"PUT", "/v1/admin/coach-requests/user-1"},
	{"PUT", "/v1/admin/reports/rep-1"},
	{"GET", "/v1/admin/users"},
	{"PUT", "/v1/admin/users/user-1"},
	{"DELETE", "/v1/admin/users/user-1"},
	{"DELETE", "/v1/admin/users/user-1/mfa"},
	{"GET", "/v1/admin/users/user-1/ips"},
}

// TestEveryRouteResolves asserts each documented endpoint is registered with
// the method it is documented under.
//
// It walks the tree rather than calling chi's Match, and that difference is the
// whole point. Mounting a subrouter on a path that already has a leaf handler
// silently replaces the leaf with the mount; Match still answers true, because
// the mount is registered for every method, so the endpoint looks fine while
// actually answering 401 or 405 from inside the subrouter. Walk reports what is
// really in the tree, so the shadowed leaf shows up as missing - which is how
// GET /v1/events was found to be broken after /v1/clubs/{id}/programs had
// already been broken the same way.
//
// Nothing is served here: the handlers hold nil stores, so a route that worked
// would fail by panicking rather than by reporting the thing under test.
func TestEveryRouteResolves(t *testing.T) {
	registered := map[string][]string{}
	err := chi.Walk(newTestRouter().(chi.Routes), func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method] = append(registered[method], pattern)
		return nil
	})
	if err != nil {
		t.Fatalf("walking routes: %v", err)
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			for _, pattern := range registered[route.method] {
				if matchesPattern(pattern, route.path) {
					return
				}
			}
			t.Errorf("%s %s is not registered - a subrouter mounted on the same path may have replaced it",
				route.method, route.path)
		})
	}
}

// TestNoRouteIsUntested walks the real route tree and fails on any endpoint
// missing from `routes`, so a newly added handler can't quietly go unchecked.
func TestNoRouteIsUntested(t *testing.T) {
	registered := map[string]bool{}
	for _, route := range routes {
		registered[route.method+" "+route.path] = true
	}

	err := chi.Walk(newTestRouter().(chi.Routes), func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !covered(registered, method, pattern) {
			t.Errorf("%s %s is registered but not covered by the route table", method, pattern)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking routes: %v", err)
	}
}

// covered reports whether one of the tested paths matches a registered pattern.
// Patterns carry placeholders ("/{clubId}") while the tested paths carry
// concrete values, so the two are compared segment by segment.
func covered(registered map[string]bool, method, pattern string) bool {
	for route := range registered {
		testedMethod, testedPath, _ := strings.Cut(route, " ")
		if testedMethod == method && matchesPattern(pattern, testedPath) {
			return true
		}
	}
	return false
}

func matchesPattern(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(patternParts) != len(pathParts) {
		return false
	}
	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") {
			continue
		}
		if part != pathParts[i] {
			return false
		}
	}
	return true
}
