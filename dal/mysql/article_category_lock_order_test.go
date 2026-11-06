package mysql

import (
	"strings"
	"testing"

	"gorm.io/gorm"
	"myecho/model"
)

type articleLockTraceEvent struct {
	table string
	sql   string
	vars  []interface{}
	lock  bool
}

func TestArticleEditsAndBatchesLockCategoriesInSortedOrder(t *testing.T) {
	t.Run("single article edit", func(t *testing.T) {
		setupMysqlRepoTestDB(t)
		repo := &ArticleDBRepo{}
		categoryA := createCategoryLockOrderCategory(t, "lock-cat-a")
		categoryB := createCategoryLockOrderCategory(t, "lock-cat-b")
		article := createBatchEditVersionArticle(t, repo, "Move category", ARTILCE_STATUS_PUBLIC, categoryB.UID)
		candidate := loadBatchEditVersionArticle(t, repo, article.ID)
		candidate.CategoryUID = categoryA.UID
		expectedVersion := candidate.EditVersion
		trace := captureArticleLockOrderTrace(t)

		if err := repo.UpdateForEditing(candidate, &expectedVersion, 12); err != nil {
			t.Fatalf("UpdateForEditing() error = %v", err)
		}
		assertArticleAndSortedCategoryLockOrder(t, *trace, 1, []string{categoryA.UID, categoryB.UID})
		assertBatchEditVersionCategoryCount(t, categoryA.UID, 1)
		assertBatchEditVersionCategoryCount(t, categoryB.UID, 0)
	})

	for _, operation := range []string{"status batch", "trash batch"} {
		t.Run(operation, func(t *testing.T) {
			setupMysqlRepoTestDB(t)
			repo := &ArticleDBRepo{}
			categoryA := createCategoryLockOrderCategory(t, "lock-cat-a")
			categoryB := createCategoryLockOrderCategory(t, "lock-cat-b")
			first := createBatchEditVersionArticle(t, repo, "First category B", ARTILCE_STATUS_PUBLIC, categoryB.UID)
			second := createBatchEditVersionArticle(t, repo, "Second category A", ARTILCE_STATUS_PUBLIC, categoryA.UID)
			trace := captureArticleLockOrderTrace(t)

			var err error
			if operation == "status batch" {
				err = repo.BatchUpdateStatus([]uint{second.ID, first.ID}, ARTICLE_STATUS_DRAFT)
			} else {
				err = repo.BatchTrash([]uint{second.ID, first.ID}, 12)
			}
			if err != nil {
				t.Fatalf("%s error = %v", operation, err)
			}
			assertArticleAndSortedCategoryLockOrder(t, *trace, 2, []string{categoryA.UID, categoryB.UID})
			assertBatchEditVersionCategoryCount(t, categoryA.UID, 0)
			assertBatchEditVersionCategoryCount(t, categoryB.UID, 0)
		})
	}
}

func TestArticleStatusAndTrashBatchCategoryFailureRollsBack(t *testing.T) {
	for _, operation := range []string{"status batch", "trash batch"} {
		t.Run(operation, func(t *testing.T) {
			setupMysqlRepoTestDB(t)
			repo := &ArticleDBRepo{}
			categoryA := createCategoryLockOrderCategory(t, "rollback-cat-a")
			categoryB := createCategoryLockOrderCategory(t, "rollback-cat-b")
			first := createBatchEditVersionArticle(t, repo, "First category B", ARTILCE_STATUS_PUBLIC, categoryB.UID)
			second := createBatchEditVersionArticle(t, repo, "Second category A", ARTILCE_STATUS_PUBLIC, categoryA.UID)
			if err := db.Exec("CREATE TRIGGER fail_category_a_count BEFORE UPDATE OF count ON categories WHEN OLD.uid = '" + categoryA.UID + "' BEGIN SELECT RAISE(ABORT, 'test category update failure'); END").Error; err != nil {
				t.Fatalf("create category failure trigger: %v", err)
			}

			var err error
			if operation == "status batch" {
				err = repo.BatchUpdateStatus([]uint{second.ID, first.ID}, ARTICLE_STATUS_DRAFT)
			} else {
				err = repo.BatchTrash([]uint{second.ID, first.ID}, 12)
			}
			if err == nil {
				t.Fatalf("%s succeeded despite category update failure", operation)
			}
			for _, id := range []uint{first.ID, second.ID} {
				stored := loadBatchEditVersionArticle(t, repo, id)
				if stored.Status != int8(ARTILCE_STATUS_PUBLIC) || stored.EditVersion != 1 {
					t.Fatalf("article %d status/version after rollback = %d/%d, want public/1", id, stored.Status, stored.EditVersion)
				}
			}
			assertBatchEditVersionCategoryCount(t, categoryA.UID, 1)
			assertBatchEditVersionCategoryCount(t, categoryB.UID, 1)
		})
	}
}

func TestDeleteArticleLocksArticleBeforeCategoryCountHook(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	category := createCategoryLockOrderCategory(t, "delete-lock-cat")
	article := createBatchEditVersionArticle(t, repo, "Delete active article", ARTILCE_STATUS_PUBLIC, category.UID)
	trace := captureArticleLockOrderTrace(t)

	if err := repo.DeleteByID(article.ID); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}
	articleLock := -1
	categoryUpdate := -1
	for index, event := range *trace {
		upperSQL := strings.ToUpper(strings.TrimSpace(event.sql))
		if event.table == "articles" && event.lock && strings.HasPrefix(upperSQL, "SELECT") && articleLock == -1 {
			articleLock = index
		}
		if event.table == "categories" && strings.HasPrefix(upperSQL, "UPDATE") && categoryUpdate == -1 {
			categoryUpdate = index
		}
	}
	if articleLock == -1 || categoryUpdate == -1 || articleLock >= categoryUpdate {
		t.Fatalf("delete category hook ran before article row lock: article lock=%d category update=%d trace=%+v", articleLock, categoryUpdate, *trace)
	}
	assertBatchEditVersionCategoryCount(t, category.UID, 0)
}

func createCategoryLockOrderCategory(t *testing.T, uid string) *CategoryModel {
	t.Helper()
	category := &CategoryModel{Name: "Category " + uid, UID: uid, Type: model.CategoryTypeArticle}
	if err := db.Create(category).Error; err != nil {
		t.Fatalf("create category %q: %v", uid, err)
	}
	return category
}

func captureArticleLockOrderTrace(t *testing.T) *[]articleLockTraceEvent {
	t.Helper()
	var events []articleLockTraceEvent
	queryCallbackName := "test:article-lock-order-query"
	updateCallbackName := "test:article-lock-order-update"
	queryCallbacks := db.Callback().Query()
	updateCallbacks := db.Callback().Update()
	if err := queryCallbacks.After("gorm:query").Register(queryCallbackName, func(tx *gorm.DB) {
		_, locked := tx.Statement.Clauses["FOR"]
		events = append(events, articleLockTraceEvent{
			table: tx.Statement.Table, sql: tx.Statement.SQL.String(),
			vars: append([]interface{}(nil), tx.Statement.Vars...), lock: locked,
		})
	}); err != nil {
		t.Fatalf("register query trace: %v", err)
	}
	if err := updateCallbacks.After("gorm:update").Register(updateCallbackName, func(tx *gorm.DB) {
		events = append(events, articleLockTraceEvent{
			table: tx.Statement.Table, sql: tx.Statement.SQL.String(),
			vars: append([]interface{}(nil), tx.Statement.Vars...),
		})
	}); err != nil {
		t.Fatalf("register update trace: %v", err)
	}
	t.Cleanup(func() {
		_ = queryCallbacks.Remove(queryCallbackName)
		_ = updateCallbacks.Remove(updateCallbackName)
	})
	return &events
}

func assertArticleAndSortedCategoryLockOrder(t *testing.T, events []articleLockTraceEvent, articleLockCount int, wantCategoryUIDs []string) {
	t.Helper()
	lastArticleLock := -1
	firstCategoryLock := -1
	lastCategoryLock := -1
	firstCategoryUpdate := -1
	articleLocks := 0
	categoryLocks := make([]string, 0, len(wantCategoryUIDs))
	for index, event := range events {
		upperSQL := strings.ToUpper(strings.TrimSpace(event.sql))
		if event.table == "articles" && event.lock && strings.HasPrefix(upperSQL, "SELECT") {
			articleLocks++
			lastArticleLock = index
		}
		if event.table == "categories" && event.lock && strings.HasPrefix(upperSQL, "SELECT") {
			if firstCategoryLock == -1 {
				firstCategoryLock = index
			}
			lastCategoryLock = index
			for _, value := range event.vars {
				if uid, ok := value.(string); ok {
					for _, expectedUID := range wantCategoryUIDs {
						if uid == expectedUID {
							categoryLocks = append(categoryLocks, uid)
						}
					}
				}
			}
		}
		if event.table == "categories" && strings.HasPrefix(upperSQL, "UPDATE") && firstCategoryUpdate == -1 {
			firstCategoryUpdate = index
		}
	}
	if articleLocks != articleLockCount {
		t.Fatalf("article row lock count = %d, want %d; trace=%+v", articleLocks, articleLockCount, events)
	}
	if firstCategoryLock == -1 || lastArticleLock >= firstCategoryLock {
		t.Fatalf("category lock began before all article locks completed: last article lock=%d first category lock=%d trace=%+v", lastArticleLock, firstCategoryLock, events)
	}
	if len(categoryLocks) != len(wantCategoryUIDs) {
		t.Fatalf("category lock UIDs = %v, want %v; trace=%+v", categoryLocks, wantCategoryUIDs, events)
	}
	for i, uid := range wantCategoryUIDs {
		if categoryLocks[i] != uid {
			t.Fatalf("category lock order = %v, want %v", categoryLocks, wantCategoryUIDs)
		}
	}
	if firstCategoryUpdate == -1 || lastCategoryLock >= firstCategoryUpdate {
		t.Fatalf("category counter update preceded all category locks: last category lock=%d first category update=%d trace=%+v", lastCategoryLock, firstCategoryUpdate, events)
	}
}
