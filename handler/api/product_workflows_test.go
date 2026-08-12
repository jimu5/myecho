package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
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
)

func TestAccountProfilePasswordAndLogout_BitsUT(t *testing.T) {
	setupAPITestDB(t)
	password, err := HashPassword("old-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	user := &model.User{
		Name:           "admin",
		NickName:       "Admin",
		Email:          "admin@example.com",
		Password:       password,
		Token:          "existing-token",
		PermissionType: int8(model.Admin),
	}
	if err := connect.Database.Select("*").Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", user)
		return c.Next()
	})
	app.Get("/profile", Profile)
	app.Patch("/profile", ProfileUpdate)
	app.Patch("/password", PasswordUpdate)
	app.Post("/logout", Logout)

	resp := doJSONRequest(t, app, fiber.MethodGet, "/profile", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Profile() status = %d, want 200", resp.StatusCode)
	}
	profileBody := readResponseBody(t, resp)
	if strings.Contains(profileBody, "old-password") || strings.Contains(profileBody, password) {
		t.Fatalf("Profile() leaked credentials: %s", profileBody)
	}
	if !strings.Contains(profileBody, "existing-token") {
		t.Fatalf("Profile() omitted the active token required by the admin client: %s", profileBody)
	}

	resp = doJSONRequest(t, app, fiber.MethodPatch, "/profile", "", `{"nick_name":" Updated Admin ","email":"updated@example.com"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("ProfileUpdate() status = %d, want 200", resp.StatusCode)
	}
	if user.NickName != "Updated Admin" || user.Email != "updated@example.com" {
		t.Fatalf("updated user = %+v", user)
	}

	resp = doJSONRequest(t, app, fiber.MethodPatch, "/password", "", `{"old_password":"wrong-password","new_password":"new-password"}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("PasswordUpdate(wrong) status = %d, want 403", resp.StatusCode)
	}
	oldToken := user.Token
	resp = doJSONRequest(t, app, fiber.MethodPatch, "/password", "", `{"old_password":"old-password","new_password":"new-password"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("PasswordUpdate() status = %d, want 200", resp.StatusCode)
	}
	if ok, _ := CheckPassword(user.Password, "new-password"); !ok {
		t.Fatal("PasswordUpdate() did not persist the new password")
	}
	if user.Token == oldToken {
		t.Fatal("PasswordUpdate() did not revoke the prior token")
	}

	passwordToken := user.Token
	resp = doJSONRequest(t, app, fiber.MethodPost, "/logout", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("Logout() status = %d, want 200", resp.StatusCode)
	}
	var stored model.User
	if err := connect.Database.First(&stored, user.ID).Error; err != nil {
		t.Fatalf("load logged-out user: %v", err)
	}
	if stored.Token == passwordToken {
		t.Fatal("Logout() did not revoke the active token")
	}
}

func TestCommentRateLimiterBoundsStoredIPs_BitsUT(t *testing.T) {
	limiter := newCommentRateLimiter(2, time.Minute, 2)
	now := time.Now()
	if !limiter.allow("192.0.2.1", now) || !limiter.allow("192.0.2.2", now) {
		t.Fatal("limiter rejected entries below the capacity")
	}
	if limiter.allow("192.0.2.3", now) {
		t.Fatal("limiter accepted a new IP after reaching capacity")
	}
	if len(limiter.hits) != 2 {
		t.Fatalf("limiter entries = %d, want bounded at 2", len(limiter.hits))
	}
	if !limiter.allow("192.0.2.3", now.Add(2*time.Minute)) {
		t.Fatal("limiter did not evict expired entries")
	}
	if len(limiter.hits) > 2 {
		t.Fatalf("limiter entries after cleanup = %d, want at most 2", len(limiter.hits))
	}
}

func TestCommentAdminFiltersAndReply_BitsUT(t *testing.T) {
	setupAPITestDB(t)
	admin := createAPIUser(t, "admin", "admin-token", int8(model.Admin))
	article := &mysql.ArticleModel{
		Title:  "Filtered article",
		Status: int8(mysql.ARTILCE_STATUS_PUBLIC),
		Type:   model.ArticleTypePost,
	}
	if err := (&mysql.ArticleDBRepo{}).Create(article); err != nil {
		t.Fatalf("create article: %v", err)
	}
	pending := int8(model.CommentStatusPending)
	comments := []model.Comment{
		{
			BaseModel:   model.BaseModel{CreatedAt: time.Now()},
			ArticleUID:  article.UID,
			AuthorName:  "alice",
			AuthorEmail: "alice@example.com",
			Content:     "release note",
			Status:      &pending,
		},
		{
			BaseModel:   model.BaseModel{CreatedAt: time.Now().AddDate(0, 0, -2)},
			ArticleUID:  article.UID,
			AuthorName:  "bob",
			AuthorEmail: "bob@example.com",
			Content:     "older comment",
			Status:      &pending,
		},
	}
	if err := connect.Database.Create(&comments).Error; err != nil {
		t.Fatalf("create comments: %v", err)
	}

	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Get("/comments", CommentAllList)
	app.Post("/comments/:id/reply", func(c *fiber.Ctx) error {
		c.Locals("user", admin)
		return c.Next()
	}, CommentReply)

	today := time.Now().Format("2006-01-02")
	resp := doJSONRequest(t, app, fiber.MethodGet, "/comments?keyword=release&date_from="+today+"&date_to="+today+"&page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("CommentAllList() status = %d, want 200", resp.StatusCode)
	}
	wrapped := decodeAPIResp(t, resp)
	if wrapped.Meta["total"].(float64) != 1 {
		t.Fatalf("filtered comment total = %v, want 1", wrapped.Meta["total"])
	}
	var filtered []rtype.CommentAdminResponse
	if err := json.Unmarshal(wrapped.Data, &filtered); err != nil {
		t.Fatalf("decode filtered comments: %v", err)
	}
	if len(filtered) != 1 || filtered[0].AuthorName != "alice" || filtered[0].ArticleTitle != article.Title {
		t.Fatalf("filtered comments = %+v", filtered)
	}

	resp = doJSONRequest(t, app, fiber.MethodGet, "/comments?date_from=invalid&page=1&page_size=10", "", "")
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("CommentAllList(invalid date) status = %d, want 403", resp.StatusCode)
	}
	resp = doJSONRequest(t, app, fiber.MethodPost, "/comments/"+strconv.Itoa(int(comments[0].ID))+"/reply", "", `{"content":" Admin reply "}`)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("CommentReply() status = %d, want 201", resp.StatusCode)
	}
	var reply model.Comment
	if err := connect.Database.Where("parent_id = ? AND user_id = ?", comments[0].ID, admin.ID).First(&reply).Error; err != nil {
		t.Fatalf("load admin reply: %v", err)
	}
	if reply.Content != "Admin reply" || reply.Status == nil || *reply.Status != int8(model.CommentStatusApproved) {
		t.Fatalf("admin reply = %+v", reply)
	}
}

func TestImportRejectsInvalidArchive_BitsUT(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	app.Post("/import", Import)

	resp := doJSONRequest(t, app, fiber.MethodPost, "/import?dry_run=invalid", "", "")
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("Import(invalid dry_run) status = %d, want 400", resp.StatusCode)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "backup.zip")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("not a zip archive")); err != nil {
		t.Fatalf("write invalid archive: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(fiber.MethodPost, "/import?dry_run=true", body)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("Import() request error = %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("Import(invalid archive) status = %d, want 400", resp.StatusCode)
	}
}

func TestNormalizeLink_BitsUT(t *testing.T) {
	got, err := normalizeHTTPURL(" example.com/path ", true)
	if err != nil || got != "https://example.com/path" {
		t.Fatalf("normalizeHTTPURL() = %q, %v", got, err)
	}
	if _, err := normalizeHTTPURL("ftp://example.com", true); err == nil {
		t.Fatal("normalizeHTTPURL() accepted a non-HTTP scheme")
	}

	link := &mysql.LinkModel{Name: " Example ", URL: "example.com", IconURL: "cdn.example.com/icon.png"}
	if err := normalizeLink(link); err != nil {
		t.Fatalf("normalizeLink() error = %v", err)
	}
	if link.Name != "Example" || link.URL != "https://example.com" || link.IconURL != "https://cdn.example.com/icon.png" {
		t.Fatalf("normalizeLink() = %+v", link)
	}
}
