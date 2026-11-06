package service

import (
	"encoding/json"
	"strings"
	"testing"

	"gorm.io/gorm"
	"myecho/dal/connect"
	"myecho/model"
)

func TestRestoreInvalidatesAgentCredentialsAndCreateResponses(t *testing.T) {
	for _, failRestore := range []bool{false, true} {
		name := "success"
		if failRestore {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			setupServiceTestDB(t)
			owner, article := createArticlePreviewRestoreFixture(t)
			token, raw, err := CreateAgentToken(owner.ID, "Restore test", []string{AgentScopeArticlesRead}, 1)
			if err != nil {
				t.Fatal(err)
			}
			request := model.ArticleCreateRequest{OwnerID: owner.ID, Key: "restore-key", RequestHash: "request-hash", ArticleID: article.ID, Response: []byte(`{"secret":"saved-response"}`)}
			if err := connect.Database.Create(&request).Error; err != nil {
				t.Fatal(err)
			}
			data, err := loadExportData()
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{raw, token.TokenHash, "restore-key", "saved-response"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatal("backup leaked agent credential or retry response")
				}
			}
			if failRestore {
				data.Tags = []model.Tag{{BaseModel: model.BaseModel{ID: 100}, UID: "a"}, {BaseModel: model.BaseModel{ID: 100}, UID: "b"}}
			}
			err = connect.Database.Transaction(func(tx *gorm.DB) error { return replaceRestoreData(tx, data, owner.ID) })
			if (err != nil) != failRestore {
				t.Fatalf("restore error = %v, failure expected=%v", err, failRestore)
			}
			_, _, resolveErr := ResolveAgentToken(raw)
			if (resolveErr == nil) != failRestore {
				t.Fatalf("credential after restore = %v; rollback=%v", resolveErr, failRestore)
			}
			var count int64
			if err := connect.Database.Model(&model.ArticleCreateRequest{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if (count == 1) != failRestore {
				t.Fatalf("retry responses after restore = %d; rollback=%v", count, failRestore)
			}
		})
	}
}
