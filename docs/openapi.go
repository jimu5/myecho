package docs

import _ "embed"

// OpenAPI is the canonical OpenAPI 3 specification for agent content APIs.
//
//go:embed openapi.json
var OpenAPI []byte
