package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	apierrors "myecho/handler/api/errors"
	"myecho/model"
	"myecho/utils"
)

func TestIsArticlePubliclyVisible_BitsUT(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		status   int8
		postTime time.Time
		want     bool
	}{
		{name: "公开文章立即可见", status: int8(mysql.ARTILCE_STATUS_PUBLIC), postTime: now, want: true},
		{name: "置顶文章立即可见", status: int8(mysql.ARTICLE_STATUS_TOP), postTime: now, want: true},
		{name: "未来发布的公开文章不可见", status: int8(mysql.ARTILCE_STATUS_PUBLIC), postTime: now.Add(time.Hour), want: false},
		{name: "草稿即使到期也不可见", status: int8(mysql.ARTICLE_STATUS_DRAFT), postTime: now.Add(-time.Hour), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsArticlePubliclyVisible(tt.status, tt.postTime); got != tt.want {
				t.Fatalf("IsArticlePubliclyVisible(%d, %v) = %v, want %v", tt.status, tt.postTime, got, tt.want)
			}
		})
	}
}

func TestIsSensitiveSettingKey_BitsUT(t *testing.T) {
	tests := map[string]bool{
		"SiteTitle":                  false,
		"smtp_password":              true,
		"API-Key":                    true,
		"oauth.client.secret":        true,
		"CommentNotificationWebhook": true,
		ArticlePasswordSecretKey:     true,
		themePreviewSecretKey:        true,
		"public_key":                 false,
	}
	for key, want := range tests {
		t.Run(key, func(t *testing.T) {
			if got := IsSensitiveSettingKey(key); got != want {
				t.Fatalf("IsSensitiveSettingKey(%q) = %v, want %v", key, got, want)
			}
		})
	}
}

func TestExportLimitWriterWrite_BitsUT(t *testing.T) {
	var output bytes.Buffer
	remaining := int64(4)
	writer := &exportLimitWriter{Writer: &output, Remaining: &remaining}

	n, err := writer.Write([]byte("abc"))
	if err != nil || n != 3 || remaining != 1 || output.String() != "abc" {
		t.Fatalf("Write(within limit) n=%d err=%v remaining=%d output=%q", n, err, remaining, output.String())
	}
	n, err = writer.Write([]byte("de"))
	if !errors.Is(err, ErrExportTooLarge) || n != 0 {
		t.Fatalf("Write(over limit) n=%d err=%v, want 0 and %v", n, err, ErrExportTooLarge)
	}
	if remaining != 1 || output.String() != "abc" {
		t.Fatalf("Write(over limit) mutated state: remaining=%d output=%q", remaining, output.String())
	}
}

func TestLoadExportDataIncludesArticleOperationsAndFiltersSensitive_BitsUT(t *testing.T) {
	setupServiceTestDB(t)
	user := model.User{
		Name:     "admin",
		Email:    "admin@example.com",
		Password: "password-hash",
		Token:    "user-token",
	}
	if err := connect.Database.Select("*").Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	article := model.Article{
		UID:            "article",
		AuthorID:       user.ID,
		Title:          "Article",
		Slug:           "article",
		SEOTitle:       "SEO title",
		SEODescription: "SEO description",
		ShareImage:     "/share.png",
		Type:           model.ArticleTypePost,
		Password:       "article-password-hash",
	}
	if err := connect.Database.Create(&article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}
	if err := connect.Database.Create(&model.ArticleRevision{ArticleID: article.ID, Title: "Old"}).Error; err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if err := connect.Database.Create(&model.ArticleSlugRedirect{ArticleUID: article.UID, Slug: "old", Type: article.Type}).Error; err != nil {
		t.Fatalf("create redirect: %v", err)
	}
	if err := connect.Database.Create(&model.ArticleDailyStat{ArticleUID: article.UID, Date: "2026-07-27", Views: 7}).Error; err != nil {
		t.Fatalf("create daily stat: %v", err)
	}
	if err := connect.Database.Create(&model.Comment{
		ArticleUID:  article.UID,
		AuthorName:  "Alice",
		AuthorEmail: "alice@example.com",
		AuthorIP:    "203.0.113.10",
		AuthorAgent: "browser",
		Content:     "comment",
	}).Error; err != nil {
		t.Fatalf("create comment: %v", err)
	}
	public := true
	private := false
	settings := []model.Setting{
		{Key: "SiteTitle", Value: "Myecho", IsPublic: &public},
		{Key: "CommentNotificationWebhook", Value: "https://203.0.113.1/hook", IsPublic: &private},
		{Key: "api_token", Value: "secret", IsPublic: &private},
	}
	if err := connect.Database.Create(&settings).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}

	data, err := loadExportData()
	if err != nil {
		t.Fatalf("loadExportData() error = %v", err)
	}
	if len(data.Articles) != 1 || data.Articles[0].SEOTitle != article.SEOTitle ||
		data.Articles[0].SEODescription != article.SEODescription || data.Articles[0].ShareImage != article.ShareImage {
		t.Fatalf("exported article = %+v", data.Articles)
	}
	if len(data.Revisions) != 1 || len(data.SlugRedirects) != 1 || len(data.DailyStats) != 1 {
		t.Fatalf("exported article operations revisions=%d redirects=%d stats=%d", len(data.Revisions), len(data.SlugRedirects), len(data.DailyStats))
	}
	if len(data.Settings) != 1 || data.Settings[0].Key != "SiteTitle" {
		t.Fatalf("exported settings = %+v", data.Settings)
	}
	if data.IncludesSensitive || data.Users[0].Email != "" || data.Users[0].Password != "" || data.Users[0].Token != "" ||
		data.Articles[0].Password != "" || data.Comments[0].AuthorEmail != "" || data.Comments[0].AuthorIP != "" {
		t.Fatalf("download export leaked sensitive data: %+v", data)
	}

	fullData, err := loadExportDataWithSensitive(true)
	if err != nil {
		t.Fatalf("loadExportDataWithSensitive(true) error = %v", err)
	}
	if !fullData.IncludesSensitive ||
		fullData.Users[0].Email != user.Email || fullData.Users[0].Password != user.Password || fullData.Users[0].Token != user.Token ||
		fullData.Articles[0].Password != article.Password ||
		fullData.Comments[0].AuthorEmail != "alice@example.com" || fullData.Comments[0].AuthorIP != "203.0.113.10" ||
		len(fullData.Settings) != 3 {
		t.Fatalf("full restore-point export = %+v", fullData)
	}
}

func TestNormalizeExportRelationsProducesRestorableData_BitsUT(t *testing.T) {
	data := exportData{
		Version:    1,
		ExportedAt: time.Now(),
		Users:      []exportUser{{BaseModel: model.BaseModel{ID: 1}, Name: "admin"}},
		Categories: []model.Category{
			{BaseModel: model.BaseModel{ID: 1}, UID: "first", FatherUID: "second", Type: model.CategoryTypeArticle},
			{BaseModel: model.BaseModel{ID: 2}, UID: "second", FatherUID: "first", Type: model.CategoryTypeArticle},
		},
		ArticleDetails: []model.ArticleDetail{
			{ID: 1, UID: "detail"},
			{ID: 2, UID: "orphan-detail"},
		},
		Articles: []exportArticle{{
			BaseModel:     model.BaseModel{ID: 1},
			UID:           "article",
			AuthorID:      99,
			Slug:          "article",
			Type:          model.ArticleTypePost,
			ContentFormat: model.ArticleContentFormatMarkdown,
			DetailUID:     "detail",
			Status:        int8(mysql.ARTILCE_STATUS_PUBLIC),
		}},
		Revisions:   []model.ArticleRevision{{ID: 1, ArticleID: 1}, {ID: 2, ArticleID: 99}},
		Comments:    []exportComment{{BaseModel: model.BaseModel{ID: 1}, ArticleUID: "article", ParentID: 99, UserID: 99}, {BaseModel: model.BaseModel{ID: 2}, ArticleUID: "missing"}},
		Links:       []model.Link{{BaseModel: model.BaseModel{ID: 1}, CategoryUID: "missing"}},
		Tags:        []model.Tag{{BaseModel: model.BaseModel{ID: 1}, UID: "tag"}},
		ArticleTags: []exportArticleTag{{ArticleUID: "article", TagUID: "tag"}, {ArticleUID: "missing", TagUID: "tag"}},
	}

	normalizeExportRelations(&data)

	if len(data.ArticleDetails) != 1 || len(data.Revisions) != 1 || len(data.Comments) != 1 || len(data.ArticleTags) != 1 {
		t.Fatalf("normalized relation counts = details:%d revisions:%d comments:%d article_tags:%d",
			len(data.ArticleDetails), len(data.Revisions), len(data.Comments), len(data.ArticleTags))
	}
	if data.Articles[0].AuthorID != 0 || data.Comments[0].ParentID != 0 || data.Comments[0].UserID != 0 ||
		data.Links[0].CategoryUID != "" {
		t.Fatalf("normalized optional relations = article:%+v comment:%+v link:%+v", data.Articles[0], data.Comments[0], data.Links[0])
	}
	if err := validateRestoreData(data); err != nil {
		t.Fatalf("validateRestoreData(normalized export) error = %v", err)
	}
}

func TestWebhookAndSettingPrivacy_BitsUT(t *testing.T) {
	setupServiceTestDB(t)
	if err := validateSettingValue("CommentNotificationWebhook", "http://127.0.0.1/hook"); err == nil {
		t.Fatal("validateSettingValue() accepted a loopback webhook")
	}
	private := false
	webhook := &mysql.SettingModel{
		Key:      "CommentNotificationWebhook",
		Value:    "https://203.0.113.1/hook",
		IsPublic: &private,
	}
	svc := &SettingService{}
	if err := svc.Create(webhook); err != nil {
		t.Fatalf("Create(webhook) error = %v", err)
	}
	if IsSettingPublic(webhook) {
		t.Fatal("webhook setting should never be public")
	}
	public := true
	if _, err := svc.UpdateValueDescAndVisibility(webhook.Key, webhook.Value, "", &public); !errors.Is(err, apierrors.ErrSettingKey) {
		t.Fatalf("UpdateValueDescAndVisibility(public webhook) error = %v, want ErrSettingKey", err)
	}

	var payload map[string]interface{}
	oldClient := utils.RemoteFileHTTPClient
	utils.RemoteFileHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Status:     "204 No Content",
			Body:       http.NoBody,
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	t.Cleanup(func() { utils.RemoteFileHTTPClient = oldClient })

	article := &model.Article{BaseModel: model.BaseModel{ID: 4}, Title: "Article"}
	comment := &model.Comment{BaseModel: model.BaseModel{ID: 9}, AuthorName: "Alice", AuthorEmail: "alice@example.com", Content: "hello"}
	if err := NotifyPendingComment(article, comment); err != nil {
		t.Fatalf("NotifyPendingComment() error = %v", err)
	}
	if payload["event"] != "comment.pending" || payload["article_id"] != float64(article.ID) || payload["comment_id"] != float64(comment.ID) {
		t.Fatalf("webhook payload = %+v", payload)
	}
	if _, leaked := payload["author_email"]; leaked {
		t.Fatalf("webhook payload leaked author email: %+v", payload)
	}
}
