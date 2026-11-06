package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"myecho/model"
)

func TestArticleDuplicateHandlerEnforcesPermissionsAndCreatesDraft(t *testing.T) {
	setupAPITestDB(t)
	owner := createAPIUser(t, "duplicate-api-owner", "duplicate-api-owner-token", model.Author)
	otherAuthor := createAPIUser(t, "duplicate-api-other", "duplicate-api-other-token", model.Author)
	editor := createAPIUser(t, "duplicate-api-editor", "duplicate-api-editor-token", model.Editor)
	admin := createAPIUser(t, "duplicate-api-admin", "duplicate-api-admin-token", model.Admin)
	source := &mysql.ArticleModel{
		AuthorID: owner.ID, Title: "API duplicate source", Slug: "api-duplicate-source",
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Summary: "source summary", Status: int8(mysql.ARTILCE_STATUS_PUBLIC), Password: "source-password-hash",
		ReadCount: 14, LikeCount: 6, CommentCount: 3,
		Detail: &model.ArticleDetail{Content: "source body"},
	}
	if err := dal.MySqlDB.Article.Create(source); err != nil {
		t.Fatalf("create source article: %v", err)
	}

	appFor := func(user *model.User) *fiber.App {
		app := fiber.New()
		withUser := func(next fiber.Handler) fiber.Handler {
			return func(c *fiber.Ctx) error {
				c.Locals("user", user)
				return next(c)
			}
		}
		app.Post("/articles/:id/duplicate", withUser(ArticleDuplicate))
		return app
	}
	target := "/articles/" + strconv.FormatUint(uint64(source.ID), 10) + "/duplicate"

	resp := doJSONRequest(t, appFor(owner), http.MethodPost, target, "", "")
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("owner duplicate status = %d, want 201", resp.StatusCode)
	}
	envelope := decodeAPIResp(t, resp)
	if envelope.Code != 0 {
		t.Fatalf("duplicate envelope code = %d, want 0 (%s)", envelope.Code, envelope.Msg)
	}
	var response rtype.ArticleResponse
	if err := json.Unmarshal(envelope.Data, &response); err != nil {
		t.Fatalf("decode duplicate article response: %v", err)
	}
	if response.ID == 0 || response.ID == source.ID || response.Status != int8(mysql.ARTICLE_STATUS_DRAFT) || response.Detail == nil || response.Detail.Content != "source body" {
		t.Fatalf("duplicate response = %+v detail=%+v", response, response.Detail)
	}
	created, err := dal.MySqlDB.Article.FindByID(response.ID)
	if err != nil {
		t.Fatalf("load created duplicate: %v", err)
	}
	if created.AuthorID != owner.ID || created.Password != "" || created.ReadCount != 0 || created.LikeCount != 0 || created.CommentCount != 0 {
		t.Fatalf("created duplicate owner/password/counters = %d/%q/%d/%d/%d", created.AuthorID, created.Password, created.ReadCount, created.LikeCount, created.CommentCount)
	}
	if unchanged, err := dal.MySqlDB.Article.FindByID(source.ID); err != nil {
		t.Fatalf("reload source: %v", err)
	} else if unchanged.Title != source.Title || unchanged.Detail == nil || unchanged.Detail.Content != "source body" || unchanged.Password != "source-password-hash" || unchanged.ReadCount != 14 || unchanged.LikeCount != 6 || unchanged.CommentCount != 3 {
		t.Fatalf("source changed after duplicate: %+v detail=%+v", unchanged, unchanged.Detail)
	}

	resp = doJSONRequest(t, appFor(otherAuthor), http.MethodPost, target, "", "")
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("other author's duplicate status = %d, want 403", resp.StatusCode)
	}
	_ = resp.Body.Close()

	for _, user := range []*model.User{editor, admin} {
		resp = doJSONRequest(t, appFor(user), http.MethodPost, target, "", "")
		if resp.StatusCode != fiber.StatusCreated {
			t.Fatalf("%v duplicate status = %d, want 201", user.PermissionType, resp.StatusCode)
		}
		var copiedEnvelope apiTestResp
		if err := json.NewDecoder(resp.Body).Decode(&copiedEnvelope); err != nil {
			t.Fatalf("decode privileged duplicate response: %v", err)
		}
		var copied rtype.ArticleResponse
		if err := json.Unmarshal(copiedEnvelope.Data, &copied); err != nil {
			t.Fatalf("decode privileged duplicate article: %v", err)
		}
		stored, err := dal.MySqlDB.Article.FindByID(copied.ID)
		if err != nil {
			t.Fatalf("load privileged duplicate: %v", err)
		}
		if stored.AuthorID != user.ID || stored.Status != int8(mysql.ARTICLE_STATUS_DRAFT) {
			t.Fatalf("privileged copy owner/status = %d/%d; want actor %d and draft", stored.AuthorID, stored.Status, user.ID)
		}
		_ = resp.Body.Close()
	}

	trashed := &mysql.ArticleModel{AuthorID: owner.ID, Title: "Trashed source", Status: int8(mysql.ARTICLE_STATUS_RECYCLE), Detail: &model.ArticleDetail{Content: "hidden"}}
	if err := dal.MySqlDB.Article.Create(trashed); err != nil {
		t.Fatalf("create trashed source: %v", err)
	}
	resp = doJSONRequest(t, appFor(owner), http.MethodPost, "/articles/"+strconv.FormatUint(uint64(trashed.ID), 10)+"/duplicate", "", "")
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("trashed source duplicate status = %d, want 409", resp.StatusCode)
	}
	_ = resp.Body.Close()

	resp = doJSONRequest(t, appFor(owner), http.MethodPost, "/articles/999999/duplicate", "", "")
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("missing source duplicate status = %d, want 404", resp.StatusCode)
	}
	_ = resp.Body.Close()
}
