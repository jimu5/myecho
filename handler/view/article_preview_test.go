package view

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
	"myecho/view_engine"
)

func TestArticlePreviewRendersFrozenSnapshotWithThemeAndReadOnlyHeaders(t *testing.T) {
	setupViewThemeTestDB(t)
	if err := connect.Database.AutoMigrate(&model.User{}, &model.Tag{}, &model.ArticlePreview{}); err != nil {
		t.Fatalf("migrate article previews: %v", err)
	}
	useViewSettings(t, map[string]string{
		"SiteTitle":          "Preview Journal",
		"AnalyticsScriptURL": "https://analytics.example/track.js",
		"AnalyticsSiteID":    "private-site-id",
	})
	activeTheme := createViewTheme(t, "active-preview-test", true, nil)
	activeTheme.CSS = "body { --preview-theme: active; }"
	if err := connect.Database.Model(activeTheme).Update("css", activeTheme.CSS).Error; err != nil {
		t.Fatalf("set active theme CSS: %v", err)
	}
	adminTheme := createViewTheme(t, "editor-preview-test", false, nil)
	adminTheme.CSS = "body { --preview-theme: editor-only; }"
	if err := connect.Database.Model(adminTheme).Update("css", adminTheme.CSS).Error; err != nil {
		t.Fatalf("set editor theme CSS: %v", err)
	}
	adminTheme, err := service.S.Theme.GetThemeByName(adminTheme.Name)
	if err != nil {
		t.Fatalf("reload editor theme: %v", err)
	}
	themeToken, _, err := service.S.Theme.CreatePreviewToken(int64(adminTheme.ID), time.Minute)
	if err != nil {
		t.Fatalf("create editor theme token: %v", err)
	}

	owner := model.User{Name: "Preview Author", NickName: "Draft Author", Email: "private@example.com", PermissionType: model.Author}
	if err := connect.Database.Create(&owner).Error; err != nil {
		t.Fatalf("create preview author: %v", err)
	}
	category := mysql.CategoryModel{Name: "Frozen Category", UID: "frozen-category", Type: model.CategoryTypeArticle}
	if err := connect.Database.Create(&category).Error; err != nil {
		t.Fatalf("create preview category: %v", err)
	}
	tag := model.Tag{Name: "Frozen Tag", UID: "frozen-tag"}
	if err := connect.Database.Create(&tag).Error; err != nil {
		t.Fatalf("create preview tag: %v", err)
	}
	article := mysql.ArticleModel{
		AuthorID: owner.ID, Title: "Live title", CategoryUID: category.UID,
		Status: int8(mysql.ARTILCE_STATUS_PUBLIC), ContentFormat: model.ArticleContentFormatMarkdown,
		Detail: &model.ArticleDetail{Content: "live body"},
	}
	if err := (&mysql.ArticleDBRepo{}).Create(&article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	if err := connect.Database.Model(&article).Association("Tags").Append(&tag); err != nil {
		t.Fatalf("attach live tag: %v", err)
	}
	draftData, err := service.EncodeArticleSubmissionSnapshot(model.ArticleSubmissionSnapshot{
		Title: "Frozen draft title", Slug: "frozen-draft", SEOTitle: "Frozen search title",
		SEODescription: "Frozen description", Type: model.ArticleTypePost,
		ContentFormat: model.ArticleContentFormatMarkdown, Summary: "Frozen summary",
		Content: "# Frozen heading\n\nFrozen paragraph.", CategoryUID: category.UID,
		TagUIDs: []string{tag.UID}, PostTime: time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC),
		PasswordHash: "never-store-this-password-hash",
	})
	if err != nil {
		t.Fatalf("encode draft: %v", err)
	}
	submission := model.ArticleSubmission{ArticleID: article.ID, SubmitterID: owner.ID, Status: model.ArticleSubmissionDraft, Snapshot: draftData}
	if err := connect.Database.Create(&submission).Error; err != nil {
		t.Fatalf("create draft submission: %v", err)
	}
	link, err := service.CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("create preview: %v", err)
	}
	var grant model.ArticlePreview
	digest := sha256.Sum256([]byte(link.Token))
	if err := connect.Database.Where("token_hash = ?", hex.EncodeToString(digest[:])).First(&grant).Error; err != nil {
		t.Fatalf("load preview grant: %v", err)
	}
	var grantSnapshot model.ArticlePreviewSnapshot
	if err := json.Unmarshal([]byte(grant.Snapshot), &grantSnapshot); err != nil {
		t.Fatalf("decode grant snapshot: %v", err)
	}
	if strings.Contains(grant.Snapshot, "never-store-this-password-hash") || strings.Contains(grant.Snapshot, owner.Email) {
		t.Fatalf("grant snapshot contains private data: %s", grant.Snapshot)
	}
	if err := connect.Database.Model(&model.ArticleSubmission{}).Where("id = ?", submission.ID).Update("snapshot", `{"title":"Later draft","content":"changed after link"}`).Error; err != nil {
		t.Fatalf("mutate current draft after grant: %v", err)
	}
	if err := connect.Database.Model(&model.Article{}).Where("id = ?", article.ID).Update("title", "Changed live title").Error; err != nil {
		t.Fatalf("mutate live article after grant: %v", err)
	}

	viewsDir, err := filepath.Abs("../../views")
	if err != nil {
		t.Fatalf("resolve views directory: %v", err)
	}
	app := fiber.New(fiber.Config{Views: view_engine.New(viewsDir, ".jet.html"), PassLocalsToViews: true})
	app.Use(middleware.MWRequestTimeCost)
	app.Get("/article-preview/:token", ArticlePreview)
	req := httptest.NewRequest(fiber.MethodGet, "/article-preview/"+link.Token, nil)
	req.AddCookie(&http.Cookie{Name: service.ThemePreviewCookieName, Value: themeToken})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET preview: %v", err)
	}
	body := readRespBody(t, resp)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("preview status = %d, want 200; body=%s", resp.StatusCode, body)
	}
	for key, want := range map[string]string{
		"Cache-Control":   "private, no-store",
		"X-Robots-Tag":    "noindex, nofollow",
		"Referrer-Policy": "no-referrer",
	} {
		if got := resp.Header.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	for _, directive := range []string{"sandbox", "script-src 'none'", "connect-src 'none'", "form-action 'none'", "frame-ancestors 'self'"} {
		if !strings.Contains(resp.Header.Get("Content-Security-Policy"), directive) {
			t.Errorf("CSP %q missing %q", resp.Header.Get("Content-Security-Policy"), directive)
		}
	}
	for _, want := range []string{
		"<title>", "Frozen search title", "class=\"article-title\"", "Frozen draft title",
		"草稿预览", "此只读分享链接将于", "Frozen heading", "Frozen paragraph.",
		"Frozen Category", "Frozen Tag",
		`href="/static/css/style.css"`, "--preview-theme: active",
		`<meta name="robots" content="noindex,nofollow,noarchive">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview body missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"Changed live title", "Later draft", "changed after link", "live body",
		"never-store-this-password-hash", owner.Email, "editor-only", "analytics.example", "private-site-id",
		"application/ld+json", "article-share-button", "comment_form", "comments-section",
		"<script", "<form",
	} {
		if strings.Contains(body, forbidden) {
			index := strings.Index(body, forbidden)
			start, end := max(0, index-80), min(len(body), index+len(forbidden)+120)
			t.Errorf("preview body contains forbidden/stale content %q near %q", forbidden, body[start:end])
		}
	}
	var reloaded mysql.ArticleModel
	if err := connect.Database.First(&reloaded, article.ID).Error; err != nil {
		t.Fatalf("reload article after preview render: %v", err)
	}
	if reloaded.ReadCount != 0 {
		t.Fatalf("preview read count = %d, want 0", reloaded.ReadCount)
	}
}

func TestArticlePreviewPublicHandlerHidesExpiredRevokedAndTrashedGrants(t *testing.T) {
	setupViewThemeTestDB(t)
	if err := service.S.Theme.InitDefaultTheme(); err != nil {
		t.Fatalf("initialize default theme: %v", err)
	}
	if err := connect.Database.AutoMigrate(&model.User{}, &model.ArticlePreview{}); err != nil {
		t.Fatalf("migrate article previews: %v", err)
	}
	owner := model.User{Name: "Preview owner", PermissionType: model.Admin}
	if err := connect.Database.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	article := mysql.ArticleModel{AuthorID: owner.ID, Title: "Preview", Status: int8(mysql.ARTILCE_STATUS_PUBLIC), Detail: &model.ArticleDetail{Content: "body"}}
	if err := (&mysql.ArticleDBRepo{}).Create(&article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	link, err := service.CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("create preview: %v", err)
	}
	viewsDir, err := filepath.Abs("../../views")
	if err != nil {
		t.Fatalf("resolve views directory: %v", err)
	}
	app := fiber.New(fiber.Config{Views: view_engine.New(viewsDir, ".jet.html"), PassLocalsToViews: true})
	app.Use(middleware.MWRequestTimeCost)
	app.Get("/article-preview/:token", ArticlePreview)
	get := func(token string) *http.Response {
		t.Helper()
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/article-preview/"+token, nil))
		if err != nil {
			t.Fatalf("GET preview: %v", err)
		}
		return resp
	}
	valid := get(link.Token)
	if valid.StatusCode != fiber.StatusOK {
		body := readRespBody(t, valid)
		t.Fatalf("valid grant status = %d, want 200; body=%s", valid.StatusCode, body)
	}
	_ = valid.Body.Close()

	if err := connect.Database.Model(&model.ArticlePreview{}).Where("article_id = ?", article.ID).
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire grant: %v", err)
	}
	var expiredGrant model.ArticlePreview
	if err := connect.Database.Where("article_id = ?", article.ID).First(&expiredGrant).Error; err != nil {
		t.Fatalf("reload expired grant: %v", err)
	}
	if !expiredGrant.ExpiresAt.Before(time.Now()) {
		t.Fatalf("grant expiry = %s, expected a past time", expiredGrant.ExpiresAt)
	}
	if _, err := service.LoadArticlePreview(link.Token); err == nil {
		t.Fatal("LoadArticlePreview accepted an expired grant")
	}
	if resp := get(link.Token); resp.StatusCode != fiber.StatusNotFound {
		_ = resp.Body.Close()
		t.Fatalf("expired grant status = %d, want 404", resp.StatusCode)
	} else {
		_ = resp.Body.Close()
	}

	link, err = service.CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("create revocable grant: %v", err)
	}
	if err := service.RevokeArticlePreviews(article.ID, owner.ID); err != nil {
		t.Fatalf("revoke grant: %v", err)
	}
	if resp := get(link.Token); resp.StatusCode != fiber.StatusNotFound {
		_ = resp.Body.Close()
		t.Fatalf("revoked grant status = %d, want 404", resp.StatusCode)
	} else {
		_ = resp.Body.Close()
	}

	link, err = service.CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("create trash-check grant: %v", err)
	}
	if err := connect.Database.Model(&model.Article{}).Where("id = ?", article.ID).
		UpdateColumn("status", int8(mysql.ARTICLE_STATUS_RECYCLE)).Error; err != nil {
		t.Fatalf("trash article: %v", err)
	}
	if resp := get(link.Token); resp.StatusCode != fiber.StatusNotFound {
		_ = resp.Body.Close()
		t.Fatalf("trashed article grant status = %d, want 404", resp.StatusCode)
	} else {
		_ = resp.Body.Close()
	}
}
