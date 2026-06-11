package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"myecho/config"
	"myecho/config/yaml_config"
	"myecho/dal/cache"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
	"myecho/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type apiTestResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
	Meta map[string]any  `json:"meta"`
}

type apiRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn apiRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupAPITestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Setting{},
		&model.Category{},
		&model.User{},
		&model.Tag{},
		&model.ArticleDetail{},
		&model.Comment{},
		&model.File{},
		&model.Article{},
		&model.Link{},
		&model.Theme{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	connect.Database = db
	mysql.InitDB()
	config.MySqlSettingModelCache = cache.InitSettingCache()
	yaml_config.Yaml.APPConfig = &yaml_config.APPConfig{AllowRegister: true}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func createAPIUser(t *testing.T, name, token string, permission int8) *model.User {
	t.Helper()
	user := &model.User{Name: name, Email: name + "@example.com", Token: token, PermissionType: permission}
	if err := connect.Database.Select("*").Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := connect.Database.Model(user).Update("permission_type", permission).Error; err != nil {
		t.Fatalf("update user permission: %v", err)
	}
	user.PermissionType = permission
	return user
}

func doJSONRequest(t *testing.T, app *fiber.App, method, target, token, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test(%s %s) error = %v", method, target, err)
	}
	return resp
}

func decodeAPIResp(t *testing.T, resp *http.Response) apiTestResp {
	t.Helper()
	var wrapped apiTestResp
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return wrapped
}

func seedArticleCategory(t *testing.T) *mysql.CategoryModel {
	t.Helper()
	category := &mysql.CategoryModel{Name: "Article", UID: "cat-article", Type: model.CategoryTypeArticle}
	if err := connect.Database.Create(category).Error; err != nil {
		t.Fatalf("create article category: %v", err)
	}
	return category
}

func TestArticleHandlersVisibilityAndCRUD(t *testing.T) {
	setupAPITestDB(t)
	admin := createAPIUser(t, "admin", "admin-token", model.Admin)
	normal := createAPIUser(t, "normal", "normal-token", model.Normal)
	category := seedArticleCategory(t)
	tag := &model.Tag{Name: "Go", UID: "tag-go"}
	if err := connect.Database.Create(tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	repo := &mysql.ArticleDBRepo{}
	publicArticle := &mysql.ArticleModel{Title: "Public", CategoryUID: category.UID, Status: int8(mysql.ARTILCE_STATUS_PUBLIC), AuthorID: admin.ID, PostTime: time.Date(2026, 5, 1, 10, 0, 0, 0, time.Local), Detail: &model.ArticleDetail{Content: "public"}}
	draftArticle := &mysql.ArticleModel{Title: "Draft", CategoryUID: category.UID, Status: int8(mysql.ARTICLE_STATUS_DRAFT), AuthorID: admin.ID, PostTime: time.Date(2026, 5, 2, 10, 0, 0, 0, time.Local), Detail: &model.ArticleDetail{Content: "draft"}}
	if err := repo.Create(publicArticle); err != nil {
		t.Fatalf("create public article: %v", err)
	}
	if err := repo.Create(draftArticle); err != nil {
		t.Fatalf("create draft article: %v", err)
	}

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Get("/all_articles", ArticleAllList)
	app.Get("/articles", ArticleDisplayList)
	app.Get("/articles/:id", ArticleRetrieve)
	app.Post("/articles", middleware.Authentication, middleware.AdminRequired, ArticleCreate)
	app.Post("/articles/batch", middleware.Authentication, middleware.AdminRequired, ArticleBatch)
	app.Patch("/articles/:id", middleware.Authentication, middleware.AdminRequired, ArticleUpdate)
	app.Delete("/articles/:id", middleware.Authentication, middleware.AdminRequired, ArticleDelete)

	resp := doJSONRequest(t, app, fiber.MethodGet, "/articles/1?no_read=true", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("public retrieve status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/articles?page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("public article list status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/articles/2?no_read=true", "", "")
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("draft anonymous status = %d, want 404", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/articles/2?no_read=true", normal.Token, "")
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("draft normal status = %d, want 404", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/articles/2?no_read=true", admin.Token, "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("draft admin status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/all_articles?keyword=Public&page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("all articles keyword status = %d, want 200", resp.StatusCode)
	}
	wrapped := decodeAPIResp(t, resp)
	if wrapped.Meta["total"].(float64) != 1 {
		t.Fatalf("keyword total = %v, want 1", wrapped.Meta["total"])
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/all_articles?status=4&page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("all articles status filter = %d, want 200", resp.StatusCode)
	}
	wrapped = decodeAPIResp(t, resp)
	if wrapped.Meta["total"].(float64) != 1 {
		t.Fatalf("draft total = %v, want 1", wrapped.Meta["total"])
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/all_articles?date_from=2026-05-01&date_to=2026-05-01&page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("all articles date filter = %d, want 200", resp.StatusCode)
	}
	wrapped = decodeAPIResp(t, resp)
	if wrapped.Meta["total"].(float64) != 1 {
		t.Fatalf("date total = %v, want 1", wrapped.Meta["total"])
	}

	createBody := `{"title":"Created","content":"hello","category_uid":"cat-article","status":1,"tag_uids":["tag-go"]}`
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles", normal.Token, createBody)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("normal create status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles", admin.Token, createBody)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("admin create status = %d, want 201", resp.StatusCode)
	}
	wrapped = decodeAPIResp(t, resp)
	var created rtype.ArticleResponse
	if err := json.Unmarshal(wrapped.Data, &created); err != nil {
		t.Fatalf("decode created article: %v", err)
	}
	if created.ID == 0 || created.Title != "Created" {
		t.Fatalf("created article = %+v", created)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/all_articles?tag_uid=tag-go&page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("all articles tag filter = %d, want 200", resp.StatusCode)
	}
	wrapped = decodeAPIResp(t, resp)
	if wrapped.Meta["total"].(float64) != 1 {
		t.Fatalf("tag total = %v, want 1", wrapped.Meta["total"])
	}

	updateBody := `{"title":"Updated","content":"new body","category_uid":"cat-article","status":1,"tag_uids":["tag-go"]}`
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/articles/3", admin.Token, updateBody)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("update status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/batch", admin.Token, `{"ids":[3],"action":"status","status":4}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("batch status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/batch", admin.Token, `{"ids":[3],"action":"invalid"}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("batch invalid action status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/batch", admin.Token, `{"ids":[3],"action":"delete"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("batch delete status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodDelete, "/articles/3", admin.Token, "")
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("delete status = %d, want 404 after batch delete", resp.StatusCode)
	}
}

func TestCategoryTagAndCommentHandlers(t *testing.T) {
	setupAPITestDB(t)
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/article/categories", ArticleCategoryCreate)
	app.Post("/link/categories", LinkCategoryCreate)
	app.Get("/categories", CategoryAll)
	app.Get("/article/categories/all", CategoryArticleAll)
	app.Get("/link/categories/all", CategoryLinkAll)
	app.Patch("/categories/:id", CategoryUpdate)
	app.Delete("/categories/:id", CategoryDelete)
	app.Post("/tags", TagCreate)
	app.Get("/tags/all", TagListAll)
	app.Patch("/tags/:id", TagUpdate)
	app.Delete("/tags/:id", TagDelete)
	app.Post("/articles/:id/comments", CommentCreate)
	app.Get("/articles/:id/comments", ArticleCommentList)
	app.Get("/comments", CommentAllList)
	app.Patch("/comments/:id", CommentUpdate)
	app.Post("/comments/batch", CommentBatch)
	app.Delete("/comments/:id", CommentDelete)
	app.Delete("/api/comments/:id", CommentDelete)

	resp := doJSONRequest(t, app, fiber.MethodPost, "/article/categories", "", `{"name":"Tech"}`)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("category create status = %d, want 201", resp.StatusCode)
	}
	categoryResp := decodeAPIResp(t, resp)
	var category mysql.CategoryModel
	if err := json.Unmarshal(categoryResp.Data, &category); err != nil {
		t.Fatalf("decode category: %v", err)
	}
	if category.UID == "" || category.Type != model.CategoryTypeArticle {
		t.Fatalf("category = %+v", category)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/link/categories", "", `{"name":"Friends"}`)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("link category create status = %d, want 201", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/categories", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("category all status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/article/categories/all", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("category list status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/link/categories/all", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("link category list status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/categories/1", "", `{"name":"Tech Updated"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("category update status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/categories/999", "", `{"name":"Missing"}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("category missing update status = %d, want 403", resp.StatusCode)
	}

	resp = doJSONRequest(t, app, fiber.MethodPost, "/tags", "", `{"name":"Go"}`)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("tag create status = %d, want 201", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/tags", "", `{}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("tag invalid create status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/tags/all", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("tag list status = %d, want 200", resp.StatusCode)
	}
	FindTags([]*model.Tag{})
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/tags/1", "", `{"name":"Golang"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("tag update status = %d, want 200", resp.StatusCode)
	}

	allowComment := true
	closedComment := false
	article := &mysql.ArticleModel{Title: "Commented", CategoryUID: category.UID, Status: int8(mysql.ARTILCE_STATUS_PUBLIC), IsAllowComment: &allowComment, Detail: &model.ArticleDetail{Content: "body"}}
	if err := (&mysql.ArticleDBRepo{}).Create(article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	closedArticle := &mysql.ArticleModel{Title: "Closed", CategoryUID: category.UID, Status: int8(mysql.ARTILCE_STATUS_PUBLIC), IsAllowComment: &closedComment, Detail: &model.ArticleDetail{Content: "closed"}}
	if err := (&mysql.ArticleDBRepo{}).Create(closedArticle); err != nil {
		t.Fatalf("create closed article: %v", err)
	}
	draftArticle := &mysql.ArticleModel{Title: "Draft", CategoryUID: category.UID, Status: int8(mysql.ARTICLE_STATUS_DRAFT), IsAllowComment: &allowComment, Detail: &model.ArticleDetail{Content: "draft"}}
	if err := (&mysql.ArticleDBRepo{}).Create(draftArticle); err != nil {
		t.Fatalf("create draft article: %v", err)
	}
	commentBody := `{"author_name":"alice","author_email":"alice@example.com","content":"hello"}`
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/1/comments", "", commentBody)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("comment create status = %d, want 201", resp.StatusCode)
	}
	commentResp := decodeAPIResp(t, resp)
	var createdComment model.Comment
	if err := json.Unmarshal(commentResp.Data, &createdComment); err != nil {
		t.Fatalf("decode comment: %v", err)
	}
	if createdComment.Status == nil || *createdComment.Status != int8(model.CommentStatusPending) {
		t.Fatalf("created comment status = %v, want pending", createdComment.Status)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/2/comments", "", commentBody)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("closed article comment status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/3/comments", "", commentBody)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("draft article comment status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/articles/1/comments", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("comment list status = %d, want 200", resp.StatusCode)
	}
	commentListResp := decodeAPIResp(t, resp)
	var publicComments []rtype.CommentResponse
	if err := json.Unmarshal(commentListResp.Data, &publicComments); err != nil {
		t.Fatalf("decode public comments: %v", err)
	}
	if len(publicComments) != 0 {
		t.Fatalf("pending public comments len = %d, want 0", len(publicComments))
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/comments?status=1&page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("admin pending comments status = %d, want 200", resp.StatusCode)
	}
	commentListResp = decodeAPIResp(t, resp)
	if commentListResp.Meta["total"].(float64) != 1 {
		t.Fatalf("pending total = %v, want 1", commentListResp.Meta["total"])
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/comments?article_id=1&page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("admin comments by article_id status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/comments?article_uid="+article.UID+"&page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("admin comments by article_uid status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/comments/batch", "", `{"ids":[1],"action":"status","status":2}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("comment batch approve status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/articles/1/comments", "", "")
	commentListResp = decodeAPIResp(t, resp)
	if err := json.Unmarshal(commentListResp.Data, &publicComments); err != nil {
		t.Fatalf("decode approved public comments: %v", err)
	}
	if len(publicComments) != 1 {
		t.Fatalf("approved public comments len = %d, want 1", len(publicComments))
	}
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/comments/1", "", `{"author_name":"bob","author_email":"bob@example.com","content":"updated","status":2}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("comment update status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/comments/batch", "", `{"ids":[1],"action":"reject"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("comment batch reject action status = %d, want 200", resp.StatusCode)
	}
	var rejected model.Comment
	if err := connect.Database.First(&rejected, 1).Error; err != nil {
		t.Fatalf("find rejected comment: %v", err)
	}
	if rejected.Status == nil || *rejected.Status != int8(model.CommentStatusRejected) {
		t.Fatalf("rejected comment status = %v, want rejected", rejected.Status)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/comments/batch", "", `{"ids":[1],"action":"spam"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("comment batch spam action status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/comments/batch", "", `{"ids":[1],"action":"pending"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("comment batch pending action status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/comments/batch", "", `{"ids":[],"action":"approve"}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("comment batch empty status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodDelete, "/comments/1", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("comment delete status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodDelete, "/api/comments/999", "", "")
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("comment missing delete status = %d, want 400", resp.StatusCode)
	}

	resp = doJSONRequest(t, app, fiber.MethodDelete, "/tags/1", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("tag delete status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodDelete, "/categories/1", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("category delete status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodDelete, "/categories/999", "", "")
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("category missing delete status = %d, want 404", resp.StatusCode)
	}
}

func TestSettingAndLinkHandlers(t *testing.T) {
	setupAPITestDB(t)
	linkCategory := &mysql.CategoryModel{Name: "Friends", UID: "cat-link", Type: model.CategoryTypeLink}
	if err := connect.Database.Create(linkCategory).Error; err != nil {
		t.Fatalf("create link category: %v", err)
	}
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/settings", SettingCreate)
	app.Get("/settings/:key", SettingRetrieve)
	app.Get("/api/settings/:key", SettingRetrieve)
	app.Get("/settings", SettingAll)
	app.Patch("/settings/:key", SettingUpdate)
	app.Delete("/settings/:key", SettingDelete)
	app.Post("/links", LinkCreate)
	app.Get("/links", LinkAll)
	app.Put("/links/:id", LinkUpdate)
	app.Delete("/links/:id", LinkDelete)

	resp := doJSONRequest(t, app, fiber.MethodPost, "/settings", "", `{"key":"SiteName","value":"Myecho","type":"string"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("setting create status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/settings/SiteName", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("setting retrieve status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/settings", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("setting all status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/settings/SiteName", "", `{"value":"New","description":"desc"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("setting update status = %d, want 200", resp.StatusCode)
	}
	if err := connect.Database.Create(&mysql.SettingModel{Key: "ThemePreviewTokenSecret", Value: "secret"}).Error; err != nil {
		t.Fatalf("create hidden setting: %v", err)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/api/settings/ThemePreviewTokenSecret", "", "")
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("hidden setting retrieve status = %d, want 400", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/settings", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("setting all with hidden status = %d, want 200", resp.StatusCode)
	}

	linkBody := `{"name":"Go","description":"golang","url":"https://go.dev","category_uid":"cat-link"}`
	resp = doJSONRequest(t, app, fiber.MethodPost, "/links", "", linkBody)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("link create status = %d, want 201", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/links?category_uid=cat-link", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("link all status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPut, "/links/1", "", `{"name":"Go Dev","url":"https://go.dev","category_uid":"cat-link"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("link update status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodDelete, "/links/1", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("link delete status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodDelete, "/settings/SiteName", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("setting delete status = %d, want 200", resp.StatusCode)
	}
}

func TestFileHandlersUploadListUpdateDelete(t *testing.T) {
	setupAPITestDB(t)
	chdirToTemp(t)
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/files/upload", FileSingleUpload)
	app.Post("/api/files/upload", FileSingleUpload)
	app.Post("/files/vditor_upload", VditorFileUpload)
	app.Post("/api/files/vditor_upload", VditorFileUpload)
	app.Post("/save_url_file", FileSaveByLinkUrl)
	app.Post("/api/save_url_file", FileSaveByLinkUrl)
	app.Get("/files", FilePageList)
	app.Put("/files/:id", FileInfoUpdate)
	app.Delete("/files/:id", FileDelete)

	resp := doJSONRequest(t, app, fiber.MethodPost, "/api/files/upload", "", "")
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("empty upload status = %d, want 400", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/api/files/vditor_upload", "", "")
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("empty vditor upload status = %d, want 400", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/api/save_url_file", "", `{}`)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("empty save url file status = %d, want 400", resp.StatusCode)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "report.txt")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := io.WriteString(part, "hello"); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(fiber.MethodPost, "/files/upload", body)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("upload app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("upload status = %d, want 200", resp.StatusCode)
	}
	wrapped := decodeAPIResp(t, resp)
	var uploaded struct {
		ID       uint   `json:"id"`
		FullName string `json:"full_name"`
	}
	if err := json.Unmarshal(wrapped.Data, &uploaded); err != nil {
		t.Fatalf("decode uploaded: %v", err)
	}
	if uploaded.ID == 0 || uploaded.FullName != "report.txt" {
		t.Fatalf("uploaded = %+v", uploaded)
	}

	vditorBody := &bytes.Buffer{}
	vditorWriter := multipart.NewWriter(vditorBody)
	for i := 0; i < 2; i++ {
		part, err := vditorWriter.CreateFormFile("file[]", "dup.txt")
		if err != nil {
			t.Fatalf("CreateFormFile(file[]) error = %v", err)
		}
		if _, err := io.WriteString(part, "duplicate content"); err != nil {
			t.Fatalf("write vditor file: %v", err)
		}
	}
	if err := vditorWriter.Close(); err != nil {
		t.Fatalf("close vditor multipart: %v", err)
	}
	req = httptest.NewRequest(fiber.MethodPost, "/files/vditor_upload", vditorBody)
	req.Header.Set(fiber.HeaderContentType, vditorWriter.FormDataContentType())
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("vditor upload app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("vditor upload status = %d, want 200", resp.StatusCode)
	}

	oldRemoteClient := utils.RemoteFileHTTPClient
	utils.RemoteFileHTTPClient = &http.Client{Transport: apiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Body:          io.NopCloser(bytes.NewBufferString("remote file")),
			ContentLength: int64(len("remote file")),
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}
	t.Cleanup(func() { utils.RemoteFileHTTPClient = oldRemoteClient })
	resp = doJSONRequest(t, app, fiber.MethodPost, "/save_url_file", "", `{"url":"http://93.184.216.34/remote.txt"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("save url file status = %d, want 200", resp.StatusCode)
	}

	resp = doJSONRequest(t, app, fiber.MethodGet, "/files?name=report", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("file list status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPut, "/files/1", "", `{"full_name":"renamed.txt","note":"note"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("file update status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodDelete, "/files/1", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("file delete status = %d, want 200", resp.StatusCode)
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
