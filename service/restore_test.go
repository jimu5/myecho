package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myecho/dal/connect"
	"myecho/model"
)

func TestPreviewRestoreArchiveValidation_BitsUT(t *testing.T) {
	data := exportData{
		Version:    1,
		ExportedAt: time.Now(),
		Users: []exportUser{{
			BaseModel: model.BaseModel{ID: 1},
			Name:      "admin",
		}},
		Articles: []exportArticle{{
			BaseModel:      model.BaseModel{ID: 1},
			UID:            "article",
			AuthorID:       1,
			Title:          "Article",
			Slug:           "article",
			SEOTitle:       "SEO title",
			SEODescription: "SEO description",
			ShareImage:     "/share.png",
			Type:           model.ArticleTypePost,
			ContentFormat:  model.ArticleContentFormatMarkdown,
			DetailUID:      "detail",
			Status:         1,
		}},
		ArticleDetails: []model.ArticleDetail{{ID: 1, UID: "detail", Content: "content"}},
		Revisions:      []model.ArticleRevision{{ID: 1, ArticleID: 1}},
		SlugRedirects:  []model.ArticleSlugRedirect{{ID: 1, ArticleUID: "article", Slug: "old", Type: model.ArticleTypePost}},
		DailyStats:     []model.ArticleDailyStat{{ID: 1, ArticleUID: "article", Date: "2026-07-27", Views: 3}},
		Settings:       []exportSetting{{BaseModel: model.BaseModel{ID: 1}, Key: "SiteTitle", Value: "Myecho"}},
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	validArchive := buildRestoreArchive(t, map[string][]byte{
		"data.json":                 dataJSON,
		"storage/uploads/image.txt": []byte("asset"),
	})

	preview, err := PreviewRestoreArchive(bytes.NewReader(validArchive), int64(len(validArchive)))
	if err != nil {
		t.Fatalf("PreviewRestoreArchive() error = %v", err)
	}
	if preview.Counts.Articles != 1 || preview.Counts.Revisions != 1 || preview.Counts.SlugRedirects != 1 ||
		preview.Counts.DailyStats != 1 || preview.StorageFiles != 1 || preview.StorageBytes != 5 {
		t.Fatalf("restore preview = %+v", preview)
	}

	duplicateData := data
	duplicateData.Articles = append(duplicateData.Articles, duplicateData.Articles[0])
	duplicateJSON, _ := json.Marshal(duplicateData)
	sensitiveData := data
	sensitiveData.Settings = []exportSetting{{BaseModel: model.BaseModel{ID: 2}, Key: "CommentNotificationWebhook", Value: "https://203.0.113.1/hook"}}
	sensitiveJSON, _ := json.Marshal(sensitiveData)
	missingDetailData := data
	missingDetailData.Articles = append([]exportArticle(nil), data.Articles...)
	missingDetailData.Articles[0].DetailUID = "missing"
	missingDetailJSON, _ := json.Marshal(missingDetailData)
	cyclicCommentData := data
	cyclicCommentData.Comments = []exportComment{
		{BaseModel: model.BaseModel{ID: 1}, ArticleUID: "article", ParentID: 2},
		{BaseModel: model.BaseModel{ID: 2}, ArticleUID: "article", ParentID: 1},
	}
	cyclicCommentJSON, _ := json.Marshal(cyclicCommentData)
	emptySlugData := data
	emptySlugData.Articles = append([]exportArticle(nil), data.Articles...)
	emptySlugData.Articles[0].Slug = " "
	emptySlugJSON, _ := json.Marshal(emptySlugData)
	invalidCommentStatusData := data
	invalidStatus := int8(9)
	invalidCommentStatusData.Comments = []exportComment{{BaseModel: model.BaseModel{ID: 1}, ArticleUID: "article", Status: &invalidStatus}}
	invalidCommentStatusJSON, _ := json.Marshal(invalidCommentStatusData)
	cyclicCategoryData := data
	cyclicCategoryData.Categories = []model.Category{
		{BaseModel: model.BaseModel{ID: 1}, UID: "first", FatherUID: "second", Type: model.CategoryTypeArticle},
		{BaseModel: model.BaseModel{ID: 2}, UID: "second", FatherUID: "first", Type: model.CategoryTypeArticle},
	}
	cyclicCategoryJSON, _ := json.Marshal(cyclicCategoryData)
	missingLinkCategoryData := data
	missingLinkCategoryData.Links = []model.Link{{BaseModel: model.BaseModel{ID: 1}, CategoryUID: "missing"}}
	missingLinkCategoryJSON, _ := json.Marshal(missingLinkCategoryData)
	for _, tc := range []struct {
		name    string
		archive []byte
	}{
		{name: "missing data", archive: buildRestoreArchive(t, map[string][]byte{"storage/file.txt": []byte("x")})},
		{name: "unsafe path", archive: buildRestoreArchive(t, map[string][]byte{"data.json": dataJSON, "../escape": []byte("x")})},
		{name: "unknown schema field", archive: buildRestoreArchive(t, map[string][]byte{"data.json": []byte(`{"version":1,"exported_at":"2026-07-27T00:00:00Z","unknown":true}`)})},
		{name: "duplicate record id", archive: buildRestoreArchive(t, map[string][]byte{"data.json": duplicateJSON})},
		{name: "sensitive setting", archive: buildRestoreArchive(t, map[string][]byte{"data.json": sensitiveJSON})},
		{name: "missing article detail", archive: buildRestoreArchive(t, map[string][]byte{"data.json": missingDetailJSON})},
		{name: "cyclic comments", archive: buildRestoreArchive(t, map[string][]byte{"data.json": cyclicCommentJSON})},
		{name: "empty article slug", archive: buildRestoreArchive(t, map[string][]byte{"data.json": emptySlugJSON})},
		{name: "invalid comment status", archive: buildRestoreArchive(t, map[string][]byte{"data.json": invalidCommentStatusJSON})},
		{name: "cyclic categories", archive: buildRestoreArchive(t, map[string][]byte{"data.json": cyclicCategoryJSON})},
		{name: "missing link category", archive: buildRestoreArchive(t, map[string][]byte{"data.json": missingLinkCategoryJSON})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := PreviewRestoreArchive(bytes.NewReader(tc.archive), int64(len(tc.archive))); !errors.Is(err, ErrInvalidBackup) {
				t.Fatalf("PreviewRestoreArchive() error = %v, want ErrInvalidBackup", err)
			}
		})
	}
	if _, err := PreviewRestoreArchive(bytes.NewReader(validArchive), maxExportBytes+1); !errors.Is(err, ErrRestoreTooLarge) {
		t.Fatalf("oversized PreviewRestoreArchive() error = %v, want ErrRestoreTooLarge", err)
	}
}

func TestRestoreStorageAndDatabaseRollbackHelpers_BitsUT(t *testing.T) {
	setupServiceTestDB(t)
	private := false
	current := []model.Setting{
		{Key: "Keep", Value: "original", IsPublic: &private},
		{Key: "CommentNotificationWebhook", Value: "https://203.0.113.1/hook", IsPublic: &private},
	}
	if err := connect.Database.Create(&current).Error; err != nil {
		t.Fatalf("create current settings: %v", err)
	}
	data := exportData{
		Version:    1,
		ExportedAt: time.Now(),
		Settings: []exportSetting{{
			BaseModel: model.BaseModel{ID: current[0].ID},
			Key:       "Restored",
			Value:     "new",
		}},
	}
	tx := connect.Database.Begin()
	if err := replaceRestoreData(tx, data, 0); err != nil {
		t.Fatalf("replaceRestoreData() error = %v", err)
	}
	var transactionSettings []model.Setting
	if err := tx.Order("id").Find(&transactionSettings).Error; err != nil {
		t.Fatalf("query transaction settings: %v", err)
	}
	if len(transactionSettings) != 2 || transactionSettings[0].Key != "Restored" || transactionSettings[1].Key != "CommentNotificationWebhook" {
		t.Fatalf("transaction settings = %+v", transactionSettings)
	}
	if transactionSettings[0].IsPublic == nil || *transactionSettings[0].IsPublic {
		t.Fatalf("legacy setting visibility = %v, want private", transactionSettings[0].IsPublic)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	var persisted []model.Setting
	if err := connect.Database.Order("id").Find(&persisted).Error; err != nil {
		t.Fatalf("query persisted settings: %v", err)
	}
	if len(persisted) != 2 || persisted[0].Key != "Keep" || persisted[1].Key != "CommentNotificationWebhook" {
		t.Fatalf("settings after rollback = %+v", persisted)
	}

	chdirServiceTestTemp(t)
	storageRoot, err := safeStorageRoot()
	if err != nil {
		t.Fatalf("safeStorageRoot() error = %v", err)
	}
	extractRoot := filepath.Join(t.TempDir(), "extract")
	if err := extractRestoreFile(strings.NewReader("content"), extractRoot, "nested/file.txt"); err != nil {
		t.Fatalf("extractRestoreFile() error = %v", err)
	}
	if !isPathInside(extractRoot, filepath.Join(extractRoot, "nested", "file.txt")) ||
		isPathInside(extractRoot, filepath.Join(extractRoot, "..", "escape")) {
		t.Fatal("isPathInside() returned an unsafe result")
	}
	if err := extractRestoreFile(strings.NewReader("escape"), extractRoot, "../escape"); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("extractRestoreFile(unsafe) error = %v, want ErrInvalidBackup", err)
	}

	stageRoot := filepath.Join(filepath.Dir(storageRoot), "stage")
	if err := os.MkdirAll(storageRoot, 0755); err != nil {
		t.Fatalf("create storage root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storageRoot, "state.txt"), []byte("old"), 0644); err != nil {
		t.Fatalf("write old storage: %v", err)
	}
	if err := os.MkdirAll(stageRoot, 0755); err != nil {
		t.Fatalf("create stage root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stageRoot, "state.txt"), []byte("new"), 0644); err != nil {
		t.Fatalf("write staged storage: %v", err)
	}
	oldRoot, hadOld, err := swapStorageRoot(storageRoot, stageRoot)
	if err != nil {
		t.Fatalf("swapStorageRoot() error = %v", err)
	}
	if !hadOld {
		t.Fatal("swapStorageRoot() did not preserve the old root")
	}
	if err := rollbackStorageRoot(storageRoot, oldRoot, hadOld); err != nil {
		t.Fatalf("rollbackStorageRoot() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(storageRoot, "state.txt"))
	if err != nil || string(content) != "old" {
		t.Fatalf("rolled-back storage = %q, %v", content, err)
	}
}

func TestReplaceRestoreDataRestoresSensitiveArchive_BitsUT(t *testing.T) {
	setupServiceTestDB(t)
	private := false
	status := int8(model.CommentStatusApproved)
	data := exportData{
		Version:           1,
		ExportedAt:        time.Now(),
		IncludesSensitive: true,
		Users: []exportUser{{
			BaseModel: model.BaseModel{ID: 1},
			Name:      "admin",
			Email:     "restored@example.com",
			Password:  "password-hash",
			Token:     "restored-token",
		}},
		ArticleDetails: []model.ArticleDetail{{ID: 1, UID: "detail", Content: "content"}},
		Articles: []exportArticle{{
			BaseModel:     model.BaseModel{ID: 1},
			UID:           "article",
			AuthorID:      1,
			Title:         "Article",
			Slug:          "article",
			Type:          model.ArticleTypePost,
			ContentFormat: model.ArticleContentFormatMarkdown,
			DetailUID:     "detail",
			Status:        1,
			Password:      "article-password-hash",
		}},
		Comments: []exportComment{{
			BaseModel:   model.BaseModel{ID: 1},
			ArticleUID:  "article",
			AuthorName:  "Alice",
			AuthorEmail: "alice@example.com",
			AuthorIP:    "203.0.113.10",
			AuthorAgent: "browser",
			Content:     "comment",
			Status:      &status,
		}},
		Settings: []exportSetting{{
			BaseModel: model.BaseModel{ID: 1},
			Key:       "CommentNotificationWebhook",
			Value:     "https://203.0.113.1/hook",
			IsPublic:  &private,
		}},
	}
	if err := validateRestoreData(data); err != nil {
		t.Fatalf("validateRestoreData() error = %v", err)
	}
	tx := connect.Database.Begin()
	if err := replaceRestoreData(tx, data, 0); err != nil {
		_ = tx.Rollback().Error
		t.Fatalf("replaceRestoreData() error = %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit restore: %v", err)
	}
	var user model.User
	if err := connect.Database.First(&user, 1).Error; err != nil {
		t.Fatalf("load restored user: %v", err)
	}
	if user.Email != "restored@example.com" || user.Password != "password-hash" || user.Token != "restored-token" {
		t.Fatalf("restored user credentials = %+v", user)
	}
	var article model.Article
	if err := connect.Database.First(&article, 1).Error; err != nil {
		t.Fatalf("load restored article: %v", err)
	}
	if article.Password != "article-password-hash" {
		t.Fatalf("restored article password = %q", article.Password)
	}
	var comment model.Comment
	if err := connect.Database.First(&comment, 1).Error; err != nil {
		t.Fatalf("load restored comment: %v", err)
	}
	if comment.AuthorEmail != "alice@example.com" || comment.AuthorIP != "203.0.113.10" || comment.AuthorAgent != "browser" {
		t.Fatalf("restored comment private fields = %+v", comment)
	}
	var setting model.Setting
	if err := connect.Database.Where("key = ?", "CommentNotificationWebhook").First(&setting).Error; err != nil {
		t.Fatalf("load restored sensitive setting: %v", err)
	}
	if setting.Value != "https://203.0.113.1/hook" {
		t.Fatalf("restored sensitive setting = %+v", setting)
	}
}

func buildRestoreArchive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%q) error = %v", name, err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}
