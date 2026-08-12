package mysql

import (
	"fmt"
	"testing"
	"time"

	"myecho/model"

	"gorm.io/gorm"
)

func TestCreateArticleRevision_BitsUT(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	article := &ArticleModel{
		Title:          "Revision 0",
		Slug:           "revision",
		Type:           model.ArticleTypePost,
		ContentFormat:  model.ArticleContentFormatMarkdown,
		Summary:        "summary",
		SEOTitle:       "SEO title",
		SEODescription: "SEO description",
		ShareImage:     "https://example.com/share.png",
		Detail:         &model.ArticleDetail{Content: "content"},
	}
	if err := repo.Create(article); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	for i := 0; i <= articleRevisionLimit; i++ {
		article.Title = fmt.Sprintf("Revision %d", i)
		if err := createArticleRevision(db, article); err != nil {
			t.Fatalf("createArticleRevision(%d) error = %v", i, err)
		}
	}

	revisions, err := repo.ListRevisions(article.ID)
	if err != nil {
		t.Fatalf("ListRevisions() error = %v", err)
	}
	if len(revisions) != articleRevisionLimit {
		t.Fatalf("ListRevisions() len = %d, want %d", len(revisions), articleRevisionLimit)
	}
	if revisions[0].Title != "Revision 20" || revisions[len(revisions)-1].Title != "Revision 1" {
		t.Fatalf("revision bounds = %q ... %q", revisions[0].Title, revisions[len(revisions)-1].Title)
	}
	if revisions[0].Content != "content" || revisions[0].SEOTitle != "SEO title" {
		t.Fatalf("latest revision fields = %+v", revisions[0])
	}

	found, err := repo.FindRevision(article.ID, revisions[0].ID)
	if err != nil {
		t.Fatalf("FindRevision() error = %v", err)
	}
	if found.ID != revisions[0].ID || found.Title != revisions[0].Title {
		t.Fatalf("FindRevision() = %+v, want revision %d", found, revisions[0].ID)
	}
}

func TestArticleRepoUpdateCreatesSlugRedirect_BitsUT(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	article := &ArticleModel{
		Title:  "Before",
		Slug:   "reserved",
		Type:   model.ArticleTypePost,
		Detail: &model.ArticleDetail{Content: "before content"},
	}
	if err := repo.Create(article); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := db.Create(&model.ArticleSlugRedirect{
		ArticleUID: "another-article",
		Slug:       "reserved-2",
		Type:       model.ArticleTypePost,
	}).Error; err != nil {
		t.Fatalf("create redirect: %v", err)
	}
	unique, err := uniqueArticleSlug(db, "reserved", model.ArticleTypePost, 0)
	if err != nil {
		t.Fatalf("uniqueArticleSlug() error = %v", err)
	}
	if unique != "reserved-3" {
		t.Fatalf("uniqueArticleSlug() = %q, want reserved-3", unique)
	}

	article.Title = "After"
	article.Slug = "after"
	article.Detail.Content = "after content"
	if err := repo.Update(article); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	redirected, err := repo.FindByRedirectSlug("reserved", model.ArticleTypePost)
	if err != nil {
		t.Fatalf("FindByRedirectSlug() error = %v", err)
	}
	if redirected.ID != article.ID || redirected.Slug != "after" {
		t.Fatalf("redirected article = %+v", redirected)
	}

	revisions, err := repo.ListRevisions(article.ID)
	if err != nil {
		t.Fatalf("ListRevisions() error = %v", err)
	}
	if len(revisions) != 1 || revisions[0].Title != "Before" || revisions[0].Content != "before content" {
		t.Fatalf("Update() revisions = %+v", revisions)
	}
}

func TestArticleRepoRecordPublicView_BitsUT(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	article := &ArticleModel{Title: "Viewed", Type: model.ArticleTypePost}
	if err := repo.Create(article); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.RecordPublicView(article); err != nil {
		t.Fatalf("RecordPublicView(first) error = %v", err)
	}
	if err := repo.RecordPublicView(article); err != nil {
		t.Fatalf("RecordPublicView(second) error = %v", err)
	}

	updated, err := repo.FindByID(article.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if updated.ReadCount != 2 {
		t.Fatalf("ReadCount = %d, want 2", updated.ReadCount)
	}

	var stat model.ArticleDailyStat
	if err := db.Where("article_uid = ? AND date = ?", article.UID, time.Now().Format("2006-01-02")).First(&stat).Error; err != nil {
		t.Fatalf("load daily stat: %v", err)
	}
	if stat.Views != 2 {
		t.Fatalf("daily views = %d, want 2", stat.Views)
	}
}

func TestArticleRepoDeleteRemovesDependentData_BitsUT(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	article := &ArticleModel{
		Title:  "Delete me",
		Slug:   "delete-me",
		Type:   model.ArticleTypePost,
		Detail: &model.ArticleDetail{Content: "content"},
	}
	if err := repo.Create(article); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	tag := model.Tag{Name: "Tag", UID: "delete-tag"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := db.Table("article_tags").Create(map[string]interface{}{"article_uid": article.UID, "tag_uid": tag.UID}).Error; err != nil {
		t.Fatalf("create article tag: %v", err)
	}
	if err := db.Create(&model.ArticleRevision{ArticleID: article.ID, Title: "Old"}).Error; err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if err := db.Create(&model.ArticleSlugRedirect{ArticleUID: article.UID, Slug: "old-delete-me", Type: article.Type}).Error; err != nil {
		t.Fatalf("create redirect: %v", err)
	}
	if err := db.Create(&model.ArticleDailyStat{ArticleUID: article.UID, Date: "2026-07-27", Views: 4}).Error; err != nil {
		t.Fatalf("create daily stat: %v", err)
	}
	if err := db.Create(&model.Comment{ArticleUID: article.UID, AuthorName: "Alice", Content: "comment"}).Error; err != nil {
		t.Fatalf("create comment: %v", err)
	}

	if err := repo.DeleteByID(article.ID); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}
	for name, query := range map[string]*gorm.DB{
		"detail":    db.Model(&model.ArticleDetail{}).Where("uid = ?", article.DetailUID),
		"revisions": db.Model(&model.ArticleRevision{}).Where("article_id = ?", article.ID),
		"redirects": db.Model(&model.ArticleSlugRedirect{}).Where("article_uid = ?", article.UID),
		"stats":     db.Model(&model.ArticleDailyStat{}).Where("article_uid = ?", article.UID),
		"comments":  db.Unscoped().Model(&model.Comment{}).Where("article_uid = ?", article.UID),
		"tags":      db.Table("article_tags").Where("article_uid = ?", article.UID),
	} {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want 0", name, count)
		}
	}
}
