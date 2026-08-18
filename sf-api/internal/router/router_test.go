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
		User:     &handlers.UserHandler{},
		MFA:      &handlers.MFAHandler{},
		OIDC:     &handlers.OIDCHandler{},
		Club:     &handlers.ClubHandler{},
		Exercise: &handlers.ExerciseHandler{},
		Program:  &handlers.ProgramHandler{},
		Training: &handlers.TrainingHandler{},
		Social:   &handlers.SocialHandler{},
		Profile:  &handlers.ProfileHandler{},
		Admin:    &handlers.AdminHandler{},
		Config:   &handlers.ConfigHandler{},
		Contact:  &handlers.ContactHandler{},
		ApiKey:   &handlers.ApiKeyHandler{},
	}, nil, nil, Options{JWTSecret: "test"})
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
	{"GET", "/v1/public/programs/prog-1"},

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
	{"GET", "/v1/clubs/club-1/members"},
	{"POST", "/v1/clubs/club-1/members"},
	{"DELETE", "/v1/clubs/club-1/members/me"},
	{"PUT", "/v1/clubs/club-1/members/user-1"},
	{"DELETE", "/v1/clubs/club-1/members/user-1"},

	// The pair that regressed: a leaf handler and a subrouter on the same path.
	{"GET", "/v1/clubs/club-1/programs"},
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

	{"POST", "/v1/reports"},

	{"GET", "/v1/admin/stats"},
	{"GET", "/v1/admin/clubs"},
	{"GET", "/v1/admin/reports"},
	{"PUT", "/v1/admin/reports/rep-1"},
	{"GET", "/v1/admin/users"},
	{"PUT", "/v1/admin/users/user-1"},
	{"DELETE", "/v1/admin/users/user-1"},
	{"DELETE", "/v1/admin/users/user-1/mfa"},
}

// TestEveryRouteResolves asserts each documented endpoint reaches a handler.
//
// It uses chi's Match rather than serving a request, so routing is checked
// without running any handler body - the handlers here hold nil stores, and a
// route that resolves would otherwise fail by panicking rather than by
// reporting the thing under test.
func TestEveryRouteResolves(t *testing.T) {
	router := newTestRouter().(chi.Routes)

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if !router.Match(chi.NewRouteContext(), route.method, route.path) {
				t.Error("route does not resolve - a subrouter may be shadowing it")
			}
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
