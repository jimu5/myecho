package middleware

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"myecho/dal/connect"
	"myecho/model"
	"myecho/service"
)

func setupAgentTokenMiddlewareDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "agent-token.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AgentToken{}); err != nil {
		t.Fatalf("migrate agent token tables: %v", err)
	}
	previousDB := connect.Database
	connect.Database = db
	t.Cleanup(func() {
		connect.Database = previousDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createAgentTokenMiddlewareIdentity(t *testing.T, db *gorm.DB, name string, role int8, scopes []string) (model.User, string) {
	t.Helper()
	user := model.User{Name: name, Email: name + "@example.com", Token: "login-" + name, PermissionType: role}
	if err := db.Select("*").Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Model(&user).Update("permission_type", role).Error; err != nil {
		t.Fatalf("set user role: %v", err)
	}
	_, rawToken, err := service.CreateAgentToken(user.ID, name+" agent", scopes, 90)
	if err != nil {
		t.Fatalf("CreateAgentToken() error = %v", err)
	}
	return user, rawToken
}

func agentRequest(t *testing.T, app *fiber.App, method, path, rawToken, body string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+rawToken)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test(%s %s): %v", method, path, err)
	}
	return resp.StatusCode
}

func TestAgentAuthenticationAllowsOnlyScopedArticleWrites(t *testing.T) {
	db := setupAgentTokenMiddlewareDB(t)
	owner, rawToken := createAgentTokenMiddlewareIdentity(t, db, "editor", model.Editor, []string{service.AgentScopeArticlesWrite})
	_, deleteToken, err := service.CreateAgentToken(owner.ID, "delete agent", []string{service.AgentScopeArticlesDelete}, 90)
	if err != nil {
		t.Fatalf("CreateAgentToken(delete) error = %v", err)
	}
	_, writeDeleteToken, err := service.CreateAgentToken(owner.ID, "write-delete agent", []string{service.AgentScopeArticlesWrite, service.AgentScopeArticlesDelete}, 90)
	if err != nil {
		t.Fatalf("CreateAgentToken(write-delete) error = %v", err)
	}
	app := fiber.New()
	app.Post("/api/articles", Authentication, AuthorRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Patch("/api/articles/:id", Authentication, AuthorRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Post("/api/articles/batch", Authentication, AuthorRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Post("/api/articles/:id/review", Authentication, EditorRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Post("/api/articles/:id/previews", Authentication, AuthorRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Delete("/api/articles/:id/previews", Authentication, AuthorRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Post("/api/articles/:id/revisions/:revision_id/restore", Authentication, AuthorRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Post("/api/articles/:id/restore", Authentication, AuthorRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Post("/api/submissions/:id/withdraw", Authentication, AuthorRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	for _, test := range []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{name: "draft status", method: fiber.MethodPost, path: "/api/articles", body: `{"status":4}`, want: fiber.StatusNoContent},
		{name: "default status", method: fiber.MethodPost, path: "/api/articles", body: `{"title":"draft"}`, want: fiber.StatusNoContent},
		{name: "submit status", method: fiber.MethodPost, path: "/api/articles", body: `{"status":5}`, want: fiber.StatusNoContent},
		{name: "autosave forces draft", method: fiber.MethodPatch, path: "/api/articles/1", body: `{"status":1,"autosave":true}`, want: fiber.StatusNoContent},
		{name: "preview create", method: fiber.MethodPost, path: "/api/articles/1/previews", body: `{}`, want: fiber.StatusNoContent},
		{name: "preview revoke", method: fiber.MethodDelete, path: "/api/articles/1/previews", body: "", want: fiber.StatusNoContent},
		{name: "revision restore", method: fiber.MethodPost, path: "/api/articles/1/revisions/1/restore", body: `{}`, want: fiber.StatusNoContent},
		{name: "trash restore", method: fiber.MethodPost, path: "/api/articles/1/restore", body: `{}`, want: fiber.StatusNoContent},
		{name: "submission withdraw", method: fiber.MethodPost, path: "/api/submissions/1/withdraw", body: `{}`, want: fiber.StatusNoContent},
		{name: "public status requires publish", method: fiber.MethodPatch, path: "/api/articles/1", body: `{"status":1}`, want: fiber.StatusForbidden},
		{name: "recycle status requires delete", method: fiber.MethodPatch, path: "/api/articles/1", body: `{"status":6}`, want: fiber.StatusForbidden},
		{name: "malformed body fails closed", method: fiber.MethodPatch, path: "/api/articles/1", body: `{"status":`, want: fiber.StatusForbidden},
		{name: "batch action is denied", method: fiber.MethodPost, path: "/api/articles/batch", body: `{"action":"delete"}`, want: fiber.StatusForbidden},
		{name: "review requires publish", method: fiber.MethodPost, path: "/api/articles/1/review", body: `{"action":"approve"}`, want: fiber.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := agentRequest(t, app, test.method, test.path, rawToken, test.body); got != test.want {
				t.Fatalf("status = %d, want %d", got, test.want)
			}
		})
	}
	if got := agentRequest(t, app, fiber.MethodPatch, "/api/articles/1", deleteToken, `{"status":6}`); got != fiber.StatusForbidden {
		t.Fatalf("status 6 should require articles:write and articles:delete, status = %d", got)
	}
	if got := agentRequest(t, app, fiber.MethodPatch, "/api/articles/1", writeDeleteToken, `{"status":6}`); got != fiber.StatusNoContent {
		t.Fatalf("articles:write and articles:delete status 6 = %d, want %d", got, fiber.StatusNoContent)
	}
}

func TestAgentAuthenticationFailsClosedAndKeepsRoleChecks(t *testing.T) {
	db := setupAgentTokenMiddlewareDB(t)
	_, importOnlyToken := createAgentTokenMiddlewareIdentity(t, db, "import-only", model.Admin, []string{service.AgentScopeContentImport})
	_, adminToken := createAgentTokenMiddlewareIdentity(t, db, "admin", model.Admin,
		[]string{service.AgentScopeContentImport, service.AgentScopeArticlesPublish, service.AgentScopeSettingsWrite})
	_, editorToken := createAgentTokenMiddlewareIdentity(t, db, "editor", model.Editor,
		[]string{service.AgentScopeContentImport, service.AgentScopeArticlesPublish, service.AgentScopeSettingsWrite})
	app := fiber.New()
	app.Post("/api/content/import", Authentication, AdminRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Post("/api/settings", Authentication, AdminRequired, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	for _, route := range []string{
		"/api/export", "/api/import", "/api/themes", "/api/themes/1/activate", "/api/users",
		"/api/account/profile", "/api/account/password", "/api/account/tokens", "/api/static-pages", "/api/articles/1/unknown",
	} {
		app.Get(route, Authentication, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	}
	app.Get("/api/articles/:id/unknown", Authentication, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	if got := agentRequest(t, app, fiber.MethodPost, "/api/content/import", importOnlyToken, `{}`); got != fiber.StatusForbidden {
		t.Fatalf("content import missing import+publish should be denied, status = %d", got)
	}
	if got := agentRequest(t, app, fiber.MethodPost, "/api/content/import", adminToken, `{}`); got != fiber.StatusNoContent {
		t.Fatalf("admin with content:import and articles:publish status = %d, want %d", got, fiber.StatusNoContent)
	}
	if got := agentRequest(t, app, fiber.MethodPost, "/api/settings", editorToken, `{}`); got != fiber.StatusForbidden {
		t.Fatalf("editor should not pass AdminRequired, status = %d", got)
	}
	if got := agentRequest(t, app, fiber.MethodPost, "/api/settings", adminToken, `{}`); got != fiber.StatusNoContent {
		t.Fatalf("admin with settings:write status = %d, want %d", got, fiber.StatusNoContent)
	}
	for _, route := range []string{
		"/api/export", "/api/import", "/api/themes", "/api/themes/1/activate", "/api/users",
		"/api/account/profile", "/api/account/password", "/api/account/tokens", "/api/static-pages", "/api/articles/1/unknown",
	} {
		if got := agentRequest(t, app, fiber.MethodGet, route, adminToken, ""); got != fiber.StatusForbidden {
			t.Errorf("GET %s status = %d, want %d", route, got, fiber.StatusForbidden)
		}
	}
	if got := agentRequest(t, app, fiber.MethodGet, "/api/articles/1/unknown", editorToken, ""); got != fiber.StatusForbidden {
		t.Fatalf("unknown article subpath should fail closed, status = %d", got)
	}
}

func TestOptionalAuthenticationChecksSuppliedAgentScope(t *testing.T) {
	db := setupAgentTokenMiddlewareDB(t)
	owner, writeToken := createAgentTokenMiddlewareIdentity(t, db, "reader", model.Author, []string{service.AgentScopeArticlesWrite})
	_, readToken, err := service.CreateAgentToken(owner.ID, "read agent", []string{service.AgentScopeArticlesRead}, 90)
	if err != nil {
		t.Fatalf("CreateAgentToken(read) error = %v", err)
	}
	app := fiber.New()
	app.Get("/api/articles/:id", OptionalAuthentication, func(c *fiber.Ctx) error {
		if c.Get(fiber.HeaderAuthorization) != "" && !IsAgentRequest(c) {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	anonymous, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/articles/1", nil))
	if err != nil || anonymous.StatusCode != fiber.StatusNoContent {
		t.Fatalf("anonymous optional request = (%v, %v), want 204", anonymous, err)
	}
	if got := agentRequest(t, app, fiber.MethodGet, "/api/articles/1", writeToken, ""); got != fiber.StatusForbidden {
		t.Fatalf("agent without articles:read status = %d, want %d", got, fiber.StatusForbidden)
	}
	if got := agentRequest(t, app, fiber.MethodGet, "/api/articles/1", readToken, ""); got != fiber.StatusNoContent {
		t.Fatalf("agent with articles:read status = %d, want %d", got, fiber.StatusNoContent)
	}
}

func TestAgentPathMatcherRejectsReservedAndEncodedIDs(t *testing.T) {
	for _, actual := range []string{"/api/articles/batch", "/api/articles/%2F", "/api/articles/../users", "/api/articles/one"} {
		if agentPathMatches(actual, "/api/articles/:id") {
			t.Errorf("agentPathMatches(%q, numeric id) = true", actual)
		}
	}
	if !agentPathMatches("/api/articles/my-post", "/api/articles/:article_ref") {
		t.Fatal("safe article slug should match article read route")
	}
}
