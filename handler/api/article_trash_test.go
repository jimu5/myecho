package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"myecho/dal"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

func TestArticleTrashHandlersEnforceOwnershipAndRestoreAsDraft(t *testing.T) {
	setupAPITestDB(t)
	owner := createAPIUser(t, "trash-owner", "trash-owner-token", model.Author)
	otherAuthor := createAPIUser(t, "trash-other", "trash-other-token", model.Author)
	article := &mysql.ArticleModel{
		AuthorID: owner.ID, Title: "Restorable article", Status: int8(mysql.ARTICLE_STATUS_RECYCLE),
		Detail: &model.ArticleDetail{Content: "restored content"},
	}
	if err := dal.MySqlDB.Article.Create(article); err != nil {
		t.Fatalf("create trashed article: %v", err)
	}
	appFor := func(user *model.User) *fiber.App {
		app := fiber.New()
		withUser := func(next fiber.Handler) fiber.Handler {
			return func(c *fiber.Ctx) error {
				c.Locals("user", user)
				return next(c)
			}
		}
		app.Post("/articles/:id/restore", withUser(ArticleTrashRestore))
		app.Delete("/articles/:id/permanent", withUser(ArticlePermanentlyDelete))
		return app
	}

	ownerApp := appFor(owner)
	otherApp := appFor(otherAuthor)
	resp := doJSONRequest(t, otherApp, http.MethodPost, "/articles/"+strconv.FormatUint(uint64(article.ID), 10)+"/restore", "", "")
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("other author's restore status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, ownerApp, http.MethodPost, "/articles/"+strconv.FormatUint(uint64(article.ID), 10)+"/restore", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("owner restore status = %d, want 200", resp.StatusCode)
	}
	var envelope apiTestResp
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode restore response: %v", err)
	}
	var restored struct {
		Status      int8   `json:"status"`
		EditVersion uint64 `json:"edit_version"`
		Detail      struct {
			Content string `json:"content"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(envelope.Data, &restored); err != nil {
		t.Fatalf("decode restored article: %v", err)
	}
	if restored.Status != int8(mysql.ARTICLE_STATUS_DRAFT) || restored.EditVersion < 2 || restored.Detail.Content != "restored content" {
		t.Fatalf("restore response = status %d, version %d, body %q; want draft and retained content", restored.Status, restored.EditVersion, restored.Detail.Content)
	}

	active := &mysql.ArticleModel{AuthorID: owner.ID, Title: "Still active", Status: int8(mysql.ARTICLE_STATUS_DRAFT)}
	if err := dal.MySqlDB.Article.Create(active); err != nil {
		t.Fatalf("create active article: %v", err)
	}
	resp = doJSONRequest(t, ownerApp, http.MethodDelete, "/articles/"+strconv.FormatUint(uint64(active.ID), 10)+"/permanent", "", "")
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("active permanent delete status = %d, want 409", resp.StatusCode)
	}
	if _, err := dal.MySqlDB.Article.FindByID(active.ID); err != nil {
		t.Fatalf("active article disappeared after rejected purge: %v", err)
	}

	resp = doJSONRequest(t, otherApp, http.MethodDelete, "/articles/"+strconv.FormatUint(uint64(article.ID), 10)+"/permanent", "", "")
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("other author's permanent delete status = %d, want 403", resp.StatusCode)
	}
	if err := dal.MySqlDB.Article.TrashByID(article.ID, owner.ID); err != nil {
		t.Fatalf("trash restored article: %v", err)
	}
	resp = doJSONRequest(t, ownerApp, http.MethodDelete, "/articles/"+strconv.FormatUint(uint64(article.ID), 10)+"/permanent", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("owner permanent delete status = %d, want 200", resp.StatusCode)
	}
	if err := connect.Database.Unscoped().First(&mysql.ArticleModel{}, article.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("permanent delete article lookup error = %v, want record not found", err)
	}
}
