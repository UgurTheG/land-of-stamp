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

// Service implements pbconnect.DocsServiceHandler.
type Service struct{}

// GetOpenAPISpec returns the raw OpenAPI YAML spec.
func (s *Service) GetOpenAPISpec(_ context.Context, _ *connect.Request[pb.GetOpenAPISpecRequest]) (*connect.Response[pb.GetOpenAPISpecResponse], error) {
	return connect.NewResponse(&pb.GetOpenAPISpecResponse{
		Content:     specYAML,
		ContentType: "application/yaml",
	}), nil
}

// GetDocsPage returns a self-contained Scalar UI HTML page with the spec inlined.
func (s *Service) GetDocsPage(_ context.Context, _ *connect.Request[pb.GetDocsPageRequest]) (*connect.Response[pb.GetDocsPageResponse], error) {
	return connect.NewResponse(&pb.GetDocsPageResponse{
		Html: buildScalarHTML(specYAML),
	}), nil
}

// buildScalarHTML returns Scalar UI HTML with the OpenAPI spec inlined
// so no additional network request is needed.
func buildScalarHTML(spec string) string {
	// Build a JSON configuration object with the spec content embedded.
	type specObj struct {
		Content string `json:"content"`
	}
	type cfgObj struct {
		Spec specObj `json:"spec"`
	}
	cfg, err := json.Marshal(cfgObj{Spec: specObj{Content: spec}})
	if err != nil {
		cfg = []byte("{}")
	}
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
