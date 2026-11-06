package mysql

import (
	"testing"

	"myecho/dal/connect"
	"myecho/model"
)

func TestArticleDuplicateDALFindsTagsAndLatestOwnEditableSnapshot(t *testing.T) {
	setupMysqlRepoTestDB(t)
	repo := &ArticleDBRepo{}

	tags := []*model.Tag{
		{Name: "first", UID: "duplicate-tag-first"},
		{Name: "second", UID: "duplicate-tag-second"},
		{Name: "other", UID: "duplicate-tag-other"},
	}
	if err := connect.Database.Create(&tags).Error; err != nil {
		t.Fatalf("create tags: %v", err)
	}
	foundTags, err := repo.FindTagsByUIDs([]string{tags[0].UID, tags[2].UID})
	if err != nil {
		t.Fatalf("FindTagsByUIDs() error = %v", err)
	}
	if len(foundTags) != 2 {
		t.Fatalf("FindTagsByUIDs() len = %d, want 2", len(foundTags))
	}
	foundUIDs := map[string]bool{}
	for _, tag := range foundTags {
		foundUIDs[tag.UID] = true
	}
	if !foundUIDs[tags[0].UID] || !foundUIDs[tags[2].UID] || foundUIDs[tags[1].UID] {
		t.Fatalf("FindTagsByUIDs() UIDs = %v", foundUIDs)
	}
	emptyTags, err := repo.FindTagsByUIDs(nil)
	if err != nil || len(emptyTags) != 0 {
		t.Fatalf("FindTagsByUIDs(nil) = %v, %v; want empty result", emptyTags, err)
	}

	owner := &model.User{Name: "duplicate-submitter", Email: "duplicate-submitter@example.com"}
	other := &model.User{Name: "other-submitter", Email: "other-submitter@example.com"}
	if err := connect.Database.Create(owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := connect.Database.Create(other).Error; err != nil {
		t.Fatalf("create other submitter: %v", err)
	}
	article := &ArticleModel{AuthorID: owner.ID, Title: "source", Status: int8(ARTICLE_STATUS_DRAFT)}
	if err := repo.Create(article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	submissions := []*model.ArticleSubmission{
		{ArticleID: article.ID, SubmitterID: owner.ID, Status: model.ArticleSubmissionDraft, Snapshot: `{"title":"older draft"}`},
		{ArticleID: article.ID, SubmitterID: owner.ID, Status: model.ArticleSubmissionRejected, Snapshot: `{"title":"latest rejected"}`},
		{ArticleID: article.ID, SubmitterID: owner.ID, Status: model.ArticleSubmissionPending, Snapshot: `{"title":"newer pending"}`},
		{ArticleID: article.ID, SubmitterID: other.ID, Status: model.ArticleSubmissionRejected, Snapshot: `{"title":"other owner"}`},
	}
	for _, submission := range submissions {
		if err := connect.Database.Create(submission).Error; err != nil {
			t.Fatalf("create submission: %v", err)
		}
	}
	latest, err := repo.FindLatestDraftOrRejectedSubmission(article.ID, owner.ID)
	if err != nil {
		t.Fatalf("FindLatestDraftOrRejectedSubmission() error = %v", err)
	}
	if latest == nil || latest.ID != submissions[1].ID || latest.Status != model.ArticleSubmissionRejected {
		t.Fatalf("latest own editable submission = %+v, want latest rejected id %d", latest, submissions[1].ID)
	}
	missing, err := repo.FindLatestDraftOrRejectedSubmission(article.ID, 999)
	if err != nil || missing != nil {
		t.Fatalf("missing submitter result = %+v, %v; want nil", missing, err)
	}
}
