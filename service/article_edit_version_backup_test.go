package service

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
)

func TestLoadExportDataPreservesArticleEditVersion(t *testing.T) {
	setupServiceTestDB(t)
	article := model.Article{
		UID:           "edit-version-export",
		Title:         "Article",
		Slug:          "article",
		Type:          model.ArticleTypePost,
		ContentFormat: model.ArticleContentFormatMarkdown,
		Status:        int8(mysql.ARTILCE_STATUS_PUBLIC),
		EditVersion:   17,
	}
	if err := connect.Database.Create(&article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}

	data, err := loadExportData()
	if err != nil {
		t.Fatalf("loadExportData() error = %v", err)
	}
	if len(data.Articles) != 1 || data.Articles[0].EditVersion != article.EditVersion {
		t.Fatalf("exported articles = %+v, want edit version %d", data.Articles, article.EditVersion)
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal export data: %v", err)
	}
	var wire struct {
		Articles []map[string]json.RawMessage `json:"articles"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("unmarshal export wire data: %v", err)
	}
	var encodedVersion uint64
	if len(wire.Articles) != 1 || json.Unmarshal(wire.Articles[0]["edit_version"], &encodedVersion) != nil || encodedVersion != article.EditVersion {
		t.Fatalf("encoded edit_version = %d, want %d", encodedVersion, article.EditVersion)
	}
}

func TestRestoreArticleEditVersionInvalidatesExistingEditors(t *testing.T) {
	for _, tc := range []struct {
		name           string
		currentID      uint
		currentUID     string
		backupID       uint
		backupUID      string
		currentVersion uint64
		backupVersion  uint64
		wantVersion    uint64
	}{
		{
			name: "same id and uid; current version is newer", currentID: 2, currentUID: "restore-edit-version",
			backupID: 2, backupUID: "restore-edit-version", currentVersion: 9, backupVersion: 4, wantVersion: 10,
		},
		{
			name: "same id and uid; backup version is newer", currentID: 2, currentUID: "restore-edit-version",
			backupID: 2, backupUID: "restore-edit-version", currentVersion: 4, backupVersion: 9, wantVersion: 10,
		},
		{
			name: "same uid with different id", currentID: 10, currentUID: "restore-edit-version",
			backupID: 2, backupUID: "restore-edit-version", currentVersion: 9, backupVersion: 4, wantVersion: 10,
		},
		{
			name: "same id with different uid", currentID: 2, currentUID: "current-article",
			backupID: 2, backupUID: "restored-article", currentVersion: 9, backupVersion: 1, wantVersion: 10,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupServiceTestDB(t)
			current := model.Article{
				BaseModel:     model.BaseModel{ID: tc.currentID},
				UID:           tc.currentUID,
				Title:         "Current article",
				Slug:          "current-article",
				Type:          model.ArticleTypePost,
				ContentFormat: model.ArticleContentFormatMarkdown,
				Status:        int8(mysql.ARTILCE_STATUS_PUBLIC),
				EditVersion:   tc.currentVersion,
			}
			if err := connect.Database.Create(&current).Error; err != nil {
				t.Fatalf("create current article: %v", err)
			}

			data := articleEditVersionRestoreData(tc.backupUID, tc.backupVersion)
			data.Articles[0].ID = tc.backupID
			tx := connect.Database.Begin()
			if err := replaceRestoreData(tx, data, 0); err != nil {
				_ = tx.Rollback().Error
				t.Fatalf("replaceRestoreData() error = %v", err)
			}
			if err := tx.Commit().Error; err != nil {
				t.Fatalf("commit restore: %v", err)
			}

			var restored model.Article
			if err := connect.Database.Where("uid = ?", tc.backupUID).First(&restored).Error; err != nil {
				t.Fatalf("load restored article: %v", err)
			}
			if restored.EditVersion != tc.wantVersion {
				t.Fatalf("restored edit version = %d, want %d", restored.EditVersion, tc.wantVersion)
			}
			if restored.EditVersion <= current.EditVersion {
				t.Fatalf("restored edit version %d did not invalidate current token %d", restored.EditVersion, current.EditVersion)
			}
			if restored.Status != int8(mysql.ARTICLE_STATUS_RECYCLE) {
				t.Fatalf("restored status = %d, want trashed status %d", restored.Status, mysql.ARTICLE_STATUS_RECYCLE)
			}
		})
	}
}

func TestRestoreLegacyArticleBackupDefaultsEditVersion(t *testing.T) {
	setupServiceTestDB(t)
	data := articleEditVersionRestoreData("legacy-edit-version", 0)
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal legacy fixture: %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("unmarshal legacy fixture: %v", err)
	}
	var articles []map[string]json.RawMessage
	if err := json.Unmarshal(wire["articles"], &articles); err != nil {
		t.Fatalf("unmarshal legacy articles: %v", err)
	}
	delete(articles[0], "edit_version")
	wire["articles"], err = json.Marshal(articles)
	if err != nil {
		t.Fatalf("marshal legacy articles: %v", err)
	}
	encoded, err = json.Marshal(wire)
	if err != nil {
		t.Fatalf("marshal legacy backup: %v", err)
	}
	archive := buildRestoreArchive(t, map[string][]byte{"data.json": encoded})
	restoredData, _, err := readRestoreArchive(bytes.NewReader(archive), int64(len(archive)), "")
	if err != nil {
		t.Fatalf("read legacy backup: %v", err)
	}
	if len(restoredData.Articles) != 1 || restoredData.Articles[0].EditVersion != 0 {
		t.Fatalf("legacy article data = %+v, want omitted edit version to decode as zero", restoredData.Articles)
	}

	tx := connect.Database.Begin()
	if err := replaceRestoreData(tx, restoredData, 0); err != nil {
		_ = tx.Rollback().Error
		t.Fatalf("replaceRestoreData() error = %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit legacy restore: %v", err)
	}
	var restored model.Article
	if err := connect.Database.Where("uid = ?", "legacy-edit-version").First(&restored).Error; err != nil {
		t.Fatalf("load legacy article: %v", err)
	}
	if restored.EditVersion != 1 {
		t.Fatalf("legacy article edit version = %d, want 1", restored.EditVersion)
	}
	if restored.Status != int8(mysql.ARTICLE_STATUS_RECYCLE) {
		t.Fatalf("legacy article status = %d, want trashed status %d", restored.Status, mysql.ARTICLE_STATUS_RECYCLE)
	}
}

func articleEditVersionRestoreData(uid string, editVersion uint64) exportData {
	detailUID := uid + "-detail"
	return exportData{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Users: []exportUser{{
			BaseModel:      model.BaseModel{ID: 1},
			Name:           "Admin",
			PermissionType: model.Admin,
		}},
		Articles: []exportArticle{{
			BaseModel:     model.BaseModel{ID: 2},
			UID:           uid,
			AuthorID:      1,
			Title:         "Trashed article",
			Slug:          "trashed-article",
			Type:          model.ArticleTypePost,
			ContentFormat: model.ArticleContentFormatMarkdown,
			DetailUID:     detailUID,
			Status:        int8(mysql.ARTICLE_STATUS_RECYCLE),
			EditVersion:   editVersion,
		}},
		ArticleDetails: []model.ArticleDetail{{
			ID:      3,
			UID:     detailUID,
			Content: "content",
		}},
	}
}
