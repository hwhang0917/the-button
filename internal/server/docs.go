package server

import (
	_ "embed"
	"net/http"
)

// The spec is hand-written and embedded; Swagger UI itself comes from a CDN so
// the binary only carries this page and the YAML. Both routes exist only when
// server.show_docs / SHOW_DOCS is on.
//
//go:embed openapi.yml
var openAPISpec []byte

const docsPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>The Button API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({ url: '/api/docs/openapi.yml', dom_id: '#swagger-ui' });
  </script>
</body>
</html>`

func handleDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(docsPage))
}

func handleDocsSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(openAPISpec)
}
