// Package openapi builds an OpenAPI 3.0 document from the live chi router by
// walking its registered routes, rather than from a spec file kept by hand.
// A hand-written spec drifts the moment somebody adds an endpoint and forgets
// it; this one cannot, because there is nowhere for it to drift to.
//
// The router walk gives every route, its path parameters, and what
// authenticates it. The parts it cannot see - the query / header parameters a
// handler reads and the JSON body it decodes - come from a hand-maintained
// route->doc map (handlers.EndpointDocs), the one place where a route's request
// shape is written down. The body schema itself is not hand-written: it is
// reflected from the handler's payload struct (see schemaOf), so it tracks the
// struct automatically and the map only has to name which struct goes with
// which route. Response bodies are still left unmodelled: there is no single
// struct to reflect for the many shapes a handler can answer with.
package openapi

import (
	"net/http"
	"reflect"
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
	Summary     string                `json:"summary,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
}

type Parameter struct {
	Name     string  `json:"name"`
	In       string  `json:"in"`
	Required bool    `json:"required"`
	Schema   *Schema `json:"schema"`
}

// EndpointDoc is the hand-maintained request shape of a route - the parts the
// router walk cannot derive: the JSON body struct, and the query / header
// parameters the handler reads. The body schema is still reflected from the
// struct (see schemaOf), so only the route->doc mapping (handlers.EndpointDocs)
// is maintained by hand.
type EndpointDoc struct {
	Body   reflect.Type
	Query  []Param
	Header []Param
}

// Param is one query or header parameter. Type defaults to "string".
type Param struct {
	Name     string
	Required bool
	Type     string
}

// RequestBody is the application/json body of a write endpoint, its schema
// reflected from the handler's payload struct.
type RequestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

// Schema is a (subset of) JSON Schema: a type, an optional format, object
// properties and array items - enough to describe the request-body structs and
// the primitive query / path parameters.
type Schema struct {
	Type       string             `json:"type,omitempty"`
	Format     string             `json:"format,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Items      *Schema            `json:"items,omitempty"`
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
	"GET /v1/public/posts/{postId}":       true,
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
func Generate(r chi.Router, title, version string, docs map[string]EndpointDoc) Spec {
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
				Schema: &Schema{Type: "string"},
			})
		}

		operation := Operation{
			Summary:   method + " " + route,
			Tags:      []string{tagFor(route)},
			Responses: map[string]Response{"200": {Description: "OK"}},
		}

		// The request shape the router walk can't see: the query / header
		// parameters the handler reads, and the JSON body it decodes. The body
		// schema is reflected from its struct, so it stays correct on its own;
		// only the route->doc map is hand-maintained.
		if doc, ok := docs[method+" "+route]; ok {
			for _, q := range doc.Query {
				params = append(params, Parameter{
					Name: q.Name, In: "query", Required: q.Required,
					Schema: &Schema{Type: paramType(q.Type)},
				})
			}
			for _, hp := range doc.Header {
				params = append(params, Parameter{
					Name: hp.Name, In: "header", Required: hp.Required,
					Schema: &Schema{Type: paramType(hp.Type)},
				})
			}
			if doc.Body != nil {
				operation.RequestBody = &RequestBody{
					Required: true,
					Content:  map[string]MediaType{"application/json": {Schema: schemaOf(doc.Body)}},
				}
			}
		}
		operation.Parameters = params

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

// schemaOf derives a JSON Schema from a Go type (a request payload struct),
// following the same rules encoding/json does: field names come from the `json`
// tag, `json:"-"` fields and unexported fields are dropped, and an anonymous
// (embedded) struct's fields are inlined. Nested structs, slices, maps and
// pointers recurse; a self-referential struct is cut off as a bare object so
// the reflection terminates.
func schemaOf(t reflect.Type) *Schema { return schemaOfSeen(t, map[reflect.Type]bool{}) }

func schemaOfSeen(t reflect.Type, seen map[reflect.Type]bool) *Schema {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.String() {
	case "time.Time":
		return &Schema{Type: "string", Format: "date-time"}
	case "json.RawMessage":
		return &Schema{} // arbitrary JSON
	}
	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 { // []byte -> base64 string in JSON
			return &Schema{Type: "string"}
		}
		return &Schema{Type: "array", Items: schemaOfSeen(t.Elem(), seen)}
	case reflect.Map:
		return &Schema{Type: "object"}
	case reflect.Interface:
		return &Schema{} // any
	case reflect.Struct:
		if seen[t] {
			return &Schema{Type: "object"}
		}
		seen[t] = true
		defer delete(seen, t)
		s := &Schema{Type: "object", Properties: map[string]*Schema{}}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name, skip, inline := jsonFieldName(f)
			if skip || (f.PkgPath != "" && !f.Anonymous) {
				continue
			}
			if inline {
				ft := f.Type
				for ft.Kind() == reflect.Ptr {
					ft = ft.Elem()
				}
				if ft.Kind() == reflect.Struct {
					for k, v := range schemaOfSeen(ft, seen).Properties {
						s.Properties[k] = v
					}
				}
				continue
			}
			s.Properties[name] = schemaOfSeen(f.Type, seen)
		}
		return s
	}
	return &Schema{}
}

// jsonFieldName reads a struct field's json tag: the property name (or the Go
// field name when untagged), whether it is skipped (`json:"-"`), and whether it
// is an untagged embedded struct whose fields should be inlined.
func jsonFieldName(f reflect.StructField) (name string, skip, inline bool) {
	tag := f.Tag.Get("json")
	first := tag
	if i := strings.IndexByte(tag, ','); i >= 0 {
		first = tag[:i]
	}
	if first == "-" {
		return "", true, false
	}
	if first == "" {
		if f.Anonymous {
			return "", false, true
		}
		return f.Name, false, false
	}
	return first, false, false
}

// paramType defaults a query / header parameter's schema type to "string".
func paramType(t string) string {
	if t == "" {
		return "string"
	}
	return t
}
