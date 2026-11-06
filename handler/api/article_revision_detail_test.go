package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"myecho/dal"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
)

func TestArticleRevisionRetrieveReturnsExactContentFieldsToAuthorizedUsers(t *testing.T) {
	setupAPITestDB(t)
	author := createAPIUser(t, "revision-detail-author", "revision-detail-author-token", model.Author)
	otherAuthor := createAPIUser(t, "revision-detail-other", "revision-detail-other-token", model.Author)
	editor := createAPIUser(t, "revision-detail-editor", "revision-detail-editor-token", model.Editor)
	admin := createAPIUser(t, "revision-detail-admin", "revision-detail-admin-token", model.Admin)
	article := createArticleForRevisionDetail(t, author.ID, int8(mysql.ARTICLE_STATUS_DRAFT), "Current article")
	revision := createRevisionDetail(t, article.ID, author.ID)

	for _, user := range []*model.User{author, editor, admin} {
		resp := doJSONRequest(t, articleRevisionDetailApp(user), http.MethodGet,
			"/articles/"+strconv.Itoa(int(article.ID))+"/revisions/"+strconv.Itoa(int(revision.ID)), "", "")
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("user role %d status = %d, want 200", user.PermissionType, resp.StatusCode)
		}

		wrapped := decodeAPIResp(t, resp)
		var got rtype.ArticleRevisionDetailResponse
		if err := json.Unmarshal(wrapped.Data, &got); err != nil {
			t.Fatalf("decode revision detail: %v", err)
		}
		if got.ID != revision.ID || got.ArticleID != article.ID || got.ActorID != author.ID ||
			got.Content != "historical article body" || got.ContentFormat != model.ArticleContentFormatHTML {
			t.Fatalf("revision detail = %+v, want revision metadata and HTML body", got)
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(wrapped.Data, &fields); err != nil {
			t.Fatalf("decode revision fields: %v", err)
		}
		wantFields := []string{
			"id", "article_id", "created_at", "actor_id", "title", "slug", "type", "content_format",
			"summary", "seo_title", "seo_description", "share_image", "content",
		}
		if len(fields) != len(wantFields) {
			t.Fatalf("revision response fields = %v, want exactly %v", fields, wantFields)
		}
		for _, name := range wantFields {
			if _, ok := fields[name]; !ok {
				t.Errorf("revision response missing field %q", name)
			}
		}
	}

	resp := doJSONRequest(t, articleRevisionDetailApp(otherAuthor), http.MethodGet,
		"/articles/"+strconv.Itoa(int(article.ID))+"/revisions/"+strconv.Itoa(int(revision.ID)), "", "")
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("other author's status = %d, want 403", resp.StatusCode)
	}
}

func TestArticleRevisionRetrieveReturnsNotFoundConflictAndBadIDStatuses(t *testing.T) {
	setupAPITestDB(t)
	author := createAPIUser(t, "revision-detail-owner", "revision-detail-owner-token", model.Author)
	otherAuthor := createAPIUser(t, "revision-detail-other-owner", "revision-detail-other-owner-token", model.Author)
	article := createArticleForRevisionDetail(t, author.ID, int8(mysql.ARTICLE_STATUS_DRAFT), "Owned article")
	articleRevision := createRevisionDetail(t, article.ID, author.ID)
	otherArticle := createArticleForRevisionDetail(t, otherAuthor.ID, int8(mysql.ARTICLE_STATUS_DRAFT), "Other article")
	otherRevision := createRevisionDetail(t, otherArticle.ID, otherAuthor.ID)
	trashedArticle := createArticleForRevisionDetail(t, author.ID, int8(mysql.ARTICLE_STATUS_RECYCLE), "Trashed article")
	trashedRevision := createRevisionDetail(t, trashedArticle.ID, author.ID)
	app := articleRevisionDetailApp(author)

	tests := []struct {
		name string
		path string
		want int
	}{
		{
			name: "missing article",
			path: "/articles/999999/revisions/1",
			want: fiber.StatusNotFound,
		},
		{
			name: "revision belongs to another article",
			path: "/articles/" + strconv.Itoa(int(article.ID)) + "/revisions/" + strconv.Itoa(int(otherRevision.ID)),
			want: fiber.StatusNotFound,
		},
		{
			name: "missing revision",
			path: "/articles/" + strconv.Itoa(int(article.ID)) + "/revisions/999999",
			want: fiber.StatusNotFound,
		},
		{
			name: "trashed article",
			path: "/articles/" + strconv.Itoa(int(trashedArticle.ID)) + "/revisions/" + strconv.Itoa(int(trashedRevision.ID)),
			want: fiber.StatusConflict,
		},
		{name: "bad article id", path: "/articles/not-an-id/revisions/1", want: fiber.StatusBadRequest},
		{name: "bad revision id", path: "/articles/" + strconv.Itoa(int(article.ID)) + "/revisions/not-an-id", want: fiber.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := doJSONRequest(t, app, http.MethodGet, test.path, "", "")
			if resp.StatusCode != test.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, test.want)
			}
		})
	}

	resp := doJSONRequest(t, articleRevisionDetailApp(otherAuthor), http.MethodGet,
		"/articles/"+strconv.Itoa(int(article.ID))+"/revisions/"+strconv.Itoa(int(articleRevision.ID)), "", "")
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("cross-author status = %d, want 403", resp.StatusCode)
	}
}

func articleRevisionDetailApp(user *model.User) *fiber.App {
	app := fiber.New()
	withUser := func(c *fiber.Ctx) error {
		c.Locals("user", user)
		return c.Next()
	}
	app.Get("/articles/:id/revisions/:revision_id", withUser, middleware.AuthorRequired, ArticleRevisionRetrieve)
	return app
}

func createArticleForRevisionDetail(t *testing.T, authorID uint, status int8, title string) *mysql.ArticleModel {
	t.Helper()
	article := &mysql.ArticleModel{
		AuthorID: authorID,
		Title:    title,
		Status:   status,
		Type:     model.ArticleTypePage,
		Detail:   &model.ArticleDetail{Content: "current article body"},
	}
	if err := dal.MySqlDB.Article.Create(article); err != nil {
		t.Fatalf("create article for revision detail: %v", err)
	}
	return article
}

func createRevisionDetail(t *testing.T, articleID, actorID uint) model.ArticleRevision {
	t.Helper()
	revision := model.ArticleRevision{
		ArticleID:      articleID,
		ActorID:        actorID,
		CreatedAt:      time.Date(2026, 10, 5, 9, 30, 0, 0, time.UTC),
		Title:          "Historical title",
		Slug:           "historical-title",
		Type:           model.ArticleTypePage,
		ContentFormat:  model.ArticleContentFormatHTML,
		Summary:        "Historical summary",
		SEOTitle:       "Historical SEO title",
		SEODescription: "Historical SEO description",
		ShareImage:     "/historical-share.png",
		Content:        "historical article body",
	}
	if err := connect.Database.Create(&revision).Error; err != nil {
		t.Fatalf("create article revision: %v", err)
	}
	return revision
}
