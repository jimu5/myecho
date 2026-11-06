package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
)

func TestArticlePreviewHandlersAuthorizeFreezeAndRevoke(t *testing.T) {
	setupAPITestDB(t)
	owner := createAPIUser(t, "preview-owner", "preview-owner-token", model.Author)
	other := createAPIUser(t, "preview-other", "preview-other-token", model.Author)
	category := seedArticleCategory(t)
	article := createEditingTestArticle(t, owner.ID, category.UID, "Live article", "live content")

	submissionSnapshot, err := service.EncodeArticleSubmissionSnapshot(model.ArticleSubmissionSnapshot{
		Title:         "My private draft",
		Slug:          "private-draft",
		Type:          model.ArticleTypePost,
		ContentFormat: model.ArticleContentFormatMarkdown,
		Content:       "draft content",
		CategoryUID:   category.UID,
		PostTime:      time.Now(),
		PasswordHash:  "must-not-leak-this-hash",
		TagUIDs:       []string{},
	})
	if err != nil {
		t.Fatalf("encode draft snapshot: %v", err)
	}
	if err := connect.Database.Create(&model.ArticleSubmission{
		ArticleID: article.ID, SubmitterID: owner.ID,
		Status: model.ArticleSubmissionDraft, Snapshot: submissionSnapshot,
	}).Error; err != nil {
		t.Fatalf("create editable submission: %v", err)
	}

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/articles/:id/previews", middleware.Authentication, middleware.AuthorRequired, ArticlePreviewCreate)
	app.Delete("/articles/:id/previews", middleware.Authentication, middleware.AuthorRequired, ArticlePreviewRevoke)
	path := "/articles/" + strconv.Itoa(int(article.ID)) + "/previews"

	resp := doJSONRequest(t, app, http.MethodPost, path, other.Token, `{"expected_edit_version":1}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("other author create status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, http.MethodPost, path, owner.Token, `{"expected_edit_version":0}`)
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("stale create status = %d, want 409", resp.StatusCode)
	}
	var grants int64
	if err := connect.Database.Model(&model.ArticlePreview{}).Count(&grants).Error; err != nil || grants != 0 {
		t.Fatalf("grant count after rejected creates = %d, err=%v; want 0", grants, err)
	}
	resp = doJSONRequest(t, app, http.MethodPost, path, owner.Token, `{}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("missing version status = %d, want 403", resp.StatusCode)
	}

	create := func() rtype.ArticlePreviewResponse {
		t.Helper()
		response := doJSONRequest(t, app, http.MethodPost, path, owner.Token, `{"expected_edit_version":1}`)
		if response.StatusCode != fiber.StatusOK {
			t.Fatalf("authorized create status = %d, want 200", response.StatusCode)
		}
		var result rtype.ArticlePreviewResponse
		if err := json.Unmarshal(decodeAPIResp(t, response).Data, &result); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		return result
	}
	first := create()
	second := create()
	for _, result := range []rtype.ArticlePreviewResponse{first, second} {
		if !strings.HasPrefix(result.PreviewURL, "/article-preview/") {
			t.Fatalf("preview_url = %q, want opaque preview path", result.PreviewURL)
		}
		token := strings.TrimPrefix(result.PreviewURL, "/article-preview/")
		if len(token) != 43 {
			t.Fatalf("raw token length = %d, want 43 base64url characters", len(token))
		}
		if remaining := time.Until(result.ExpiresAt); remaining < 59*time.Minute || remaining > 61*time.Minute {
			t.Fatalf("preview expiry in %v, want about one hour", remaining)
		}
		digest := sha256.Sum256([]byte(token))
		var grant model.ArticlePreview
		if err := connect.Database.Where("token_hash = ?", hex.EncodeToString(digest[:])).First(&grant).Error; err != nil {
			t.Fatalf("look up hashed grant: %v", err)
		}
		if grant.TokenHash == token || strings.Contains(grant.Snapshot, token) {
			t.Fatal("raw bearer token was persisted")
		}
		if strings.Contains(grant.Snapshot, "must-not-leak-this-hash") || strings.Contains(grant.Snapshot, owner.Email) {
			t.Fatalf("preview snapshot contains private data: %s", grant.Snapshot)
		}
		var frozen model.ArticlePreviewSnapshot
		if err := json.Unmarshal([]byte(grant.Snapshot), &frozen); err != nil {
			t.Fatalf("decode frozen preview: %v", err)
		}
		if frozen.Title != "My private draft" || frozen.Content != "draft content" {
			t.Fatalf("frozen snapshot title/content = %q/%q; want own editable draft", frozen.Title, frozen.Content)
		}
	}
	stored, err := dalArticleByID(article.ID)
	if err != nil {
		t.Fatalf("reload article: %v", err)
	}
	if stored.EditVersion != 1 || stored.Title != "Live article" || stored.Detail == nil || stored.Detail.Content != "live content" {
		t.Fatalf("preview creation changed live article: version=%d title=%q detail=%+v", stored.EditVersion, stored.Title, stored.Detail)
	}

	resp = doJSONRequest(t, app, http.MethodDelete, path, other.Token, "")
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("other author revoke status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, http.MethodDelete, path, owner.Token, "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("owner revoke status = %d, want 200", resp.StatusCode)
	}
	if got := decodeAPIResp(t, resp).Data; string(got) != "null" {
		t.Fatalf("revoke data = %s, want null", got)
	}
	if err := connect.Database.Model(&model.ArticlePreview{}).
		Where("article_id = ? AND creator_id = ? AND revoked_at IS NOT NULL", article.ID, owner.ID).
		Count(&grants).Error; err != nil || grants != 2 {
		t.Fatalf("revoked grant count = %d, err=%v; want 2", grants, err)
	}
}

func TestArticlePreviewCreateReturnsConflictWhenArticleIsTrashed(t *testing.T) {
	setupAPITestDB(t)
	admin := createAPIUser(t, "preview-admin", "preview-admin-token", model.Admin)
	article := createEditingTestArticle(t, admin.ID, "", "Trashed", "content")
	if err := connect.Database.Model(&model.Article{}).Where("id = ?", article.ID).
		UpdateColumn("status", int8(mysql.ARTICLE_STATUS_RECYCLE)).Error; err != nil {
		t.Fatalf("mark article trashed: %v", err)
	}
	app := fiber.New()
	app.Post("/articles/:id/previews", middleware.Authentication, middleware.AuthorRequired, ArticlePreviewCreate)
	resp := doJSONRequest(t, app, http.MethodPost, "/articles/"+strconv.Itoa(int(article.ID))+"/previews", admin.Token, `{"expected_edit_version":1}`)
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("trashed create status = %d, want 409", resp.StatusCode)
	}
	var grants int64
	if err := connect.Database.Model(&model.ArticlePreview{}).Count(&grants).Error; err != nil || grants != 0 {
		t.Fatalf("grant count = %d, err=%v; want 0", grants, err)
	}
}
