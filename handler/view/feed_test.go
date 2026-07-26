package view

import (
	"io"
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
)

func setupFeedTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Article{}, &model.ArticleDetail{}); err != nil {
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

func TestRSSAndSitemapOnlyIncludeDisplayableArticles(t *testing.T) {
	setupFeedTestDB(t)
	useViewSettings(t, map[string]string{"BaseURL": "https://blog.example.com"})
	repo := &mysql.ArticleDBRepo{}
	publicArticle := &mysql.ArticleModel{Title: "Public", Slug: "public-post", Type: model.ArticleTypePost, Summary: "public summary", Status: int8(mysql.ARTILCE_STATUS_PUBLIC), PostTime: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC), Detail: &model.ArticleDetail{Content: "public"}}
	draftArticle := &mysql.ArticleModel{Title: "Draft", Status: int8(mysql.ARTICLE_STATUS_DRAFT), PostTime: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC), Detail: &model.ArticleDetail{Content: "draft"}}
	pageArticle := &mysql.ArticleModel{Title: "About", Slug: "about", Type: model.ArticleTypePage, Status: int8(mysql.ARTILCE_STATUS_PUBLIC), PostTime: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC), Detail: &model.ArticleDetail{Content: "about"}}
	futureArticle := &mysql.ArticleModel{Title: "Future", Slug: "future-post", Type: model.ArticleTypePost, Status: int8(mysql.ARTILCE_STATUS_PUBLIC), PostTime: time.Now().Add(24 * time.Hour), Detail: &model.ArticleDetail{Content: "future"}}
	if err := repo.Create(publicArticle); err != nil {
		t.Fatalf("create public article: %v", err)
	}
	if err := repo.Create(draftArticle); err != nil {
		t.Fatalf("create draft article: %v", err)
	}
	if err := repo.Create(pageArticle); err != nil {
		t.Fatalf("create page article: %v", err)
	}
	if err := repo.Create(futureArticle); err != nil {
		t.Fatalf("create future article: %v", err)
	}

	app := fiber.New()
	app.Get("/rss.xml", RSS)
	app.Get("/sitemap.xml", Sitemap)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/rss.xml", nil))
	if err != nil {
		t.Fatalf("rss app.Test() error = %v", err)
	}
	body := readRespBody(t, resp)
	if !strings.Contains(body, "https://blog.example.com/posts/public-post") || strings.Contains(body, "Draft") || strings.Contains(body, "About") || strings.Contains(body, "Future") {
		t.Fatalf("rss body = %s", body)
	}

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/sitemap.xml", nil))
	if err != nil {
		t.Fatalf("sitemap app.Test() error = %v", err)
	}
	body = readRespBody(t, resp)
	for _, path := range []string{"/article/categories", "/tags", "/archive", "/links", "/posts/public-post", "/pages/about"} {
		if !strings.Contains(body, "https://blog.example.com"+path) {
			t.Fatalf("sitemap body missing %q: %s", path, body)
		}
	}
	if strings.Contains(body, "/posts/draft") || strings.Contains(body, "/posts/future-post") {
		t.Fatalf("sitemap body = %s", body)
	}
}

func TestSiteMetadataSettingsFallbackAndFilterLinks(t *testing.T) {
	useViewSettings(t, map[string]string{
		"BaseURL":         "javascript:alert(1)",
		"SiteLogo":        "data:text/plain,bad",
		"SiteSocialLinks": "https://github.com/myecho\nftp://files.example.com\nhttp://example.com/profile",
	})

	app := fiber.New()
	app.Get("/probe", func(c *fiber.Ctx) error {
		if got := siteBaseURL(c); got != "http://fallback.example" {
			t.Errorf("siteBaseURL() = %q, want request origin", got)
		}
		if got := settingAbsoluteURL(c, "SiteLogo"); got != "" {
			t.Errorf("settingAbsoluteURL() = %q, want invalid scheme filtered", got)
		}
		links := siteSocialLinks()
		if len(links) != 2 || links[0] != "https://github.com/myecho" || links[1] != "http://example.com/profile" {
			t.Errorf("siteSocialLinks() = %#v", links)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "http://fallback.example/probe", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
}

func TestIsAllowCommentDefaultsToTrue(t *testing.T) {
	disabled := false
	enabled := true

	if !isAllowComment(nil) {
		t.Fatalf("nil is_allow_comment should default to true")
	}
	if isAllowComment(&disabled) {
		t.Fatalf("false is_allow_comment should close comments")
	}
	if !isAllowComment(&enabled) {
		t.Fatalf("true is_allow_comment should allow comments")
	}
}

func readRespBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(data)
}

func useViewSettings(t *testing.T, values map[string]string) *cache.MysqlSettingMap {
	t.Helper()
	old := config.MySqlSettingModelCache
	settings := &cache.MysqlSettingMap{}
	for key, value := range values {
		settings.Set(key, &mysql.SettingModel{Key: key, Value: value})
	}
	config.MySqlSettingModelCache = settings
	t.Cleanup(func() {
		config.MySqlSettingModelCache = old
	})
	return settings
}
