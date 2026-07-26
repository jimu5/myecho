package view

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"myecho/config"
	"myecho/dal/cache"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/service"
)

func TestRespToMapIncludesActiveThemeAndMetaDefaults(t *testing.T) {
	setupViewThemeTestDB(t)
	active := createViewTheme(t, "default", true, map[string]interface{}{"color": "black", "supports_color_mode": true})
	useViewSettings(t, map[string]string{
		"BaseURL":         "https://blog.example.com/",
		"SiteDescription": "A personal blog",
		"SiteLogo":        "/logo.svg",
		"SiteShareImage":  "/share.png",
		"SiteSocialLinks": `["https://github.com/myecho","mailto:ignore@example.com","http://example.com/profile"]`,
	})

	got := respToMap(nil, "payload", PageMeta{Canonical: "https://example.com/posts"})
	if got["Data"] != "payload" {
		t.Fatalf("Data = %v, want payload", got["Data"])
	}
	if got["CurrentYear"] != time.Now().Year() {
		t.Fatalf("CurrentYear = %v, want %d", got["CurrentYear"], time.Now().Year())
	}
	meta := got["Meta"].(PageMeta)
	if meta.OGType != "website" || meta.OGURL != "https://example.com/posts" {
		t.Fatalf("Meta defaults = %+v", meta)
	}
	if meta.Description != "A personal blog" || meta.Image != "https://blog.example.com/share.png" {
		t.Fatalf("Meta setting defaults = %+v", meta)
	}
	if got["SiteLogo"] != "https://blog.example.com/logo.svg" {
		t.Fatalf("SiteLogo = %v", got["SiteLogo"])
	}
	links := got["SiteSocialLinks"].([]string)
	if len(links) != 2 || links[0] != "https://github.com/myecho" || links[1] != "http://example.com/profile" {
		t.Fatalf("SiteSocialLinks = %#v", links)
	}
	theme := got["Theme"].(*mysql.ThemeModel)
	if theme.ID != active.ID || got["ThemeAssetBase"] != "/themes/default/" || got["IsThemePreview"].(bool) {
		t.Fatalf("theme fields = %+v", got)
	}
	config := got["ThemeConfig"].(map[string]interface{})
	if config["color"] != "black" {
		t.Fatalf("ThemeConfig = %+v", config)
	}
	if got["SupportsColorMode"] != true {
		t.Fatalf("SupportsColorMode = %v, want true", got["SupportsColorMode"])
	}
}

func TestRespToMapUsesPreviewThemeCookie(t *testing.T) {
	setupViewThemeTestDB(t)
	createViewTheme(t, "default", true, map[string]interface{}{"color": "black"})
	preview := createViewTheme(t, "preview", false, map[string]interface{}{"color": "blue"})
	token, _, err := service.S.Theme.CreatePreviewToken(int64(preview.ID), time.Minute)
	if err != nil {
		t.Fatalf("CreatePreviewToken() error = %v", err)
	}

	app := fiber.New()
	app.Get("/probe", func(c *fiber.Ctx) error {
		data := respToMap(c, nil)
		theme := data["Theme"].(*mysql.ThemeModel)
		config := data["ThemeConfig"].(map[string]interface{})
		return c.SendString(theme.Name + "|" + data["ThemeAssetBase"].(string) + "|" + config["color"].(string))
	})
	req := httptest.NewRequest(fiber.MethodGet, "/probe", nil)
	req.AddCookie(&http.Cookie{Name: service.ThemePreviewCookieName, Value: token})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	body := readRespBody(t, resp)
	if body != "preview|/themes/preview/|blue" {
		t.Fatalf("preview response = %q", body)
	}
}

func TestInvalidPreviewCookieFallsBackAndExpiresCookie(t *testing.T) {
	setupViewThemeTestDB(t)
	createViewTheme(t, "default", true, nil)
	app := fiber.New()
	app.Get("/probe", func(c *fiber.Ctx) error {
		data := respToMap(c, nil)
		return c.SendString(data["Theme"].(*mysql.ThemeModel).Name)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/probe", nil)
	req.AddCookie(&http.Cookie{Name: service.ThemePreviewCookieName, Value: "invalid"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if body := readRespBody(t, resp); body != "default" {
		t.Fatalf("body = %q, want active theme fallback", body)
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), service.ThemePreviewCookieName+"=;") {
		t.Fatalf("Set-Cookie = %q, want expired preview cookie", resp.Header.Get("Set-Cookie"))
	}
}

func TestThemePreviewHandlersSetAndClearCookie(t *testing.T) {
	setupViewThemeTestDB(t)
	preview := createViewTheme(t, "preview", false, nil)
	token, _, err := service.S.Theme.CreatePreviewToken(int64(preview.ID), time.Minute)
	if err != nil {
		t.Fatalf("CreatePreviewToken() error = %v", err)
	}

	app := fiber.New()
	app.Get("/theme-preview", ThemePreview)
	app.Get("/theme-preview/clear", ClearThemePreview)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/theme-preview", nil))
	if err != nil {
		t.Fatalf("missing token request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("missing token status = %d, want 403", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/theme-preview?token="+token+"&path=%2Farticles%2F1", nil))
	if err != nil {
		t.Fatalf("valid token request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusFound || resp.Header.Get("Location") != "/articles/1" {
		t.Fatalf("preview response status=%d location=%q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), service.ThemePreviewCookieName+"="+token) {
		t.Fatalf("Set-Cookie = %q, want preview token", resp.Header.Get("Set-Cookie"))
	}

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/theme-preview/clear", nil))
	if err != nil {
		t.Fatalf("clear request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("clear status = %d, want 204", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), service.ThemePreviewCookieName+"=;") {
		t.Fatalf("clear Set-Cookie = %q", resp.Header.Get("Set-Cookie"))
	}
}

func setupViewThemeTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Theme{},
		&model.Setting{},
		&model.Category{},
		&model.ArticleDetail{},
		&model.Article{},
		&model.Comment{},
		&model.Link{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	connect.Database = db
	mysql.InitDB()
	config.MySqlSettingModelCache = cache.InitSettingCache()
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func createViewTheme(t *testing.T, name string, active bool, themeConfig map[string]interface{}) *mysql.ThemeModel {
	t.Helper()
	theme := &mysql.ThemeModel{
		Name:        name,
		DisplayName: name,
		IsActive:    active,
	}
	if err := (*model.Theme)(theme).SetConfig(themeConfig); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	if err := service.S.Theme.CreateTheme(theme); err != nil {
		t.Fatalf("CreateTheme(%s) error = %v", name, err)
	}
	return theme
}
