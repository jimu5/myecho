package docs

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestAgentOpenAPIIsDiscoverableAndSelfContained(t *testing.T) {
	if !json.Valid(OpenAPI) {
		t.Fatal("embedded OpenAPI document is not valid JSON")
	}
	var spec map[string]any
	if err := json.Unmarshal(OpenAPI, &spec); err != nil {
		t.Fatal(err)
	}
	if spec["openapi"] != "3.1.0" {
		t.Fatalf("openapi version = %v, want 3.1.0", spec["openapi"])
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("paths is missing")
	}
	for path, methods := range map[string][]string{
		"/api/openapi.json":                                  {"get"},
		"/api/account/tokens":                                {"get", "post"},
		"/api/account/tokens/{id}":                           {"delete"},
		"/api/all_articles":                                  {"get"},
		"/api/articles":                                      {"post"},
		"/api/articles/{id}":                                 {"get", "patch", "delete"},
		"/api/articles/{id}/revisions/{revision_id}":         {"get"},
		"/api/articles/{id}/revisions/{revision_id}/restore": {"post"},
		"/api/submissions/{id}/review":                       {"post"},
		"/api/article/categories":                            {"post"},
		"/api/tags":                                          {"post"},
		"/mos/files":                                         {"get"},
		"/mos/files/upload":                                  {"post"},
		"/api/content/export":                                {"get"},
		"/api/content/import":                                {"post"},
	} {
		pathItem, ok := paths[path].(map[string]any)
		if !ok {
			t.Fatalf("path %s is missing", path)
		}
		for _, method := range methods {
			op, ok := pathItem[method].(map[string]any)
			if !ok {
				t.Fatalf("%s %s is missing", strings.ToUpper(method), path)
			}
			if op["operationId"] == nil || op["responses"] == nil {
				t.Fatalf("%s %s lacks operationId or responses", strings.ToUpper(method), path)
			}
		}
	}

	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("components is missing")
	}
	schemes, ok := components["securitySchemes"].(map[string]any)
	if !ok || schemes["LoginToken"] == nil || schemes["AgentToken"] == nil {
		t.Fatal("login and agent security schemes are required")
	}
	checkOpenAPIRefs(t, spec, spec)

	articleCreate := operationAt(t, paths, "/api/articles", "post")
	params, ok := articleCreate["parameters"].([]any)
	if !ok || len(params) != 1 {
		t.Fatal("article create must declare Idempotency-Key")
	}
	param, ok := params[0].(map[string]any)
	if !ok || param["name"] != "Idempotency-Key" || param["required"] != false || param["x-required-for-agent"] != true {
		t.Fatal("article create Idempotency-Key must be optional for login tokens and required for agent credentials")
	}
	if articleCreate["x-agent-scope"] != "articles:write" {
		t.Fatalf("article create agent scope = %v", articleCreate["x-agent-scope"])
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("schemas is missing")
	}
	status, ok := schemas["ArticleStatus"].(map[string]any)
	if !ok || len(status["enum"].([]any)) != 6 {
		t.Fatal("ArticleStatus must document all six article states")
	}
	tokenScope, ok := schemas["TokenScope"].(map[string]any)
	if !ok || len(tokenScope["enum"].([]any)) != 14 {
		t.Fatal("TokenScope must document all supported credential scopes")
	}
}

func operationAt(t *testing.T, paths map[string]any, path, method string) map[string]any {
	t.Helper()
	pathItem, ok := paths[path].(map[string]any)
	if !ok {
		t.Fatalf("path %s is missing", path)
	}
	op, ok := pathItem[method].(map[string]any)
	if !ok {
		t.Fatalf("%s %s is missing", strings.ToUpper(method), path)
	}
	return op
}

func checkOpenAPIRefs(t *testing.T, root, value any) {
	t.Helper()
	switch item := value.(type) {
	case map[string]any:
		if ref, ok := item["$ref"].(string); ok {
			if !strings.HasPrefix(ref, "#/") {
				t.Fatalf("unsupported external reference %q", ref)
			}
			node := root
			for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
				object, ok := node.(map[string]any)
				if !ok {
					t.Fatalf("reference %q does not resolve at %q", ref, part)
				}
				node, ok = object[part]
				if !ok {
					t.Fatalf("reference %q does not resolve at %q", ref, part)
				}
			}
			if node == nil {
				t.Fatalf("reference %q resolves to null", ref)
			}
		}
		for key, child := range item {
			checkOpenAPIRefs(t, root, child)
			_ = key
		}
	case []any:
		for _, child := range item {
			checkOpenAPIRefs(t, root, child)
		}
	case nil, string, float64, bool:
	default:
		t.Fatalf("unexpected OpenAPI JSON value %s (%T)", fmt.Sprint(item), item)
	}
}
