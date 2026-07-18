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
	repo := &mysql.ArticleDBRepo{}
	publicArticle := &mysql.ArticleModel{Title: "Public", Slug: "public-post", Type: model.ArticleTypePost, Summary: "public summary", Status: int8(mysql.ARTILCE_STATUS_PUBLIC), PostTime: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC), Detail: &model.ArticleDetail{Content: "public"}}
	draftArticle := &mysql.ArticleModel{Title: "Draft", Status: int8(mysql.ARTICLE_STATUS_DRAFT), PostTime: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC), Detail: &model.ArticleDetail{Content: "draft"}}
	pageArticle := &mysql.ArticleModel{Title: "About", Slug: "about", Type: model.ArticleTypePage, Status: int8(mysql.ARTILCE_STATUS_PUBLIC), PostTime: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC), Detail: &model.ArticleDetail{Content: "about"}}
	if err := repo.Create(publicArticle); err != nil {
		t.Fatalf("create public article: %v", err)
	}
	if err := repo.Create(draftArticle); err != nil {
		t.Fatalf("create draft article: %v", err)
	}
	if err := repo.Create(pageArticle); err != nil {
		t.Fatalf("create page article: %v", err)
	}

	app := fiber.New()
	app.Get("/rss.xml", RSS)
	app.Get("/sitemap.xml", Sitemap)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/rss.xml", nil))
	if err != nil {
		t.Fatalf("rss app.Test() error = %v", err)
	}
	body := readRespBody(t, resp)
	if !strings.Contains(body, "Public") || strings.Contains(body, "Draft") || strings.Contains(body, "About") {
		t.Fatalf("rss body = %s", body)
	}

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/sitemap.xml", nil))
	if err != nil {
		t.Fatalf("sitemap app.Test() error = %v", err)
	}
	body = readRespBody(t, resp)
	for _, path := range []string{"/article/categories", "/tags", "/archive", "/links", "/posts/public-post", "/pages/about"} {
		if !strings.Contains(body, path) {
			t.Fatalf("sitemap body missing %q: %s", path, body)
		}
	}
	if strings.Contains(body, "/posts/draft") {
		t.Fatalf("sitemap body = %s", body)
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
