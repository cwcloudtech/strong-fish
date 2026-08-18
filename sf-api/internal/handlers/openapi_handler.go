package handlers

import (
	"net/http"

	"strong-fish-api/internal/openapi"
)

// NewOpenAPIHandler serves the spec built once at startup (see
// openapi.Generate), not regenerated per request - the routes cannot change
// while the process is running.
func NewOpenAPIHandler(spec openapi.Spec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, spec)
	}
}

const swaggerUIPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>strong-fish API</title>
  <link rel="icon" href="/v1/assets/logo.png" />
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "/openapi.json",
        dom_id: "#swagger-ui",
      });
    };
  </script>
</body>
</html>
`

// ServeSwaggerUI serves the API's root: a Swagger UI pointed at
// /openapi.json, so hitting the API in a browser explains itself instead of
// answering 404.
func ServeSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIPage))
}
