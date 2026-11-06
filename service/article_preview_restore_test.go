package service

import (
	"encoding/json"
	"strings"
	"testing"

	"gorm.io/gorm"
	"myecho/dal"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

func TestRestoreInvalidatesArticlePreviewAndExportOmitsCredentials(t *testing.T) {
	setupServiceTestDB(t)
	owner, article := createArticlePreviewRestoreFixture(t)
	link, err := CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("CreateArticlePreview() error = %v", err)
	}
	if _, err := LoadArticlePreview(link.Token); err != nil {
		t.Fatalf("LoadArticlePreview() before restore error = %v", err)
	}

	var stored model.ArticlePreview
	if err := connect.Database.Where("article_id = ?", article.ID).First(&stored).Error; err != nil {
		t.Fatalf("load stored preview grant: %v", err)
	}
	data, err := loadExportData()
	if err != nil {
		t.Fatalf("load export data: %v", err)
	}
	serialized, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal export data: %v", err)
	}
	for _, credential := range []string{link.Token, stored.TokenHash, "article_previews"} {
		if strings.Contains(string(serialized), credential) {
			t.Fatalf("export JSON contains preview credential/table marker %q", credential)
		}
	}

	if err := connect.Database.Transaction(func(tx *gorm.DB) error {
		return replaceRestoreData(tx, exportData{}, owner.ID)
	}); err != nil {
		t.Fatalf("replaceRestoreData() transaction error = %v", err)
	}
	if _, err := LoadArticlePreview(link.Token); err == nil {
		t.Fatal("LoadArticlePreview() succeeded after successful restore; old grant must be invalidated")
	}
	var count int64
	if err := connect.Database.Model(&model.ArticlePreview{}).Count(&count).Error; err != nil {
		t.Fatalf("count preview grants after restore: %v", err)
	}
	if count != 0 {
		t.Fatalf("preview grants after restore = %d, want 0", count)
	}
}

func TestFailedRestoreTransactionPreservesArticlePreviewGrant(t *testing.T) {
	setupServiceTestDB(t)
	owner, article := createArticlePreviewRestoreFixture(t)
	link, err := CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("CreateArticlePreview() error = %v", err)
	}

	duplicateTagID := uint(987654)
	invalidData := exportData{Tags: []model.Tag{
		{BaseModel: model.BaseModel{ID: duplicateTagID}, UID: "restore-tag-a"},
		{BaseModel: model.BaseModel{ID: duplicateTagID}, UID: "restore-tag-b"},
	}}
	err = connect.Database.Transaction(func(tx *gorm.DB) error {
		return replaceRestoreData(tx, invalidData, owner.ID)
	})
	if err == nil {
		t.Fatal("replaceRestoreData() transaction succeeded with duplicate tag IDs, want rollback trigger")
	}
	if _, err := LoadArticlePreview(link.Token); err != nil {
		t.Fatalf("LoadArticlePreview() after failed restore error = %v, want existing grant", err)
	}
	var count int64
	if err := connect.Database.Model(&model.ArticlePreview{}).Count(&count).Error; err != nil {
		t.Fatalf("count preview grants after failed restore: %v", err)
	}
	if count != 1 {
		t.Fatalf("preview grants after failed restore = %d, want 1", count)
	}
}

func createArticlePreviewRestoreFixture(t *testing.T) (*model.User, *mysql.ArticleModel) {
	t.Helper()
	owner := &model.User{
		Name:           "Preview restore owner",
		Email:          "preview-restore@example.com",
		Token:          "preview-restore-token",
		PermissionType: model.Author,
	}
	if err := connect.Database.Create(owner).Error; err != nil {
		t.Fatalf("create preview restore owner: %v", err)
	}
	article := &mysql.ArticleModel{
		AuthorID:      owner.ID,
		Title:         "Article before restore",
		Status:        int8(mysql.ARTICLE_STATUS_DRAFT),
		ContentFormat: model.ArticleContentFormatMarkdown,
		Detail:        &model.ArticleDetail{Content: "body before restore"},
	}
	if err := dal.MySqlDB.Article.Create(article); err != nil {
		t.Fatalf("create preview restore article: %v", err)
	}
	return owner, article
}
