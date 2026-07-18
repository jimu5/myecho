package mysql

import (
	"testing"
	"time"

	"myecho/model"
)

func TestArticleRepoFindPostNeighbors(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}
	baseTime := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	previous := &ArticleModel{Title: "Previous", Type: model.ArticleTypePost, Status: int8(ARTILCE_STATUS_PUBLIC), PostTime: baseTime.Add(-time.Hour)}
	current := &ArticleModel{Title: "Current", Type: model.ArticleTypePost, Status: int8(ARTICLE_STATUS_TOP), PostTime: baseTime}
	next := &ArticleModel{Title: "Next", Type: model.ArticleTypePost, Status: int8(ARTILCE_STATUS_PUBLIC), PostTime: baseTime.Add(time.Hour)}
	draft := &ArticleModel{Title: "Draft", Type: model.ArticleTypePost, Status: int8(ARTICLE_STATUS_DRAFT), PostTime: baseTime.Add(30 * time.Minute)}
	page := &ArticleModel{Title: "Page", Type: model.ArticleTypePage, Status: int8(ARTILCE_STATUS_PUBLIC), PostTime: baseTime.Add(45 * time.Minute)}
	for _, article := range []*ArticleModel{previous, current, next, draft, page} {
		if err := repo.Create(article); err != nil {
			t.Fatalf("create %s: %v", article.Title, err)
		}
	}

	gotPrevious, gotNext, err := repo.FindPostNeighbors(current)
	if err != nil {
		t.Fatalf("FindPostNeighbors() error = %v", err)
	}
	if gotPrevious == nil || gotPrevious.ID != previous.ID || gotPrevious.Title != previous.Title {
		t.Fatalf("previous = %+v, want %d", gotPrevious, previous.ID)
	}
	if gotNext == nil || gotNext.ID != next.ID || gotNext.Title != next.Title {
		t.Fatalf("next = %+v, want %d", gotNext, next.ID)
	}

	gotPrevious, gotNext, err = repo.FindPostNeighbors(page)
	if err != nil || gotPrevious != nil || gotNext != nil {
		t.Fatalf("page neighbors = %+v %+v, err=%v", gotPrevious, gotNext, err)
	}
}
