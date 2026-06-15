package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"myecho/middleware"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestStaticPageHandlersUploadListAndDelete(t *testing.T) {
	chdirStaticPageHandlerTemp(t)
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Get("/api/static-pages", StaticPageList)
	app.Post("/api/static-pages/upload", UploadStaticPage)
	app.Delete("/api/static-pages/:name", DeleteStaticPage)

	resp := doJSONRequest(t, app, fiber.MethodGet, "/api/static-pages", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("empty list status = %d, want 200", resp.StatusCode)
	}

	req := httptest.NewRequest(fiber.MethodPost, "/api/static-pages/upload", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("missing upload app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("missing upload file status = %d, want 400", resp.StatusCode)
	}

	resp = postStaticPageUpload(t, app, "page.txt", []byte("not zip"))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("non zip status = %d, want 400", resp.StatusCode)
	}

	resp = postStaticPageUpload(t, app, "campaign.zip", buildStaticPageZipBytes(t))
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("zip status = %d, want 200 body=%s", resp.StatusCode, body)
	}
	wrapped := decodeAPIResp(t, resp)
	var page map[string]any
	if err := json.Unmarshal(wrapped.Data, &page); err != nil {
		t.Fatalf("decode static page data: %v", err)
	}
	if page["name"] != "campaign" || page["url"] != "/static-pages/campaign/" {
		t.Fatalf("static page data = %+v", page)
	}

	resp = doJSONRequest(t, app, fiber.MethodGet, "/api/static-pages", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("list status = %d, want 200", resp.StatusCode)
	}
	wrapped = decodeAPIResp(t, resp)
	var pages []map[string]any
	if err := json.Unmarshal(wrapped.Data, &pages); err != nil {
		t.Fatalf("decode static pages data: %v", err)
	}
	if len(pages) != 1 || pages[0]["name"] != "campaign" {
		t.Fatalf("static pages = %+v", pages)
	}

	resp = doJSONRequest(t, app, fiber.MethodDelete, "/api/static-pages/campaign", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("delete status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/api/static-pages", "", "")
	wrapped = decodeAPIResp(t, resp)
	if err := json.Unmarshal(wrapped.Data, &pages); err != nil {
		t.Fatalf("decode static pages after delete: %v", err)
	}
	if len(pages) != 0 {
		t.Fatalf("static pages after delete = %+v, want empty", pages)
	}
}

func postStaticPageUpload(t *testing.T, app *fiber.App, filename string, content []byte) *http.Response {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write upload content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(fiber.MethodPost, "/api/static-pages/upload", body)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("upload app.Test() error = %v", err)
	}
	return resp
}

func buildStaticPageZipBytes(t *testing.T) []byte {
	t.Helper()
	body := &bytes.Buffer{}
	writer := zip.NewWriter(body)
	files := map[string]string{
		"campaign/static-page.json": `{"name":"campaign","display_name":"Campaign","description":"Launch page"}`,
		"campaign/index.html":       "<!doctype html><title>Campaign</title>",
		"campaign/assets/app.css":   "body { color: #111; }",
	}
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return body.Bytes()
}

func chdirStaticPageHandlerTemp(t *testing.T) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}
