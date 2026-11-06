package api

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
)

func TestAuthorReviewWorkflowAndOwnership(t *testing.T) {
	setupAPITestDB(t)
	author := createAPIUser(t, "author", "author-token", model.Author)
	other := createAPIUser(t, "other", "other-token", model.Author)
	editor := createAPIUser(t, "editor", "editor-token", model.Editor)
	category := seedArticleCategory(t)

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Get("/all_articles", middleware.Authentication, middleware.AuthorRequired, ArticleAllList)
	app.Post("/articles", middleware.Authentication, middleware.AuthorRequired, ArticleCreate)
	app.Patch("/articles/:id", middleware.Authentication, middleware.AuthorRequired, ArticleUpdate)
	app.Get("/articles/:id/reviews", middleware.Authentication, middleware.AuthorRequired, ArticleReviewList)
	app.Post("/articles/:id/review", middleware.Authentication, middleware.EditorRequired, ArticleReview)

	create := func(token, title string, status int) rtype.ArticleResponse {
		body := `{"title":"` + title + `","content":"body","category_uid":"` + category.UID + `","status":` + strconv.Itoa(status) + `}`
		resp := doJSONRequest(t, app, fiber.MethodPost, "/articles", token, body)
		if resp.StatusCode != fiber.StatusCreated {
			t.Fatalf("create %s status = %d, want 201", title, resp.StatusCode)
		}
		var article rtype.ArticleResponse
		if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &article); err != nil {
			t.Fatalf("decode %s: %v", title, err)
		}
		return article
	}

	pending := create(author.Token, "Pending", int(mysql.ARTICLE_STATUS_WAIT_REVIEW))
	_ = create(other.Token, "Other draft", int(mysql.ARTICLE_STATUS_DRAFT))
	var submitCount int64
	if err := connect.Database.Model(&model.ArticleReview{}).
		Where("article_id = ? AND action = ?", pending.ID, model.ArticleReviewSubmit).
		Count(&submitCount).Error; err != nil || submitCount != 1 {
		t.Fatalf("submit audit count = %d, err=%v", submitCount, err)
	}

	resp := doJSONRequest(t, app, fiber.MethodGet, "/all_articles?page=1&page_size=10", author.Token, "")
	if resp.StatusCode != fiber.StatusOK || decodeAPIResp(t, resp).Meta["total"] != float64(1) {
		t.Fatalf("author list did not restrict ownership, status=%d", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/all_articles?status=5&page=1&page_size=10", editor.Token, "")
	listBody := decodeAPIResp(t, resp)
	if resp.StatusCode != fiber.StatusOK || listBody.Meta["total"] != float64(1) {
		t.Fatalf("review queue list = status %d meta %+v", resp.StatusCode, listBody.Meta)
	}
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/articles/"+strconv.Itoa(int(pending.ID)), other.Token,
		`{"title":"Hijacked","content":"body","category_uid":"`+category.UID+`","status":4}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("other author update status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/articles/"+strconv.Itoa(int(pending.ID)), author.Token,
		`{"title":"Published","content":"body","category_uid":"`+category.UID+`","status":1}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("author publish status = %d, want 403", resp.StatusCode)
	}
	var beforeReviewCategory model.Category
	if err := connect.Database.Where("uid = ?", category.UID).First(&beforeReviewCategory).Error; err != nil || beforeReviewCategory.Count != 0 {
		t.Fatalf("category count before review = %d, err=%v", beforeReviewCategory.Count, err)
	}

	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/"+strconv.Itoa(int(pending.ID))+"/review", editor.Token,
		`{"action":"approve","note":"ready"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("approve status = %d, want 200", resp.StatusCode)
	}
	var stored model.Article
	if err := connect.Database.First(&stored, pending.ID).Error; err != nil || stored.Status != int8(mysql.ARTILCE_STATUS_PUBLIC) {
		t.Fatalf("approved article status = %d, err=%v", stored.Status, err)
	}
	var storedCategory model.Category
	if err := connect.Database.Where("uid = ?", category.UID).First(&storedCategory).Error; err != nil || storedCategory.Count != 1 {
		t.Fatalf("approved category count = %d, err=%v", storedCategory.Count, err)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/"+strconv.Itoa(int(pending.ID))+"/review", editor.Token,
		`{"action":"approve"}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("second review status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/articles/"+strconv.Itoa(int(pending.ID)), author.Token,
		`{"title":"Pending v2","content":"new reviewed body","category_uid":"`+category.UID+`","status":5}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("submit published edit status = %d, want 200", resp.StatusCode)
	}
	if err := connect.Database.Preload("Detail").First(&stored, pending.ID).Error; err != nil {
		t.Fatalf("load online article: %v", err)
	}
	if stored.Title != "Pending" || stored.Detail == nil || stored.Detail.Content != "body" || stored.Status != int8(mysql.ARTILCE_STATUS_PUBLIC) {
		t.Fatalf("pending edit overwrote online article: %+v detail=%+v", stored, stored.Detail)
	}
	submission, err := service.PendingArticleSubmission(pending.ID)
	if err != nil || submission == nil {
		t.Fatalf("pending submission = %+v, err=%v", submission, err)
	}
	snapshot, err := service.DecodeArticleSubmissionSnapshot(submission)
	if err != nil || snapshot.Title != "Pending v2" || snapshot.Content != "new reviewed body" {
		t.Fatalf("submission snapshot = %+v, err=%v", snapshot, err)
	}

	rejected := create(author.Token, "Rejected", int(mysql.ARTICLE_STATUS_WAIT_REVIEW))
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/"+strconv.Itoa(int(rejected.ID))+"/review", editor.Token,
		`{"action":"reject"}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("reject without note status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/articles/"+strconv.Itoa(int(rejected.ID))+"/review", editor.Token,
		`{"action":"reject","note":"needs work"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("reject status = %d, want 200", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/articles/"+strconv.Itoa(int(rejected.ID))+"/reviews", author.Token, "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("review history status = %d, want 200", resp.StatusCode)
	}
	var reviews []articleReviewResponse
	if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &reviews); err != nil || len(reviews) != 2 || reviews[0].Action != model.ArticleReviewReject {
		t.Fatalf("review history = %+v, err=%v", reviews, err)
	}
}
