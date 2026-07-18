package rtype

import (
	"myecho/dal/mysql"
	"myecho/model"
	"strings"
	"testing"
	"time"
)

func TestArticleRequestSetSummaryStripsMarkdownAndTruncatesRunes(t *testing.T) {
	req := &ArticleRequest{Content: "# 标题\n\n" + strings.Repeat("中", 300)}
	req.SetSummary()
	if strings.Contains(req.Summary, "#") {
		t.Fatalf("summary still contains markdown marker: %q", req.Summary)
	}
	if got := len([]rune(req.Summary)); got != 255 {
		t.Fatalf("summary rune length = %d, want 255", got)
	}
}

func TestArticleRequestSetSummaryStripsHTMLAndTruncatesRunes(t *testing.T) {
	req := &ArticleRequest{
		ContentFormat: model.ArticleContentFormatHTML,
		Content:       `<section><h1>标题</h1><script>alert("x")</script><p>` + strings.Repeat("中", 300) + `</p></section>`,
	}
	req.SetSummary()
	if strings.Contains(req.Summary, "<") || strings.Contains(req.Summary, "alert") {
		t.Fatalf("summary still contains html or script text: %q", req.Summary)
	}
	if got := len([]rune(req.Summary)); got != 255 {
		t.Fatalf("summary rune length = %d, want 255", got)
	}
}

func TestArticleRequestSetSummaryPreservesManualSummary(t *testing.T) {
	req := &ArticleRequest{
		Summary: "手动摘要",
		Content: "正文生成的摘要不应覆盖手动值",
	}
	req.SetSummary()
	if req.Summary != "手动摘要" {
		t.Fatalf("summary = %q, want manual summary", req.Summary)
	}
}

func TestArticleRequestSetSummaryTreatsWhitespaceAsEmpty(t *testing.T) {
	req := &ArticleRequest{Summary: " \n\t ", Content: "自动摘要"}
	req.SetSummary()
	if req.Summary != "自动摘要" {
		t.Fatalf("summary = %q, want generated summary", req.Summary)
	}
}

func TestArticleRequestPreHandleDeduplicatesTagUIDs(t *testing.T) {
	req := &ArticleRequest{TagUIDs: []string{"a", "b", "a"}}
	req.PreHandle()
	want := []string{"a", "b"}
	if len(req.TagUIDs) != len(want) || req.TagUIDs[0] != want[0] || req.TagUIDs[1] != want[1] {
		t.Fatalf("TagUIDs = %#v, want %#v", req.TagUIDs, want)
	}
}

func TestModelToArticleResponseHandlesNilAndNestedModels(t *testing.T) {
	if ModelToUser(nil) != nil {
		t.Fatal("ModelToUser(nil) should return nil")
	}
	if ModelToCategory(nil) != nil {
		t.Fatal("ModelToCategory(nil) should return nil")
	}
	if ModelToArticleResponse(nil) != nil {
		t.Fatal("ModelToArticleResponse(nil) should return nil")
	}

	allowComment := true
	postTime := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	article := &mysql.ArticleModel{
		AuthorID:       7,
		Author:         &model.User{BaseModel: model.BaseModel{ID: 7}, NickName: "Admin"},
		Title:          "Title",
		Summary:        "Summary",
		ContentFormat:  model.ArticleContentFormatHTML,
		DetailUID:      "detail-uid",
		Detail:         &model.ArticleDetail{Content: "content"},
		CategoryUID:    "cat",
		Category:       &model.Category{Name: "Tech"},
		IsAllowComment: &allowComment,
		ReadCount:      11,
		LikeCount:      3,
		CommentCount:   2,
		PostTime:       postTime,
		Status:         1,
		Tags:           []*model.Tag{{Name: "go"}},
	}
	got := ModelToArticleResponse(article)
	if got.Author.NickName != "Admin" || got.Category.Name != "Tech" || got.Detail.Content != "content" {
		t.Fatalf("unexpected nested response: %+v", got)
	}
	if got.PostTime != postTime || got.ReadCount != 11 || len(got.Tags) != 1 || got.ContentFormat != model.ArticleContentFormatHTML {
		t.Fatalf("unexpected scalar response: %+v", got)
	}
}

func TestMultiModelToArticleResponsePreservesLength(t *testing.T) {
	articles := []*mysql.ArticleModel{{Title: "a"}, nil, {Title: "b"}}
	got := MultiModelToArticleResponse(articles)
	if len(got) != len(articles) {
		t.Fatalf("len = %d, want %d", len(got), len(articles))
	}
	if got[1] != nil {
		t.Fatalf("nil article should map to nil response")
	}
}
