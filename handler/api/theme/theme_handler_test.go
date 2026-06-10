package theme

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type themeTestResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func setupThemeHandlerTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Theme{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	connect.Database = db
	mysql.InitDB()
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func doThemeJSONRequest(t *testing.T, app *fiber.App, method, target, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test(%s %s) error = %v", method, target, err)
	}
	return resp
}

func TestThemeHandlersCRUDAndConfig(t *testing.T) {
	setupThemeHandlerTestDB(t)
	if err := service.S.Theme.InitDefaultTheme(); err != nil {
		t.Fatalf("InitDefaultTheme() error = %v", err)
	}
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/api/themes", CreateTheme)
	app.Get("/api/themes", GetAllThemes)
	app.Get("/api/themes/active", GetActiveTheme)
	app.Get("/api/themes/:id", GetTheme)
	app.Patch("/api/themes/:id", UpdateTheme)
	app.Patch("/api/themes/:id/config", UpdateThemeConfig)
	app.Post("/api/themes/:id/activate", ActivateTheme)
	app.Delete("/api/themes/:id", DeleteTheme)

	resp := doThemeJSONRequest(t, app, fiber.MethodPost, "/api/themes", `{"name":"custom","display_name":"Custom"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("create status = %d, want 200", resp.StatusCode)
	}
	resp = doThemeJSONRequest(t, app, fiber.MethodGet, "/api/themes", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("list status = %d, want 200", resp.StatusCode)
	}
	resp = doThemeJSONRequest(t, app, fiber.MethodGet, "/api/themes/2", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("get status = %d, want 200", resp.StatusCode)
	}
	resp = doThemeJSONRequest(t, app, fiber.MethodPatch, "/api/themes/2", `{"display_name":"Updated","author":"Codex","config":{"size":"large"}}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("update status = %d, want 200", resp.StatusCode)
	}
	resp = doThemeJSONRequest(t, app, fiber.MethodPatch, "/api/themes/2/config", `{"color":"blue"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("config update status = %d, want 200", resp.StatusCode)
	}
	resp = doThemeJSONRequest(t, app, fiber.MethodPost, "/api/themes/2/activate", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("activate status = %d, want 200", resp.StatusCode)
	}
	resp = doThemeJSONRequest(t, app, fiber.MethodGet, "/api/themes/active", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("active status = %d, want 200", resp.StatusCode)
	}

	resp = doThemeJSONRequest(t, app, fiber.MethodPost, "/api/themes", `{"name":"spare","display_name":"Spare"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("spare create status = %d, want 200", resp.StatusCode)
	}
	resp = doThemeJSONRequest(t, app, fiber.MethodDelete, "/api/themes/3", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("delete status = %d, want 200", resp.StatusCode)
	}
	resp = doThemeJSONRequest(t, app, fiber.MethodGet, "/api/themes/bad", "")
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("bad id status = %d, want 400", resp.StatusCode)
	}
}

func TestUploadThemeHandler(t *testing.T) {
	setupThemeHandlerTestDB(t)
	chdirToTemp(t)
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/api/themes/upload", UploadTheme)

	resp := postThemeUpload(t, app, "theme.txt", []byte("not zip"))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("non zip status = %d, want 400", resp.StatusCode)
	}
	zipBytes := buildThemeZip(t)
	resp = postThemeUpload(t, app, "theme.zip", zipBytes)
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("zip status = %d, want 200 body=%s", resp.StatusCode, body)
	}
	var wrapped themeTestResp
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(wrapped.Data, &data); err != nil {
		t.Fatalf("decode upload data: %v", err)
	}
	if data["name"] != "upload_theme" || data["preview"] != "/themes/upload_theme/preview.png" {
		t.Fatalf("upload data = %+v", data)
	}
}

func chdirToTemp(t *testing.T) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
}

func postThemeUpload(t *testing.T, app *fiber.App, filename string, content []byte) *http.Response {
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
	req := httptest.NewRequest(fiber.MethodPost, "/api/themes/upload", body)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("upload app.Test() error = %v", err)
	}
	return resp
}

func buildThemeZip(t *testing.T) []byte {
	t.Helper()
	body := &bytes.Buffer{}
	writer := zip.NewWriter(body)
	files := map[string]string{
		"theme/theme.json":  `{"name":"upload_theme","display_name":"Upload","author":"Myecho","version":"1.0.0","preview":"preview.png","css":"style.css","js":"script.js","config":{"layout":"wide"}}`,
		"theme/style.css":   "body { color: red; }",
		"theme/script.js":   "window.theme = true;",
		"theme/preview.png": "png",
	}
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip file %s: %v", name, err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write zip file %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return body.Bytes()
}
