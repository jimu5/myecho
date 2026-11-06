package mysql

import (
	"errors"
	"sync"
	"testing"

	"myecho/model"
)

func TestArticleCreateRequestRollsBackResponseFailure(t *testing.T) {
	setupMysqlRepoTestDB(t)
	if err := db.AutoMigrate(&model.ArticleCreateRequest{}); err != nil {
		t.Fatal(err)
	}
	repo := &ArticleDBRepo{}
	wantErr := errors.New("cannot serialize")
	article := &ArticleModel{Title: "Retry", AuthorID: 1, Status: int8(ARTICLE_STATUS_DRAFT), Detail: &model.ArticleDetail{Content: "body"}}
	_, _, err := repo.CreateForRequest(article, nil, nil, "same-key", "digest", func() ([]byte, error) { return nil, wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v", err)
	}
	for _, value := range []interface{}{&model.Article{}, &model.ArticleDetail{}, &model.ArticleCreateRequest{}} {
		var count int64
		if err := db.Model(value).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%T count = %d err=%v; want transaction rollback", value, count, err)
		}
	}
}

func TestArticleCreateRequestConcurrentKeyCreatesOnce(t *testing.T) {
	setupMysqlRepoTestDB(t)
	if err := db.AutoMigrate(&model.ArticleCreateRequest{}); err != nil {
		t.Fatal(err)
	}
	// One SQLite connection avoids lock errors while concurrent requests race for the key.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			article := &ArticleModel{Title: "Concurrent", AuthorID: 1, Status: int8(ARTICLE_STATUS_DRAFT), Detail: &model.ArticleDetail{Content: "body"}}
			body, _, err := (&ArticleDBRepo{}).CreateForRequest(article, nil, nil, "same-key", "digest", func() ([]byte, error) { return []byte(`{"created":true}`), nil })
			if err != nil || string(body) != `{"created":true}` {
				t.Errorf("create/replay = %q, %v", body, err)
			}
		}()
	}
	wg.Wait()
	var count int64
	if err := db.Model(&model.Article{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("article count = %d err=%v; want 1", count, err)
	}
}

func TestArticlePermanentDeletePurgesCachedCreateResponse(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	article := &ArticleModel{Title: "Purge", AuthorID: 1, Status: int8(ARTICLE_STATUS_DRAFT), Detail: &model.ArticleDetail{Content: "private content"}}
	_, _, err := repo.CreateForRequest(article, nil, nil, "purge-key", "digest", func() ([]byte, error) { return []byte("private content"), nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.TrashByID(article.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.PermanentlyDelete(article.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplayCreateRequest(1, "purge-key", "digest"); !errors.Is(err, ErrArticleCreateGone) {
		t.Fatalf("deleted response replay error = %v", err)
	}
	var request model.ArticleCreateRequest
	if err := db.First(&request).Error; err != nil || len(request.Response) != 0 {
		t.Fatalf("cached response after purge = %q, err=%v", request.Response, err)
	}
}
