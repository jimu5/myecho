package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"myecho/dal"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/utils"
)

func TestCreateArticlePreviewUsesOwnSubmissionAndStoresSafeSnapshot(t *testing.T) {
	for _, status := range []model.ArticleSubmissionStatus{
		model.ArticleSubmissionDraft,
		model.ArticleSubmissionRejected,
		model.ArticleSubmissionPending,
	} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			setupServiceTestDB(t)
			owner, article, category, tag := createPreviewTestFixture(t, model.Author, mysql.ARTICLE_STATUS_DRAFT)
			other := createPreviewTestUser(t, "preview-other", model.Author)
			if status == model.ArticleSubmissionPending {
				createPreviewTestSubmission(t, article.ID, owner.ID, model.ArticleSubmissionDraft, previewSnapshot("older draft", category.UID, tag.UID))
			}
			privateSnapshot := previewSnapshot("owner private "+fmt.Sprint(status), category.UID, tag.UID)
			createPreviewTestSubmission(t, article.ID, owner.ID, status, privateSnapshot)
			createPreviewTestSubmission(t, article.ID, other.ID, model.ArticleSubmissionPending, previewSnapshot("other person's private snapshot", category.UID, tag.UID))

			createdAt := time.Now()
			link, err := CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
			finishedAt := time.Now()
			if err != nil {
				t.Fatalf("CreateArticlePreview() error = %v", err)
			}
			rawToken, err := base64.RawURLEncoding.DecodeString(link.Token)
			if err != nil || len(rawToken) != 32 {
				t.Fatalf("preview token = %q, want a 32-byte base64url bearer token: %v", link.Token, err)
			}
			if link.ExpiresAt.Before(createdAt.Add(ArticlePreviewTTL-time.Second)) || link.ExpiresAt.After(finishedAt.Add(ArticlePreviewTTL+time.Second)) {
				t.Fatalf("expires_at = %s, want roughly one hour after creation", link.ExpiresAt)
			}

			page, err := LoadArticlePreview(link.Token)
			if err != nil {
				t.Fatalf("LoadArticlePreview() error = %v", err)
			}
			if page.Snapshot.Title != privateSnapshot.Title || page.Snapshot.Content != privateSnapshot.Content || page.Snapshot.Summary != privateSnapshot.Summary || page.Snapshot.SEOTitle != privateSnapshot.SEOTitle || page.Snapshot.SEODescription != privateSnapshot.SEODescription || page.Snapshot.ShareImage != privateSnapshot.ShareImage {
				t.Fatalf("preview snapshot = %+v, want owner's status %d snapshot", page.Snapshot, status)
			}
			if page.Snapshot.CategoryName != category.Name || len(page.Snapshot.Tags) != 1 || page.Snapshot.Tags[0].UID != tag.UID || page.Snapshot.Tags[0].Name != tag.Name {
				t.Fatalf("preview taxonomy = category %q tags %+v", page.Snapshot.CategoryName, page.Snapshot.Tags)
			}
			if page.Snapshot.EditVersion != article.EditVersion || page.Snapshot.UID != article.UID || page.Snapshot.AuthorName != owner.Name || page.Snapshot.AuthorNickName != owner.NickName {
				t.Fatalf("preview identity/editor metadata = %+v", page.Snapshot)
			}
			if !page.ExpiresAt.Equal(link.ExpiresAt) {
				t.Fatalf("loaded expiry = %s, want %s", page.ExpiresAt, link.ExpiresAt)
			}

			var stored model.ArticlePreview
			if err := connect.Database.Where("article_id = ? AND creator_id = ?", article.ID, owner.ID).First(&stored).Error; err != nil {
				t.Fatalf("load stored grant: %v", err)
			}
			digest := sha256.Sum256([]byte(link.Token))
			if stored.TokenHash != hex.EncodeToString(digest[:]) || stored.TokenHash == link.Token {
				t.Fatalf("stored token hash = %q, want SHA-256 of the returned token", stored.TokenHash)
			}
			serialized, err := json.Marshal(page.Snapshot)
			if err != nil {
				t.Fatalf("marshal preview snapshot: %v", err)
			}
			for _, secret := range []string{
				link.Token, stored.TokenHash, owner.Email, owner.Token, owner.Password,
				article.Password, privateSnapshot.PasswordHash,
			} {
				if secret != "" && strings.Contains(string(serialized), secret) {
					t.Errorf("snapshot contains credential/private value %q", secret)
				}
				if secret != "" && strings.Contains(stored.Snapshot, secret) {
					t.Errorf("stored snapshot contains credential/private value %q", secret)
				}
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(stored.Snapshot), &fields); err != nil {
				t.Fatalf("decode stored snapshot: %v", err)
			}
			for _, forbidden := range []string{"password", "password_hash", "email", "token", "token_hash"} {
				if _, exists := fields[forbidden]; exists {
					t.Errorf("stored snapshot includes forbidden field %q", forbidden)
				}
			}
		})
	}
}

func TestArticlePreviewUsesCanonicalForOtherCreatorAndIsImmutable(t *testing.T) {
	setupServiceTestDB(t)
	owner, article, _, _ := createPreviewTestFixture(t, model.Author, mysql.ARTILCE_STATUS_PUBLIC)
	creator := createPreviewTestUser(t, "preview-editor", model.Editor)
	privateSnapshot := previewSnapshot("owner-only pending", "", "")
	createPreviewTestSubmission(t, article.ID, owner.ID, model.ArticleSubmissionPending, privateSnapshot)

	link, err := CreateArticlePreview(article.ID, creator.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("CreateArticlePreview(editor) error = %v", err)
	}
	before, err := LoadArticlePreview(link.Token)
	if err != nil {
		t.Fatalf("LoadArticlePreview() before autosave error = %v", err)
	}
	if before.Snapshot.Title != article.Title || before.Snapshot.Content != "canonical preview body" || before.Snapshot.Title == privateSnapshot.Title {
		t.Fatalf("editor preview leaked another submitter's snapshot: %+v", before.Snapshot)
	}

	if err := connect.Database.Model(&mysql.ArticleModel{}).Where("id = ?", article.ID).Updates(map[string]any{
		"title": "autosaved title", "summary": "autosaved summary", "edit_version": article.EditVersion + 1,
	}).Error; err != nil {
		t.Fatalf("simulate article autosave: %v", err)
	}
	if err := connect.Database.Model(&model.ArticleDetail{}).Where("uid = ?", article.DetailUID).Update("content", "autosaved body").Error; err != nil {
		t.Fatalf("simulate detail autosave: %v", err)
	}
	createPreviewTestSubmission(t, article.ID, creator.ID, model.ArticleSubmissionDraft, previewSnapshot("new editor draft", "", ""))

	after, err := LoadArticlePreview(link.Token)
	if err != nil {
		t.Fatalf("LoadArticlePreview() after autosave error = %v", err)
	}
	if after.Snapshot.Title != before.Snapshot.Title || after.Snapshot.Content != before.Snapshot.Content || after.Snapshot.Summary != before.Snapshot.Summary {
		t.Fatalf("preview changed after autosave: before=%+v after=%+v", before.Snapshot, after.Snapshot)
	}
}

func TestArticlePreviewCASAndTrashGuards(t *testing.T) {
	setupServiceTestDB(t)
	owner, article, _, _ := createPreviewTestFixture(t, model.Author, mysql.ARTICLE_STATUS_DRAFT)
	if _, err := CreateArticlePreview(article.ID, owner.ID, article.EditVersion+1); !errors.Is(err, mysql.ErrArticleEditConflict) {
		t.Fatalf("CreateArticlePreview(stale version) error = %v, want edit conflict", err)
	}
	var grants int64
	if err := connect.Database.Model(&model.ArticlePreview{}).Where("article_id = ?", article.ID).Count(&grants).Error; err != nil {
		t.Fatalf("count grants after stale version: %v", err)
	}
	if grants != 0 {
		t.Fatalf("stale version created %d grants, want none", grants)
	}

	link, err := CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("CreateArticlePreview() before trash error = %v", err)
	}
	if err := connect.Database.Model(&mysql.ArticleModel{}).Where("id = ?", article.ID).
		UpdateColumn("status", int8(mysql.ARTICLE_STATUS_RECYCLE)).Error; err != nil {
		t.Fatalf("trash article: %v", err)
	}
	if _, err := LoadArticlePreview(link.Token); !errors.Is(err, ErrArticlePreviewUnavailable) {
		t.Fatalf("LoadArticlePreview(trashed) error = %v, want unavailable", err)
	}
	if _, err := CreateArticlePreview(article.ID, owner.ID, article.EditVersion); !errors.Is(err, mysql.ErrArticleTrashed) {
		t.Fatalf("CreateArticlePreview(trashed) error = %v, want ErrArticleTrashed", err)
	}
}

func TestArticlePreviewRevokeExpiresOnlyScopedGrants(t *testing.T) {
	setupServiceTestDB(t)
	owner, article, _, _ := createPreviewTestFixture(t, model.Author, mysql.ARTICLE_STATUS_DRAFT)
	editor := createPreviewTestUser(t, "preview-revoke-editor", model.Editor)
	secondArticle := createPreviewTestArticle(t, owner, mysql.ARTICLE_STATUS_DRAFT)

	createdAt := time.Now()
	ownerLinkOne, err := CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("create first owner preview: %v", err)
	}
	ownerLinkTwo, err := CreateArticlePreview(article.ID, owner.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("create second owner preview: %v", err)
	}
	otherCreatorLink, err := CreateArticlePreview(article.ID, editor.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("create editor preview: %v", err)
	}
	otherArticleLink, err := CreateArticlePreview(secondArticle.ID, owner.ID, secondArticle.EditVersion)
	if err != nil {
		t.Fatalf("create other article preview: %v", err)
	}
	finishedAt := time.Now()
	for _, link := range []ArticlePreviewLink{ownerLinkOne, ownerLinkTwo, otherCreatorLink, otherArticleLink} {
		if link.ExpiresAt.Before(createdAt.Add(ArticlePreviewTTL-time.Second)) || link.ExpiresAt.After(finishedAt.Add(ArticlePreviewTTL+time.Second)) {
			t.Fatalf("expires_at = %s, want roughly one hour after creation", link.ExpiresAt)
		}
	}

	if err := RevokeArticlePreviews(article.ID, owner.ID); err != nil {
		t.Fatalf("RevokeArticlePreviews() error = %v", err)
	}
	for _, token := range []string{ownerLinkOne.Token, ownerLinkTwo.Token} {
		if _, err := LoadArticlePreview(token); !errors.Is(err, ErrArticlePreviewUnavailable) {
			t.Errorf("revoked owner preview load error = %v, want unavailable", err)
		}
	}
	if _, err := LoadArticlePreview(otherCreatorLink.Token); err != nil {
		t.Fatalf("revoke affected another creator's preview: %v", err)
	}
	if _, err := LoadArticlePreview(otherArticleLink.Token); err != nil {
		t.Fatalf("revoke affected another article's preview: %v", err)
	}

	var stored model.ArticlePreview
	if err := connect.Database.Where("article_id = ? AND creator_id = ?", article.ID, editor.ID).First(&stored).Error; err != nil {
		t.Fatalf("load editor grant: %v", err)
	}
	if err := connect.Database.Model(&model.ArticlePreview{}).Where("id = ?", stored.ID).
		Update("expires_at", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatalf("expire editor grant: %v", err)
	}
	if _, err := LoadArticlePreview(otherCreatorLink.Token); !errors.Is(err, ErrArticlePreviewUnavailable) {
		t.Fatalf("expired preview load error = %v, want unavailable", err)
	}
}

func TestArticlePreviewRevalidatesCreatorAccessAndDoesNotMutateSource(t *testing.T) {
	setupServiceTestDB(t)
	owner, article, category, _ := createPreviewTestFixture(t, model.Author, mysql.ARTILCE_STATUS_PUBLIC)
	disabledEditor := createPreviewTestUser(t, "preview-disabled-editor", model.Editor)
	downgradedEditor := createPreviewTestUser(t, "preview-downgraded-editor", model.Editor)

	if err := connect.Database.Create(&model.ArticleRevision{ArticleID: article.ID, ActorID: owner.ID, Title: "existing", Content: "existing revision"}).Error; err != nil {
		t.Fatalf("create existing revision: %v", err)
	}
	if err := connect.Database.Create(&model.Comment{ArticleUID: article.UID, AuthorName: "reader", Content: "existing comment"}).Error; err != nil {
		t.Fatalf("create existing comment: %v", err)
	}
	var revisionCountBefore, commentCountBefore int64
	if err := connect.Database.Model(&model.ArticleRevision{}).Where("article_id = ?", article.ID).Count(&revisionCountBefore).Error; err != nil {
		t.Fatalf("count revisions before: %v", err)
	}
	if err := connect.Database.Model(&model.Comment{}).Where("article_uid = ?", article.UID).Count(&commentCountBefore).Error; err != nil {
		t.Fatalf("count comments before: %v", err)
	}
	var categoryBefore mysql.CategoryModel
	if err := connect.Database.Where("uid = ?", category.UID).First(&categoryBefore).Error; err != nil {
		t.Fatalf("load category before: %v", err)
	}
	before, err := dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		t.Fatalf("load source before preview: %v", err)
	}

	disabledLink, err := CreateArticlePreview(article.ID, disabledEditor.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("create disabled-editor preview: %v", err)
	}
	downgradedLink, err := CreateArticlePreview(article.ID, downgradedEditor.ID, article.EditVersion)
	if err != nil {
		t.Fatalf("create downgrade-editor preview: %v", err)
	}
	if err := connect.Database.Model(&model.User{}).Where("id = ?", disabledEditor.ID).Update("disabled", true).Error; err != nil {
		t.Fatalf("disable creator: %v", err)
	}
	if _, err := LoadArticlePreview(disabledLink.Token); !errors.Is(err, ErrArticlePreviewUnavailable) {
		t.Errorf("disabled creator preview load error = %v, want unavailable", err)
	}
	if err := connect.Database.Model(&model.User{}).Where("id = ?", downgradedEditor.ID).Update("permission_type", model.Author).Error; err != nil {
		t.Fatalf("downgrade creator permission: %v", err)
	}
	if _, err := LoadArticlePreview(downgradedLink.Token); !errors.Is(err, ErrArticlePreviewUnavailable) {
		t.Errorf("downgraded creator preview load error = %v, want unavailable", err)
	}

	after, err := dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		t.Fatalf("load source after preview: %v", err)
	}
	if after.EditVersion != before.EditVersion || after.ReadCount != before.ReadCount || after.LikeCount != before.LikeCount || after.CommentCount != before.CommentCount || after.UpdatedAt != before.UpdatedAt || after.Title != before.Title || after.Detail == nil || before.Detail == nil || after.Detail.Content != before.Detail.Content {
		t.Fatalf("preview creation mutated source: before=%+v after=%+v", before, after)
	}
	var revisionCountAfter, commentCountAfter int64
	if err := connect.Database.Model(&model.ArticleRevision{}).Where("article_id = ?", article.ID).Count(&revisionCountAfter).Error; err != nil {
		t.Fatalf("count revisions after: %v", err)
	}
	if err := connect.Database.Model(&model.Comment{}).Where("article_uid = ?", article.UID).Count(&commentCountAfter).Error; err != nil {
		t.Fatalf("count comments after: %v", err)
	}
	if revisionCountAfter != revisionCountBefore || commentCountAfter != commentCountBefore {
		t.Fatalf("preview creation changed history counts: revisions %d->%d comments %d->%d", revisionCountBefore, revisionCountAfter, commentCountBefore, commentCountAfter)
	}
	var categoryAfter mysql.CategoryModel
	if err := connect.Database.Where("uid = ?", category.UID).First(&categoryAfter).Error; err != nil {
		t.Fatalf("load category after: %v", err)
	}
	if categoryAfter.Count != categoryBefore.Count {
		t.Fatalf("preview creation changed category count %d->%d", categoryBefore.Count, categoryAfter.Count)
	}
}

func createPreviewTestFixture(t *testing.T, permission int8, status mysql.ArticleStatus) (*model.User, *mysql.ArticleModel, *model.Category, *model.Tag) {
	t.Helper()
	owner := createPreviewTestUser(t, "preview-owner", permission)
	category := &model.Category{Name: "Preview category", UID: "preview-category-" + utils.GenUID20(), Type: model.CategoryTypeArticle}
	if err := connect.Database.Create(category).Error; err != nil {
		t.Fatalf("create preview category: %v", err)
	}
	tag := &model.Tag{Name: "Preview tag", UID: "preview-tag-" + utils.GenUID20()}
	if err := connect.Database.Create(tag).Error; err != nil {
		t.Fatalf("create preview tag: %v", err)
	}
	allowComment := false
	article := &mysql.ArticleModel{
		AuthorID: owner.ID, Title: "Canonical preview title", Slug: "canonical-preview-slug",
		SEOTitle: "Canonical SEO title", SEODescription: "Canonical SEO description", ShareImage: "/canonical.jpg",
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		Summary: "canonical summary", CategoryUID: category.UID, IsAllowComment: &allowComment,
		PostTime: time.Now().Add(24 * time.Hour), Status: int8(status), EditVersion: 7,
		ReadCount: 23, LikeCount: 9, CommentCount: 4, Password: "source-password-hash-secret",
		Detail: &model.ArticleDetail{Content: "canonical preview body"}, Tags: []*model.Tag{tag},
	}
	if err := dal.MySqlDB.Article.Create(article); err != nil {
		t.Fatalf("create preview source article: %v", err)
	}
	return owner, article, category, tag
}

func createPreviewTestUser(t *testing.T, name string, permission int8) *model.User {
	t.Helper()
	user := &model.User{
		Name: name, NickName: "Preview Nickname", Email: name + "@private.example",
		Password: "private-user-password-secret", Token: "private-auth-token-secret",
		PermissionType: permission,
	}
	if err := connect.Database.Create(user).Error; err != nil {
		t.Fatalf("create preview user %q: %v", name, err)
	}
	return user
}

func createPreviewTestArticle(t *testing.T, owner *model.User, status mysql.ArticleStatus) *mysql.ArticleModel {
	t.Helper()
	category := &model.Category{Name: "Second preview category", UID: "preview-category-" + utils.GenUID20(), Type: model.CategoryTypeArticle}
	if err := connect.Database.Create(category).Error; err != nil {
		t.Fatalf("create second preview category: %v", err)
	}
	article := &mysql.ArticleModel{
		AuthorID: owner.ID, Title: "Second article", Status: int8(status), CategoryUID: category.UID,
		Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
		EditVersion: 2, Detail: &model.ArticleDetail{Content: "second body"},
	}
	if err := dal.MySqlDB.Article.Create(article); err != nil {
		t.Fatalf("create second preview article: %v", err)
	}
	return article
}

func createPreviewTestSubmission(t *testing.T, articleID, submitterID uint, status model.ArticleSubmissionStatus, snapshot model.ArticleSubmissionSnapshot) {
	t.Helper()
	encoded, err := EncodeArticleSubmissionSnapshot(snapshot)
	if err != nil {
		t.Fatalf("encode preview submission: %v", err)
	}
	if err := connect.Database.Create(&model.ArticleSubmission{
		ArticleID: articleID, SubmitterID: submitterID, Status: status, Snapshot: encoded,
	}).Error; err != nil {
		t.Fatalf("create preview submission: %v", err)
	}
}

func previewSnapshot(title, categoryUID, tagUID string) model.ArticleSubmissionSnapshot {
	allowComment := true
	return model.ArticleSubmissionSnapshot{
		Title: title, Slug: "private-submission-slug", SEOTitle: "private submission SEO title",
		SEODescription: "private submission SEO description", ShareImage: "/private.jpg",
		Type: model.ArticleTypePage, ContentFormat: model.ArticleContentFormatHTML,
		Summary: "private submission summary", Content: "private submission body",
		CategoryUID: categoryUID, IsAllowComment: &allowComment, PostTime: time.Now().Add(48 * time.Hour),
		PasswordHash: "private-submission-password-hash-secret", TagUIDs: []string{tagUID},
	}
}
