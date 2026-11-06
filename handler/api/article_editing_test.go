package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
)

func TestArticleEditingSavesPrivateDraftForEveryPublisherRole(t *testing.T) {
	setupAPITestDB(t)
	admin := createAPIUser(t, "edit-admin", "edit-admin-token", model.Admin)
	editor := createAPIUser(t, "edit-editor", "edit-editor-token", model.Editor)
	trustedAuthor := createAPIUser(t, "edit-trusted", "edit-trusted-token", model.Author)
	if err := connect.Database.Model(trustedAuthor).Update("can_publish", true).Error; err != nil {
		t.Fatalf("trust author: %v", err)
	}
	trustedAuthor.CanPublish = true
	category := seedArticleCategory(t)

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Patch("/articles/:id", middleware.Authentication, ArticleUpdate)

	for _, testCase := range []struct {
		name  string
		user  *model.User
		token string
	}{
		{name: "admin", user: admin, token: admin.Token},
		{name: "editor", user: editor, token: editor.Token},
		{name: "trusted author", user: trustedAuthor, token: trustedAuthor.Token},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			article := createEditingTestArticle(t, testCase.user.ID, category.UID, "Live "+testCase.name, "original live body")
			body := `{"title":"Draft ` + testCase.name + `","content":"private draft body","category_uid":"` + category.UID + `","status":4,"expected_edit_version":1}`
			resp := doJSONRequest(t, app, fiber.MethodPatch, "/articles/"+strconv.Itoa(int(article.ID)), testCase.token, body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("save draft status = %d, want 200", resp.StatusCode)
			}
			var response rtype.ArticleResponse
			if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.EditVersion != 2 || response.Status != int8(mysql.ARTILCE_STATUS_PUBLIC) {
				t.Fatalf("response edit_version/status = %d/%d, want 2/public", response.EditVersion, response.Status)
			}
			if response.Submission == nil || response.Submission.Status != model.ArticleSubmissionDraft || response.Submission.Snapshot.Content != "private draft body" {
				t.Fatalf("response submission = %+v, want private draft", response.Submission)
			}

			stored, err := dalArticleByID(article.ID)
			if err != nil {
				t.Fatalf("reload live article: %v", err)
			}
			if stored.Status != int8(mysql.ARTILCE_STATUS_PUBLIC) || stored.Title != article.Title || stored.Detail == nil || stored.Detail.Content != "original live body" {
				t.Fatalf("draft changed online article: status=%d title=%q detail=%+v", stored.Status, stored.Title, stored.Detail)
			}
			if stored.EditVersion != 2 {
				t.Fatalf("stored edit_version = %d, want 2", stored.EditVersion)
			}
		})
	}
}

func TestArticleEditingAutosaveConflictAndPublish(t *testing.T) {
	setupAPITestDB(t)
	admin := createAPIUser(t, "edit-admin", "edit-admin-token", model.Admin)
	category := seedArticleCategory(t)
	article := createEditingTestArticle(t, admin.ID, category.UID, "Live", "original live body")
	tag := &model.Tag{Name: "Editing tag", UID: "editing-tag"}
	if err := connect.Database.Create(tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := connect.Database.Model(&article).Association("Tags").Append(tag); err != nil {
		t.Fatalf("attach existing tag: %v", err)
	}

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Patch("/articles/:id", middleware.Authentication, ArticleUpdate)

	staleDirect := `{"title":"should not apply","content":"stale body","category_uid":"` + category.UID + `","status":1,"expected_edit_version":0}`
	resp := doJSONRequest(t, app, fiber.MethodPatch, "/articles/"+strconv.Itoa(int(article.ID)), admin.Token, staleDirect)
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("stale direct update status = %d, want 409", resp.StatusCode)
	}
	stored, err := dalArticleByID(article.ID)
	if err != nil {
		t.Fatalf("reload after stale direct update: %v", err)
	}
	if stored.Title != "Live" || stored.Detail == nil || stored.Detail.Content != "original live body" || stored.EditVersion != 1 {
		t.Fatalf("stale direct update partially changed article: %+v detail=%+v", stored, stored.Detail)
	}

	autosave := `{"title":"Autosaved","content":"private autosave body","category_uid":"` + category.UID + `","status":1,"autosave":true,"expected_edit_version":1}`
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/articles/"+strconv.Itoa(int(article.ID)), admin.Token, autosave)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("autosave status = %d, want 200", resp.StatusCode)
	}
	var autosaved rtype.ArticleResponse
	if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &autosaved); err != nil {
		t.Fatalf("decode autosave response: %v", err)
	}
	if autosaved.EditVersion != 2 || autosaved.Status != int8(mysql.ARTILCE_STATUS_PUBLIC) || autosaved.Submission == nil || autosaved.Submission.Snapshot.Content != "private autosave body" {
		t.Fatalf("autosave response = %+v", autosaved)
	}
	stored, err = dalArticleByID(article.ID)
	if err != nil {
		t.Fatalf("reload after autosave: %v", err)
	}
	if stored.Title != "Live" || stored.Status != int8(mysql.ARTILCE_STATUS_PUBLIC) || stored.Detail == nil || stored.Detail.Content != "original live body" || stored.EditVersion != 2 {
		t.Fatalf("autosave changed live article: %+v detail=%+v", stored, stored.Detail)
	}

	staleDraft := `{"title":"Stale draft","content":"stale draft body","category_uid":"` + category.UID + `","status":4,"expected_edit_version":1}`
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/articles/"+strconv.Itoa(int(article.ID)), admin.Token, staleDraft)
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("stale draft status = %d, want 409", resp.StatusCode)
	}
	var staleDraftCount int64
	if err := connect.Database.Model(&model.ArticleSubmission{}).
		Where("article_id = ? AND submitter_id = ? AND status = ?", article.ID, admin.ID, model.ArticleSubmissionDraft).
		Count(&staleDraftCount).Error; err != nil || staleDraftCount != 1 {
		t.Fatalf("draft count after stale request = %d, err=%v; want original snapshot only", staleDraftCount, err)
	}

	publish := `{"title":"Published","content":"published body","category_uid":"` + category.UID + `","status":1,"expected_edit_version":2}`
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/articles/"+strconv.Itoa(int(article.ID)), admin.Token, publish)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("publish with refreshed token status = %d, want 200", resp.StatusCode)
	}
	var published rtype.ArticleResponse
	if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &published); err != nil {
		t.Fatalf("decode published response: %v", err)
	}
	if published.EditVersion != 3 || published.Title != "Published" {
		t.Fatalf("published response edit_version/title = %d/%q, want 3/Published", published.EditVersion, published.Title)
	}
	stored, err = dalArticleByID(article.ID)
	if err != nil {
		t.Fatalf("reload published article: %v", err)
	}
	if stored.Title != "Published" || stored.Detail == nil || stored.Detail.Content != "published body" || stored.EditVersion != 3 {
		t.Fatalf("published article = %+v detail=%+v", stored, stored.Detail)
	}
	if len(stored.Tags) != 0 {
		t.Fatalf("published article tags = %+v, want empty after clearing tags", stored.Tags)
	}
	var activeDrafts int64
	if err := connect.Database.Model(&model.ArticleSubmission{}).
		Where("article_id = ? AND submitter_id = ? AND status IN ?", article.ID, admin.ID, []model.ArticleSubmissionStatus{model.ArticleSubmissionDraft, model.ArticleSubmissionRejected}).
		Count(&activeDrafts).Error; err != nil || activeDrafts != 0 {
		t.Fatalf("active own drafts after publish = %d, err=%v; want 0", activeDrafts, err)
	}
}

func TestArticleEditingReadCountDoesNotConflictOrGetOverwritten(t *testing.T) {
	setupAPITestDB(t)
	admin := createAPIUser(t, "edit-admin", "edit-admin-token", model.Admin)
	category := seedArticleCategory(t)
	article := createEditingTestArticle(t, admin.ID, category.UID, "Live", "original live body")
	if err := dalArticleAddReadCount(article.ID, 7); err != nil {
		t.Fatalf("increase read count: %v", err)
	}

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Patch("/articles/:id", middleware.Authentication, ArticleUpdate)
	body := `{"title":"Updated","content":"updated body","category_uid":"` + category.UID + `","status":1,"expected_edit_version":1}`
	resp := doJSONRequest(t, app, fiber.MethodPatch, "/articles/"+strconv.Itoa(int(article.ID)), admin.Token, body)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("versioned update after read count status = %d, want 200", resp.StatusCode)
	}
	stored, err := dalArticleByID(article.ID)
	if err != nil {
		t.Fatalf("reload after read count and update: %v", err)
	}
	if stored.ReadCount != 7 || stored.EditVersion != 2 || stored.Title != "Updated" {
		t.Fatalf("stored read_count/edit_version/title = %d/%d/%q, want 7/2/Updated", stored.ReadCount, stored.EditVersion, stored.Title)
	}
}

func TestArticleEditingPasswordProtectionSurvivesReloadedDraftAndPublish(t *testing.T) {
	setupAPITestDB(t)
	admin := createAPIUser(t, "edit-admin", "edit-admin-token", model.Admin)
	category := seedArticleCategory(t)
	article := createEditingTestArticle(t, admin.ID, category.UID, "Live", "original live body")

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Patch("/articles/:id", middleware.Authentication, ArticleUpdate)
	app.Get("/articles/:id", middleware.Authentication, ArticleRetrieve)
	articlePath := "/articles/" + strconv.Itoa(int(article.ID))

	firstSave := `{"title":"Protected draft","content":"private body","category_uid":"` + category.UID + `","status":4,"autosave":true,"password":"draft secret","expected_edit_version":1}`
	resp := doJSONRequest(t, app, fiber.MethodPatch, articlePath, admin.Token, firstSave)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("first autosave status = %d, want 200", resp.StatusCode)
	}
	var firstResponse rtype.ArticleResponse
	if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &firstResponse); err != nil {
		t.Fatalf("decode first autosave: %v", err)
	}
	if firstResponse.Submission == nil || !firstResponse.Submission.Snapshot.IsPasswordProtected {
		t.Fatalf("first autosave password state = %+v, want protected", firstResponse.Submission)
	}

	resp = doJSONRequest(t, app, fiber.MethodGet, articlePath+"?no_read=true", admin.Token, "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("reload article status = %d, want 200", resp.StatusCode)
	}
	var reloaded rtype.ArticleResponse
	if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &reloaded); err != nil {
		t.Fatalf("decode reloaded article: %v", err)
	}
	if reloaded.Submission == nil || !reloaded.Submission.Snapshot.IsPasswordProtected {
		t.Fatalf("reloaded draft password state = %+v, want protected", reloaded.Submission)
	}

	secondSave := `{"title":"Protected draft","content":"private body v2","category_uid":"` + category.UID + `","status":4,"autosave":true,"expected_edit_version":2}`
	resp = doJSONRequest(t, app, fiber.MethodPatch, articlePath, admin.Token, secondSave)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("second autosave status = %d, want 200", resp.StatusCode)
	}
	var secondResponse rtype.ArticleResponse
	if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &secondResponse); err != nil {
		t.Fatalf("decode second autosave: %v", err)
	}
	if secondResponse.Submission == nil || !secondResponse.Submission.Snapshot.IsPasswordProtected {
		t.Fatalf("second autosave password state = %+v, want protected", secondResponse.Submission)
	}

	publish := `{"title":"Protected published","content":"published protected body","category_uid":"` + category.UID + `","status":1,"expected_edit_version":3}`
	resp = doJSONRequest(t, app, fiber.MethodPatch, articlePath, admin.Token, publish)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("publish status = %d, want 200", resp.StatusCode)
	}
	stored, err := dalArticleByID(article.ID)
	if err != nil {
		t.Fatalf("reload published article: %v", err)
	}
	if stored.Password == "" {
		t.Fatal("published article lost password protection")
	}
	if err := service.CheckArticlePassword(stored.Password, "draft secret"); err != nil {
		t.Fatalf("published password hash no longer matches: %v", err)
	}
}

func TestArticleRevisionRestoreCreatesIsolatedDraftForAuthorAndAdmin(t *testing.T) {
	setupAPITestDB(t)
	admin := createAPIUser(t, "revision-admin", "revision-admin-token", model.Admin)
	author := createAPIUser(t, "revision-author", "revision-author-token", model.Author)
	category := seedArticleCategory(t)
	article := createEditingTestArticle(t, author.ID, category.UID, "Live title", "live body")
	if err := connect.Database.Model(&article).UpdateColumn("password", "live-password-hash").Error; err != nil {
		t.Fatalf("set live password hash: %v", err)
	}
	revision := model.ArticleRevision{
		ArticleID: article.ID, Title: "Restored title", Slug: "restored-slug",
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Summary: "restored summary", Content: "restored body",
	}
	if err := connect.Database.Create(&revision).Error; err != nil {
		t.Fatalf("create revision: %v", err)
	}
	priorSnapshot, err := service.EncodeArticleSubmissionSnapshot(model.ArticleSubmissionSnapshot{
		PasswordHash: "author-draft-password-hash",
	})
	if err != nil {
		t.Fatalf("encode prior author draft: %v", err)
	}
	priorDraft := model.ArticleSubmission{
		ArticleID: article.ID, SubmitterID: author.ID,
		Status: model.ArticleSubmissionDraft, Snapshot: priorSnapshot,
	}
	if err := connect.Database.Create(&priorDraft).Error; err != nil {
		t.Fatalf("create prior author draft: %v", err)
	}

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/articles/:id/revisions/:revision_id/restore", middleware.Authentication, ArticleRevisionRestore)
	for i, testCase := range []struct {
		name  string
		user  *model.User
		token string
	}{
		{name: "admin", user: admin, token: admin.Token},
		{name: "author", user: author, token: author.Token},
	} {
		path := "/articles/" + strconv.Itoa(int(article.ID)) + "/revisions/" + strconv.Itoa(int(revision.ID)) + "/restore"
		resp := doJSONRequest(t, app, fiber.MethodPost, path, testCase.token, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s restore status = %d, want 200", testCase.name, resp.StatusCode)
		}
		envelope := decodeAPIResp(t, resp)
		var response rtype.ArticleResponse
		if err := json.Unmarshal(envelope.Data, &response); err != nil {
			t.Fatalf("decode %s restore response: %v", testCase.name, err)
		}
		if response.EditVersion != uint64(i+2) || response.Status != int8(mysql.ARTILCE_STATUS_PUBLIC) || response.Title != "Live title" {
			t.Fatalf("%s restore response version/status/title = %d/%d/%q", testCase.name, response.EditVersion, response.Status, response.Title)
		}
		if response.Submission == nil || response.Submission.Status != model.ArticleSubmissionDraft ||
			response.Submission.Submitter == nil || response.Submission.Submitter.ID != testCase.user.ID ||
			response.Submission.Snapshot.Title != revision.Title || response.Submission.Snapshot.Content != revision.Content ||
			!response.Submission.Snapshot.IsPasswordProtected {
			t.Fatalf("%s restore response submission = %+v", testCase.name, response.Submission)
		}
		if strings.Contains(string(envelope.Data), "live-password-hash") || strings.Contains(string(envelope.Data), "author-draft-password-hash") {
			t.Fatalf("%s restore response leaked a stored password hash: %s", testCase.name, envelope.Data)
		}
		stored, err := dalArticleByID(article.ID)
		if err != nil {
			t.Fatalf("reload live article after %s restore: %v", testCase.name, err)
		}
		if stored.EditVersion != uint64(i+2) || stored.Status != int8(mysql.ARTILCE_STATUS_PUBLIC) || stored.Title != "Live title" ||
			stored.Detail == nil || stored.Detail.Content != "live body" {
			t.Fatalf("%s restore changed live article: version=%d status=%d title=%q detail=%+v", testCase.name, stored.EditVersion, stored.Status, stored.Title, stored.Detail)
		}
		wantPasswordHash := "live-password-hash"
		if testCase.user.ID == author.ID {
			wantPasswordHash = "author-draft-password-hash"
		}
		var saved model.ArticleSubmission
		if err := connect.Database.Where("id = ?", response.Submission.ID).First(&saved).Error; err != nil {
			t.Fatalf("reload %s restore submission: %v", testCase.name, err)
		}
		savedSnapshot, err := service.DecodeArticleSubmissionSnapshot(&saved)
		if err != nil {
			t.Fatalf("decode stored %s restore submission: %v", testCase.name, err)
		}
		if savedSnapshot.PasswordHash != wantPasswordHash {
			t.Fatalf("%s restore password hash = %q, want preserved hash", testCase.name, savedSnapshot.PasswordHash)
		}
	}
}

func TestArticleRevisionRestorePendingAndTrashedReturnConflict(t *testing.T) {
	setupAPITestDB(t)
	admin := createAPIUser(t, "revision-guard-admin", "revision-guard-admin-token", model.Admin)
	category := seedArticleCategory(t)
	pendingArticle := createEditingTestArticle(t, admin.ID, category.UID, "Pending live", "pending live body")
	trashedArticle := createEditingTestArticle(t, admin.ID, category.UID, "Trashed live", "trashed live body")
	revision := model.ArticleRevision{
		ArticleID: pendingArticle.ID, Title: "Restored", Slug: "restored",
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Content: "restored body",
	}
	if err := connect.Database.Create(&revision).Error; err != nil {
		t.Fatalf("create pending article revision: %v", err)
	}
	if err := connect.Database.Model(&model.Article{}).Where("id = ?", trashedArticle.ID).
		UpdateColumn("status", int8(mysql.ARTICLE_STATUS_RECYCLE)).Error; err != nil {
		t.Fatalf("trash article: %v", err)
	}
	pending := model.ArticleSubmission{
		ArticleID: pendingArticle.ID, SubmitterID: admin.ID,
		Status: model.ArticleSubmissionPending, Snapshot: `{"title":"pending"}`,
	}
	if err := connect.Database.Create(&pending).Error; err != nil {
		t.Fatalf("create pending submission: %v", err)
	}

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/articles/:id/revisions/:revision_id/restore", middleware.Authentication, ArticleRevisionRestore)
	for _, articleID := range []uint{pendingArticle.ID, trashedArticle.ID} {
		path := "/articles/" + strconv.Itoa(int(articleID)) + "/revisions/" + strconv.Itoa(int(revision.ID)) + "/restore"
		resp := doJSONRequest(t, app, fiber.MethodPost, path, admin.Token, "")
		if resp.StatusCode != fiber.StatusConflict {
			t.Fatalf("guarded restore status = %d, want 409 (article %d)", resp.StatusCode, articleID)
		}
		stored, err := dalArticleByID(articleID)
		if err != nil {
			t.Fatalf("reload guarded article %d: %v", articleID, err)
		}
		if stored.EditVersion != 1 {
			t.Fatalf("guarded article %d edit_version = %d, want 1", articleID, stored.EditVersion)
		}
	}
}

func createEditingTestArticle(t *testing.T, authorID uint, categoryUID, title, content string) mysql.ArticleModel {
	t.Helper()
	article := mysql.ArticleModel{
		Title: title, CategoryUID: categoryUID, AuthorID: authorID,
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Status: int8(mysql.ARTILCE_STATUS_PUBLIC), Detail: &model.ArticleDetail{Content: content},
	}
	if err := dalCreateArticle(&article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	return article
}

func dalArticleByID(id uint) (mysql.ArticleModel, error) {
	return (&mysql.ArticleDBRepo{}).FindByID(id)
}

func dalArticleAddReadCount(id uint, count uint) error {
	return (&mysql.ArticleDBRepo{}).AddReadCountByID(id, count)
}

func dalCreateArticle(article *mysql.ArticleModel) error {
	return (&mysql.ArticleDBRepo{}).Create(article)
}
