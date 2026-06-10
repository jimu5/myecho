package mysql

import (
	"errors"
	"myecho/dal/connect"
	"myecho/model"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupMysqlRepoTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := testDB.AutoMigrate(
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
	connect.Database = testDB
	InitDB()
	t.Cleanup(func() {
		sqlDB, err := testDB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestArticleRepoQueriesMutationsAndReadCount(t *testing.T) {
	setupMysqlRepoTestDB(t)
	category := &CategoryModel{Name: "Root", UID: "cat-root", Type: model.CategoryTypeArticle}
	if err := (&CategoryRepo{}).Create(category); err != nil {
		t.Fatalf("create category: %v", err)
	}
	repo := &ArticleDBRepo{}
	publicArticle := &ArticleModel{Title: "Public", CategoryUID: category.UID, Status: int8(ARTILCE_STATUS_PUBLIC), Detail: &model.ArticleDetail{Content: "public"}}
	topArticle := &ArticleModel{Title: "Top", CategoryUID: category.UID, Status: int8(ARTICLE_STATUS_TOP), Detail: &model.ArticleDetail{Content: "top"}}
	draftArticle := &ArticleModel{Title: "Draft", CategoryUID: category.UID, Status: int8(ARTICLE_STATUS_DRAFT), Detail: &model.ArticleDetail{Content: "draft"}}
	for _, article := range []*ArticleModel{publicArticle, topArticle, draftArticle} {
		if err := repo.Create(article); err != nil {
			t.Fatalf("create article %s: %v", article.Title, err)
		}
	}

	all, err := repo.PageFindAll(&PageFindParam{NoPage: true}, &struct{}{})
	if err != nil {
		t.Fatalf("PageFindAll() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("PageFindAll() len = %d, want 3", len(all))
	}
	notTop, err := repo.PageFindByNotVisibility(&PageFindParam{NoPage: true}, PageFindArticleByNotStatusParam{
		ArticleCommonQueryParam: ArticleCommonQueryParam{Status: ptrArticleStatus(ARTICLE_STATUS_TOP)},
	})
	if err != nil {
		t.Fatalf("PageFindByNotVisibility() error = %v", err)
	}
	if len(notTop) != 2 {
		t.Fatalf("PageFindByNotVisibility() len = %d, want 2", len(notTop))
	}
	if total, err := repo.CountDisplayable(ArticleCommonQueryParam{}); err != nil || total != 2 {
		t.Fatalf("CountDisplayable() total=%d err=%v, want 2 nil", total, err)
	}

	publicArticle.Title = "Updated"
	publicArticle.Detail = &model.ArticleDetail{Content: "updated content"}
	if err := repo.Update(publicArticle); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, err := repo.FindByID(publicArticle.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if updated.Title != "Updated" || updated.Detail == nil || updated.Detail.Content != "updated content" {
		t.Fatalf("updated article = %+v", updated)
	}
	if err := repo.AddReadCountByID(publicArticle.ID, 3); err != nil {
		t.Fatalf("AddReadCountByID() error = %v", err)
	}
	updated, err = repo.FindByID(publicArticle.ID)
	if err != nil {
		t.Fatalf("FindByID(after read) error = %v", err)
	}
	if updated.ReadCount != 3 {
		t.Fatalf("ReadCount = %d, want 3", updated.ReadCount)
	}
	if err := repo.AddReadCountByID(9999, 1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("AddReadCountByID(missing) error = %v, want ErrRecordNotFound", err)
	}
	if err := repo.DeleteByID(draftArticle.ID); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}
}

func TestCategoryRepoChildrenAndDuplicateNames(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &CategoryRepo{}
	root := &CategoryModel{Name: "Root", UID: "root", Type: model.CategoryTypeArticle}
	child := &CategoryModel{Name: "Child", UID: "child", FatherUID: "root", Type: model.CategoryTypeArticle}
	grandChild := &CategoryModel{Name: "Grand", UID: "grand", FatherUID: "child", Type: model.CategoryTypeArticle}
	for _, category := range []*CategoryModel{root, child, grandChild} {
		if err := repo.Create(category); err != nil {
			t.Fatalf("create category %s: %v", category.Name, err)
		}
	}
	children, err := repo.GetAllChildrenUID("root")
	if err != nil {
		t.Fatalf("GetAllChildrenUID() error = %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("children = %+v, want two descendants", children)
	}
	if err := repo.Create(&CategoryModel{Name: "Root", UID: "duplicate", Type: model.CategoryTypeArticle}); err == nil {
		t.Fatal("Create(duplicate name/type) expected error")
	}
	if err := repo.Create(&CategoryModel{Name: "Root", UID: "link-root", Type: model.CategoryTypeLink}); err != nil {
		t.Fatalf("Create(same name different type) error = %v", err)
	}
	root.Name = "Renamed"
	if err := db.Save(root).Error; err != nil {
		t.Fatalf("Save(renamed root) error = %v", err)
	}
}

func TestSettingRepoDefaultsAndTypeUpdates(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &SettingRepo{}
	repo.InitDefaultSetting()
	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("default settings len = %d, want 3", len(all))
	}
	if err := repo.MCreate(nil); err != nil {
		t.Fatalf("MCreate(nil) error = %v", err)
	}
	if err := repo.MUpdateIsSystem(nil, true); err != nil {
		t.Fatalf("MUpdateIsSystem(nil) error = %v", err)
	}
	updated, err := repo.UpdateValueAndType("SiteTitle", model.SettingModelTypeBool, "true")
	if err != nil {
		t.Fatalf("UpdateValueAndType() error = %v", err)
	}
	if updated.Type != model.SettingModelTypeBool || updated.Value != "true" {
		t.Fatalf("updated setting = %+v", updated)
	}
	if _, err := repo.UpdateValueAndType("missing", model.SettingModelTypeString, "x"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateValueAndType(missing) error = %v, want ErrRecordNotFound", err)
	}
	if !repo.CheckIsInitKey("SiteTitle") || repo.CheckIsInitKey("Other") {
		t.Fatalf("CheckIsInitKey returned unexpected values")
	}
}

func TestThemeRepoErrorPaths(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ThemeRepo{}
	if _, err := repo.GetByID(1); !errors.Is(err, ErrThemeNotExist) {
		t.Fatalf("GetByID(missing) error = %v, want ErrThemeNotExist", err)
	}
	if err := repo.InitDefaultTheme(); err != nil {
		t.Fatalf("InitDefaultTheme() error = %v", err)
	}
	if err := repo.InitDefaultTheme(); err != nil {
		t.Fatalf("InitDefaultTheme(second) error = %v", err)
	}
	active, err := repo.GetActiveTheme()
	if err != nil {
		t.Fatalf("GetActiveTheme() error = %v", err)
	}
	if err := repo.Delete(int64(active.ID)); !errors.Is(err, ErrThemeCantDeleteActive) {
		t.Fatalf("Delete(active) error = %v, want ErrThemeCantDeleteActive", err)
	}
	custom := &ThemeModel{Name: "custom", DisplayName: "Custom"}
	if err := repo.Create(custom); err != nil {
		t.Fatalf("Create(custom) error = %v", err)
	}
	if err := repo.ActivateTheme(int64(custom.ID)); err != nil {
		t.Fatalf("ActivateTheme() error = %v", err)
	}
	if err := repo.Delete(int64(active.ID)); !errors.Is(err, ErrThemeCantDeleteDefault) {
		t.Fatalf("Delete(default inactive) error = %v, want ErrThemeCantDeleteDefault", err)
	}
	if err := repo.ActivateTheme(9999); !errors.Is(err, ErrThemeNotExist) {
		t.Fatalf("ActivateTheme(missing) error = %v, want ErrThemeNotExist", err)
	}
	spare := &ThemeModel{Name: "spare", DisplayName: "Spare"}
	if err := repo.Create(spare); err != nil {
		t.Fatalf("Create(spare) error = %v", err)
	}
	if err := repo.Delete(int64(spare.ID)); err != nil {
		t.Fatalf("Delete(spare) error = %v", err)
	}
}

func ptrArticleStatus(status ArticleStatus) *ArticleStatus {
	return &status
}
