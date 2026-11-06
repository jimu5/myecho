package mysql

import (
	"errors"
	"testing"

	"gorm.io/gorm"
	"myecho/model"
)

func TestArticleBatchEditVersionFailuresRollback(t *testing.T) {
	for _, failure := range []string{"missing", "trashed", "pending"} {
		t.Run(failure, func(t *testing.T) {
			setupMysqlRepoTestDB(t)
			repo := &ArticleDBRepo{}
			category := createBatchEditVersionCategory(t)
			first := createBatchEditVersionArticle(t, repo, "First public", ARTILCE_STATUS_PUBLIC, category.UID)
			second := createBatchEditVersionArticle(t, repo, "Second draft", ARTICLE_STATUS_DRAFT, "")

			ids := []uint{first.ID, second.ID}
			var wantErr error
			switch failure {
			case "missing":
				ids = []uint{first.ID, second.ID + 1000000}
				wantErr = gorm.ErrRecordNotFound
			case "trashed":
				if err := repo.TrashByID(second.ID, 31); err != nil {
					t.Fatalf("trash second article: %v", err)
				}
				wantErr = ErrArticleTrashed
			case "pending":
				submission := &model.ArticleSubmission{
					ArticleID: second.ID, SubmitterID: 31,
					Status: model.ArticleSubmissionPending, Snapshot: `{"title":"pending"}`,
				}
				if err := db.Create(submission).Error; err != nil {
					t.Fatalf("create pending submission: %v", err)
				}
				wantErr = ErrArticleSubmissionChanged
			}

			beforeFirst := loadBatchEditVersionArticle(t, repo, first.ID)
			beforeSecond := loadBatchEditVersionArticle(t, repo, second.ID)
			if err := repo.BatchUpdateStatus(ids, ARTICLE_STATUS_PRIVATE); !errors.Is(err, wantErr) {
				t.Fatalf("BatchUpdateStatus() error = %v, want %v", err, wantErr)
			}
			assertBatchEditVersionArticle(t, repo, first.ID, beforeFirst)
			assertBatchEditVersionArticle(t, repo, second.ID, beforeSecond)
			assertBatchEditVersionCategoryCount(t, category.UID, 1)

			if failure == "pending" {
				var submission model.ArticleSubmission
				if err := db.Where("article_id = ? AND status = ?", second.ID, model.ArticleSubmissionPending).First(&submission).Error; err != nil {
					t.Fatalf("load pending submission after rejected batch: %v", err)
				}
			}
		})
	}
}

func TestArticleBatchEditVersionDeduplicatesAndAdjustsCategoryOnce(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	category := createBatchEditVersionCategory(t)
	first := createBatchEditVersionArticle(t, repo, "First public", ARTILCE_STATUS_PUBLIC, category.UID)
	second := createBatchEditVersionArticle(t, repo, "Second public", ARTILCE_STATUS_PUBLIC, category.UID)
	if first.EditVersion != 1 || second.EditVersion != 1 {
		t.Fatalf("initial edit versions = %d/%d, want 1/1", first.EditVersion, second.EditVersion)
	}
	assertBatchEditVersionCategoryCount(t, category.UID, 2)

	if err := repo.BatchUpdateStatus([]uint{second.ID, first.ID, second.ID}, ARTICLE_STATUS_DRAFT); err != nil {
		t.Fatalf("BatchUpdateStatus(duplicate reversed IDs) error = %v", err)
	}
	for _, article := range []*ArticleModel{first, second} {
		updated := loadBatchEditVersionArticle(t, repo, article.ID)
		if updated.Status != int8(ARTICLE_STATUS_DRAFT) || updated.EditVersion != article.EditVersion+1 {
			t.Fatalf("article %d status/version = %d/%d, want draft/version %d", article.ID, updated.Status, updated.EditVersion, article.EditVersion+1)
		}
	}
	assertBatchEditVersionCategoryCount(t, category.UID, 0)
}

func TestArticleBatchEditVersionRejectsRecycleTarget(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	category := createBatchEditVersionCategory(t)
	article := createBatchEditVersionArticle(t, repo, "Public article", ARTILCE_STATUS_PUBLIC, category.UID)
	before := loadBatchEditVersionArticle(t, repo, article.ID)

	if err := repo.BatchUpdateStatus([]uint{article.ID}, ARTICLE_STATUS_RECYCLE); !errors.Is(err, ErrArticleTrashed) {
		t.Fatalf("BatchUpdateStatus(recycle target) error = %v, want ErrArticleTrashed", err)
	}
	assertBatchEditVersionArticle(t, repo, article.ID, before)
	assertBatchEditVersionCategoryCount(t, category.UID, 1)
}

func createBatchEditVersionCategory(t *testing.T) *CategoryModel {
	t.Helper()
	category := &CategoryModel{
		Name: "Batch edit version category", UID: "batch-edit-version-category",
		Type: model.CategoryTypeArticle,
	}
	if err := db.Create(category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	return category
}

func createBatchEditVersionArticle(t *testing.T, repo *ArticleDBRepo, title string, status ArticleStatus, categoryUID string) *ArticleModel {
	t.Helper()
	article := &ArticleModel{
		Title: title, Type: model.ArticleTypePost, Status: int8(status), CategoryUID: categoryUID,
	}
	if err := repo.Create(article); err != nil {
		t.Fatalf("create article %q: %v", title, err)
	}
	return article
}

func loadBatchEditVersionArticle(t *testing.T, repo *ArticleDBRepo, id uint) *ArticleModel {
	t.Helper()
	article, err := repo.FindByID(id)
	if err != nil {
		t.Fatalf("load article %d: %v", id, err)
	}
	return &article
}

func assertBatchEditVersionArticle(t *testing.T, repo *ArticleDBRepo, id uint, want *ArticleModel) {
	t.Helper()
	got := loadBatchEditVersionArticle(t, repo, id)
	if got.Status != want.Status || got.EditVersion != want.EditVersion {
		t.Fatalf("article %d status/version = %d/%d, want %d/%d", id, got.Status, got.EditVersion, want.Status, want.EditVersion)
	}
}

func assertBatchEditVersionCategoryCount(t *testing.T, uid string, want uint) {
	t.Helper()
	var category CategoryModel
	if err := db.Where("uid = ?", uid).First(&category).Error; err != nil {
		t.Fatalf("load category %q: %v", uid, err)
	}
	if category.Count != want {
		t.Fatalf("category %q count = %d, want %d", uid, category.Count, want)
	}
}
