// Package docs serves a Scalar-powered OpenAPI documentation UI.
// The OpenAPI 3.1 spec is embedded at compile time and served alongside
// the Scalar CDN-based viewer.
package docs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var specYAML []byte

// Register mounts the API documentation endpoints on the provided mux:
//   - GET /api/docs          → Scalar UI
//   - GET /api/docs/openapi.yaml → raw OpenAPI spec
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(specYAML)
	})

	mux.HandleFunc("GET /api/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(scalarHTML))
	})
}

const scalarHTML = `<!doctype html>
<html>
<head>
  <title>Länd of Stamp – API Docs</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <style>body { margin: 0; }</style>
</head>
<body>
  <script id="api-reference" data-url="/api/docs/openapi.yaml"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>
`

