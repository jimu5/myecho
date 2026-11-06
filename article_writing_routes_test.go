package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	fibercache "github.com/gofiber/fiber/v2/middleware/cache"
	"gorm.io/gorm"
	"myecho/config"
	settingcache "myecho/dal/cache"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
)

func TestArticleWritingRoutesRequireAuthAndPreviewLifecycle(t *testing.T) {
	setupArticleWritingRouteTestDB(t)
	owner := createArticleWritingRouteUser(t)
	article := &mysql.ArticleModel{
		AuthorID:      owner.ID,
		Title:         "Source article",
		Type:          model.ArticleTypePage,
		ContentFormat: model.ArticleContentFormatMarkdown,
		Status:        int8(mysql.ARTICLE_STATUS_DRAFT),
		Password:      "must-not-be-copied",
		Detail:        &model.ArticleDetail{Content: "snapshot body"},
	}
	if err := (&mysql.ArticleDBRepo{}).Create(article); err != nil {
		t.Fatalf("create source article: %v", err)
	}
	revision := model.ArticleRevision{
		ArticleID:     article.ID,
		ActorID:       owner.ID,
		Title:         "Historical title",
		Slug:          "historical-title",
		Type:          model.ArticleTypePage,
		ContentFormat: model.ArticleContentFormatMarkdown,
		Content:       "historical revision body",
	}
	if err := connect.Database.Create(&revision).Error; err != nil {
		t.Fatalf("create article revision: %v", err)
	}

	views := &articleWritingSpyViews{}
	app := fiber.New(fiber.Config{Views: views})
	app.Use(fibercache.New(middleware.CacheConfig))
	SetupApiRouter(app)
	SetupViewRouter(app)
	articlePath := "/api/articles/" + strconv.Itoa(int(article.ID))

	previewResp := articleWritingRouteRequest(t, app, http.MethodPost, articlePath+"/previews", owner.Token,
		`{"expected_edit_version":1}`)
	if previewResp.StatusCode != fiber.StatusOK {
		t.Fatalf("authenticated preview create status = %d, want 200", previewResp.StatusCode)
	}
	var previewEnvelope struct {
		Code int                          `json:"code"`
		Data rtype.ArticlePreviewResponse `json:"data"`
	}
	if err := json.NewDecoder(previewResp.Body).Decode(&previewEnvelope); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	_ = previewResp.Body.Close()
	if previewEnvelope.Data.PreviewURL == "" || !strings.HasPrefix(previewEnvelope.Data.PreviewURL, "/article-preview/") {
		t.Fatalf("preview URL = %q, want article-preview path", previewEnvelope.Data.PreviewURL)
	}

	var articlesBefore int64
	if err := connect.Database.Model(&model.Article{}).Count(&articlesBefore).Error; err != nil {
		t.Fatalf("count articles before anonymous API requests: %v", err)
	}
	var previewsBefore int64
	if err := connect.Database.Model(&model.ArticlePreview{}).Count(&previewsBefore).Error; err != nil {
		t.Fatalf("count previews before anonymous API requests: %v", err)
	}
	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: articlePath + "/duplicate"},
		{method: http.MethodPost, path: articlePath + "/previews", body: `{"expected_edit_version":1}`},
		{method: http.MethodDelete, path: articlePath + "/previews"},
		{method: http.MethodGet, path: articlePath + "/revisions/" + strconv.Itoa(int(revision.ID))},
	} {
		resp := articleWritingRouteRequest(t, app, test.method, test.path, "", test.body)
		if resp.StatusCode != fiber.StatusUnauthorized && resp.StatusCode != fiber.StatusForbidden {
			t.Errorf("anonymous %s %s status = %d, want 401 or 403", test.method, test.path, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}
	var articlesAfter int64
	if err := connect.Database.Model(&model.Article{}).Count(&articlesAfter).Error; err != nil {
		t.Fatalf("count articles after anonymous API requests: %v", err)
	}
	var previewsAfter int64
	if err := connect.Database.Model(&model.ArticlePreview{}).Count(&previewsAfter).Error; err != nil {
		t.Fatalf("count previews after anonymous API requests: %v", err)
	}
	if articlesAfter != articlesBefore || previewsAfter != previewsBefore {
		t.Fatalf("anonymous requests changed business data: articles %d→%d previews %d→%d",
			articlesBefore, articlesAfter, previewsBefore, previewsAfter)
	}

	revisionResp := articleWritingRouteRequest(t, app, http.MethodGet,
		articlePath+"/revisions/"+strconv.Itoa(int(revision.ID)), owner.Token, "")
	if revisionResp.StatusCode != fiber.StatusOK {
		t.Fatalf("authenticated revision detail status = %d, want 200", revisionResp.StatusCode)
	}
	var revisionEnvelope struct {
		Data rtype.ArticleRevisionDetailResponse `json:"data"`
	}
	if err := json.NewDecoder(revisionResp.Body).Decode(&revisionEnvelope); err != nil {
		t.Fatalf("decode revision detail: %v", err)
	}
	_ = revisionResp.Body.Close()
	if revisionEnvelope.Data.Content != "historical revision body" || revisionEnvelope.Data.ArticleID != article.ID {
		t.Fatalf("revision detail = %+v, want historical content for article %d", revisionEnvelope.Data, article.ID)
	}

	duplicateResp := articleWritingRouteRequest(t, app, http.MethodPost, articlePath+"/duplicate", owner.Token, "")
	if duplicateResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("authenticated duplicate status = %d, want 201", duplicateResp.StatusCode)
	}
	var duplicateEnvelope struct {
		Data struct {
			ID                  uint `json:"id"`
			Status              int8 `json:"status"`
			IsPasswordProtected bool `json:"is_password_protected"`
			Detail              *struct {
				Content string `json:"content"`
			} `json:"detail"`
		} `json:"data"`
	}
	if err := json.NewDecoder(duplicateResp.Body).Decode(&duplicateEnvelope); err != nil {
		t.Fatalf("decode duplicate response: %v", err)
	}
	_ = duplicateResp.Body.Close()
	if duplicateEnvelope.Data.ID == 0 || duplicateEnvelope.Data.ID == article.ID ||
		duplicateEnvelope.Data.Status != int8(mysql.ARTICLE_STATUS_DRAFT) ||
		duplicateEnvelope.Data.IsPasswordProtected || duplicateEnvelope.Data.Detail == nil ||
		duplicateEnvelope.Data.Detail.Content != "snapshot body" {
		t.Fatalf("duplicate response = %+v, want a new unlocked draft copied from source", duplicateEnvelope.Data)
	}

	if err := connect.Database.Model(&model.ArticleDetail{}).
		Where("uid = ?", article.DetailUID).
		Update("content", "updated live body").Error; err != nil {
		t.Fatalf("update live article body after preview creation: %v", err)
	}
	previewPath := previewEnvelope.Data.PreviewURL
	for index := 1; index <= 2; index++ {
		resp := articleWritingRouteRequest(t, app, http.MethodGet, previewPath, "", "")
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("read preview response %d: %v", index, err)
		}
		if resp.StatusCode != fiber.StatusOK || !strings.Contains(string(body), "snapshot body") || strings.Contains(string(body), "updated live body") {
			t.Fatalf("preview response %d = %d %q, want frozen snapshot body", index, resp.StatusCode, body)
		}
		if resp.Header.Get("Cache-Control") != "private, no-store" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
			t.Fatalf("preview response headers = Cache-Control %q Referrer-Policy %q",
				resp.Header.Get("Cache-Control"), resp.Header.Get("Referrer-Policy"))
		}
	}
	if views.articleRenders != 2 {
		t.Fatalf("article preview renders = %d, want two fresh renders before revocation", views.articleRenders)
	}

	unauthorizedRevoke := articleWritingRouteRequest(t, app, http.MethodDelete, articlePath+"/previews", "", "")
	if unauthorizedRevoke.StatusCode != fiber.StatusUnauthorized && unauthorizedRevoke.StatusCode != fiber.StatusForbidden {
		t.Fatalf("anonymous preview revoke status = %d, want 401 or 403", unauthorizedRevoke.StatusCode)
	}
	_ = unauthorizedRevoke.Body.Close()
	stillValid := articleWritingRouteRequest(t, app, http.MethodGet, previewPath, "", "")
	if stillValid.StatusCode != fiber.StatusOK {
		t.Fatalf("preview after anonymous revoke status = %d, want 200", stillValid.StatusCode)
	}
	_ = stillValid.Body.Close()

	revokeResp := articleWritingRouteRequest(t, app, http.MethodDelete, articlePath+"/previews", owner.Token, "")
	if revokeResp.StatusCode != fiber.StatusOK {
		t.Fatalf("authenticated preview revoke status = %d, want 200", revokeResp.StatusCode)
	}
	_ = revokeResp.Body.Close()
	afterRevoke := articleWritingRouteRequest(t, app, http.MethodGet, previewPath, "", "")
	if afterRevoke.StatusCode != fiber.StatusNotFound {
		t.Fatalf("preview after revoke status = %d, want 404", afterRevoke.StatusCode)
	}
	_ = afterRevoke.Body.Close()
	if views.articleRenders != 3 {
		t.Fatalf("article preview renders after revoked read = %d, want 3 (404 must not render article)", views.articleRenders)
	}
}

func setupArticleWritingRouteTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "article-writing-routes.db")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open temporary sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Setting{}, &model.Category{}, &model.User{}, &model.Tag{}, &model.ArticleDetail{},
		&model.Comment{}, &model.File{}, &model.Article{}, &model.ArticleReview{}, &model.ArticleSubmission{},
		&model.ArticlePreview{}, &model.ArticleRevision{}, &model.ArticleSlugRedirect{}, &model.ArticleDailyStat{},
		&model.Link{}, &model.Theme{},
	); err != nil {
		t.Fatalf("migrate temporary sqlite: %v", err)
	}
	connect.Database = db
	mysql.InitDB()
	config.MySqlSettingModelCache = settingcache.InitSettingCache()
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func createArticleWritingRouteUser(t *testing.T) *model.User {
	t.Helper()
	user := &model.User{
		Name:           "Route test author",
		Email:          "route-author@example.com",
		Token:          "article-writing-route-token",
		PermissionType: model.Author,
	}
	if err := connect.Database.Select("*").Create(user).Error; err != nil {
		t.Fatalf("create route test author: %v", err)
	}
	return user
}

func articleWritingRouteRequest(t *testing.T, app *fiber.App, method, path, token, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test(%s %s) error = %v", method, path, err)
	}
	return resp
}

type articleWritingSpyViews struct {
	articleRenders int
}

func (v *articleWritingSpyViews) Load() error { return nil }

func (v *articleWritingSpyViews) Render(out io.Writer, name string, binding interface{}, _ ...string) error {
	if name != "article" {
		_, err := io.WriteString(out, name)
		return err
	}
	data, ok := binding.(fiber.Map)
	if !ok {
		return fiber.ErrInternalServerError
	}
	article, ok := data["Data"].(mysql.ArticleModel)
	if !ok {
		return fiber.ErrInternalServerError
	}
	v.articleRenders++
	content := ""
	if article.Detail != nil {
		content = article.Detail.Content
	}
	_, err := io.WriteString(out, article.Title+"\n"+content)
	return err
}
