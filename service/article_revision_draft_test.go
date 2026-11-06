package service

import (
	"errors"
	"testing"
	"time"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

func TestRestoreOnlineArticleRevisionCreatesDraftAndPreservesLiveMetadata(t *testing.T) {
	setupServiceTestDB(t)
	category := model.Category{Name: "Current category", UID: "revision-current-cat", Type: model.CategoryTypeArticle}
	if err := connect.Database.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	tag := model.Tag{Name: "Current tag", UID: "revision-current-tag"}
	if err := connect.Database.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	allowComment := false
	postTime := time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC)
	statuses := []int8{
		int8(mysql.ARTILCE_STATUS_PUBLIC),
		int8(mysql.ARTICLE_STATUS_TOP),
		int8(mysql.ARTICLE_STATUS_PRIVATE),
	}
	for i, status := range statuses {
		article := mysql.ArticleModel{
			UID: "revision-live-" + string(rune('a'+i)), AuthorID: 91,
			Title: "Live title", Slug: "live-slug", Type: model.ArticleTypePost,
			ContentFormat: model.ArticleContentFormatMarkdown, Summary: "live summary",
			CategoryUID: category.UID, IsAllowComment: &allowComment, PostTime: postTime,
			Status: status, Password: "live-password-hash",
			Detail: &model.ArticleDetail{Content: "live body"},
		}
		if err := (&mysql.ArticleDBRepo{}).Create(&article); err != nil {
			t.Fatalf("create online article for status %d: %v", status, err)
		}
		if err := connect.Database.Model(&article).Association("Tags").Append(&tag); err != nil {
			t.Fatalf("attach current tag: %v", err)
		}
		revision := model.ArticleRevision{
			ArticleID: article.ID, Title: "Restored title", Slug: "restored-slug",
			Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
			Summary: "restored summary", SEOTitle: "restored SEO title",
			SEODescription: "restored SEO description", ShareImage: "restored-image",
			Content: "restored body",
		}
		if err := connect.Database.Create(&revision).Error; err != nil {
			t.Fatalf("create revision: %v", err)
		}
		actorID := uint(200 + i)
		priorPasswordHash := "live-password-hash"
		if i == 0 {
			priorSnapshot, err := EncodeArticleSubmissionSnapshot(model.ArticleSubmissionSnapshot{PasswordHash: "own-draft-password-hash"})
			if err != nil {
				t.Fatalf("encode prior own draft: %v", err)
			}
			prior := model.ArticleSubmission{
				ArticleID: article.ID, SubmitterID: actorID,
				Status: model.ArticleSubmissionDraft, Snapshot: priorSnapshot,
			}
			if err := connect.Database.Create(&prior).Error; err != nil {
				t.Fatalf("create prior own draft: %v", err)
			}
			priorPasswordHash = "own-draft-password-hash"
		}

		live, submission, err := (&ArticleService{}).RestoreArticleRevisionWithSubmission(article.ID, revision.ID, actorID)
		if err != nil {
			t.Fatalf("restore revision for status %d: %v", status, err)
		}
		if live.EditVersion != 2 || live.Status != status || live.Title != "Live title" || live.Detail == nil || live.Detail.Content != "live body" {
			t.Fatalf("restore changed live article for status %d: version=%d status=%d title=%q detail=%+v", status, live.EditVersion, live.Status, live.Title, live.Detail)
		}
		if submission == nil || submission.Status != model.ArticleSubmissionDraft || submission.SubmitterID != actorID {
			t.Fatalf("restore submission = %+v, want own draft", submission)
		}
		snapshot, err := DecodeArticleSubmissionSnapshot(submission)
		if err != nil {
			t.Fatalf("decode restore submission: %v", err)
		}
		if snapshot.Title != revision.Title || snapshot.Slug != revision.Slug || snapshot.Content != revision.Content ||
			snapshot.Summary != revision.Summary || snapshot.SEOTitle != revision.SEOTitle || snapshot.ShareImage != revision.ShareImage {
			t.Fatalf("restored revision fields = %+v, want revision values", snapshot)
		}
		if snapshot.CategoryUID != category.UID || snapshot.IsAllowComment == nil || *snapshot.IsAllowComment ||
			!snapshot.PostTime.Equal(postTime) || snapshot.TargetStatus != status || snapshot.PasswordHash != priorPasswordHash ||
			len(snapshot.TagUIDs) != 1 || snapshot.TagUIDs[0] != tag.UID {
			t.Fatalf("restored snapshot metadata = %+v", snapshot)
		}
		var editableDrafts int64
		if err := connect.Database.Model(&model.ArticleSubmission{}).
			Where("article_id = ? AND submitter_id = ? AND status = ?", article.ID, actorID, model.ArticleSubmissionDraft).
			Count(&editableDrafts).Error; err != nil || editableDrafts != 1 {
			t.Fatalf("editable drafts for actor = %d, err=%v; want one upserted draft", editableDrafts, err)
		}
	}
}

func TestRestoreDraftArticleRevisionStillUpdatesArticle(t *testing.T) {
	setupServiceTestDB(t)
	article := mysql.ArticleModel{
		UID: "revision-draft-article", Title: "Draft current", Slug: "draft-current",
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Status: int8(mysql.ARTICLE_STATUS_DRAFT), Detail: &model.ArticleDetail{Content: "draft current body"},
	}
	if err := (&mysql.ArticleDBRepo{}).Create(&article); err != nil {
		t.Fatalf("create draft article: %v", err)
	}
	revision := model.ArticleRevision{
		ArticleID: article.ID, Title: "Draft restored", Slug: "draft-restored",
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Content: "draft restored body",
	}
	if err := connect.Database.Create(&revision).Error; err != nil {
		t.Fatalf("create revision: %v", err)
	}

	updated, err := (&ArticleService{}).RestoreArticleRevision(article.ID, revision.ID, 77)
	if err != nil {
		t.Fatalf("restore draft revision: %v", err)
	}
	if updated.EditVersion != 2 || updated.Status != int8(mysql.ARTICLE_STATUS_DRAFT) || updated.Title != revision.Title || updated.Detail == nil || updated.Detail.Content != revision.Content {
		t.Fatalf("restored draft article = version:%d status:%d title:%q detail:%+v", updated.EditVersion, updated.Status, updated.Title, updated.Detail)
	}
	var submissions int64
	if err := connect.Database.Model(&model.ArticleSubmission{}).Where("article_id = ?", article.ID).Count(&submissions).Error; err != nil || submissions != 0 {
		t.Fatalf("draft restore created submissions = %d, err=%v; want none", submissions, err)
	}
}

func TestRestoreArticleRevisionRejectsPendingAndTrashedArticles(t *testing.T) {
	setupServiceTestDB(t)
	for _, testCase := range []struct {
		name           string
		status         int8
		wantErr        error
		withSubmission bool
	}{
		{name: "pending", status: int8(mysql.ARTICLE_STATUS_WAIT_REVIEW), wantErr: mysql.ErrArticleSubmissionChanged, withSubmission: true},
		{name: "review status without submission", status: int8(mysql.ARTICLE_STATUS_WAIT_REVIEW), wantErr: mysql.ErrArticleSubmissionChanged},
		{name: "trashed", status: int8(mysql.ARTICLE_STATUS_RECYCLE), wantErr: mysql.ErrArticleTrashed},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			article := mysql.ArticleModel{
				UID: "revision-guard-" + testCase.name, Title: "Guarded current", Slug: "guarded-current",
				Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
				Status: testCase.status, Detail: &model.ArticleDetail{Content: "guarded body"},
			}
			if err := (&mysql.ArticleDBRepo{}).Create(&article); err != nil {
				t.Fatalf("create guarded article: %v", err)
			}
			revision := model.ArticleRevision{
				ArticleID: article.ID, Title: "Restored", Slug: "restored",
				Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
				Content: "restored body",
			}
			if err := connect.Database.Create(&revision).Error; err != nil {
				t.Fatalf("create revision: %v", err)
			}
			if testCase.withSubmission {
				pending := model.ArticleSubmission{
					ArticleID: article.ID, SubmitterID: 55, Status: model.ArticleSubmissionPending,
					Snapshot: `{"title":"pending"}`,
				}
				if err := connect.Database.Create(&pending).Error; err != nil {
					t.Fatalf("create pending submission: %v", err)
				}
			}

			if _, err := (&ArticleService{}).RestoreArticleRevision(article.ID, revision.ID, 77); !errors.Is(err, testCase.wantErr) {
				t.Fatalf("restore error = %v, want %v", err, testCase.wantErr)
			}
			stored, err := (&mysql.ArticleDBRepo{}).FindByID(article.ID)
			if err != nil {
				t.Fatalf("reload guarded article: %v", err)
			}
			if stored.EditVersion != 1 || stored.Status != testCase.status || stored.Title != "Guarded current" || stored.Detail == nil || stored.Detail.Content != "guarded body" {
				t.Fatalf("guarded article changed: version=%d status=%d title=%q detail=%+v", stored.EditVersion, stored.Status, stored.Title, stored.Detail)
			}
		})
	}
}
