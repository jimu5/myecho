package service

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"gorm.io/gorm"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

type contentZipFile struct {
	name string
	body string
}

func buildContentZip(t *testing.T, files ...contentZipFile) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, file := range files {
		entry, err := writer.Create(file.name)
		if err != nil {
			t.Fatalf("create ZIP entry: %v", err)
		}
		if _, err := entry.Write([]byte(file.body)); err != nil {
			t.Fatalf("write ZIP entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return archive.Bytes()
}

func contentMarkdown(frontMatter, body string) string {
	return "---\n" + frontMatter + "---\n" + body
}

func TestContentPortableRoundTripAndUpdate(t *testing.T) {
	setupServiceTestDB(t)
	category := &mysql.CategoryModel{Name: "Engineering", UID: "category-engineering", Type: model.CategoryTypeArticle}
	if err := connect.Database.Create(category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	tag := &model.Tag{Name: "Go", UID: "tag-go"}
	if err := connect.Database.Create(tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	allowComment := false
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	article := &mysql.ArticleModel{
		BaseModel:      model.BaseModel{CreatedAt: createdAt, UpdatedAt: updatedAt},
		UID:            "portable-article",
		Title:          "Portable Post",
		Slug:           "portable-post",
		SEOTitle:       "Portable SEO",
		SEODescription: "Portable description",
		ShareImage:     "https://example.com/share.png",
		Type:           model.ArticleTypePost,
		ContentFormat:  model.ArticleContentFormatMarkdown,
		Summary:        "A portable summary",
		IsAllowComment: &allowComment,
		CategoryUID:    category.UID,
		PostTime:       createdAt.Add(24 * time.Hour),
		Status:         int8(mysql.ARTILCE_STATUS_PUBLIC),
		Detail:         &model.ArticleDetail{Content: "# Portable\n\nBody"},
		Tags:           []*model.Tag{tag},
	}
	if err := (&mysql.ArticleDBRepo{}).Create(article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	sourceCreatedAt := article.CreatedAt
	sourceUpdatedAt := article.UpdatedAt

	archive, err := CreateContentExportArchive()
	if err != nil {
		t.Fatalf("CreateContentExportArchive() error = %v", err)
	}
	zipReader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open exported ZIP: %v", err)
	}
	if len(zipReader.File) != 1 || zipReader.File[0].Name != "post/portable-post.md" {
		t.Fatalf("exported files = %v, want post/portable-post.md", zipReader.File)
	}
	exportedFile, err := zipReader.File[0].Open()
	if err != nil {
		t.Fatalf("open exported Markdown: %v", err)
	}
	exportedBody, err := io.ReadAll(exportedFile)
	_ = exportedFile.Close()
	if err != nil {
		t.Fatalf("read exported Markdown: %v", err)
	}
	for _, want := range []string{"title: Portable Post", "type: post", "category: Engineering", "- Go", "content_format: markdown", "# Portable\n\nBody"} {
		if !strings.Contains(string(exportedBody), want) {
			t.Fatalf("exported Markdown missing %q:\n%s", want, exportedBody)
		}
	}

	preview, err := PreviewContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("PreviewContentImportArchive() error = %v", err)
	}
	if !preview.Valid || preview.Files != 1 || preview.Created != 0 || preview.Updated != 1 || preview.CategoriesCreated != 0 || preview.TagsCreated != 0 {
		t.Fatalf("preview = %+v", preview)
	}
	result, err := ApplyContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("ApplyContentImportArchive() error = %v", err)
	}
	if !result.Applied || result.Created != 0 || result.Updated != 1 {
		t.Fatalf("apply result = %+v", result)
	}

	var stored mysql.ArticleModel
	if err := connect.Database.Preload("Detail").Preload("Tags").First(&stored, article.ID).Error; err != nil {
		t.Fatalf("load imported article: %v", err)
	}
	if stored.Detail == nil || stored.Detail.Content != "# Portable\n\nBody" || stored.Slug != article.Slug || stored.Status != article.Status {
		t.Fatalf("stored article = %+v detail=%+v", stored, stored.Detail)
	}
	if stored.CreatedAt.UTC() != sourceCreatedAt.UTC() || stored.UpdatedAt.UTC() != sourceUpdatedAt.UTC() {
		t.Fatalf("timestamps = %s / %s, want %s / %s", stored.CreatedAt, stored.UpdatedAt, sourceCreatedAt, sourceUpdatedAt)
	}
	if stored.IsAllowComment == nil || *stored.IsAllowComment {
		t.Fatalf("comment flag = %v, want false", stored.IsAllowComment)
	}
	var storedCategory mysql.CategoryModel
	if err := connect.Database.First(&storedCategory, category.ID).Error; err != nil {
		t.Fatalf("load category: %v", err)
	}
	if storedCategory.Count != 1 {
		t.Fatalf("category count = %d, want 1", storedCategory.Count)
	}
}

func TestContentPortableImportCreatesRelationsAndPreservesHTML(t *testing.T) {
	setupServiceTestDB(t)
	author := model.User{Name: "mapped-author", NickName: "Mapped", Email: "mapped@example.com", PermissionType: model.Author, Password: "hash"}
	if err := connect.Database.Create(&author).Error; err != nil {
		t.Fatalf("create mapped author: %v", err)
	}
	createdAt := "2026-02-03T04:05:06Z"
	updatedAt := "2026-02-03T05:05:06Z"
	markdown := contentMarkdown(strings.Join([]string{
		"title: About",
		"slug: about",
		"type: page",
		"status: 3",
		"summary: About page",
		"seo_title: About SEO",
		"seo_description: About description",
		"share_image: https://example.com/about.png",
		"category: Documentation",
		"tags:",
		"  - Go",
		"  - Docs",
		"post_time: 2026-02-03T06:05:06Z",
		"created_at: " + createdAt,
		"updated_at: " + updatedAt,
		"is_allow_comment: false",
		"content_format: html",
		"author: mapped-author",
		"",
	}, "\n"), "<p>keep this HTML</p>")
	archive := buildContentZip(t, contentZipFile{name: "page/about.md", body: markdown})
	preview, err := PreviewContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("preview error = %v", err)
	}
	if !preview.Valid || preview.Files != 1 || preview.Pages != 1 || preview.Created != 1 || preview.Updated != 0 || preview.CategoriesCreated != 1 || preview.TagsCreated != 2 || len(preview.UnmappedAuthors) != 0 {
		t.Fatalf("preview = %+v", preview)
	}
	var articleCount int64
	if err := connect.Database.Model(&mysql.ArticleModel{}).Count(&articleCount).Error; err != nil {
		t.Fatalf("count articles: %v", err)
	}
	if articleCount != 0 {
		t.Fatalf("dry-run created %d articles", articleCount)
	}

	result, err := ApplyContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("apply error = %v", err)
	}
	if !result.Applied || result.Created != 1 || result.CategoriesCreated != 1 || result.TagsCreated != 2 {
		t.Fatalf("apply result = %+v", result)
	}
	var article mysql.ArticleModel
	if err := connect.Database.Preload("Detail").Preload("Category").Preload("Tags").Where("slug = ? AND type = ?", "about", model.ArticleTypePage).First(&article).Error; err != nil {
		t.Fatalf("load imported page: %v", err)
	}
	if article.Detail == nil || article.Detail.Content != "<p>keep this HTML</p>" || article.ContentFormat != model.ArticleContentFormatHTML || article.Status != 3 {
		t.Fatalf("imported page = %+v detail=%+v", article, article.Detail)
	}
	if article.AuthorID != author.ID {
		t.Fatalf("article author = %d, want %d", article.AuthorID, author.ID)
	}
	if article.Category == nil || article.Category.Name != "Documentation" || len(article.Tags) != 2 {
		t.Fatalf("relations = category=%+v tags=%+v", article.Category, article.Tags)
	}
	if article.IsAllowComment == nil || *article.IsAllowComment {
		t.Fatalf("comment flag = %v, want false", article.IsAllowComment)
	}
	if article.CreatedAt.UTC().Format(time.RFC3339) != createdAt || article.UpdatedAt.UTC().Format(time.RFC3339) != updatedAt {
		t.Fatalf("timestamps = %s / %s", article.CreatedAt, article.UpdatedAt)
	}
}

func TestContentPortableImportRejectsUnsafePathAndRollsBack(t *testing.T) {
	setupServiceTestDB(t)
	unsafeArchive := buildContentZip(t, contentZipFile{name: "../escape.md", body: "---\ntitle: Bad\n---\n"})
	if _, err := PreviewContentImportArchive(bytes.NewReader(unsafeArchive), int64(len(unsafeArchive))); !errors.Is(err, ErrInvalidContentArchive) {
		t.Fatalf("unsafe path error = %v, want %v", err, ErrInvalidContentArchive)
	}

	first := contentMarkdown(strings.Join([]string{
		"title: First",
		"slug: first",
		"type: post",
		"status: 4",
		"post_time: 2026-03-01T00:00:00Z",
		"content_format: markdown",
		"category: Imported",
		"tags: [one]",
		"",
	}, "\n"), "first")
	second := contentMarkdown(strings.Join([]string{
		"title: Second",
		"slug: Second Slug",
		"type: page",
		"status: 4",
		"post_time: 2026-03-02T00:00:00Z",
		"content_format: markdown",
		"category: Imported",
		"tags: [two]",
		"",
	}, "\n"), "second")
	archive := buildContentZip(t,
		contentZipFile{name: "post/first.md", body: first},
		contentZipFile{name: "page/second.md", body: second},
	)
	if _, err := ApplyContentImportArchive(bytes.NewReader(archive), int64(len(archive))); !errors.Is(err, ErrContentImportConflict) {
		t.Fatalf("normalized slug error = %v, want %v", err, ErrContentImportConflict)
	}
	var articles, categories, tags int64
	if err := connect.Database.Model(&mysql.ArticleModel{}).Count(&articles).Error; err != nil {
		t.Fatalf("count articles: %v", err)
	}
	if err := connect.Database.Model(&mysql.CategoryModel{}).Count(&categories).Error; err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if err := connect.Database.Model(&model.Tag{}).Count(&tags).Error; err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if articles != 0 || categories != 0 || tags != 0 {
		t.Fatalf("failed import left rows: articles=%d categories=%d tags=%d", articles, categories, tags)
	}
}

func TestContentPortableMediaRoundTrip(t *testing.T) {
	setupServiceTestDB(t)
	data := []byte("portable media")
	digest := md5.Sum(data)
	file := model.File{
		Name: "asset", DirPath: filepath.ToSlash(filepath.Join("content-portability-test", t.Name())),
		UUID: "12345678-1234-1234-1234-123456789012", ExtensionName: ".txt",
		MD5: hex.EncodeToString(digest[:]), AltText: "portable asset",
	}
	if err := os.MkdirAll(file.GetActualSaveDir(), 0755); err != nil {
		t.Fatalf("create media dir: %v", err)
	}
	defer os.RemoveAll(filepath.Join("storage", "mos", "content-portability-test"))
	if err := os.WriteFile(file.GetActualSavePath(), data, 0644); err != nil {
		t.Fatalf("write media: %v", err)
	}
	if err := connect.Database.Session(&gorm.Session{SkipHooks: true}).Create(&file).Error; err != nil {
		t.Fatalf("create media row: %v", err)
	}
	article := &mysql.ArticleModel{
		UID: "portable-media-article", Title: "Media", Slug: "media", Type: model.ArticleTypePost,
		ContentFormat: model.ArticleContentFormatMarkdown, Status: int8(mysql.ARTICLE_STATUS_DRAFT),
		PostTime: time.Now(), Detail: &model.ArticleDetail{Content: "[asset](" + file.GetUrlPath() + ")"},
	}
	if err := (&mysql.ArticleDBRepo{}).Create(article); err != nil {
		t.Fatalf("create article: %v", err)
	}

	archive, err := CreateContentExportArchiveWithMedia(true)
	if err != nil {
		t.Fatalf("export media archive: %v", err)
	}
	preview, err := PreviewContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || !preview.Valid {
		t.Fatalf("preview media archive = %+v, %v", preview, err)
	}
	if err := connect.Database.Unscoped().Delete(&file).Error; err != nil {
		t.Fatalf("delete media row: %v", err)
	}
	if err := os.Remove(file.GetActualSavePath()); err != nil {
		t.Fatalf("delete media file: %v", err)
	}
	if _, err := ApplyContentImportArchive(bytes.NewReader(archive), int64(len(archive))); err != nil {
		t.Fatalf("import media archive: %v", err)
	}
	restored, err := os.ReadFile(file.GetActualSavePath())
	if err != nil || !bytes.Equal(restored, data) {
		t.Fatalf("restored media = %q, %v", restored, err)
	}
	var count int64
	if err := connect.Database.Model(&model.File{}).Where("uuid = ?", file.UUID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("restored media rows = %d, %v", count, err)
	}
}
