package mysql

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	"myecho/model"
)

func TestArticleTrashRestoreAndPermanentDelete(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	category := &model.Category{Name: "Trash category", UID: "trash-category", Type: model.CategoryTypeArticle}
	if err := db.Create(category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	tag := &model.Tag{Name: "trash-tag", UID: "trash-tag"}
	if err := db.Create(tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	article := &ArticleModel{
		AuthorID: 9, Title: "Trash preserves content", Slug: "trash-preserves-content",
		Summary: "summary", CategoryUID: category.UID, Status: int8(ARTILCE_STATUS_PUBLIC),
		PostTime: time.Now().Add(-time.Hour), Detail: &model.ArticleDetail{Content: "preserved body"},
	}
	if err := repo.Create(article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	if err := db.Table("article_tags").Create(map[string]interface{}{"article_uid": article.UID, "tag_uid": tag.UID}).Error; err != nil {
		t.Fatalf("create article tag: %v", err)
	}
	if err := db.Create(&model.ArticleRevision{ArticleID: article.ID, Title: article.Title, Content: "revision body"}).Error; err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if err := db.Create(&model.ArticleSlugRedirect{ArticleUID: article.UID, Slug: "older-trash-slug", Type: model.ArticleTypePost}).Error; err != nil {
		t.Fatalf("create slug redirect: %v", err)
	}
	if err := db.Create(&model.ArticleDailyStat{ArticleUID: article.UID, Date: "2026-10-04", Views: 7}).Error; err != nil {
		t.Fatalf("create daily stat: %v", err)
	}
	if err := db.Create(&model.Comment{ArticleUID: article.UID, Content: "kept comment"}).Error; err != nil {
		t.Fatalf("create comment: %v", err)
	}
	submissions := []model.ArticleSubmission{
		{ArticleID: article.ID, SubmitterID: 9, Status: model.ArticleSubmissionPending, Snapshot: `{"title":"pending"}`},
		{ArticleID: article.ID, SubmitterID: 9, Status: model.ArticleSubmissionDraft, Snapshot: `{"title":"draft"}`},
		{ArticleID: article.ID, SubmitterID: 9, Status: model.ArticleSubmissionRejected, Snapshot: `{"title":"rejected"}`},
	}
	if err := db.Create(&submissions).Error; err != nil {
		t.Fatalf("create submissions: %v", err)
	}
	pendingID := submissions[0].ID
	if err := db.Create(&model.ArticleReview{ArticleID: article.ID, SubmissionID: pendingID, ActorID: 12, Action: model.ArticleReviewSubmit}).Error; err != nil {
		t.Fatalf("create review: %v", err)
	}
	originalVersion := article.EditVersion

	if err := repo.TrashByID(article.ID, 12); err != nil {
		t.Fatalf("TrashByID() error = %v", err)
	}
	trashed, err := repo.FindByID(article.ID)
	if err != nil {
		t.Fatalf("load trashed article: %v", err)
	}
	if trashed.Status != int8(ARTICLE_STATUS_RECYCLE) || trashed.EditVersion <= originalVersion {
		t.Fatalf("trashed status/version = %d/%d, want trash and version > %d", trashed.Status, trashed.EditVersion, originalVersion)
	}
	if trashed.Detail == nil || trashed.Detail.Content != "preserved body" || trashed.Summary != "summary" {
		t.Fatalf("trash changed content: detail=%#v summary=%q", trashed.Detail, trashed.Summary)
	}
	var keptTags int64
	if err := db.Table("article_tags").Where("article_uid = ? AND tag_uid = ?", article.UID, tag.UID).Count(&keptTags).Error; err != nil || keptTags != 1 {
		t.Fatalf("article tag count = %d, err=%v; want relation preserved", keptTags, err)
	}
	for _, check := range []struct {
		name  string
		value interface{}
		query *gorm.DB
	}{
		{"revision", &model.ArticleRevision{}, db.Where("article_id = ?", article.ID)},
		{"review", &model.ArticleReview{}, db.Unscoped().Where("article_id = ?", article.ID)},
		{"slug redirect", &model.ArticleSlugRedirect{}, db.Where("article_uid = ?", article.UID)},
		{"daily stat", &model.ArticleDailyStat{}, db.Where("article_uid = ?", article.UID)},
		{"comment", &model.Comment{}, db.Unscoped().Where("article_uid = ?", article.UID)},
	} {
		var count int64
		if err := check.query.Model(check.value).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s count = %d, err=%v; want one preserved record", check.name, count, err)
		}
	}
	var savedSubmissions []model.ArticleSubmission
	if err := db.Where("article_id = ?", article.ID).Order("id").Find(&savedSubmissions).Error; err != nil {
		t.Fatalf("load submissions: %v", err)
	}
	if len(savedSubmissions) != 3 || savedSubmissions[0].Status != model.ArticleSubmissionWithdrawn || savedSubmissions[1].Status != model.ArticleSubmissionDraft || savedSubmissions[2].Status != model.ArticleSubmissionRejected {
		t.Fatalf("submission statuses after trash = %#v, want withdrawn/draft/rejected", savedSubmissions)
	}
	var countedCategory model.Category
	if err := db.Where("uid = ?", category.UID).First(&countedCategory).Error; err != nil {
		t.Fatalf("load category: %v", err)
	}
	if countedCategory.Count != 0 {
		t.Fatalf("category count after trash = %d, want 0", countedCategory.Count)
	}
	publicCount, err := repo.CountDisplayable(ArticleCommonQueryParam{})
	if err != nil || publicCount != 0 {
		t.Fatalf("displayable count after trash = %d, err=%v; want hidden article", publicCount, err)
	}
	reviewCandidate := &ArticleModel{Status: int8(ARTILCE_STATUS_PUBLIC)}
	reviewCandidate.ID = article.ID
	if err := repo.ReviewSubmission(pendingID, reviewCandidate, nil, 12, model.ArticleReviewApprove, ""); !errors.Is(err, ErrArticleSubmissionChanged) && !errors.Is(err, ErrArticleTrashed) {
		t.Fatalf("approving canceled submission error = %v, want a canceled/trashed conflict", err)
	}

	if err := repo.RestoreFromTrash(article.ID, 12); err != nil {
		t.Fatalf("RestoreFromTrash() error = %v", err)
	}
	restored, err := repo.FindByID(article.ID)
	if err != nil {
		t.Fatalf("load restored article: %v", err)
	}
	if restored.Status != int8(ARTICLE_STATUS_DRAFT) || restored.EditVersion <= trashed.EditVersion {
		t.Fatalf("restored status/version = %d/%d, want draft with a newer version", restored.Status, restored.EditVersion)
	}
	if restored.Detail == nil || restored.Detail.Content != "preserved body" {
		t.Fatalf("restore changed article body: %#v", restored.Detail)
	}
	if err := repo.PermanentlyDelete(article.ID); !errors.Is(err, ErrArticleNotTrashed) {
		t.Fatalf("PermanentlyDelete(active) error = %v, want ErrArticleNotTrashed", err)
	}
	if _, err := repo.FindByID(article.ID); err != nil {
		t.Fatalf("active article was deleted after rejected permanent delete: %v", err)
	}

	if err := repo.TrashByID(article.ID, 12); err != nil {
		t.Fatalf("second TrashByID() error = %v", err)
	}
	if err := db.Create(&model.ArticlePreview{ArticleID: article.ID, TokenHash: "permanent-delete-preview", Snapshot: "private draft"}).Error; err != nil {
		t.Fatalf("create preview: %v", err)
	}
	if err := repo.PermanentlyDelete(article.ID); err != nil {
		t.Fatalf("PermanentlyDelete(trashed) error = %v", err)
	}
	var deleted ArticleModel
	if err := db.Unscoped().First(&deleted, article.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("permanently deleted article lookup error = %v, want record not found", err)
	}
	for _, check := range []struct {
		name  string
		value interface{}
		query *gorm.DB
	}{
		{"preview", &model.ArticlePreview{}, db.Unscoped().Where("article_id = ?", article.ID)},
		{"revision", &model.ArticleRevision{}, db.Where("article_id = ?", article.ID)},
		{"submission", &model.ArticleSubmission{}, db.Unscoped().Where("article_id = ?", article.ID)},
		{"review", &model.ArticleReview{}, db.Unscoped().Where("article_id = ?", article.ID)},
		{"slug redirect", &model.ArticleSlugRedirect{}, db.Where("article_uid = ?", article.UID)},
		{"daily stat", &model.ArticleDailyStat{}, db.Where("article_uid = ?", article.UID)},
		{"comment", &model.Comment{}, db.Unscoped().Where("article_uid = ?", article.UID)},
	} {
		var count int64
		if err := check.query.Model(check.value).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s count after permanent delete = %d, err=%v; want 0", check.name, count, err)
		}
	}
	if err := db.Table("article_tags").Where("article_uid = ?", article.UID).Count(&keptTags).Error; err != nil || keptTags != 0 {
		t.Fatalf("article tag count after permanent delete = %d, err=%v; want 0", keptTags, err)
	}
	if err := db.Where("uid = ?", category.UID).First(&countedCategory).Error; err != nil {
		t.Fatalf("reload category: %v", err)
	}
	if countedCategory.Count != 0 {
		t.Fatalf("category count after permanent delete = %d, want 0 (no double decrement)", countedCategory.Count)
	}
}

func TestArticleTrashBatchesAreAtomic(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	first := &ArticleModel{Title: "first batch article", Status: int8(ARTILCE_STATUS_PUBLIC), PostTime: time.Now().Add(-time.Hour)}
	second := &ArticleModel{Title: "second batch article", Status: int8(ARTICLE_STATUS_DRAFT)}
	for _, article := range []*ArticleModel{first, second} {
		if err := repo.Create(article); err != nil {
			t.Fatalf("create %q: %v", article.Title, err)
		}
	}
	if err := repo.BatchTrash([]uint{first.ID, 999999}); err == nil {
		t.Fatal("BatchTrash() with missing article succeeded")
	}
	unchanged, err := repo.FindByID(first.ID)
	if err != nil {
		t.Fatalf("load first article after failed batch: %v", err)
	}
	if unchanged.Status != int8(ARTILCE_STATUS_PUBLIC) {
		t.Fatalf("first article status after failed batch = %d, want public", unchanged.Status)
	}
	if err := repo.BatchTrash([]uint{first.ID, second.ID}); err != nil {
		t.Fatalf("BatchTrash(valid) error = %v", err)
	}
	if err := repo.BatchRestoreFromTrash([]uint{first.ID, second.ID}); err != nil {
		t.Fatalf("BatchRestoreFromTrash(valid) error = %v", err)
	}
	if err := repo.BatchRestoreFromTrash([]uint{first.ID, second.ID}); !errors.Is(err, ErrArticleNotTrashed) {
		t.Fatalf("BatchRestoreFromTrash(mixed state) error = %v, want ErrArticleNotTrashed", err)
	}
	firstAfter, err := repo.FindByID(first.ID)
	if err != nil {
		t.Fatalf("load first article after rejected restore batch: %v", err)
	}
	secondAfter, err := repo.FindByID(second.ID)
	if err != nil {
		t.Fatalf("load second article after rejected restore batch: %v", err)
	}
	if firstAfter.Status != int8(ARTICLE_STATUS_DRAFT) || secondAfter.Status != int8(ARTICLE_STATUS_DRAFT) {
		t.Fatalf("statuses after rejected restore batch = %d/%d, want both drafts", firstAfter.Status, secondAfter.Status)
	}
}
