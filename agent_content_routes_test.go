package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/docs"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
)

func TestAgentOpenAPIOperationsMatchRegisteredRoutes(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(docs.OpenAPI, &spec); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	SetupApiRouter(app)
	SetupThemeRouter(app)
	routes := map[string]bool{}
	for _, route := range app.GetRoutes(true) {
		routes[route.Method+" "+route.Path] = true
	}
	for path, operations := range spec.Paths {
		path = strings.ReplaceAll(strings.ReplaceAll(path, "{", ":"), "}", "")
		for method := range operations {
			switch method {
			case "get", "post", "put", "patch", "delete":
				if !routes[strings.ToUpper(method)+" "+path] {
					t.Errorf("documented route is missing: %s %s", method, path)
				}
			}
		}
	}
}

func TestAgentContentRoutesDraftSubmitAndScopeBoundaries(t *testing.T) {
	setupArticleWritingRouteTestDB(t)
	if err := connect.Database.AutoMigrate(&model.AgentToken{}, &model.ArticleCreateRequest{}); err != nil {
		t.Fatal(err)
	}
	editor := createArticleWritingRouteUser(t)
	if err := connect.Database.Model(editor).Update("permission_type", model.Editor).Error; err != nil {
		t.Fatal(err)
	}
	token, raw, err := service.CreateAgentToken(editor.ID, "Draft editor", []string{service.AgentScopeArticlesRead, service.AgentScopeArticlesWrite}, 30)
	if err != nil {
		t.Fatal(err)
	}
	category := &mysql.CategoryModel{Name: "Agents", UID: "agent-route-category", Type: model.CategoryTypeArticle}
	if err := connect.Database.Create(category).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(middleware.CommonErrorHandler)
	SetupApiRouter(app)
	SetupThemeRouter(app)
	request := func(method, path, auth, key, body string, expected int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != expected {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, resp.StatusCode, expected, data)
		}
		return data
	}
	auth := "Bearer " + raw
	body := `{"title":"Agent draft","content":"Draft body","status":4,"category_uid":"` + category.UID + `"}`
	request("POST", "/api/articles", auth, "", body, 400)
	created := request("POST", "/api/articles", auth, "agent-create", body, 201)
	if replay := request("POST", "/api/articles", auth, "agent-create", body, 201); string(replay) != string(created) {
		t.Fatalf("idempotent create changed response: %s", replay)
	}
	var envelope struct {
		Data rtype.ArticleResponse `json:"data"`
	}
	if err := json.Unmarshal(created, &envelope); err != nil {
		t.Fatal(err)
	}
	articlePath := "/api/articles/" + strconv.Itoa(int(envelope.Data.ID))
	request("GET", articlePath, auth, "", "", 200)
	request("PATCH", articlePath, auth, "", body, 400)
	stale := strings.TrimSuffix(body, "}") + `,"expected_edit_version":0}`
	request("PATCH", articlePath, auth, "", stale, 409)
	publish := strings.Replace(body, `"status":4`, `"status":1`, 1)
	request("PATCH", articlePath, auth, "", publish, 403)
	request("DELETE", articlePath, auth, "", "", 403)
	request("POST", "/api/articles/batch", auth, "", `{"ids":[`+strconv.Itoa(int(envelope.Data.ID))+`],"action":"delete"}`, 403)
	request("POST", "/api/account/tokens", auth, "", `{"name":"escalate","scopes":["articles:publish"]}`, 403)
	request("GET", "/api/account/profile", auth, "", "", 403)
	request("GET", "/api/export", auth, "", "", 403)
	request("GET", "/api/themes/active", auth, "", "", 403)

	live := &mysql.ArticleModel{AuthorID: editor.ID, Title: "Live", Status: int8(mysql.ARTILCE_STATUS_PUBLIC), CategoryUID: category.UID, Detail: &model.ArticleDetail{Content: "Live body"}}
	if err := (&mysql.ArticleDBRepo{}).Create(live); err != nil {
		t.Fatal(err)
	}
	livePath := "/api/articles/" + strconv.Itoa(int(live.ID))
	submit := strings.Replace(body, `"status":4`, `"status":5`, 1)
	submit = strings.TrimSuffix(submit, "}") + `,"expected_edit_version":1}`
	request("PATCH", livePath, auth, "", submit, 200)
	stored, err := (&mysql.ArticleDBRepo{}).FindByID(live.ID)
	if err != nil || stored.Status != int8(mysql.ARTILCE_STATUS_PUBLIC) || stored.Detail.Content != "Live body" {
		t.Fatalf("agent submit changed published content: %+v, err=%v", stored, err)
	}
	pending, err := service.PendingArticleSubmission(live.ID)
	if err != nil || pending == nil {
		t.Fatalf("agent submit did not create pending submission: %+v, %v", pending, err)
	}
	request("POST", livePath+"/review", auth, "", `{"action":"approve"}`, 403)
	request("GET", "/api/openapi.json", "", "", "", 200)
	request("DELETE", "/api/account/tokens/"+strconv.Itoa(int(token.ID)), "token "+editor.Token, "", "", 200)
	request("GET", articlePath, auth, "", "", 401)
}

func TestAgentArticleReadScopeCannotFallBackToAnonymous(t *testing.T) {
	setupArticleWritingRouteTestDB(t)
	if err := connect.Database.AutoMigrate(&model.AgentToken{}); err != nil {
		t.Fatal(err)
	}
	owner := createArticleWritingRouteUser(t)
	_, raw, err := service.CreateAgentToken(owner.ID, "Write only", []string{service.AgentScopeArticlesWrite}, 1)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	SetupApiRouter(app)
	req := httptest.NewRequest("GET", "/api/articles/1", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GET without read scope = %d, want 403", resp.StatusCode)
	}
}
