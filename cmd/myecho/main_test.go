package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSendsStableIdempotencyKeyAndAuthScheme(t *testing.T) {
	temp := t.TempDir()
	contentPath := filepath.Join(temp, "article.md")
	if err := os.WriteFile(contentPath, []byte("# hello\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var bodies []string
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/articles" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer mye_secret" {
			t.Errorf("Authorization = %q, want bearer agent token", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "create-2026-11-07-article-1" {
			t.Errorf("Idempotency-Key = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		bodies = append(bodies, string(body))
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":0,"msg":"ok","data":{"id":42},"meta":{}}`)
	}))
	defer server.Close()
	args := []string{"--url", server.URL, "article", "create", "--file", contentPath, "--title", "Hello", "--idempotency-key", "create-2026-11-07-article-1"}
	for i := 0; i < 2; i++ {
		code, stdout, stderr := runCaptured(args, map[string]string{"MYECHO_TOKEN": "mye_secret"})
		if code != 0 || stderr != "" || !strings.Contains(stdout, `"id":42`) {
			t.Fatalf("run() = %d, stdout %q stderr %q", code, stdout, stderr)
		}
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Fatalf("retry request bodies differ: %q", bodies)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(bodies[0]), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != float64(4) || payload["content"] != "# hello\n" || payload["content_format"] != "markdown" {
		t.Fatalf("create payload = %#v", payload)
	}
}

func TestEditMergesSubmissionSnapshotAndUsesExpectedVersion(t *testing.T) {
	contentPath := filepath.Join(t.TempDir(), "changed.md")
	if err := os.WriteFile(contentPath, []byte("new Markdown"), 0600); err != nil {
		t.Fatal(err)
	}
	var patch map[string]any
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "token user-secret" {
			t.Errorf("Authorization = %q, want legacy token scheme", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			if r.URL.Path != "/api/articles/7" {
				t.Errorf("GET path = %q", r.URL.Path)
			}
			io.WriteString(w, `{"code":0,"data":{"edit_version":17,"title":"Live title","slug":"live-slug","seo_title":"live SEO","seo_description":"live description","share_image":"live.png","type":1,"content_format":"html","summary":"live summary","category_uid":"live-category","is_allow_comment":true,"post_time":"2026-01-02T03:04:05Z","status":1,"visibility":2,"detail":{"content":"live content"},"tags":[{"uid":"live-tag"}],"submission":{"status":1,"snapshot":{"title":"Draft title","slug":"draft-slug","seo_title":"draft SEO","seo_description":"draft description","share_image":"draft.png","type":1,"content_format":"markdown","summary":"draft summary","content":"draft content","category_uid":"draft-category","is_allow_comment":false,"post_time":"2026-02-03T04:05:06Z","target_status":1,"password_hash":"hidden","tag_uids":["draft-tag"],"is_password_protected":true}}},"meta":{}}`)
			return
		}
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&patch); err != nil {
			t.Error(err)
		}
		io.WriteString(w, `{"code":0,"msg":"ok","data":{},"meta":{}}`)
	}))
	defer server.Close()
	code, stdout, stderr := runCaptured([]string{"--url", server.URL, "article", "save", "--id", "7", "--file", contentPath, "--data", `{"seo_title":"patched SEO","expected_edit_version":17}`}, map[string]string{"MYECHO_TOKEN": "user-secret"})
	if code != 0 || stderr != "" || stdout == "" {
		t.Fatalf("run() = %d, stdout %q stderr %q", code, stdout, stderr)
	}
	if patch["edit_version"] != nil || patch["expected_edit_version"] != json.Number("17") || patch["status"] != json.Number("4") {
		t.Fatalf("version/status fields = %#v", patch)
	}
	for key, want := range map[string]any{
		"title": "Draft title", "slug": "draft-slug", "summary": "draft summary", "content": "new Markdown",
		"content_format": "markdown", "category_uid": "draft-category", "seo_title": "patched SEO",
	} {
		if patch[key] != want {
			t.Errorf("patch[%q] = %#v, want %#v", key, patch[key], want)
		}
	}
	tags, ok := patch["tag_uids"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "draft-tag" {
		t.Errorf("tag_uids = %#v, want own draft tag", patch["tag_uids"])
	}
	if patch["is_allow_comment"] != false {
		t.Errorf("is_allow_comment = %#v, want draft snapshot value", patch["is_allow_comment"])
	}
}

func TestSubmitAndPublishUseExplicitStatuses(t *testing.T) {
	for _, test := range []struct {
		command string
		status  json.Number
	}{{"submit", "5"}, {"publish", "1"}} {
		t.Run(test.command, func(t *testing.T) {
			var patched map[string]any
			server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet {
					io.WriteString(w, `{"code":0,"data":{"edit_version":9,"title":"Draft","slug":"draft","type":1,"content_format":"markdown","summary":"","category_uid":"","is_allow_comment":true,"post_time":"2026-03-01T00:00:00Z","status":4,"visibility":2,"detail":{"content":"body"},"tags":[]},"meta":{}}`)
					return
				}
				decoder := json.NewDecoder(r.Body)
				decoder.UseNumber()
				if err := decoder.Decode(&patched); err != nil {
					t.Error(err)
				}
				io.WriteString(w, `{"code":0,"data":{},"meta":{}}`)
			}))
			defer server.Close()
			code, _, stderr := runCaptured([]string{"--url", server.URL, "article", test.command, "--id", "12"}, nil)
			if code != 0 || stderr != "" {
				t.Fatalf("run() = %d, stderr %q", code, stderr)
			}
			if patched["status"] != test.status || patched["expected_edit_version"] != json.Number("9") {
				t.Fatalf("PATCH status/version = %#v/%#v", patched["status"], patched["expected_edit_version"])
			}
		})
	}
}

func TestEditConflictIsNonzeroAndNeverRetried(t *testing.T) {
	contentPath := filepath.Join(t.TempDir(), "article.md")
	if err := os.WriteFile(contentPath, []byte("new body"), 0600); err != nil {
		t.Fatal(err)
	}
	gets, patches := 0, 0
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			gets++
			io.WriteString(w, `{"code":0,"data":{"edit_version":2,"title":"Draft","type":1,"content_format":"markdown","post_time":"2026-03-01T00:00:00Z","detail":{"content":"body"},"tags":[]},"meta":{}}`)
			return
		}
		patches++
		w.WriteHeader(http.StatusConflict)
		io.WriteString(w, `{"code":4090,"msg":"article edit conflict","data":null,"meta":{}}`)
	}))
	defer server.Close()
	code, stdout, stderr := runCaptured([]string{"--url", server.URL, "article", "save", "--id", "1", "--file", contentPath}, nil)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "edit conflict") {
		t.Fatalf("run() = %d, stdout %q stderr %q", code, stdout, stderr)
	}
	if gets != 1 || patches != 1 {
		t.Fatalf("GET/PATCH counts = %d/%d, want 1/1", gets, patches)
	}
}

func TestPinnedStaleEditAbortsAfterGetWithoutPatch(t *testing.T) {
	contentPath := filepath.Join(t.TempDir(), "article.md")
	if err := os.WriteFile(contentPath, []byte("new body"), 0600); err != nil {
		t.Fatal(err)
	}
	gets, patches := 0, 0
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			gets++
			io.WriteString(w, `{"code":0,"data":{"edit_version":8,"title":"Draft","type":1,"content_format":"markdown","post_time":"2026-03-01T00:00:00Z","detail":{"content":"body"},"tags":[]},"meta":{}}`)
			return
		}
		patches++
		io.WriteString(w, `{"code":0,"data":{},"meta":{}}`)
	}))
	defer server.Close()
	code, stdout, stderr := runCaptured([]string{"--url", server.URL, "article", "save", "--id", "1", "--file", contentPath, "--expected-edit-version", "7"}, nil)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "expected 7, found 8") {
		t.Fatalf("run() = %d, stdout %q stderr %q", code, stdout, stderr)
	}
	if gets != 1 || patches != 0 {
		t.Fatalf("GET/PATCH counts = %d/%d, want 1/0", gets, patches)
	}
}

func TestMediaUploadUsesFileFormField(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(filePath, []byte("image-data"), 0600); err != nil {
		t.Fatal(err)
	}
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/mos/files/upload" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "token user-token" {
			t.Errorf("Authorization = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		part, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("file form field: %v", err)
			return
		}
		defer part.Close()
		if header.Filename != "image.png" {
			t.Errorf("filename = %q", header.Filename)
		}
		content, _ := io.ReadAll(part)
		if string(content) != "image-data" {
			t.Errorf("file content = %q", content)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":0,"msg":"ok","data":{"url":"/mos/image.png"},"meta":{}}`)
	}))
	defer server.Close()
	code, stdout, stderr := runCaptured([]string{"--url", server.URL, "media", "upload", "--file", filePath}, map[string]string{"MYECHO_TOKEN": "user-token"})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "/mos/image.png") {
		t.Fatalf("run() = %d, stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestServerErrorsAndNonzeroEnvelopeFailWithoutLeakingToken(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{{"http error", http.StatusInternalServerError, `{"code":5000,"msg":"failed","data":null}`}, {"api code", http.StatusOK, `{"code":4011,"msg":"secret-token was rejected","data":null}`}} {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				io.WriteString(w, test.body)
			}))
			defer server.Close()
			code, stdout, stderr := runCaptured([]string{"--url", server.URL, "article", "get", "--id", "1"}, map[string]string{"MYECHO_TOKEN": "secret-token"})
			if code == 0 || stdout != "" || strings.Contains(stderr, "secret-token") {
				t.Fatalf("run() = %d, stdout %q stderr %q", code, stdout, stderr)
			}
		})
	}
}

func TestRedirectIsNotFollowedWithCredentials(t *testing.T) {
	redirectTargetHits := 0
	target := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHits++
		w.Write([]byte(`{"code":0,"data":null}`))
	}))
	defer target.Close()
	redirect := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token user-secret" {
			t.Errorf("source Authorization = %q", r.Header.Get("Authorization"))
		}
		http.Redirect(w, r, target.URL+"/redirected", http.StatusFound)
	}))
	defer redirect.Close()
	c, err := newClient(redirect.URL, "user-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.request(http.MethodGet, "/api/articles/1", "", "", nil); err == nil {
		t.Fatal("redirect response should fail")
	}
	if redirectTargetHits != 0 {
		t.Fatalf("redirect target received %d requests", redirectTargetHits)
	}
}

func TestTokenCreateUsesScopesCSVAndOptionalExpiry(t *testing.T) {
	var payload map[string]any
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/account/tokens" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "token login-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":0,"data":{"token":"mye_once"},"meta":{}}`)
	}))
	defer server.Close()
	code, stdout, stderr := runCaptured([]string{"--url", server.URL, "token", "create", "--name", "agent", "--scopes", "content:read, content:write", "--expires-in-days", "30"}, map[string]string{"MYECHO_TOKEN": "login-token"})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "mye_once") {
		t.Fatalf("run() = %d, stdout %q stderr %q", code, stdout, stderr)
	}
	if payload["name"] != "agent" || payload["expires_in_days"] != json.Number("30") {
		t.Fatalf("token request = %#v", payload)
	}
	scopes, ok := payload["scopes"].([]any)
	if !ok || len(scopes) != 2 || scopes[0] != "content:read" || scopes[1] != "content:write" {
		t.Fatalf("scopes = %#v", payload["scopes"])
	}
}

func TestArticleListBuildsEscapedFilters(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/all_articles" || r.URL.Query().Get("keyword") != "go & me" ||
			r.URL.Query().Get("category_uid") != "cat a" || r.URL.Query().Get("page") != "3" || r.URL.Query().Get("page_size") != "25" {
			t.Errorf("list path/query = %q?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":0,"data":[],"meta":{"page":3,"page_size":25}}`)
	}))
	defer server.Close()
	code, _, stderr := runCaptured([]string{"--url", server.URL, "article", "list", "--page", "3", "--page-size", "25", "--keyword", "go & me", "--category-uid", "cat a"}, nil)
	if code != 0 || stderr != "" {
		t.Fatalf("list run() = %d, stderr %q", code, stderr)
	}
}

func TestCLIHelpAndFlagErrors(t *testing.T) {
	code, stdout, stderr := runCaptured([]string{"--help"}, nil)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "MYECHO_URL") || !strings.Contains(stdout, "JSON envelopes on stdout") {
		t.Fatalf("global help = %d, stdout %q stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runCaptured([]string{"article", "list", "--help"}, nil)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "-page-size") {
		t.Fatalf("nested help = %d, stdout %q stderr %q", code, stdout, stderr)
	}
	for _, args := range [][]string{
		{"--unknown-global-flag", "article", "list"},
		{"--url", "https://example.invalid", "article", "list", "--unknown-command-flag"},
	} {
		code, stdout, stderr = runCaptured(args, nil)
		if code == 0 || stdout != "" || !strings.Contains(stderr, "flag provided but not defined") {
			t.Fatalf("invalid flags run() = %d, stdout %q stderr %q", code, stdout, stderr)
		}
	}
}

func TestContentExportRefusesOverwriteAndWritesBoundedArchive(t *testing.T) {
	archive := testZIP(t)
	requests := 0
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/api/content/export" || r.URL.Query().Get("include_media") != "true" {
			t.Errorf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer mye_export" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	output := filepath.Join(t.TempDir(), "export.zip")
	args := []string{"--url", server.URL, "content", "export", "--output", output, "--include-media"}
	code, stdout, stderr := runCaptured(args, map[string]string{"MYECHO_TOKEN": "mye_export"})
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"bytes"`) {
		t.Fatalf("export run() = %d, stdout %q stderr %q", code, stdout, stderr)
	}
	stored, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(stored, archive) {
		t.Fatalf("export archive = %d bytes, error %v", len(stored), err)
	}
	if _, err := zip.NewReader(bytes.NewReader(stored), int64(len(stored))); err != nil {
		t.Fatalf("downloaded file is not ZIP: %v", err)
	}
	code, _, stderr = runCaptured(args, map[string]string{"MYECHO_TOKEN": "mye_export"})
	if code == 0 || !strings.Contains(stderr, "already exists") || requests != 1 {
		t.Fatalf("overwrite run() = %d, stderr %q, export requests %d", code, stderr, requests)
	}
}

func TestContentExportRemovesPartialFileOnLimitError(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = io.WriteString(w, "more-than-four")
	}))
	defer server.Close()
	c, err := newClient(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "partial.zip")
	if _, err := c.download("/api/content/export", output, 4); err == nil {
		t.Fatal("oversize archive should fail")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatalf("partial output remains: stat error = %v", err)
	}
}

func TestContentImportDefaultsToPreviewAndAppliesOnlyWithFlag(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "content.zip")
	if err := os.WriteFile(filePath, testZIP(t), 0600); err != nil {
		t.Fatal(err)
	}
	var requests []struct {
		dryRun    string
		authorMap string
	}
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/content/import" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "token user-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("file form field: %v", err)
		} else {
			content, _ := io.ReadAll(file)
			_ = file.Close()
			if header.Filename != "content.zip" || len(content) == 0 {
				t.Errorf("uploaded file name/content = %q/%d", header.Filename, len(content))
			}
		}
		requests = append(requests, struct {
			dryRun    string
			authorMap string
		}{r.URL.Query().Get("dry_run"), r.FormValue("author_map")})
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"data":{"dry_run":true,"applied":false},"meta":{}}`)
	}))
	defer server.Close()
	base := []string{"--url", server.URL, "content", "import", "--file", filePath}
	code, _, stderr := runCaptured(base, map[string]string{"MYECHO_TOKEN": "user-token"})
	if code != 0 || stderr != "" {
		t.Fatalf("preview run() = %d, stderr %q", code, stderr)
	}
	code, _, stderr = runCaptured(append(append([]string{}, base...), "--apply", "--author-map", `{"alice":12}`), map[string]string{"MYECHO_TOKEN": "user-token"})
	if code != 0 || stderr != "" {
		t.Fatalf("apply run() = %d, stderr %q", code, stderr)
	}
	if len(requests) != 2 || requests[0].dryRun != "true" || requests[0].authorMap != "" || requests[1].dryRun != "false" || requests[1].authorMap != `{"alice":12}` {
		t.Fatalf("import requests = %+v", requests)
	}
}

func testZIP(t *testing.T) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create("post/example.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(entry, "---\ntitle: Example\n---\nbody"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func runCaptured(args []string, env map[string]string) (int, string, string) {
	var stdout, stderr strings.Builder
	getenv := func(key string) string { return env[key] }
	code := run(args, &stdout, &stderr, getenv)
	return code, stdout.String(), stderr.String()
}

func newTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &httptest.Server{Config: &http.Server{Handler: handler}, Listener: listener}
	server.Start()
	return server
}
