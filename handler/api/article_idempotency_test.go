package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"myecho/dal/connect"
	"myecho/middleware"
	"myecho/model"
)

func TestArticleCreateIdempotencyReplayConflictAndOwnerIsolation(t *testing.T) {
	setupAPITestDB(t)
	owner := createAPIUser(t, "idempotent-author", "idempotent-login", model.Author)
	other := createAPIUser(t, "second-author", "second-login", model.Author)
	category := seedArticleCategory(t)
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/articles", middleware.Authentication, middleware.AuthorRequired, ArticleCreate)
	body := `{"title":"One article","slug":"one-article","content":"body","status":5,"category_uid":"` + category.UID + `"}`
	post := func(token, key, payload string) (int, string, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/articles", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Idempotency-Key", key)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(data), resp.Header.Get("Idempotency-Replayed")
	}
	status, original, _ := post(owner.Token, "create-1", body)
	if status != http.StatusCreated {
		t.Fatalf("first create = %d: %s", status, original)
	}
	status, replay, replayHeader := post(owner.Token, "create-1", body)
	if status != http.StatusCreated || original != replay || replayHeader != "true" {
		t.Fatalf("replay = %d, header=%q, body=%s; want original %s", status, replayHeader, replay, original)
	}
	for _, table := range []interface{}{&model.Article{}, &model.ArticleSubmission{}, &model.ArticleReview{}, &model.ArticleCreateRequest{}} {
		var count int64
		if err := connect.Database.Model(table).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%T count = %d, err=%v; want 1", table, count, err)
		}
	}
	status, response, _ := post(owner.Token, "create-1", strings.Replace(body, "One article", "Different article", 1))
	if status != http.StatusConflict {
		t.Fatalf("changed request = %d: %s", status, response)
	}
	status, response, _ = post(other.Token, "create-1", body)
	if status != http.StatusCreated {
		t.Fatalf("another owner's key = %d: %s", status, response)
	}
	status, response, _ = post(owner.Token, "invalid key", body)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid key = %d: %s", status, response)
	}
}

func TestArticleCreateIdempotencyDoesNotReserveInvalidRequest(t *testing.T) {
	setupAPITestDB(t)
	owner := createAPIUser(t, "retry-author", "retry-login", model.Author)
	category := seedArticleCategory(t)
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/articles", middleware.Authentication, middleware.AuthorRequired, ArticleCreate)
	for i, title := range []string{"", "Fixed"} {
		req := httptest.NewRequest(http.MethodPost, "/articles", strings.NewReader(`{"title":"`+title+`","content":"body","category_uid":"`+category.UID+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "token "+owner.Token)
		req.Header.Set("Idempotency-Key", "retry-after-validation")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if (i == 0 && resp.StatusCode != http.StatusForbidden) || (i == 1 && resp.StatusCode != http.StatusCreated) {
			t.Fatalf("attempt %d status = %d", i, resp.StatusCode)
		}
	}
}
