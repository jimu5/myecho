package mysql

import (
	"errors"
	"myecho/dal/connect"
	"myecho/model"
	"path/filepath"
	"testing"
	"time"

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
	childCategory := &CategoryModel{Name: "Child", UID: "cat-child", FatherUID: category.UID, Type: model.CategoryTypeArticle}
	if err := (&CategoryRepo{}).Create(childCategory); err != nil {
		t.Fatalf("create child category: %v", err)
	}
	tag := &model.Tag{Name: "Go", UID: "tag-go"}
	if err := db.Create(tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	repo := &ArticleDBRepo{}
	publicArticle := &ArticleModel{Title: "Public", Summary: "Go summary", CategoryUID: category.UID, Status: int8(ARTILCE_STATUS_PUBLIC), PostTime: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC), Detail: &model.ArticleDetail{Content: "body-only-token"}}
	topArticle := &ArticleModel{Title: "Top", CategoryUID: childCategory.UID, Status: int8(ARTICLE_STATUS_TOP), PostTime: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC), Detail: &model.ArticleDetail{Content: "top"}}
	draftArticle := &ArticleModel{Title: "Draft", CategoryUID: category.UID, Status: int8(ARTICLE_STATUS_DRAFT), PostTime: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC), Detail: &model.ArticleDetail{Content: "draft"}}
	futureArticle := &ArticleModel{Title: "Scheduled", Status: int8(ARTILCE_STATUS_PUBLIC), PostTime: time.Now().Add(24 * time.Hour), Detail: &model.ArticleDetail{Content: "scheduled"}}
	for _, article := range []*ArticleModel{publicArticle, topArticle, draftArticle, futureArticle} {
		if err := repo.Create(article); err != nil {
			t.Fatalf("create article %s: %v", article.Title, err)
		}
	}
	var countedRoot, countedChild CategoryModel
	if err := db.Where("uid = ?", category.UID).First(&countedRoot).Error; err != nil {
		t.Fatalf("load root category count: %v", err)
	}
	if err := db.Where("uid = ?", childCategory.UID).First(&countedChild).Error; err != nil {
		t.Fatalf("load child category count: %v", err)
	}
	if countedRoot.Count != 1 || countedChild.Count != 1 {
		t.Fatalf("category counts root=%d child=%d, want public and top counted", countedRoot.Count, countedChild.Count)
	}
	if err := db.Table("article_tags").Create(map[string]interface{}{"article_uid": publicArticle.UID, "tag_uid": tag.UID}).Error; err != nil {
		t.Fatalf("create article tag relation: %v", err)
	}

	all, err := repo.PageFindAll(&PageFindParam{NoPage: true}, &struct{}{})
	if err != nil {
		t.Fatalf("PageFindAll() error = %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("PageFindAll() len = %d, want 4", len(all))
	}
	for _, article := range all {
		if article.Detail != nil {
			t.Fatalf("PageFindAll() loaded article detail for %q", article.Title)
		}
	}
	notTop, err := repo.PageFindByNotVisibility(&PageFindParam{NoPage: true}, PageFindArticleByNotStatusParam{
		ArticleCommonQueryParam: ArticleCommonQueryParam{Status: ptrArticleStatus(ARTICLE_STATUS_TOP)},
	})
	if err != nil {
		t.Fatalf("PageFindByNotVisibility() error = %v", err)
	}
	if len(notTop) != 3 {
		t.Fatalf("PageFindByNotVisibility() len = %d, want 3", len(notTop))
	}
	if total, err := repo.CountDisplayable(ArticleCommonQueryParam{}); err != nil || total != 2 {
		t.Fatalf("CountDisplayable() total=%d err=%v, want two already-published articles", total, err)
	}
	publicStatus := ARTILCE_STATUS_PUBLIC
	keyword := "Go"
	dateFrom := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	dateTo := time.Date(2026, 5, 1, 23, 59, 59, 0, time.UTC)
	filtered, err := repo.PageFindByCommonParam(&PageFindParam{NoPage: true}, ArticleCommonQueryParam{
		CategoryUID: &category.UID,
		Status:      &publicStatus,
		Keyword:     &keyword,
		TagUID:      &tag.UID,
		DateFrom:    &dateFrom,
		DateTo:      &dateTo,
	})
	if err != nil {
		t.Fatalf("PageFindByCommonParam() error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != publicArticle.ID {
		t.Fatalf("PageFindByCommonParam() = %+v, want public article", filtered)
	}
	bodyKeyword := "body-only-token"
	matches, err := repo.PageFindByCommonParam(&PageFindParam{NoPage: true}, ArticleCommonQueryParam{Keyword: &bodyKeyword})
	if err != nil {
		t.Fatalf("PageFindByCommonParam(body keyword) error = %v", err)
	}
	if len(matches) != 1 || matches[0].ID != publicArticle.ID {
		t.Fatalf("PageFindByCommonParam(body keyword) = %+v, want public article", matches)
	}
	total, err := repo.CountAll(ArticleCommonQueryParam{CategoryUID: &category.UID})
	if err != nil {
		t.Fatalf("CountAll(category) error = %v", err)
	}
	if total != 3 {
		t.Fatalf("CountAll(category) = %d, want 3 including child category", total)
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
	if err := db.Model(&CategoryModel{}).Where("uid = ?", childCategory.UID).Update("count", 0).Error; err != nil {
		t.Fatalf("simulate legacy top category count: %v", err)
	}
	if err := repo.BatchUpdateStatus([]uint{topArticle.ID}, ARTICLE_STATUS_DRAFT); err != nil {
		t.Fatalf("BatchUpdateStatus() error = %v", err)
	}
	updatedTop, err := repo.FindByID(topArticle.ID)
	if err != nil {
		t.Fatalf("FindByID(top after batch status) error = %v", err)
	}
	if updatedTop.Status != int8(ARTICLE_STATUS_DRAFT) {
		t.Fatalf("top status = %d, want draft", updatedTop.Status)
	}
	if err := db.Where("uid = ?", childCategory.UID).First(&countedChild).Error; err != nil {
		t.Fatalf("reload child category count: %v", err)
	}
	if countedChild.Count != 0 {
		t.Fatalf("child category count = %d after top becomes draft, want 0", countedChild.Count)
	}
	if err := repo.BatchDelete([]uint{topArticle.ID}); err != nil {
		t.Fatalf("BatchDelete() error = %v", err)
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
	defaults := getDefaultSettings()
	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(all) != len(defaults) || len(defaults) != 12 {
		t.Fatalf("default settings persisted=%d defined=%d, want 12", len(all), len(defaults))
	}
	for _, key := range []string{"SiteTitle", "SiteDescription", "SiteLogo", "SiteAuthor", "SiteAuthorBio", "SiteFooter", "SiteICP", "SiteSocialLinks", "SiteShareImage", "BaseURL", "SiteIndexMetaKeyword", "SiteFaviconIcon"} {
		setting, ok := defaults[key]
		if !ok || !setting.IsSystem {
			t.Fatalf("default setting %q = %+v, exists=%v", key, setting, ok)
		}
	}
	if defaults["SiteSocialLinks"].Value != "[]" {
		t.Fatalf("SiteSocialLinks default = %q, want []", defaults["SiteSocialLinks"].Value)
	}
	if err := repo.MCreate(nil); err != nil {
		t.Fatalf("MCreate(nil) error = %v", err)
	}
	if err := repo.MUpdateIsSystem(nil, true); err != nil {
		t.Fatalf("MUpdateIsSystem(nil) error = %v", err)
	}
	if err := repo.Create(&SettingModel{Key: "Custom", Value: "old"}); err != nil {
		t.Fatalf("Create(custom setting) error = %v", err)
	}
	updatedDesc, err := repo.UpdateValueAndDesc("Custom", "new", "desc")
	if err != nil {
		t.Fatalf("UpdateValueAndDesc() error = %v", err)
	}
	if updatedDesc.Value != "new" || updatedDesc.Description != "desc" {
		t.Fatalf("UpdateValueAndDesc() = %+v", updatedDesc)
	}
	if _, err := repo.UpdateValueAndDesc("missing-desc", "x", "y"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateValueAndDesc(missing) error = %v, want ErrRecordNotFound", err)
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
	if err := repo.DeleteByKey("Custom"); err != nil {
		t.Fatalf("DeleteByKey() error = %v", err)
	}
	if err := repo.DeleteByKey("Custom"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("DeleteByKey(missing) error = %v, want ErrRecordNotFound", err)
	}
}

func TestLinkRepoCRUDAndCategoryFiltering(t *testing.T) {
	setupMysqlRepoTestDB(t)
	categoryRepo := &CategoryRepo{}
	root := &CategoryModel{Name: "Friends", UID: "friends", Type: model.CategoryTypeLink}
	child := &CategoryModel{Name: "Blogs", UID: "blogs", FatherUID: "friends", Type: model.CategoryTypeLink}
	for _, category := range []*CategoryModel{root, child} {
		if err := categoryRepo.Create(category); err != nil {
			t.Fatalf("create category %s: %v", category.Name, err)
		}
	}

	repo := &LinkRepo{}
	link := &LinkModel{Name: "Myecho", URL: "https://example.com", CategoryUID: root.UID}
	if err := repo.Create(link); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	childLink := &LinkModel{Name: "Friend", URL: "https://friend.example.com", CategoryUID: child.UID}
	if err := repo.Create(childLink); err != nil {
		t.Fatalf("Create(child) error = %v", err)
	}
	all, err := repo.All(&LinkCommonQueryParam{})
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("All() len = %d, want 2", len(all))
	}
	filtered, err := repo.All(&LinkCommonQueryParam{CategoryUID: &root.UID})
	if err != nil {
		t.Fatalf("All(category) error = %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("All(category) len = %d, want root and child links", len(filtered))
	}

	link.Name = "Updated"
	link.CategoryUID = child.UID
	if err := repo.UpdateByID(link.ID, link); err != nil {
		t.Fatalf("UpdateByID() error = %v", err)
	}
	updated, err := repo.TxGet(db, link.ID)
	if err != nil {
		t.Fatalf("TxGet() error = %v", err)
	}
	if updated.Name != "Updated" || updated.CategoryUID != child.UID {
		t.Fatalf("updated link = %+v", updated)
	}
	if err := repo.DeleteByID(childLink.ID); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}
}

func TestFileRepoQueriesAndMutations(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &FileRepo{}
	generated := GenFileModel("generated", ".png")
	if generated.Name != "generated" || generated.ExtensionName != ".png" || generated.DirPath == "" {
		t.Fatalf("GenFileModel() = %+v", generated)
	}

	first := &FileModel{Name: "avatar", ExtensionName: ".png", UUID: "file-avatar", MD5: "md5-avatar", Note: "old"}
	second := &FileModel{Name: "banner", ExtensionName: ".jpg", UUID: "file-banner", MD5: "md5-banner"}
	for _, file := range []*FileModel{first, second} {
		if err := repo.Create(file); err != nil {
			t.Fatalf("Create(%s) error = %v", file.Name, err)
		}
	}
	got, err := repo.Get(first.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "avatar" {
		t.Fatalf("Get() = %+v", got)
	}
	first.Name = "avatar-new"
	first.ExtensionName = ".webp"
	first.Note = "new"
	if err := repo.UpdateBasicInfo(first); err != nil {
		t.Fatalf("UpdateBasicInfo() error = %v", err)
	}
	if total, err := repo.CountByName("avatar"); err != nil || total != 1 {
		t.Fatalf("CountByName(avatar) total=%d err=%v, want 1 nil", total, err)
	}
	if total, err := repo.CountByName(""); err != nil || total != 2 {
		t.Fatalf("CountByName(empty) total=%d err=%v, want 2 nil", total, err)
	}
	files, err := repo.PageQueryByName(&PageFindParam{NoPage: true}, "avatar")
	if err != nil {
		t.Fatalf("PageQueryByName(avatar) error = %v", err)
	}
	if len(files) != 1 || files[0].Name != "avatar-new" {
		t.Fatalf("PageQueryByName(avatar) = %+v", files)
	}
	files, err = repo.FindFilesByMD5s([]string{"md5-avatar", "missing"})
	if err != nil {
		t.Fatalf("FindFilesByMD5s() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("FindFilesByMD5s() len = %d, want 1", len(files))
	}
	if err := repo.DeleteByUUID(second.UUID); err != nil {
		t.Fatalf("DeleteByUUID() error = %v", err)
	}
	if err := repo.Delete(first.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestPaginationHelpersApplyDefaultsAndOffsets(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &SettingRepo{}
	for _, setting := range []*SettingModel{
		{Key: "A", Value: "a"},
		{Key: "B", Value: "b"},
		{Key: "C", Value: "c"},
	} {
		if err := repo.Create(setting); err != nil {
			t.Fatalf("Create setting %s: %v", setting.Key, err)
		}
	}
	param := &PageFindParam{Page: 2, PageSize: 1}
	var settings []SettingModel
	if err := db.Model(&SettingModel{}).Order("key").Scopes(Paginate(param)).Find(&settings).Error; err != nil {
		t.Fatalf("paginated query error = %v", err)
	}
	if len(settings) != 1 || settings[0].Key != "B" {
		t.Fatalf("paginated settings = %+v, want B", settings)
	}
	param = &PageFindParam{UseForceOffset: true, ForceOffset: 2, PageSize: 1}
	settings = nil
	if err := db.Model(&SettingModel{}).Order("key").Scopes(Paginate(param)).Find(&settings).Error; err != nil {
		t.Fatalf("forced offset query error = %v", err)
	}
	if len(settings) != 1 || settings[0].Key != "C" {
		t.Fatalf("forced settings = %+v, want C", settings)
	}
	noPage := &PageFindParam{NoPage: true}
	settings = nil
	if err := db.Model(&SettingModel{}).Order("key").Scopes(Paginate(noPage)).Find(&settings).Error; err != nil {
		t.Fatalf("no page query error = %v", err)
	}
	if len(settings) != 3 {
		t.Fatalf("no page settings len = %d, want 3", len(settings))
	}
	var info PageInfo
	info.FillInfoFromParam(&PageFindParam{})
	if info.Page != 1 || info.PageSize != 10 {
		t.Fatalf("FillInfoFromParam(default) = %+v", info)
	}
	info.FillInfoFromParam(&PageFindParam{Page: 3, PageSize: 7})
	if info.Page != 3 || info.PageSize != 7 {
		t.Fatalf("FillInfoFromParam(custom) = %+v", info)
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
	byName, err := repo.GetByName("custom")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if byName.ID != custom.ID {
		t.Fatalf("GetByName() ID = %d, want %d", byName.ID, custom.ID)
	}
	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("GetAll() len = %d, want 2", len(all))
	}
	custom.DisplayName = "Custom Updated"
	if err := repo.Update(custom); err != nil {
		t.Fatalf("Update() error = %v", err)
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
