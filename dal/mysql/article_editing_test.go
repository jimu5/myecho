package mysql

import (
	"errors"
	"testing"

	"myecho/model"
)

func TestArticleUpdateUsesEditVersionAndPreservesReadCount(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	article := &ArticleModel{
		Title: "Live", Status: int8(ARTILCE_STATUS_PUBLIC),
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Detail: &model.ArticleDetail{Content: "original"},
	}
	if err := repo.Create(article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	if article.EditVersion != 1 {
		t.Fatalf("created edit_version = %d, want 1", article.EditVersion)
	}

	expected := uint64(1)
	article.Title = "Updated"
	article.Detail = &model.ArticleDetail{Content: "updated"}
	if err := repo.UpdateForEditing(article, &expected); err != nil {
		t.Fatalf("versioned update: %v", err)
	}
	updated, err := repo.FindByID(article.ID)
	if err != nil {
		t.Fatalf("load updated article: %v", err)
	}
	if updated.EditVersion != 2 || updated.Title != "Updated" || updated.Detail == nil || updated.Detail.Content != "updated" {
		t.Fatalf("updated article version/title/content = %d/%q/%+v", updated.EditVersion, updated.Title, updated.Detail)
	}

	staleCopy := updated
	if err := repo.AddReadCountByID(article.ID, 9); err != nil {
		t.Fatalf("increment read count: %v", err)
	}
	expected = 2
	staleCopy.Title = "Updated again"
	staleCopy.Detail = &model.ArticleDetail{Content: "updated again"}
	if err := repo.UpdateForEditing(&staleCopy, &expected); err != nil {
		t.Fatalf("update after read count: %v", err)
	}
	updated, err = repo.FindByID(article.ID)
	if err != nil {
		t.Fatalf("load after read count update: %v", err)
	}
	if updated.ReadCount != 9 || updated.EditVersion != 3 || updated.Title != "Updated again" {
		t.Fatalf("read_count/edit_version/title = %d/%d/%q, want 9/3/Updated again", updated.ReadCount, updated.EditVersion, updated.Title)
	}

	staleCopy.Title = "Stale overwrite"
	staleCopy.Detail = &model.ArticleDetail{Content: "stale body"}
	expected = 2
	if err := repo.UpdateForEditing(&staleCopy, &expected); !errors.Is(err, ErrArticleEditConflict) {
		t.Fatalf("stale update error = %v, want ErrArticleEditConflict", err)
	}
	updated, err = repo.FindByID(article.ID)
	if err != nil {
		t.Fatalf("reload after stale update: %v", err)
	}
	if updated.EditVersion != 3 || updated.Title != "Updated again" || updated.Detail == nil || updated.Detail.Content != "updated again" || updated.ReadCount != 9 {
		t.Fatalf("stale update partially changed article: version=%d title=%q detail=%+v reads=%d", updated.EditVersion, updated.Title, updated.Detail, updated.ReadCount)
	}
}

func TestArticleSaveSubmissionChecksAndConsumesEditVersion(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	article := &ArticleModel{Title: "Online", Status: int8(ARTILCE_STATUS_PUBLIC), Detail: &model.ArticleDetail{Content: "live body"}}
	if err := repo.Create(article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	submission := &model.ArticleSubmission{
		ArticleID: article.ID, SubmitterID: 44, Status: model.ArticleSubmissionDraft, Snapshot: `{"title":"draft","content":"private body"}`,
	}
	stale := uint64(0)
	if err := repo.SaveSubmission(submission, nil, &stale); !errors.Is(err, ErrArticleEditConflict) {
		t.Fatalf("stale SaveSubmission error = %v, want ErrArticleEditConflict", err)
	}
	var count int64
	if err := db.Model(&model.ArticleSubmission{}).Where("article_id = ?", article.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("submissions after stale save = %d, err=%v; want 0", count, err)
	}
	if err := db.Model(&ArticleModel{}).Where("id = ?", article.ID).Count(&count).Error; err != nil {
		t.Fatalf("confirm article exists: %v", err)
	}
	expected := uint64(1)
	committedVersion, err := repo.SaveSubmissionForEditing(submission, nil, &expected)
	if err != nil {
		t.Fatalf("valid SaveSubmission: %v", err)
	}
	if committedVersion != 2 {
		t.Fatalf("submission token = %d, want 2", committedVersion)
	}
	updated, err := repo.FindByID(article.ID)
	if err != nil {
		t.Fatalf("reload article after submission save: %v", err)
	}
	if updated.EditVersion != 2 || updated.Status != int8(ARTILCE_STATUS_PUBLIC) || updated.Detail == nil || updated.Detail.Content != "live body" {
		t.Fatalf("submission save changed live article: version=%d status=%d detail=%+v", updated.EditVersion, updated.Status, updated.Detail)
	}
	if err := db.Model(&model.ArticleSubmission{}).Where("article_id = ? AND status = ?", article.ID, model.ArticleSubmissionDraft).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("draft submissions after valid save = %d, err=%v; want 1", count, err)
	}
}
