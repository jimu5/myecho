package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"myecho/dal"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

func TestDuplicateAsDraftUsesOwnEditableSnapshotAndResetsIdentity(t *testing.T) {
	for _, status := range []model.ArticleSubmissionStatus{model.ArticleSubmissionDraft, model.ArticleSubmissionRejected} {
		t.Run(fmt.Sprintf("submission_status_%d", status), func(t *testing.T) {
			setupServiceTestDB(t)
			owner := createArticleDuplicateTestUser(t, "duplicate-owner")
			category := createArticleDuplicateTestCategory(t, "duplicate-category")
			tag := createArticleDuplicateTestTag(t, "duplicate-tag")
			source := createArticleDuplicateTestArticle(t, owner.ID, category.UID, []*model.Tag{tag}, mysql.ARTILCE_STATUS_PUBLIC)
			if err := connect.Database.Model(&mysql.ArticleModel{}).Where("id = ?", source.ID).Updates(map[string]any{
				"read_count": 29, "like_count": 11, "comment_count": 7, "password": "source-password-hash",
			}).Error; err != nil {
				t.Fatalf("seed source counters/password: %v", err)
			}
			seedArticleDuplicateHistory(t, source, owner.ID)

			snapshotTag := createArticleDuplicateTestTag(t, "snapshot-tag")
			allowSnapshotComment := false
			snapshot := model.ArticleSubmissionSnapshot{
				Title: "Private draft title", Slug: "private-draft-slug", SEOTitle: "Private SEO title",
				SEODescription: "Private SEO description", ShareImage: "/private/share.jpg",
				Type: model.ArticleTypePage, ContentFormat: model.ArticleContentFormatHTML,
				Summary: "Private draft summary", Content: "<h1>Private draft body</h1>",
				CategoryUID: category.UID, IsAllowComment: &allowSnapshotComment,
				PostTime: time.Now().Add(48 * time.Hour), TargetStatus: int8(mysql.ARTILCE_STATUS_PUBLIC),
				PasswordHash: "private-submission-password-hash", TagUIDs: []string{snapshotTag.UID},
			}
			encoded, err := EncodeArticleSubmissionSnapshot(snapshot)
			if err != nil {
				t.Fatalf("encode submission snapshot: %v", err)
			}
			if err := connect.Database.Create(&model.ArticleSubmission{
				ArticleID: source.ID, SubmitterID: owner.ID, Status: status, Snapshot: encoded,
			}).Error; err != nil {
				t.Fatalf("create editable submission: %v", err)
			}
			pendingSnapshot := snapshot
			pendingSnapshot.Title = "Newer pending snapshot must not mask draft"
			pendingEncoded, err := EncodeArticleSubmissionSnapshot(pendingSnapshot)
			if err != nil {
				t.Fatalf("encode pending snapshot: %v", err)
			}
			if err := connect.Database.Create(&model.ArticleSubmission{
				ArticleID: source.ID, SubmitterID: owner.ID, Status: model.ArticleSubmissionPending, Snapshot: pendingEncoded,
			}).Error; err != nil {
				t.Fatalf("create pending submission: %v", err)
			}

			beforeCreate := time.Now()
			duplicate, err := (&ArticleService{}).DuplicateAsDraft(source.ID, owner.ID)
			finishedAt := time.Now()
			if err != nil {
				t.Fatalf("DuplicateAsDraft() error = %v", err)
			}
			if duplicate.ID == 0 || duplicate.ID == source.ID || duplicate.UID == "" || duplicate.UID == source.UID {
				t.Fatalf("duplicate identity = id:%d uid:%q; source id:%d uid:%q", duplicate.ID, duplicate.UID, source.ID, source.UID)
			}
			if duplicate.Detail == nil || duplicate.Detail.UID == "" || duplicate.Detail.UID == source.DetailUID || duplicate.DetailUID == source.DetailUID {
				t.Fatalf("duplicate detail identity = detail_uid:%q detail:%+v, source detail_uid:%q", duplicate.DetailUID, duplicate.Detail, source.DetailUID)
			}
			if duplicate.AuthorID != owner.ID || duplicate.Status != int8(mysql.ARTICLE_STATUS_DRAFT) || duplicate.EditVersion != 1 {
				t.Fatalf("duplicate owner/status/version = %d/%d/%d", duplicate.AuthorID, duplicate.Status, duplicate.EditVersion)
			}
			if duplicate.Title != snapshot.Title || duplicate.SEOTitle != snapshot.SEOTitle || duplicate.SEODescription != snapshot.SEODescription || duplicate.ShareImage != snapshot.ShareImage || duplicate.Type != snapshot.Type || duplicate.ContentFormat != snapshot.ContentFormat || duplicate.Summary != snapshot.Summary || duplicate.Detail.Content != snapshot.Content || duplicate.CategoryUID != snapshot.CategoryUID {
				t.Fatalf("duplicate content metadata did not use snapshot: %+v detail=%+v", duplicate, duplicate.Detail)
			}
			if duplicate.Slug == snapshot.Slug || duplicate.Slug == "" || duplicate.Password != "" {
				t.Fatalf("duplicate slug/password = %q/%q; snapshot slug must be regenerated and password cleared", duplicate.Slug, duplicate.Password)
			}
			if duplicate.IsAllowComment == nil || *duplicate.IsAllowComment != allowSnapshotComment {
				t.Fatalf("duplicate comment setting = %v, want %v", duplicate.IsAllowComment, allowSnapshotComment)
			}
			if duplicate.PostTime.Before(beforeCreate) || duplicate.PostTime.After(finishedAt) {
				t.Fatalf("duplicate post time %s is outside creation interval [%s, %s]", duplicate.PostTime, beforeCreate, finishedAt)
			}
			if duplicate.ReadCount != 0 || duplicate.LikeCount != 0 || duplicate.CommentCount != 0 {
				t.Fatalf("duplicate counters = read:%d like:%d comment:%d", duplicate.ReadCount, duplicate.LikeCount, duplicate.CommentCount)
			}
			if len(duplicate.Tags) != 1 || duplicate.Tags[0].UID != snapshotTag.UID {
				t.Fatalf("duplicate tags = %+v, want only %q", duplicate.Tags, snapshotTag.UID)
			}
			assertArticleDuplicateHasNoHistory(t, duplicate)

			unchanged, err := dal.MySqlDB.Article.FindByID(source.ID)
			if err != nil {
				t.Fatalf("reload source: %v", err)
			}
			if unchanged.Title != source.Title || unchanged.Slug != source.Slug || unchanged.Detail == nil || unchanged.Detail.Content != source.Detail.Content || unchanged.Password != "source-password-hash" || unchanged.ReadCount != 29 || unchanged.LikeCount != 11 || unchanged.CommentCount != 7 || len(unchanged.Tags) != 1 || unchanged.Tags[0].UID != tag.UID {
				t.Fatalf("source changed during duplication: %+v detail=%+v tags=%+v", unchanged, unchanged.Detail, unchanged.Tags)
			}
			var updatedCategory mysql.CategoryModel
			if err := connect.Database.Where("uid = ?", category.UID).First(&updatedCategory).Error; err != nil {
				t.Fatalf("reload category: %v", err)
			}
			if updatedCategory.Count != 1 {
				t.Fatalf("category count = %d, want 1 from source only", updatedCategory.Count)
			}
		})
	}
}

func TestDuplicateAsDraftUsesCanonicalArticleForOtherOwner(t *testing.T) {
	setupServiceTestDB(t)
	owner := createArticleDuplicateTestUser(t, "duplicate-source-owner")
	actor := createArticleDuplicateTestUser(t, "duplicate-editor")
	category := createArticleDuplicateTestCategory(t, "duplicate-canonical-category")
	tag := createArticleDuplicateTestTag(t, "duplicate-canonical-tag")
	source := createArticleDuplicateTestArticle(t, owner.ID, category.UID, []*model.Tag{tag}, mysql.ARTILCE_STATUS_PUBLIC)
	privateDraft := model.ArticleSubmissionSnapshot{
		Title: "Owner private draft", Slug: "owner-private", Type: model.ArticleTypePost,
		ContentFormat: model.ArticleContentFormatMarkdown, Summary: "private summary", Content: "private body",
		CategoryUID: category.UID, TagUIDs: []string{tag.UID}, PasswordHash: "private hash",
	}
	encoded, err := EncodeArticleSubmissionSnapshot(privateDraft)
	if err != nil {
		t.Fatalf("encode owner snapshot: %v", err)
	}
	if err := connect.Database.Create(&model.ArticleSubmission{
		ArticleID: source.ID, SubmitterID: owner.ID, Status: model.ArticleSubmissionDraft, Snapshot: encoded,
	}).Error; err != nil {
		t.Fatalf("create owner's private submission: %v", err)
	}

	duplicate, err := (&ArticleService{}).DuplicateAsDraft(source.ID, actor.ID)
	if err != nil {
		t.Fatalf("DuplicateAsDraft() error = %v", err)
	}
	if duplicate.AuthorID != actor.ID || duplicate.Status != int8(mysql.ARTICLE_STATUS_DRAFT) || duplicate.Title != source.Title || duplicate.Detail == nil || duplicate.Detail.Content != source.Detail.Content || duplicate.Password != "" {
		t.Fatalf("duplicate should use canonical source for different actor: %+v detail=%+v", duplicate, duplicate.Detail)
	}
	if duplicate.Title == privateDraft.Title || duplicate.Detail.Content == privateDraft.Content || duplicate.Slug == privateDraft.Slug {
		t.Fatalf("duplicate leaked source owner's private snapshot: %+v", duplicate)
	}
	if len(duplicate.Tags) != 1 || duplicate.Tags[0].UID != tag.UID {
		t.Fatalf("duplicate tags = %+v, want canonical source tags", duplicate.Tags)
	}
	assertArticleDuplicateHasNoHistory(t, duplicate)
}

func TestDuplicateAsDraftRejectsTrashAndRollsBackFailedCreate(t *testing.T) {
	setupServiceTestDB(t)
	owner := createArticleDuplicateTestUser(t, "duplicate-atomic-owner")
	trashed := createArticleDuplicateTestArticle(t, owner.ID, "", nil, mysql.ARTICLE_STATUS_RECYCLE)
	if _, err := (&ArticleService{}).DuplicateAsDraft(trashed.ID, owner.ID); !errors.Is(err, mysql.ErrArticleTrashed) {
		t.Fatalf("DuplicateAsDraft(trashed) error = %v, want ErrArticleTrashed", err)
	}

	source := createArticleDuplicateTestArticle(t, owner.ID, "", nil, mysql.ARTICLE_STATUS_DRAFT)
	var articlesBefore, detailsBefore int64
	if err := connect.Database.Model(&mysql.ArticleModel{}).Count(&articlesBefore).Error; err != nil {
		t.Fatalf("count articles before failure: %v", err)
	}
	if err := connect.Database.Model(&model.ArticleDetail{}).Count(&detailsBefore).Error; err != nil {
		t.Fatalf("count details before failure: %v", err)
	}
	if err := connect.Database.Exec("CREATE TRIGGER fail_duplicate_article BEFORE INSERT ON articles BEGIN SELECT RAISE(ABORT, 'duplicate insert failure'); END").Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	if _, err := (&ArticleService{}).DuplicateAsDraft(source.ID, owner.ID); err == nil {
		t.Fatal("DuplicateAsDraft() error = nil with failing article insert")
	}
	var articlesAfter, detailsAfter int64
	if err := connect.Database.Model(&mysql.ArticleModel{}).Count(&articlesAfter).Error; err != nil {
		t.Fatalf("count articles after failure: %v", err)
	}
	if err := connect.Database.Model(&model.ArticleDetail{}).Count(&detailsAfter).Error; err != nil {
		t.Fatalf("count details after failure: %v", err)
	}
	if articlesAfter != articlesBefore || detailsAfter != detailsBefore {
		t.Fatalf("failed duplicate left partial rows: articles %d->%d, details %d->%d", articlesBefore, articlesAfter, detailsBefore, detailsAfter)
	}
}

func createArticleDuplicateTestUser(t *testing.T, name string) *model.User {
	t.Helper()
	user := &model.User{Name: name, Email: name + "@example.com", PermissionType: model.Author}
	if err := connect.Database.Create(user).Error; err != nil {
		t.Fatalf("create user %q: %v", name, err)
	}
	return user
}

func createArticleDuplicateTestCategory(t *testing.T, uid string) *mysql.CategoryModel {
	t.Helper()
	category := &mysql.CategoryModel{Name: uid, UID: uid, Type: model.CategoryTypeArticle}
	if err := connect.Database.Create(category).Error; err != nil {
		t.Fatalf("create category %q: %v", uid, err)
	}
	return category
}

func createArticleDuplicateTestTag(t *testing.T, uid string) *model.Tag {
	t.Helper()
	tag := &model.Tag{Name: uid, UID: uid}
	if err := connect.Database.Create(tag).Error; err != nil {
		t.Fatalf("create tag %q: %v", uid, err)
	}
	return tag
}

func createArticleDuplicateTestArticle(t *testing.T, ownerID uint, categoryUID string, tags []*model.Tag, status mysql.ArticleStatus) *mysql.ArticleModel {
	t.Helper()
	allowComment := true
	article := &mysql.ArticleModel{
		AuthorID: ownerID, Title: "Original live title", Slug: "source-live-slug",
		SEOTitle: "Original SEO title", SEODescription: "Original SEO description", ShareImage: "/original.jpg",
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Summary: "Original summary", IsAllowComment: &allowComment, CategoryUID: categoryUID,
		PostTime: time.Now().Add(24 * time.Hour), Status: int8(status), Password: "source-password",
		ReadCount: 13, LikeCount: 5, CommentCount: 2,
		Detail: &model.ArticleDetail{Content: "Original canonical body"}, Tags: tags,
	}
	if err := dal.MySqlDB.Article.Create(article); err != nil {
		t.Fatalf("create source article: %v", err)
	}
	return article
}

func seedArticleDuplicateHistory(t *testing.T, article *mysql.ArticleModel, actorID uint) {
	t.Helper()
	if err := connect.Database.Create(&model.ArticleRevision{ArticleID: article.ID, ActorID: actorID, Title: "old revision", Content: "old body"}).Error; err != nil {
		t.Fatalf("create source revision: %v", err)
	}
	if err := connect.Database.Create(&model.Comment{ArticleUID: article.UID, AuthorName: "reader", Content: "source comment"}).Error; err != nil {
		t.Fatalf("create source comment: %v", err)
	}
	if err := connect.Database.Create(&model.ArticleDailyStat{ArticleUID: article.UID, Date: "2026-10-01", Views: 19}).Error; err != nil {
		t.Fatalf("create source daily stat: %v", err)
	}
	if err := connect.Database.Create(&model.ArticleSlugRedirect{ArticleUID: article.UID, Slug: "source-old-slug", Type: model.ArticleTypePost}).Error; err != nil {
		t.Fatalf("create source slug redirect: %v", err)
	}
	if err := connect.Database.Create(&model.ArticleReview{ArticleID: article.ID, ActorID: actorID, Action: model.ArticleReviewSubmit}).Error; err != nil {
		t.Fatalf("create source review: %v", err)
	}
}

func assertArticleDuplicateHasNoHistory(t *testing.T, article mysql.ArticleModel) {
	t.Helper()
	checks := []struct {
		name  string
		model any
		where string
		value any
	}{
		{name: "revision", model: &model.ArticleRevision{}, where: "article_id = ?", value: article.ID},
		{name: "comment", model: &model.Comment{}, where: "article_uid = ?", value: article.UID},
		{name: "submission", model: &model.ArticleSubmission{}, where: "article_id = ?", value: article.ID},
		{name: "review", model: &model.ArticleReview{}, where: "article_id = ?", value: article.ID},
		{name: "daily stat", model: &model.ArticleDailyStat{}, where: "article_uid = ?", value: article.UID},
		{name: "slug redirect", model: &model.ArticleSlugRedirect{}, where: "article_uid = ?", value: article.UID},
	}
	for _, check := range checks {
		var count int64
		if err := connect.Database.Model(check.model).Where(check.where, check.value).Count(&count).Error; err != nil {
			t.Fatalf("count duplicate %s rows: %v", check.name, err)
		}
		if count != 0 {
			t.Errorf("duplicate has %d %s rows, want none", count, check.name)
		}
	}
	if article.Detail == nil || article.Detail.Content == "" {
		t.Errorf("duplicate detail is missing: %+v", article.Detail)
	}
	if article.Tags == nil {
		t.Errorf("duplicate tags association was not loaded")
	}
}
