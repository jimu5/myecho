package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
)

func buildAPIContentZip(t *testing.T, name, body string) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatalf("create ZIP entry: %v", err)
	}
	if _, err := entry.Write([]byte(body)); err != nil {
		t.Fatalf("write ZIP entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return archive.Bytes()
}

type contentImportHTTPResponse struct {
	statusCode int
	body       []byte
}

func postContentImport(t *testing.T, app *fiber.App, archive []byte, query string) contentImportHTTPResponse {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "content.zip")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(archive); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(fiber.MethodPost, "/content-import"+query, body)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("ContentImport() request error = %v", err)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read ContentImport() response: %v", err)
	}
	_ = resp.Body.Close()
	return contentImportHTTPResponse{statusCode: resp.StatusCode, body: data}
}

func TestContentPortabilityHandlersDefaultDryRunAndApply(t *testing.T) {
	setupAPITestDB(t)
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Get("/content-export", ContentExport)
	app.Post("/content-import", ContentImport)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/content-export", nil))
	if err != nil {
		t.Fatalf("ContentExport() request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK || resp.Header.Get(fiber.HeaderContentType) != "application/zip" {
		t.Fatalf("ContentExport() status/content-type = %d/%q", resp.StatusCode, resp.Header.Get(fiber.HeaderContentType))
	}
	exportBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read ContentExport() body: %v", err)
	}
	if _, err := zip.NewReader(bytes.NewReader(exportBody), int64(len(exportBody))); err != nil {
		t.Fatalf("ContentExport() returned invalid ZIP: %v", err)
	}

	markdown := "---\n" + strings.Join([]string{
		"title: Imported",
		"slug: imported",
		"type: post",
		"status: 4",
		"summary: Preview me",
		"is_allow_comment: true",
		"content_format: markdown",
		"category: Imported",
		"tags: [portable]",
		"post_time: 2026-04-01T00:00:00Z",
		"",
	}, "\n") + "---\n# Imported"
	archive := buildAPIContentZip(t, "post/imported.md", markdown)

	// dry_run defaults to true.
	body := postContentImport(t, app, archive, "")
	if body.statusCode != fiber.StatusOK {
		t.Fatalf("ContentImport(default dry_run) status = %d, body=%s", body.statusCode, body.body)
	}
	var preview struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body.body, &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	var previewResult service.ContentImportResult
	if err := json.Unmarshal(preview.Data, &previewResult); err != nil {
		t.Fatalf("decode preview result: %v", err)
	}
	if !previewResult.DryRun || previewResult.Applied || previewResult.Created != 1 || previewResult.CategoriesCreated != 1 || previewResult.TagsCreated != 1 {
		t.Fatalf("preview result = %+v", previewResult)
	}
	var count int64
	if err := connect.Database.Model(&model.Article{}).Count(&count).Error; err != nil {
		t.Fatalf("count preview articles: %v", err)
	}
	if count != 0 {
		t.Fatalf("dry-run created %d articles", count)
	}

	body = postContentImport(t, app, archive, "?dry_run=false")
	if body.statusCode != fiber.StatusOK {
		t.Fatalf("ContentImport(apply) status = %d, body=%s", body.statusCode, body.body)
	}
	if err := json.Unmarshal(body.body, &preview); err != nil {
		t.Fatalf("decode apply: %v", err)
	}
	var applyResult service.ContentImportResult
	if err := json.Unmarshal(preview.Data, &applyResult); err != nil {
		t.Fatalf("decode apply result: %v", err)
	}
	if applyResult.DryRun || !applyResult.Applied || applyResult.Created != 1 {
		t.Fatalf("apply result = %+v", applyResult)
	}
	if err := connect.Database.Model(&model.Article{}).Count(&count).Error; err != nil {
		t.Fatalf("count applied articles: %v", err)
	}
	if count != 1 {
		t.Fatalf("applied article count = %d, want 1", count)
	}
}

func TestContentImportHandlerRejectsInvalidDryRun(t *testing.T) {
	setupAPITestDB(t)
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/content-import", ContentImport)
	req := httptest.NewRequest(fiber.MethodPost, "/content-import?dry_run=maybe", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("ContentImport(invalid dry_run) request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("ContentImport(invalid dry_run) status = %d", resp.StatusCode)
	}
}

func TestContentImportHandlerReturnsConflictForStaleVersion(t *testing.T) {
	setupAPITestDB(t)
	article := model.Article{
		Title: "Before", Slug: "handler-import-conflict", Type: model.ArticleTypePost,
		ContentFormat: model.ArticleContentFormatMarkdown, Status: 4,
		PostTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Detail:   &model.ArticleDetail{Content: "before body"},
	}
	if err := connect.Database.Create(&article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}
	archive, err := service.CreateContentExportArchive()
	if err != nil {
		t.Fatalf("export archive: %v", err)
	}
	if err := connect.Database.Model(&model.Article{}).Where("id = ?", article.ID).
		UpdateColumn("edit_version", article.EditVersion+1).Error; err != nil {
		t.Fatalf("advance article version: %v", err)
	}

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/content-import", ContentImport)
	response := postContentImport(t, app, archive, "?dry_run=false")
	if response.statusCode != fiber.StatusConflict {
		t.Fatalf("ContentImport(stale version) status = %d, body=%s; want 409", response.statusCode, response.body)
	}
}
