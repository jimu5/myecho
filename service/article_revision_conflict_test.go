package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

func TestRestoreArticleRevisionRejectsConcurrentEdit(t *testing.T) {
	setupServiceTestDB(t)
	detail := model.ArticleDetail{UID: "revision-conflict-detail", Content: "published body"}
	if err := connect.Database.Create(&detail).Error; err != nil {
		t.Fatalf("create article detail: %v", err)
	}
	article := model.Article{
		UID:           "revision-conflict-article",
		Title:         "Published article",
		Slug:          "published-article",
		Type:          model.ArticleTypePost,
		ContentFormat: model.ArticleContentFormatMarkdown,
		DetailUID:     detail.UID,
		Status:        int8(mysql.ARTILCE_STATUS_PUBLIC),
		EditVersion:   1,
	}
	if err := connect.Database.Create(&article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}
	revision := model.ArticleRevision{
		ArticleID:     article.ID,
		Title:         "Older revision",
		Slug:          "older-revision",
		Type:          model.ArticleTypePost,
		ContentFormat: model.ArticleContentFormatMarkdown,
		Content:       "older revision body",
	}
	if err := connect.Database.Create(&revision).Error; err != nil {
		t.Fatalf("create article revision: %v", err)
	}

	triggered := false
	var injectionErr error
	queryCallbacks := connect.Database.Callback().Query()
	const callbackName = "test:restore-article-revision-concurrent-edit"
	if err := queryCallbacks.Before("gorm:query").Register(callbackName, func(query *gorm.DB) {
		if query.Statement.Table != "article_revisions" || triggered {
			return
		}
		triggered = true
		if err := connect.Database.Model(&model.Article{}).Where("id = ?", article.ID).Updates(map[string]interface{}{
			"title":        "Concurrent draft",
			"status":       int8(mysql.ARTICLE_STATUS_DRAFT),
			"edit_version": gorm.Expr("edit_version + 1"),
		}).Error; err != nil {
			injectionErr = err
			return
		}
		injectionErr = connect.Database.Model(&model.ArticleDetail{}).
			Where("uid = ?", detail.UID).
			Update("content", "concurrent draft body").Error
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() { _ = queryCallbacks.Remove(callbackName) })

	if _, err := (&ArticleService{}).RestoreArticleRevision(article.ID, revision.ID); !errors.Is(err, mysql.ErrArticleEditConflict) {
		t.Fatalf("RestoreArticleRevision() error = %v, want ErrArticleEditConflict", err)
	}
	if !triggered {
		t.Fatal("concurrent edit callback did not run")
	}
	if injectionErr != nil {
		t.Fatalf("simulate concurrent edit: %v", injectionErr)
	}

	var current model.Article
	if err := connect.Database.Preload("Detail").First(&current, article.ID).Error; err != nil {
		t.Fatalf("load current article: %v", err)
	}
	if current.Status != int8(mysql.ARTICLE_STATUS_DRAFT) {
		t.Fatalf("current article status = %d, want draft status %d", current.Status, mysql.ARTICLE_STATUS_DRAFT)
	}
	if current.Title != "Concurrent draft" || current.Detail == nil || current.Detail.Content != "concurrent draft body" {
		t.Fatalf("concurrent draft was overwritten: title=%q detail=%+v", current.Title, current.Detail)
	}
	if current.EditVersion != 2 {
		t.Fatalf("current article edit version = %d, want 2", current.EditVersion)
	}
}
