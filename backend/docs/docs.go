// Package docs provides a ConnectRPC DocsService that serves a
// Scalar-powered OpenAPI documentation UI and the raw OpenAPI 3.1 spec.
// The spec is embedded at compile time.
package docs

import (
	"context"
	_ "embed"
	"encoding/json"
	"strings"

	pb "land-of-stamp-backend/gen/pb"

	"connectrpc.com/connect"
)

//go:embed openapi.yaml
var specYAML string

// DocsService implements pbconnect.DocsServiceHandler.
type DocsService struct{}

// GetOpenAPISpec returns the raw OpenAPI YAML spec.
func (s *DocsService) GetOpenAPISpec(_ context.Context, _ *connect.Request[pb.GetOpenAPISpecRequest]) (*connect.Response[pb.GetOpenAPISpecResponse], error) {
	return connect.NewResponse(&pb.GetOpenAPISpecResponse{
		Content:     specYAML,
		ContentType: "application/yaml",
	}), nil
}

// GetDocsPage returns a self-contained Scalar UI HTML page with the spec inlined.
func (s *DocsService) GetDocsPage(_ context.Context, _ *connect.Request[pb.GetDocsPageRequest]) (*connect.Response[pb.GetDocsPageResponse], error) {
	return connect.NewResponse(&pb.GetDocsPageResponse{
		Html: buildScalarHTML(specYAML),
	}), nil
}

// buildScalarHTML returns Scalar UI HTML with the OpenAPI spec inlined
// so no additional network request is needed.
func buildScalarHTML(spec string) string {
	// Build a JSON configuration object with the spec content embedded.
	cfg, _ := json.Marshal(map[string]any{
		"spec": map[string]any{
			"content": spec,
		},
	})
	// HTML-safe: single-quote the attribute value so the JSON double-quotes are fine.
	var b strings.Builder
	b.WriteString(`<!doctype html>
<html>
<head>
  <title>Länd of Stamp – API Docs</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <style>body { margin: 0; }</style>
</head>
<body>
  <script id="api-reference" data-configuration='`)
	b.Write(cfg)
	b.WriteString(`'></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>
`)
	return b.String()
}
