// Package openapi builds an OpenAPI 3.0 document from the live chi router by
// walking its registered routes, rather than from a spec file kept by hand.
// A hand-written spec drifts the moment somebody adds an endpoint and forgets
// it; this one cannot, because there is nowhere for it to drift to.
//
// What it describes is deliberately shallow - every route, its path
// parameters, and what authenticates it. Request and response bodies are not
// modelled: chi knows the routes, not the shapes behind them, and inventing
// schemas here would be a second source of truth doing exactly what the spec
// file was rejected for.
package openapi

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"strong-fish-api/internal/utils"
)

type Spec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Paths      map[string]PathItem `json:"paths"`
	Components Components          `json:"components"`
}

type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// PathItem maps a lowercased HTTP method to its operation.
type PathItem map[string]Operation

type Operation struct {
	Summary    string                `json:"summary,omitempty"`
	Tags       []string              `json:"tags,omitempty"`
	Parameters []Parameter           `json:"parameters,omitempty"`
	Responses  map[string]Response   `json:"responses"`
	Security   []map[string][]string `json:"security,omitempty"`
}

type Parameter struct {
	Name     string            `json:"name"`
	In       string            `json:"in"`
	Required bool              `json:"required"`
	Schema   map[string]string `json:"schema"`
}

type Response struct {
	Description string `json:"description"`
}

type Components struct {
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	In           string `json:"in,omitempty"`
	Name         string `json:"name,omitempty"`
}

var pathParam = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

// publicRoutes are the endpoints that answer without credentials: the ones you
// need before you have an account (register, log in, the contact form, where
// to get the mobile app), the OIDC dance, and the things a coach chose to
// share with the world.
//
// The routes are listed rather than detected because chi.Walk hands back
// middleware as anonymous functions, with nothing to compare against. Getting
// an entry wrong only mislabels the padlock in the Swagger UI - the router is
// what actually enforces access - but it should still be kept in step.
var publicRoutes = map[string]bool{
	"GET /v1/health":                      true,
	"GET /v1/manifest":                    true,
	"GET /v1/config":                      true,
	"GET /v1/assets/logo.png":             true,
	"GET /v1/mobile-app":                  true,
	"POST /v1/contact":                    true,
	"GET /v1/public/programs/{programId}": true,
	"GET /v1/calendar/{token}":            true,
	// The calendar is readable logged out; writing it is not.
	"GET /v1/events":                   true,
	"GET /v1/events/{eventId}":         true,
	"GET /v1/oidc/":                    true,
	"GET /v1/oidc/callback":            true,
	"GET /v1/oidc/{provider}/login":    true,
	"GET /v1/oidc/{provider}/callback": true,
	"GET /v1/user/confirmation":        true,
	"POST /v1/users/":                  true,
	"POST /v1/users/login":             true,
	"POST /v1/users/forgot-password":   true,
	"POST /v1/users/reset-password":    true,
	// The second factor of a login in progress: authenticated by the
	// short-lived challenge token in the body, not by a session.
	"POST /v1/users/login/mfa/totp":            true,
	"POST /v1/users/login/mfa/webauthn/begin":  true,
	"POST /v1/users/login/mfa/webauthn/finish": true,
	// Public profiles, readable logged out when their owner opted in.
	"GET /v1/profiles/{handle}":         true,
	"GET /v1/profiles/{handle}/posts":   true,
	"GET /v1/profiles/{handle}/follows": true,
}

// Generate walks every route registered on r and describes it: method, path,
// path parameters, a tag taken from the first segment after /v1, and - for
// everything outside publicRoutes - that either a bearer token or an
// X-Api-Key satisfies it, mirroring middleware.Auth.
func Generate(r chi.Router, title, version string) Spec {
	paths := map[string]PathItem{}

	_ = chi.Walk(r, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		// The documentation endpoints themselves aren't part of the API.
		if route == "/" || route == "/openapi.json" {
			return nil
		}

		item, ok := paths[route]
		if !ok {
			item = PathItem{}
			paths[route] = item
		}

		var params []Parameter
		for _, match := range pathParam.FindAllStringSubmatch(route, -1) {
			params = append(params, Parameter{
				Name: match[1], In: "path", Required: true,
				Schema: map[string]string{"type": "string"},
			})
		}

		operation := Operation{
			Summary:    method + " " + route,
			Tags:       []string{tagFor(route)},
			Parameters: params,
			Responses:  map[string]Response{"200": {Description: "OK"}},
		}
		if !publicRoutes[method+" "+route] {
			operation.Security = []map[string][]string{{"bearerAuth": {}}, {"apiKeyAuth": {}}}
		}

		item[strings.ToLower(method)] = operation
		return nil
	})

	return Spec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       title,
			Version:     version,
			Description: "Generated from the live router: every registered route is reflected here automatically.",
		},
		Paths: paths,
		Components: Components{
			SecuritySchemes: map[string]SecurityScheme{
				"bearerAuth": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
				"apiKeyAuth": {Type: "apiKey", In: "header", Name: "X-Api-Key"},
			},
		},
	}
}

// tagFor groups routes in the Swagger UI sidebar by the first segment after
// the /v1 prefix ("/v1/clubs/{clubId}/programs" -> "clubs").
func tagFor(route string) string {
	segments := strings.Split(strings.Trim(route, "/"), "/")
	if len(segments) >= 2 && segments[0] == "v1" {
		return segments[1]
	}
	if len(segments) >= 1 && utils.IsNotBlank(segments[0]) {
		return segments[0]
	}
	return "default"
}
