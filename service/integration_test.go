package service

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"myecho/config"
	"myecho/dal/cache"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/utils"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupServiceTestDB(t *testing.T) {
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
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestSettingServiceCRUD(t *testing.T) {
	setupServiceTestDB(t)
	svc := &SettingService{}
	setting := &mysql.SettingModel{Key: "SiteName", Value: "Myecho", Description: "site"}
	if err := svc.Create(setting); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := svc.GetByKey("SiteName"); err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}
	updated, err := svc.UpdateValueAndDesc("SiteName", "New", "updated")
	if err != nil {
		t.Fatalf("UpdateValueAndDesc() error = %v", err)
	}
	if updated.Value != "New" || updated.Description != "updated" {
		t.Fatalf("updated setting = %+v", updated)
	}
	all, err := svc.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("GetAll() len = %d, want 1", len(all))
	}
	if err := os.MkdirAll("storage", 0755); err != nil {
		t.Fatalf("mkdir storage: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join("storage", "favicon.ico")) })
	oldClient := utils.RemoteFileHTTPClient
	utils.RemoteFileHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader("ico")),
			ContentLength: 3,
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}
	t.Cleanup(func() { utils.RemoteFileHTTPClient = oldClient })
	if err := saveIcon("SiteFaviconIcon", "http://93.184.216.34/favicon.ico"); err != nil {
		t.Fatalf("saveIcon() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join("storage", "favicon.ico")); err != nil {
		t.Fatalf("favicon was not saved: %v", err)
	}
	if err := svc.DeleteByKey("SiteName"); err != nil {
		t.Fatalf("DeleteByKey() error = %v", err)
	}
}

func TestCategoryAndLinkServices(t *testing.T) {
	setupServiceTestDB(t)
	categorySvc := &CategoryService{}
	linkSvc := &LinkService{}

	category := &mysql.CategoryModel{Name: "Links", UID: "cat-link"}
	if err := categorySvc.CreateByType(category, model.CategoryTypeLink); err != nil {
		t.Fatalf("CreateByType() error = %v", err)
	}
	if err := categorySvc.ValidateUIDExist("cat-link"); err != nil {
		t.Fatalf("ValidateUIDExist() error = %v", err)
	}
	categories, err := categorySvc.AllByType(model.CategoryTypeLink)
	if err != nil {
		t.Fatalf("AllByType() error = %v", err)
	}
	if len(categories) != 1 || categories[0].Type != model.CategoryTypeLink {
		t.Fatalf("AllByType() = %+v", categories)
	}
	allCategories, err := categorySvc.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if len(allCategories) != 1 {
		t.Fatalf("All() len = %d, want 1", len(allCategories))
	}

	link := &mysql.LinkModel{Name: "Go", URL: "https://go.dev", CategoryUID: "cat-link"}
	if err := linkSvc.Create(link); err != nil {
		t.Fatalf("Link Create() error = %v", err)
	}
	links, err := linkSvc.All(nil)
	if err != nil {
		t.Fatalf("Link All(nil) error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("Link All(nil) len = %d, want 1", len(links))
	}
	link.Name = "Golang"
	if err := linkSvc.UpdateByID(link.ID, link); err != nil {
		t.Fatalf("Link UpdateByID() error = %v", err)
	}
	if err := linkSvc.DeleteByID(link.ID); err != nil {
		t.Fatalf("Link DeleteByID() error = %v", err)
	}
}

func TestThemeServiceCRUD(t *testing.T) {
	setupServiceTestDB(t)
	svc := &ThemeService{}
	if err := svc.InitDefaultTheme(); err != nil {
		t.Fatalf("InitDefaultTheme() error = %v", err)
	}
	active, err := svc.GetActiveTheme()
	if err != nil {
		t.Fatalf("GetActiveTheme() error = %v", err)
	}
	if !active.IsActive || !active.IsDefault {
		t.Fatalf("active theme = %+v", active)
	}
	custom := &mysql.ThemeModel{Name: "custom", DisplayName: "Custom"}
	if err := svc.CreateTheme(custom); err != nil {
		t.Fatalf("CreateTheme() error = %v", err)
	}
	byName, err := svc.GetThemeByName("custom")
	if err != nil {
		t.Fatalf("GetThemeByName() error = %v", err)
	}
	byName.Description = "updated"
	if err := svc.UpdateTheme(byName); err != nil {
		t.Fatalf("UpdateTheme() error = %v", err)
	}
	if _, err := svc.GetThemeByID(int64(byName.ID)); err != nil {
		t.Fatalf("GetThemeByID() error = %v", err)
	}
	allThemes, err := svc.GetAllThemes()
	if err != nil {
		t.Fatalf("GetAllThemes() error = %v", err)
	}
	if len(allThemes) != 2 {
		t.Fatalf("GetAllThemes() len = %d, want 2", len(allThemes))
	}
	if err := svc.ActivateTheme(int64(byName.ID)); err != nil {
		t.Fatalf("ActivateTheme() error = %v", err)
	}
	if err := svc.ActivateTheme(99999); err == nil {
		t.Fatal("ActivateTheme(missing) expected an error")
	}
	stillActive, err := svc.GetActiveTheme()
	if err != nil {
		t.Fatalf("GetActiveTheme() after failed activation error = %v", err)
	}
	if stillActive.ID != byName.ID {
		t.Fatalf("failed activation changed active theme to %+v, want %d", stillActive, byName.ID)
	}
	if err := svc.DeleteTheme(int64(active.ID)); err == nil {
		t.Fatal("DeleteTheme(default) expected an error")
	}
}

func TestArticleServiceDisplayAndRetrieve(t *testing.T) {
	setupServiceTestDB(t)
	if err := (&CategoryService{}).CreateByType(&mysql.CategoryModel{Name: "Article", UID: "cat-article"}, model.CategoryTypeArticle); err != nil {
		t.Fatalf("create category: %v", err)
	}
	articles := []*mysql.ArticleModel{
		{UID: "top", Title: "Top", CategoryUID: "cat-article", Status: int8(mysql.ARTICLE_STATUS_TOP), Detail: &model.ArticleDetail{Content: "top"}},
		{UID: "public", Title: "Public", CategoryUID: "cat-article", Status: int8(mysql.ARTILCE_STATUS_PUBLIC), Detail: &model.ArticleDetail{Content: "public"}},
	}
	articleRepo := &mysql.ArticleDBRepo{}
	for _, article := range articles {
		if err := articleRepo.Create(article); err != nil {
			t.Fatalf("create article %s: %v", article.UID, err)
		}
	}
	page, got, err := (&ArticleService{}).ArticleDisplayList(&ArticleDisplayListQueryParam{PageFindParam: mysql.PageFindParam{Page: 1, PageSize: 10}})
	if err != nil {
		t.Fatalf("ArticleDisplayList() error = %v", err)
	}
	if page.Total != 2 || len(got) != 2 {
		t.Fatalf("ArticleDisplayList() page=%+v len=%d", page, len(got))
	}
	retrieved, err := (&ArticleService{}).ArticleRetrieve(&ArticleRetrieveQueryParam{ID: articles[0].ID, NoRead: true})
	if err != nil {
		t.Fatalf("ArticleRetrieve() error = %v", err)
	}
	if retrieved.Title != "Top" {
		t.Fatalf("ArticleRetrieve() = %+v", retrieved)
	}
}

func TestArticleServiceDisplayListPaginatesTopAndPublicArticles(t *testing.T) {
	setupServiceTestDB(t)
	if err := (&CategoryService{}).CreateByType(&mysql.CategoryModel{Name: "Article", UID: "cat-article"}, model.CategoryTypeArticle); err != nil {
		t.Fatalf("create category: %v", err)
	}
	articleRepo := &mysql.ArticleDBRepo{}
	for i := 0; i < 12; i++ {
		article := &mysql.ArticleModel{Title: "Top", CategoryUID: "cat-article", Status: int8(mysql.ARTICLE_STATUS_TOP), Detail: &model.ArticleDetail{Content: "top"}}
		if err := articleRepo.Create(article); err != nil {
			t.Fatalf("create top article: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		article := &mysql.ArticleModel{Title: "Public", CategoryUID: "cat-article", Status: int8(mysql.ARTILCE_STATUS_PUBLIC), Detail: &model.ArticleDetail{Content: "public"}}
		if err := articleRepo.Create(article); err != nil {
			t.Fatalf("create public article: %v", err)
		}
	}

	page1, got1, err := (&ArticleService{}).ArticleDisplayList(&ArticleDisplayListQueryParam{PageFindParam: mysql.PageFindParam{Page: 1, PageSize: 10}})
	if err != nil {
		t.Fatalf("ArticleDisplayList(page1) error = %v", err)
	}
	if page1.Total != 15 || len(got1) != 10 {
		t.Fatalf("page1 page=%+v len=%d", page1, len(got1))
	}
	page2, got2, err := (&ArticleService{}).ArticleDisplayList(&ArticleDisplayListQueryParam{PageFindParam: mysql.PageFindParam{Page: 2, PageSize: 10}})
	if err != nil {
		t.Fatalf("ArticleDisplayList(page2) error = %v", err)
	}
	if page2.Total != 15 || len(got2) != 5 {
		t.Fatalf("page2 page=%+v len=%d", page2, len(got2))
	}
	topCount := 0
	publicCount := 0
	for _, article := range got2 {
		switch article.Status {
		case int8(mysql.ARTICLE_STATUS_TOP):
			topCount++
		case int8(mysql.ARTILCE_STATUS_PUBLIC):
			publicCount++
		}
	}
	if topCount != 2 || publicCount != 3 {
		t.Fatalf("page2 status counts top=%d public=%d", topCount, publicCount)
	}
}

func TestFileServiceListUpdateDelete(t *testing.T) {
	setupServiceTestDB(t)
	if err := os.MkdirAll(filepath.Join("storage", "test"), 0755); err != nil {
		t.Fatalf("mkdir storage: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll("storage/test") })
	fileModel := &mysql.FileModel{Name: "report", ExtensionName: ".txt", DirPath: "test", UUID: "file-service"}
	if err := os.WriteFile(fileModel.GetActualSavePath(), []byte("content"), 0644); err != nil {
		t.Fatalf("write actual file: %v", err)
	}
	fileRepo := &mysql.FileRepo{}
	if err := fileRepo.Create(fileModel); err != nil {
		t.Fatalf("create file model: %v", err)
	}
	page, files, err := (&FileService{}).PageList(&FilePageListParam{PageFindParam: mysql.PageFindParam{Page: 1, PageSize: 10}, Name: "report"})
	if err != nil {
		t.Fatalf("PageList() error = %v", err)
	}
	if page.Total != 1 || len(files) != 1 {
		t.Fatalf("PageList() page=%+v files=%+v", page, files)
	}
	updated, err := (&FileService{}).UpdateFile(fileModel.ID, &UpdateFileParam{FullName: "updated.txt", Note: "note"})
	if err != nil {
		t.Fatalf("UpdateFile() error = %v", err)
	}
	if updated.FullName != "updated.txt" || updated.Note != "note" {
		t.Fatalf("UpdateFile() = %+v", updated)
	}
	if _, err := (&FileService{}).UpdateFile(fileModel.ID, &UpdateFileParam{FullName: "updated.md", Note: "note"}); err == nil {
		t.Fatal("UpdateFile() changing extension expected an error")
	}
	if err := (&FileService{}).Delete(fileModel.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := (&FileService{}).DeleteByUUID("missing"); err != nil {
		t.Fatalf("DeleteByUUID(missing) error = %v", err)
	}
}
