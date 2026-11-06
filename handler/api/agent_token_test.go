package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"myecho/dal/connect"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
)

type agentTokenTestEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

func setupAgentTokenAPI(t *testing.T) (*gorm.DB, *model.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "agent-token-api.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AgentToken{}); err != nil {
		t.Fatalf("migrate agent token tables: %v", err)
	}
	previousDB := connect.Database
	connect.Database = db
	user := &model.User{Name: "token owner", Email: "token-owner@example.com", Token: "login-token", PermissionType: model.Author}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		connect.Database = previousDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db, user
}

func agentTokenManagementApp() *fiber.App {
	app := fiber.New()
	app.Get("/api/account/tokens", middleware.Authentication, AgentTokenList)
	app.Post("/api/account/tokens", middleware.Authentication, AgentTokenCreate)
	app.Delete("/api/account/tokens/:id", middleware.Authentication, AgentTokenRevoke)
	return app
}

func agentTokenAPIRequest(t *testing.T, app *fiber.App, method, path, loginToken, body string) (*httpResponse, error) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set(fiber.HeaderAuthorization, "token "+loginToken)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		return nil, err
	}
	return &httpResponse{status: resp.StatusCode, body: resp.Body}, nil
}

type httpResponse struct {
	status int
	body   interface{ Read([]byte) (int, error) }
}

func TestAgentTokenHandlersCreateListAndRevokeOwnTokens(t *testing.T) {
	db, owner := setupAgentTokenAPI(t)
	other := &model.User{Name: "other owner", Email: "other-owner@example.com", Token: "other-login", PermissionType: model.Author}
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("create other user: %v", err)
	}
	app := agentTokenManagementApp()
	created, err := agentTokenAPIRequest(t, app, fiber.MethodPost, "/api/account/tokens", owner.Token,
		`{"name":"nightly import","scopes":["articles:read"],"expires_in_days":1}`)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if created.status != fiber.StatusCreated {
		t.Fatalf("create status = %d, want %d", created.status, fiber.StatusCreated)
	}
	var createdEnvelope agentTokenTestEnvelope
	if err := json.NewDecoder(created.body).Decode(&createdEnvelope); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	var createdToken rtype.AgentTokenResponse
	if err := json.Unmarshal(createdEnvelope.Data, &createdToken); err != nil {
		t.Fatalf("decode created token: %v", err)
	}
	if createdEnvelope.Code != 0 || createdToken.ID == 0 || createdToken.OwnerID != owner.ID || createdToken.Name != "nightly import" ||
		createdToken.Token == "" || len(createdToken.Scopes) != 1 || createdToken.Scopes[0] != service.AgentScopeArticlesRead {
		t.Fatalf("created token response = %#v", createdToken)
	}
	if createdToken.ExpiresAt.IsZero() {
		t.Fatal("create response has no expires_at")
	}

	listed, err := agentTokenAPIRequest(t, app, fiber.MethodGet, "/api/account/tokens", owner.Token, "")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	if listed.status != fiber.StatusOK {
		t.Fatalf("list status = %d, want %d", listed.status, fiber.StatusOK)
	}
	var listEnvelope struct {
		Code int               `json:"code"`
		Data []json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(listed.body).Decode(&listEnvelope); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listEnvelope.Code != 0 || len(listEnvelope.Data) != 1 {
		t.Fatalf("list response = %#v", listEnvelope)
	}
	var listedFields map[string]json.RawMessage
	if err := json.Unmarshal(listEnvelope.Data[0], &listedFields); err != nil {
		t.Fatalf("decode listed token fields: %v", err)
	}
	if _, hasRawToken := listedFields["token"]; hasRawToken {
		t.Fatal("list response includes the one-time raw credential")
	}

	revoked, err := agentTokenAPIRequest(t, app, fiber.MethodDelete, "/api/account/tokens/"+strconv.FormatUint(uint64(createdToken.ID), 10), owner.Token, "")
	if err != nil {
		t.Fatalf("revoke request: %v", err)
	}
	if revoked.status != fiber.StatusOK {
		t.Fatalf("revoke status = %d, want %d", revoked.status, fiber.StatusOK)
	}
	var revokedEnvelope agentTokenTestEnvelope
	if err := json.NewDecoder(revoked.body).Decode(&revokedEnvelope); err != nil {
		t.Fatalf("decode revoke response: %v", err)
	}
	var revokedToken rtype.AgentTokenResponse
	if err := json.Unmarshal(revokedEnvelope.Data, &revokedToken); err != nil {
		t.Fatalf("decode revoked token: %v", err)
	}
	if revokedToken.RevokedAt == nil || revokedToken.Token != "" {
		t.Fatalf("revoke response = %#v", revokedToken)
	}

	otherRevoke, err := agentTokenAPIRequest(t, app, fiber.MethodDelete, "/api/account/tokens/"+strconv.FormatUint(uint64(createdToken.ID), 10), other.Token, "")
	if err != nil {
		t.Fatalf("other-owner revoke request: %v", err)
	}
	if otherRevoke.status != fiber.StatusNotFound {
		t.Fatalf("other-owner revoke status = %d, want %d", otherRevoke.status, fiber.StatusNotFound)
	}
}

func TestAgentTokenHandlersRejectUnknownScopesAndAgentCredentialManagement(t *testing.T) {
	_, owner := setupAgentTokenAPI(t)
	_, agentToken, err := service.CreateAgentToken(owner.ID, "agent", []string{service.AgentScopeSettingsWrite}, 90)
	if err != nil {
		t.Fatalf("create agent credential: %v", err)
	}
	app := agentTokenManagementApp()
	invalid, err := agentTokenAPIRequest(t, app, fiber.MethodPost, "/api/account/tokens", owner.Token,
		`{"name":"bad","scopes":["admin:*"],"expires_in_days":90}`)
	if err != nil {
		t.Fatalf("invalid-scope request: %v", err)
	}
	if invalid.status != fiber.StatusBadRequest {
		t.Fatalf("invalid-scope status = %d, want %d", invalid.status, fiber.StatusBadRequest)
	}

	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{method: fiber.MethodGet, path: "/api/account/tokens"},
		{method: fiber.MethodPost, path: "/api/account/tokens", body: `{"name":"escalation","scopes":["articles:read"]}`},
		{method: fiber.MethodDelete, path: "/api/account/tokens/1"},
	} {
		req := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+agentToken)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("agent management %s %s: %v", test.method, test.path, err)
		}
		if resp.StatusCode != fiber.StatusForbidden {
			t.Errorf("agent management %s %s status = %d, want %d", test.method, test.path, resp.StatusCode, fiber.StatusForbidden)
		}
	}
}

func TestAgentTokenHandlersGuardAgainstDirectAgentLocals(t *testing.T) {
	_, owner := setupAgentTokenAPI(t)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", owner)
		c.Locals("agent_token", &model.AgentToken{OwnerID: owner.ID})
		return c.Next()
	})
	app.Get("/api/account/tokens", AgentTokenList)
	app.Post("/api/account/tokens", AgentTokenCreate)
	app.Delete("/api/account/tokens/:id", AgentTokenRevoke)
	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{method: fiber.MethodGet, path: "/api/account/tokens"},
		{method: fiber.MethodPost, path: "/api/account/tokens", body: `{"name":"escalation","scopes":["articles:read"]}`},
		{method: fiber.MethodDelete, path: "/api/account/tokens/1"},
	} {
		req := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("direct agent handler %s %s: %v", test.method, test.path, err)
		}
		if resp.StatusCode != fiber.StatusForbidden {
			t.Errorf("direct agent handler %s %s status = %d, want %d", test.method, test.path, resp.StatusCode, fiber.StatusForbidden)
		}
	}
}
