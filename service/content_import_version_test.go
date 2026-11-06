package service

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"gopkg.in/yaml.v2"

	"myecho/config/static_config"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/utils"
)

func TestContentImportUpdateUsesExpectedVersionAndRecordsRevision(t *testing.T) {
	setupServiceTestDB(t)
	allowComment := false
	original := &mysql.ArticleModel{
		UID: "content-import-version", Title: "Before", Slug: "import-target",
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Summary: "old summary", Status: int8(mysql.ARTICLE_STATUS_PRIVATE),
		PostTime:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EditVersion: 7, Password: "stored-hash", IsAllowComment: &allowComment,
		Detail: &model.ArticleDetail{Content: "before body"},
	}
	if err := (&mysql.ArticleDBRepo{}).Create(original); err != nil {
		t.Fatalf("create original article: %v", err)
	}
	if original.EditVersion != 7 {
		t.Fatalf("initial edit version = %d, want 7", original.EditVersion)
	}
	targetAuthor := model.User{Name: "target-author", NickName: "Target", Email: "target@example.com", PermissionType: model.Author}
	actor := model.User{Name: "import-admin", NickName: "Admin", Email: "admin@example.com", PermissionType: model.Admin}
	if err := connect.Database.Create(&targetAuthor).Error; err != nil {
		t.Fatalf("create target author: %v", err)
	}
	if err := connect.Database.Create(&actor).Error; err != nil {
		t.Fatalf("create import actor: %v", err)
	}

	createdAt := "2026-02-03T04:05:06Z"
	updatedAt := "2026-02-03T05:05:06Z"
	archive := buildContentZip(t, contentZipFile{name: "post/import-target.md", body: contentMarkdown(
		"title: After\nslug: import-target\ntype: post\nstatus: 4\nsummary: new summary\npost_time: 2026-02-04T00:00:00Z\ncreated_at: "+createdAt+"\nupdated_at: "+updatedAt+"\nedit_version: 7\ncontent_format: markdown\nauthor: target-author\n",
		"after body",
	)})
	result, err := ApplyContentImportArchiveWithOptions(bytes.NewReader(archive), int64(len(archive)), ContentImportOptions{
		AuthorMap: map[string]uint{"target-author": targetAuthor.ID}, ActorID: actor.ID,
	})
	if err != nil {
		t.Fatalf("apply versioned update: %v", err)
	}
	if !result.Applied || result.Updated != 1 || len(result.Plans) != 1 || result.Plans[0].Action != "update" || result.Plans[0].ExpectedVersion != 7 || result.Plans[0].CurrentVersion != 7 {
		t.Fatalf("apply result = %+v", result)
	}

	var stored mysql.ArticleModel
	if err := connect.Database.Preload("Detail").First(&stored, original.ID).Error; err != nil {
		t.Fatalf("load updated article: %v", err)
	}
	if stored.EditVersion != 8 || stored.Title != "After" || stored.Detail == nil || stored.Detail.Content != "after body" {
		t.Fatalf("updated article version/title/content = %d/%q/%+v", stored.EditVersion, stored.Title, stored.Detail)
	}
	if stored.Password != "stored-hash" || stored.IsAllowComment == nil || *stored.IsAllowComment || stored.AuthorID != targetAuthor.ID {
		t.Fatalf("preserved/mapped fields: password=%q allow_comment=%v author_id=%d", stored.Password, stored.IsAllowComment, stored.AuthorID)
	}
	if stored.CreatedAt.UTC().Format(time.RFC3339) != createdAt || stored.UpdatedAt.UTC().Format(time.RFC3339) != updatedAt {
		t.Fatalf("timestamps = %s / %s", stored.CreatedAt, stored.UpdatedAt)
	}
	var revisions []model.ArticleRevision
	if err := connect.Database.Where("article_id = ?", original.ID).Order("id").Find(&revisions).Error; err != nil {
		t.Fatalf("load article revisions: %v", err)
	}
	if len(revisions) != 1 || revisions[0].ActorID != actor.ID || revisions[0].Title != "Before" || revisions[0].Content != "before body" {
		t.Fatalf("before revision = %+v, want original state and import actor %d", revisions, actor.ID)
	}
}

func TestContentImportRejectsMissingAndStaleVersionsWithoutWrites(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		version      string
		wantExpected uint64
	}{
		{name: "missing", version: "", wantExpected: 0},
		{name: "stale", version: "edit_version: 2\n", wantExpected: 2},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			setupServiceTestDB(t)
			article := contentImportVersionArticle(t, 3)
			archive := contentImportVersionArchive(t, article.Slug, "Imported", testCase.version)

			preview, err := PreviewContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
			if err != nil {
				t.Fatalf("preview: %v", err)
			}
			if preview.Valid || preview.Updated != 1 || len(preview.Conflicts) == 0 || len(preview.Plans) != 1 || preview.Plans[0].Action != "update" || preview.Plans[0].CurrentVersion != 3 || preview.Plans[0].ExpectedVersion != testCase.wantExpected {
				t.Fatalf("preview result = %+v", preview)
			}
			result, err := ApplyContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
			if !errors.Is(err, ErrContentImportConflict) || result.Applied {
				t.Fatalf("apply stale/missing token = %+v, %v; want conflict", result, err)
			}
			assertContentImportVersionArticle(t, article.ID, 3, "Before", "before body")
			var revisions int64
			if err := connect.Database.Model(&model.ArticleRevision{}).Where("article_id = ?", article.ID).Count(&revisions).Error; err != nil {
				t.Fatalf("count revisions: %v", err)
			}
			if revisions != 0 {
				t.Fatalf("failed import created %d revisions", revisions)
			}
		})
	}
}

func TestContentImportApplyRejectsWriterAfterPreview(t *testing.T) {
	setupServiceTestDB(t)
	article := contentImportVersionArticle(t, 2)
	archive := contentImportVersionArchive(t, article.Slug, "Imported", "edit_version: 2\n")
	preview, err := PreviewContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || !preview.Valid || preview.Plans[0].CurrentVersion != 2 {
		t.Fatalf("preview = %+v, %v", preview, err)
	}

	var candidate mysql.ArticleModel
	if err := connect.Database.Preload("Detail").First(&candidate, article.ID).Error; err != nil {
		t.Fatalf("load writer candidate: %v", err)
	}
	candidate.Title = "Another writer"
	candidate.Detail.Content = "writer body"
	expected := candidate.EditVersion
	if err := (&mysql.ArticleDBRepo{}).UpdateForEditing(&candidate, &expected, 91); err != nil {
		t.Fatalf("apply another writer: %v", err)
	}

	result, err := ApplyContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
	if !errors.Is(err, ErrContentImportConflict) || result.Applied {
		t.Fatalf("apply after writer = %+v, %v; want conflict", result, err)
	}
	assertContentImportVersionArticle(t, article.ID, 3, "Another writer", "writer body")
}

func TestContentImportPreviewRejectsPendingAndTrashedArticles(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		status        int8
		pendingReview bool
	}{
		{name: "awaiting review", status: int8(mysql.ARTICLE_STATUS_WAIT_REVIEW)},
		{name: "pending submission", status: int8(mysql.ARTICLE_STATUS_PRIVATE), pendingReview: true},
		{name: "trashed", status: int8(mysql.ARTICLE_STATUS_RECYCLE)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			setupServiceTestDB(t)
			article := contentImportVersionArticle(t, 5)
			if err := connect.Database.Model(&mysql.ArticleModel{}).Where("id = ?", article.ID).
				UpdateColumn("status", testCase.status).Error; err != nil {
				t.Fatalf("set article status: %v", err)
			}
			if testCase.pendingReview {
				pending := model.ArticleSubmission{
					ArticleID: article.ID, SubmitterID: 17, Status: model.ArticleSubmissionPending, Snapshot: `{"title":"pending"}`,
				}
				if err := connect.Database.Create(&pending).Error; err != nil {
					t.Fatalf("create pending submission: %v", err)
				}
			}
			archive := contentImportVersionArchive(t, article.Slug, "Imported", "edit_version: 5\n")
			preview, err := PreviewContentImportArchive(bytes.NewReader(archive), int64(len(archive)))
			if err != nil {
				t.Fatalf("preview: %v", err)
			}
			if preview.Valid || len(preview.Conflicts) == 0 {
				t.Fatalf("preview = %+v, want review/trash conflict", preview)
			}
			if _, err := ApplyContentImportArchive(bytes.NewReader(archive), int64(len(archive))); !errors.Is(err, ErrContentImportConflict) {
				t.Fatalf("apply conflict = %v, want %v", err, ErrContentImportConflict)
			}
			assertContentImportVersionArticle(t, article.ID, 5, "Before", "before body")
		})
	}
}

func TestContentImportConflictRollsBackPriorRowsTaxonomyAndMedia(t *testing.T) {
	setupServiceTestDB(t)
	article := contentImportVersionArticle(t, 4)
	first := contentImportVersionMarkdown(article.Slug, "Imported", "edit_version: 4\n", "category: Rollback category\ntags: [rollback-tag]\n")
	second := contentMarkdown("title: New page\nslug: new-page\ntype: page\nstatus: 4\npost_time: 2026-02-04T00:00:00Z\ncontent_format: markdown\nauthor: bad-author\n", "new page")
	mediaData := []byte("rollback media")
	digest := md5.Sum(mediaData)
	file := model.File{
		Name: "rollback asset", DirPath: "content-import-rollback-test", UUID: utils.GenUID20(),
		ExtensionName: ".txt", MD5: hex.EncodeToString(digest[:]),
	}
	defer os.RemoveAll(file.GetActualSaveDir())
	archivePath, _, err := contentMediaPaths(&file)
	if err != nil {
		t.Fatalf("media paths: %v", err)
	}
	media := contentMediaEntry{
		URL:         path.Join(static_config.StorageRootUrl, file.DirPath, file.UUID) + file.ExtensionName,
		ArchivePath: archivePath, Name: file.Name, DirPath: file.DirPath, UUID: file.UUID,
		ExtensionName: file.ExtensionName, MD5: file.MD5,
	}
	manifest, err := yaml.Marshal(contentMediaManifest{Version: 1, Files: []contentMediaEntry{media}})
	if err != nil {
		t.Fatalf("encode media manifest: %v", err)
	}
	archive := buildContentZip(t,
		contentZipFile{name: "post/first.md", body: first},
		contentZipFile{name: "page/new-page.md", body: second},
		contentZipFile{name: "media/manifest.yaml", body: string(manifest)},
		contentZipFile{name: archivePath, body: string(mediaData)},
	)
	result, err := ApplyContentImportArchiveWithOptions(bytes.NewReader(archive), int64(len(archive)), ContentImportOptions{
		AuthorMap: map[string]uint{"bad-author": 999999},
	})
	if !errors.Is(err, ErrContentImportConflict) || result.Applied {
		t.Fatalf("mixed import = %+v, %v; want transaction conflict", result, err)
	}
	assertContentImportVersionArticle(t, article.ID, 4, "Before", "before body")
	var articles, categories, tags, files, revisions int64
	if err := connect.Database.Model(&mysql.ArticleModel{}).Where("id <> ?", article.ID).Count(&articles).Error; err != nil {
		t.Fatalf("count new articles after rollback: %v", err)
	}
	if err := connect.Database.Model(&mysql.CategoryModel{}).Count(&categories).Error; err != nil {
		t.Fatalf("count categories after rollback: %v", err)
	}
	if err := connect.Database.Model(&model.Tag{}).Count(&tags).Error; err != nil {
		t.Fatalf("count tags after rollback: %v", err)
	}
	if err := connect.Database.Model(&model.File{}).Count(&files).Error; err != nil {
		t.Fatalf("count media rows after rollback: %v", err)
	}
	if err := connect.Database.Model(&model.ArticleRevision{}).Count(&revisions).Error; err != nil {
		t.Fatalf("count revisions after rollback: %v", err)
	}
	if articles != 0 || categories != 0 || tags != 0 || files != 0 || revisions != 0 {
		t.Fatalf("rollback left articles/categories/tags/files/revisions = %d/%d/%d/%d/%d", articles, categories, tags, files, revisions)
	}
	if _, err := os.Stat(file.GetActualSavePath()); !os.IsNotExist(err) {
		t.Fatalf("rollback media file stat error = %v, want missing", err)
	}
}

func contentImportVersionArticle(t *testing.T, version uint64) *mysql.ArticleModel {
	t.Helper()
	article := &mysql.ArticleModel{
		UID: "content-import-test", Title: "Before", Slug: "import-version-test",
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Status: int8(mysql.ARTICLE_STATUS_PRIVATE), EditVersion: version,
		PostTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Detail:   &model.ArticleDetail{Content: "before body"},
	}
	if err := (&mysql.ArticleDBRepo{}).Create(article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	return article
}

func contentImportVersionArchive(t *testing.T, slug, title, extra string) []byte {
	t.Helper()
	return buildContentZip(t, contentZipFile{
		name: "post/" + slug + ".md",
		body: contentImportVersionMarkdown(slug, title, extra, ""),
	})
}

func contentImportVersionMarkdown(slug, title, extra, more string) string {
	return contentMarkdown("title: "+title+"\nslug: "+slug+"\ntype: post\nstatus: 4\npost_time: 2026-02-04T00:00:00Z\ncontent_format: markdown\n"+extra+more, "imported body")
}

func assertContentImportVersionArticle(t *testing.T, articleID uint, version uint64, title, body string) {
	t.Helper()
	var stored mysql.ArticleModel
	if err := connect.Database.Preload("Detail").First(&stored, articleID).Error; err != nil {
		t.Fatalf("load article: %v", err)
	}
	if stored.EditVersion != version || stored.Title != title || stored.Detail == nil || stored.Detail.Content != body {
		t.Fatalf("article version/title/content = %d/%q/%+v, want %d/%q/%q", stored.EditVersion, stored.Title, stored.Detail, version, title, body)
	}
}

func TestContentImportExportCarriesEditVersion(t *testing.T) {
	setupServiceTestDB(t)
	article := contentImportVersionArticle(t, 13)
	archive, err := CreateContentExportArchive()
	if err != nil {
		t.Fatalf("export archive: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	file, err := reader.File[0].Open()
	if err != nil {
		t.Fatalf("open exported Markdown: %v", err)
	}
	body, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil {
		t.Fatalf("read exported Markdown: %v", err)
	}
	entry, err := parseContentMarkdown(reader.File[0].Name, body)
	if err != nil || !entry.hasVersion || entry.expectedVersion != article.EditVersion {
		t.Fatalf("exported version = %d present=%v, err=%v; want %d", entry.expectedVersion, entry.hasVersion, err, article.EditVersion)
	}
}

func TestContentImportChangedFieldsMatchesAuthorSlug(t *testing.T) {
	article := &mysql.ArticleModel{
		Title: "Title", Slug: "article", Status: int8(mysql.ARTICLE_STATUS_PRIVATE),
		ContentFormat: model.ArticleContentFormatMarkdown,
		PostTime:      time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC),
		Detail:        &model.ArticleDetail{Content: "body"},
		Author:        &model.User{Name: "Target Author", Slug: "target-author"},
	}
	entry := contentImportEntry{
		title: "Title", slug: "article", status: int8(mysql.ARTICLE_STATUS_PRIVATE),
		contentFormat: model.ArticleContentFormatMarkdown,
		postTime:      article.PostTime,
		content:       "body", author: "target-author",
	}
	for _, field := range contentChangedFields(article, entry) {
		if field == "author" {
			t.Fatal("author slug was reported as a changed field")
		}
	}
}
