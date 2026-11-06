package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

func TestSubmissionBackupArchiveRoundTrip(t *testing.T) {
	for _, sensitive := range []bool{false, true} {
		t.Run(fmt.Sprintf("sensitive=%t", sensitive), func(t *testing.T) {
			chdirServiceTestTemp(t)
			setupServiceTestDB(t)
			admin := model.User{Name: "backup-admin", PermissionType: model.Admin}
			if err := connect.Database.Create(&admin).Error; err != nil {
				t.Fatalf("create admin: %v", err)
			}
			if err := connect.Database.Model(&admin).Update("permission_type", model.Admin).Error; err != nil {
				t.Fatalf("set admin role: %v", err)
			}
			if err := connect.Database.Create(&model.Category{UID: "snapshot-category", Name: "Snapshots", Type: model.CategoryTypeArticle}).Error; err != nil {
				t.Fatalf("create snapshot category: %v", err)
			}
			if err := connect.Database.Create(&model.Tag{UID: "snapshot-tag", Name: "Snapshot"}).Error; err != nil {
				t.Fatalf("create snapshot tag: %v", err)
			}
			article := mysql.ArticleModel{
				UID: "backup-submissions", AuthorID: admin.ID, Title: "Live title", Slug: "backup-submissions",
				Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
				Status: int8(mysql.ARTILCE_STATUS_PUBLIC), EditVersion: 7, Detail: &model.ArticleDetail{Content: "Live body"},
			}
			if err := (&mysql.ArticleDBRepo{}).Create(&article); err != nil {
				t.Fatalf("create article: %v", err)
			}
			now := time.Date(2026, time.November, 7, 8, 9, 10, 0, time.UTC)
			allowComment := false
			submissions := make([]model.ArticleSubmission, 0, 3)
			snapshots := make(map[uint]model.ArticleSubmissionSnapshot)
			for _, status := range []model.ArticleSubmissionStatus{
				model.ArticleSubmissionDraft, model.ArticleSubmissionPending, model.ArticleSubmissionApproved,
			} {
				snapshot := model.ArticleSubmissionSnapshot{
					Title: fmt.Sprintf("Snapshot %d", status), Slug: fmt.Sprintf("snapshot-%d", status),
					SEOTitle: "Snapshot SEO", SEODescription: "Snapshot description", ShareImage: "/mos/snapshot.png",
					Type: model.ArticleTypePost, ContentFormat: model.ArticleContentFormatMarkdown,
					Summary: "Snapshot summary", Content: fmt.Sprintf("# Saved body %d\n\nOriginal content", status),
					CategoryUID: "snapshot-category", IsAllowComment: &allowComment, PostTime: now,
					TargetStatus: int8(mysql.ARTILCE_STATUS_PUBLIC), PasswordHash: fmt.Sprintf("private-snapshot-hash-%d", status),
					TagUIDs: []string{"snapshot-tag"},
				}
				encoded, err := EncodeArticleSubmissionSnapshot(snapshot)
				if err != nil {
					t.Fatalf("encode snapshot: %v", err)
				}
				submission := model.ArticleSubmission{
					ArticleID: article.ID, SubmitterID: admin.ID, ReviewerID: admin.ID, Status: status,
					Snapshot: encoded, BaseEditVersion: article.EditVersion,
					Note: fmt.Sprintf("Review note %d", status), SubmittedAt: &now, ReviewedAt: &now,
				}
				if err := connect.Database.Create(&submission).Error; err != nil {
					t.Fatalf("create submission: %v", err)
				}
				apiJSON, err := json.Marshal(submission)
				if err != nil {
					t.Fatalf("marshal API submission: %v", err)
				}
				if bytes.Contains(apiJSON, []byte(`"snapshot"`)) || bytes.Contains(apiJSON, []byte(snapshot.PasswordHash)) {
					t.Fatal("ordinary submission JSON exposes its private snapshot")
				}
				submissions = append(submissions, submission)
				if !sensitive {
					snapshot.PasswordHash = ""
				}
				snapshots[submission.ID] = snapshot
			}

			createArchive := CreateExportArchive
			if sensitive {
				createArchive = func() (string, int64, error) { return createExportArchive(true) }
			}
			archivePath, size, err := createArchive()
			if err != nil {
				t.Fatalf("create archive: %v", err)
			}
			t.Cleanup(func() { _ = os.Remove(archivePath) })
			archive, err := os.Open(archivePath)
			if err != nil {
				t.Fatalf("open archive: %v", err)
			}
			defer archive.Close()
			zr, err := zip.NewReader(archive, size)
			if err != nil {
				t.Fatalf("read archive: %v", err)
			}
			entry, err := zr.Open("data.json")
			if err != nil {
				t.Fatalf("open archive data: %v", err)
			}
			dataJSON, err := io.ReadAll(entry)
			_ = entry.Close()
			if err != nil {
				t.Fatalf("read archive data: %v", err)
			}
			var wire struct {
				Submissions []struct {
					ID              uint   `json:"id"`
					Snapshot        string `json:"snapshot"`
					BaseEditVersion uint64 `json:"base_edit_version"`
				} `json:"article_submissions"`
			}
			if err := json.Unmarshal(dataJSON, &wire); err != nil {
				t.Fatalf("decode archive JSON: %v", err)
			}
			if len(wire.Submissions) != len(submissions) {
				t.Fatalf("archived submissions = %d, want %d", len(wire.Submissions), len(submissions))
			}
			for _, submission := range wire.Submissions {
				if submission.BaseEditVersion != article.EditVersion {
					t.Fatalf("submission %d archived base edit version = %d, want %d", submission.ID, submission.BaseEditVersion, article.EditVersion)
				}
				var snapshot model.ArticleSubmissionSnapshot
				if err := json.Unmarshal([]byte(submission.Snapshot), &snapshot); err != nil {
					t.Fatalf("submission %d archive snapshot: %v", submission.ID, err)
				}
				if !reflect.DeepEqual(snapshot, snapshots[submission.ID]) {
					t.Fatalf("submission %d archive snapshot does not preserve content and metadata", submission.ID)
				}
			}
			if !sensitive && bytes.Contains(dataJSON, []byte("private-snapshot-hash-")) {
				t.Fatal("public export contains a snapshot password hash")
			}
			preview, err := PreviewRestoreArchive(archive, size)
			if err != nil || preview.Counts.ArticleSubmissions != len(submissions) {
				t.Fatalf("preview archive: submissions=%d, error=%v", preview.Counts.ArticleSubmissions, err)
			}

			var invalid map[string]json.RawMessage
			if err := json.Unmarshal(dataJSON, &invalid); err != nil {
				t.Fatalf("decode archive for missing snapshot check: %v", err)
			}
			var invalidSubmissions []map[string]json.RawMessage
			if err := json.Unmarshal(invalid["article_submissions"], &invalidSubmissions); err != nil {
				t.Fatalf("decode submissions for missing snapshot check: %v", err)
			}
			delete(invalidSubmissions[0], "snapshot")
			invalid["article_submissions"], err = json.Marshal(invalidSubmissions)
			if err != nil {
				t.Fatalf("encode submissions without snapshot: %v", err)
			}
			invalidJSON, err := json.Marshal(invalid)
			if err != nil {
				t.Fatalf("encode archive without snapshot: %v", err)
			}
			invalidArchive := buildRestoreArchive(t, map[string][]byte{"data.json": invalidJSON})
			if _, err := PreviewRestoreArchive(bytes.NewReader(invalidArchive), int64(len(invalidArchive))); !errors.Is(err, ErrInvalidBackup) {
				t.Fatalf("preview archive without snapshot: %v, want ErrInvalidBackup", err)
			}

			if err := connect.Database.Model(&model.ArticleSubmission{}).Where("article_id = ?", article.ID).
				Updates(map[string]interface{}{"snapshot": `{}`, "note": "Changed after archive"}).Error; err != nil {
				t.Fatalf("change submissions after archive: %v", err)
			}
			if _, err := RestoreArchive(archive, size, admin.ID); err != nil {
				t.Fatalf("restore archive: %v", err)
			}
			var restoredArticle model.Article
			if err := connect.Database.First(&restoredArticle, article.ID).Error; err != nil {
				t.Fatalf("load restored article: %v", err)
			}
			if restoredArticle.EditVersion <= article.EditVersion {
				t.Fatalf("restored article version = %d, want greater than %d", restoredArticle.EditVersion, article.EditVersion)
			}
			var restored []model.ArticleSubmission
			if err := connect.Database.Order("id").Find(&restored).Error; err != nil {
				t.Fatalf("load restored submissions: %v", err)
			}
			if len(restored) != len(submissions) {
				t.Fatalf("restored submissions = %d, want %d", len(restored), len(submissions))
			}
			for i, submission := range restored {
				original := submissions[i]
				if submission.ID != original.ID || submission.ArticleID != original.ArticleID ||
					submission.SubmitterID != original.SubmitterID || submission.ReviewerID != original.ReviewerID ||
					submission.Status != original.Status || submission.Note != original.Note ||
					!submission.CreatedAt.Equal(original.CreatedAt) || !submission.UpdatedAt.Equal(original.UpdatedAt) ||
					submission.SubmittedAt == nil || !submission.SubmittedAt.Equal(*original.SubmittedAt) ||
					submission.ReviewedAt == nil || !submission.ReviewedAt.Equal(*original.ReviewedAt) {
					t.Fatalf("submission %d did not preserve review metadata", original.ID)
				}
				if submission.BaseEditVersion != original.BaseEditVersion || submission.BaseEditVersion == restoredArticle.EditVersion {
					t.Fatalf("submission %d base edit version = %d, want original %d below restored article version %d",
						submission.ID, submission.BaseEditVersion, original.BaseEditVersion, restoredArticle.EditVersion)
				}
				snapshot, err := DecodeArticleSubmissionSnapshot(&submission)
				if err != nil || !reflect.DeepEqual(snapshot, snapshots[submission.ID]) {
					t.Fatalf("submission %d did not restore its snapshot: %v", submission.ID, err)
				}
			}
		})
	}
}
