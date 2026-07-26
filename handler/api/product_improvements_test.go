package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/middleware"
	"myecho/model"
)

func TestSetupFlow_BitsUT(t *testing.T) {
	setupAPITestDB(t)
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Get("/setup/status", SetupStatus)
	app.Post("/setup", Setup)

	resp := doJSONRequest(t, app, fiber.MethodGet, "/setup/status", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("initial setup status = %d, want 200", resp.StatusCode)
	}
	var statusData struct {
		NeedsSetup bool `json:"needs_setup"`
	}
	wrapped := decodeAPIResp(t, resp)
	if err := json.Unmarshal(wrapped.Data, &statusData); err != nil {
		t.Fatalf("decode initial setup status: %v", err)
	}
	if !statusData.NeedsSetup {
		t.Fatal("initial setup status should require setup")
	}

	shortPasswordBody := `{"name":"admin","email":"admin@example.com","password":"1234567","site_title":"My Echo","site_description":"Personal notes"}`
	resp = doJSONRequest(t, app, fiber.MethodPost, "/setup", "", shortPasswordBody)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("short password setup status = %d, want 403", resp.StatusCode)
	}
	var userCount int64
	if err := connect.Database.Model(&model.User{}).Count(&userCount).Error; err != nil {
		t.Fatalf("count users after invalid setup: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("user count after invalid setup = %d, want 0", userCount)
	}

	validBody := `{"name":"admin","email":"admin@example.com","password":"password123","site_title":"My Echo","site_description":"Personal notes"}`
	resp = doJSONRequest(t, app, fiber.MethodPost, "/setup", "", validBody)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("valid setup status = %d, want 200", resp.StatusCode)
	}
	var setupData setupResponse
	wrapped = decodeAPIResp(t, resp)
	if err := json.Unmarshal(wrapped.Data, &setupData); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}
	if setupData.NeedsSetup {
		t.Fatal("successful setup still reports setup required")
	}
	if setupData.User.Name != "admin" || setupData.User.Email != "admin@example.com" || setupData.User.PermissionType != int8(model.Admin) || setupData.User.Token == "" {
		t.Fatalf("setup user = %+v", setupData.User)
	}

	var user model.User
	if err := connect.Database.Where("name = ?", "admin").First(&user).Error; err != nil {
		t.Fatalf("find setup user: %v", err)
	}
	if ok, shouldUpgrade := CheckPassword(user.Password, "password123"); !ok || shouldUpgrade {
		t.Fatalf("stored password check = ok:%v shouldUpgrade:%v", ok, shouldUpgrade)
	}
	for key, want := range map[string]string{
		"SiteTitle":       "My Echo",
		"SiteDescription": "Personal notes",
	} {
		var setting mysql.SettingModel
		if err := connect.Database.Where("key = ?", key).First(&setting).Error; err != nil {
			t.Fatalf("find %s setting: %v", key, err)
		}
		if setting.Value != want || !setting.IsSystem {
			t.Fatalf("%s setting = %+v, want value %q and system setting", key, setting, want)
		}
	}

	resp = doJSONRequest(t, app, fiber.MethodGet, "/setup/status", "", "")
	wrapped = decodeAPIResp(t, resp)
	if err := json.Unmarshal(wrapped.Data, &statusData); err != nil {
		t.Fatalf("decode completed setup status: %v", err)
	}
	if statusData.NeedsSetup {
		t.Fatal("completed setup status should not require setup")
	}

	secondBody := `{"name":"other","email":"other@example.com","password":"password456","site_title":"Changed","site_description":"Changed"}`
	resp = doJSONRequest(t, app, fiber.MethodPost, "/setup", "", secondBody)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("second setup status = %d, want 403", resp.StatusCode)
	}
	if err := connect.Database.Model(&model.User{}).Count(&userCount).Error; err != nil {
		t.Fatalf("count users after second setup: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("user count after second setup = %d, want 1", userCount)
	}
	var siteTitle mysql.SettingModel
	if err := connect.Database.Where("key = ?", "SiteTitle").First(&siteTitle).Error; err != nil {
		t.Fatalf("find site title after second setup: %v", err)
	}
	if siteTitle.Value != "My Echo" {
		t.Fatalf("site title after second setup = %q, want %q", siteTitle.Value, "My Echo")
	}
}

func TestDashboard_BitsUT(t *testing.T) {
	setupAPITestDB(t)
	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	articles := []model.Article{
		{
			BaseModel:    model.BaseModel{CreatedAt: older},
			Title:        "Popular",
			Slug:         "popular",
			Type:         model.ArticleTypePost,
			Status:       int8(mysql.ARTILCE_STATUS_PUBLIC),
			ReadCount:    100,
			CommentCount: 3,
			PostTime:     older,
		},
		{
			BaseModel: model.BaseModel{CreatedAt: newer},
			Title:     "Recent draft",
			Slug:      "recent-draft",
			Type:      model.ArticleTypePost,
			Status:    int8(mysql.ARTICLE_STATUS_DRAFT),
			ReadCount: 1,
			PostTime:  newer,
		},
	}
	if err := connect.Database.Create(&articles).Error; err != nil {
		t.Fatalf("create dashboard articles: %v", err)
	}
	pending := int8(model.CommentStatusPending)
	approved := int8(model.CommentStatusApproved)
	if err := connect.Database.Create([]model.Comment{
		{ArticleUID: "popular", AuthorName: "pending", Content: "pending", Status: &pending},
		{ArticleUID: "popular", AuthorName: "approved", Content: "approved", Status: &approved},
	}).Error; err != nil {
		t.Fatalf("create dashboard comments: %v", err)
	}

	app := fiber.New()
	app.Get("/dashboard", Dashboard)
	resp := doJSONRequest(t, app, fiber.MethodGet, "/dashboard", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("dashboard status = %d, want 200", resp.StatusCode)
	}
	wrapped := decodeAPIResp(t, resp)
	var got dashboardResponse
	if err := json.Unmarshal(wrapped.Data, &got); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if got.ArticleCount != 2 || got.DraftCount != 1 || got.PendingCommentCount != 1 {
		t.Fatalf("dashboard counts = articles:%d drafts:%d pending:%d", got.ArticleCount, got.DraftCount, got.PendingCommentCount)
	}
	if len(got.PopularArticles) != 2 || got.PopularArticles[0].Title != "Popular" {
		t.Fatalf("popular articles = %+v", got.PopularArticles)
	}
	if len(got.RecentArticles) != 2 || got.RecentArticles[0].Title != "Recent draft" {
		t.Fatalf("recent articles = %+v", got.RecentArticles)
	}
}

func TestExport_BitsUT(t *testing.T) {
	setupAPITestDB(t)
	chdirToTemp(t)
	app := fiber.New()
	app.Get("/export", Export)

	resp := doJSONRequest(t, app, fiber.MethodGet, "/export", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("export status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get(fiber.HeaderContentType) != "application/zip" {
		t.Fatalf("export content type = %q", resp.Header.Get(fiber.HeaderContentType))
	}
	if !strings.Contains(resp.Header.Get(fiber.HeaderContentDisposition), "myecho-export-") {
		t.Fatalf("export content disposition = %q", resp.Header.Get(fiber.HeaderContentDisposition))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read export body: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open export zip: %v", err)
	}
	if len(reader.File) == 0 || reader.File[0].Name != "data.json" {
		t.Fatalf("export entries = %+v", reader.File)
	}
}

func TestExportArchiveFileClose_BitsUT(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "export-*.zip")
	if err != nil {
		t.Fatalf("create export file: %v", err)
	}
	path := file.Name()
	stream := &exportArchiveFile{File: file, path: path}

	if err := stream.Close(); err != nil {
		t.Fatalf("close export stream: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("export file still exists after close: %v", err)
	}
}
