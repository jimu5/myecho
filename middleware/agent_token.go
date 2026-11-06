package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"myecho/config/static_config"
	"myecho/model"
	"myecho/service"

	"github.com/gofiber/fiber/v2"
)

var (
	ErrRequestUnauthorized = errors.New("unauthorized")
	ErrRequestForbidden    = errors.New("forbidden")
)

const (
	userLocalKey       = "user"
	agentTokenLocalKey = "agent_token"
)

// RequestUser resolves and authorizes the request's login or agent credential.
// An empty Authorization header is allowed for optional-auth handlers.
func RequestUser(c *fiber.Ctx) (*model.User, error) {
	user, token, err := resolveRequestCredential(c)
	if err != nil || user == nil {
		return user, err
	}
	if token != nil {
		if !agentRequestAllowed(c, token.Scopes) {
			return nil, ErrRequestForbidden
		}
		c.Locals(agentTokenLocalKey, token)
	} else {
		c.Locals(agentTokenLocalKey, nil)
	}
	c.Locals(userLocalKey, user)
	return user, nil
}

func Authentication(c *fiber.Ctx) error {
	user, err := RequestUser(c)
	if err != nil {
		if errors.Is(err, ErrRequestForbidden) {
			return forbiddenErrorResponse(c)
		}
		if errors.Is(err, ErrRequestUnauthorized) {
			return unauthorizedErrorResponse(c)
		}
		return c.Status(http.StatusInternalServerError).JSON(Error{Code: 5001, Msg: "internal server error"})
	}
	if user == nil {
		return unauthorizedErrorResponse(c)
	}
	return c.Next()
}

// OptionalAuthentication authenticates requests that supply credentials and
// leaves anonymous requests untouched.
func OptionalAuthentication(c *fiber.Ctx) error {
	if c.Get(fiber.HeaderAuthorization) == "" {
		return c.Next()
	}
	return Authentication(c)
}

func IsAgentRequest(c *fiber.Ctx) bool {
	token, ok := c.Locals(agentTokenLocalKey).(*model.AgentToken)
	return ok && token != nil
}

func resolveRequestCredential(c *fiber.Ctx) (*model.User, *model.AgentToken, error) {
	authorization := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	if authorization == "" {
		return nil, nil, nil
	}
	fields := strings.Fields(authorization)
	if len(fields) != 2 {
		return nil, nil, ErrRequestUnauthorized
	}
	switch {
	case strings.EqualFold(fields[0], "token"):
		user, err := GetUserByToken(fields[1])
		if err != nil || user.Disabled || (!user.TokenExpiresAt.IsZero() && !time.Now().Before(user.TokenExpiresAt)) {
			return nil, nil, ErrRequestUnauthorized
		}
		return &user, nil, nil
	case strings.EqualFold(fields[0], "bearer") && strings.HasPrefix(fields[1], service.AgentTokenPrefix):
		token, user, err := service.ResolveAgentToken(fields[1])
		if errors.Is(err, service.ErrAgentTokenInvalid) {
			return nil, nil, ErrRequestUnauthorized
		}
		if err != nil {
			return nil, nil, err
		}
		return &user, &token, nil
	default:
		return nil, nil, ErrRequestUnauthorized
	}
}

func agentRequestAllowed(c *fiber.Ctx, scopes []string) bool {
	required, ok := requiredAgentScopes(c.Method(), c.Path(), c.Body())
	if !ok {
		return false
	}
	granted := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		granted[scope] = struct{}{}
	}
	for _, scope := range required {
		if _, ok := granted[scope]; !ok {
			return false
		}
	}
	return true
}

func requiredAgentScopes(method, actualPath string, body []byte) ([]string, bool) {
	const (
		read    = service.AgentScopeArticlesRead
		write   = service.AgentScopeArticlesWrite
		publish = service.AgentScopeArticlesPublish
		delete  = service.AgentScopeArticlesDelete
	)
	method = strings.ToUpper(method)

	if method == fiber.MethodGet {
		switch {
		case agentPathMatches(actualPath, "/api/articles"),
			agentPathMatches(actualPath, "/api/articles/:article_ref"),
			agentPathMatches(actualPath, "/api/all_articles"),
			agentPathMatches(actualPath, "/api/articles/:id/revisions"),
			agentPathMatches(actualPath, "/api/articles/:id/revisions/:revision_id"),
			agentPathMatches(actualPath, "/api/articles/:id/reviews"),
			agentPathMatches(actualPath, "/api/article/categories/all"),
			agentPathMatches(actualPath, "/api/link/categories/all"),
			agentPathMatches(actualPath, "/api/tags/all"):
			return []string{read}, true
		case agentPathMatches(actualPath, "/api/content/export"):
			return []string{service.AgentScopeContentExport}, true
		case agentPathMatches(actualPath, "/api/comments"):
			return []string{service.AgentScopeCommentsRead}, true
		case agentPathMatches(actualPath, "/api/settings/admin"),
			agentPathMatches(actualPath, "/api/settings"),
			agentPathMatches(actualPath, "/api/settings/:key"):
			return []string{service.AgentScopeSettingsRead}, true
		}
	}

	if method == fiber.MethodPost && agentPathMatches(actualPath, "/api/articles") ||
		method == fiber.MethodPatch && agentPathMatches(actualPath, "/api/articles/:id") {
		return articleWriteScopes(body)
	}
	if method == fiber.MethodPost && agentPathMatches(actualPath, "/api/content/import") {
		return []string{service.AgentScopeContentImport, publish}, true
	}
	if method == fiber.MethodPost && agentPathMatches(actualPath, "/api/articles/:id/review") {
		return []string{publish}, true
	}
	if method == fiber.MethodPost && agentPathMatches(actualPath, "/api/submissions/:id/review") {
		return []string{publish}, true
	}
	if method == fiber.MethodPost && agentPathMatches(actualPath, "/api/submissions/:id/withdraw") {
		return []string{write}, true
	}
	if method == fiber.MethodPost && (agentPathMatches(actualPath, "/api/articles/:id/previews") ||
		agentPathMatches(actualPath, "/api/articles/:id/revisions/:revision_id/restore") ||
		agentPathMatches(actualPath, "/api/articles/:id/restore")) {
		return []string{write}, true
	}
	if method == fiber.MethodDelete && agentPathMatches(actualPath, "/api/articles/:id/previews") {
		return []string{write}, true
	}
	if method == fiber.MethodDelete && (agentPathMatches(actualPath, "/api/articles/:id") ||
		agentPathMatches(actualPath, "/api/articles/:id/permanent")) {
		return []string{delete}, true
	}
	if method == fiber.MethodPost && (agentPathMatches(actualPath, "/api/article/categories") ||
		agentPathMatches(actualPath, "/api/link/categories") ||
		agentPathMatches(actualPath, "/api/tags")) {
		return []string{service.AgentScopeTaxonomyWrite}, true
	}
	if (method == fiber.MethodPatch || method == fiber.MethodDelete) &&
		(agentPathMatches(actualPath, "/api/categories/:id") || agentPathMatches(actualPath, "/api/tags/:id")) {
		return []string{service.AgentScopeTaxonomyWrite}, true
	}
	if mediaScopes := requiredMediaScopes(method, actualPath); len(mediaScopes) > 0 {
		return mediaScopes, true
	}
	if method == fiber.MethodGet && agentPathMatches(actualPath, "/api/submissions") {
		return []string{read}, true
	}
	if method == fiber.MethodGet && agentPathMatches(actualPath, "/api/submissions/:id") {
		return []string{read}, true
	}
	if method == fiber.MethodPatch && agentPathMatches(actualPath, "/api/comments/:id") ||
		method == fiber.MethodPost && (agentPathMatches(actualPath, "/api/comments/:id/reply") || agentPathMatches(actualPath, "/api/comments/batch")) ||
		method == fiber.MethodDelete && agentPathMatches(actualPath, "/api/comments/:id") {
		return []string{service.AgentScopeCommentsWrite}, true
	}
	if method == fiber.MethodPost && agentPathMatches(actualPath, "/api/settings") ||
		method == fiber.MethodPatch && agentPathMatches(actualPath, "/api/settings/:key") ||
		method == fiber.MethodDelete && agentPathMatches(actualPath, "/api/settings/:key") {
		return []string{service.AgentScopeSettingsWrite}, true
	}
	return nil, false
}

func articleWriteScopes(body []byte) ([]string, bool) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, false
	}
	var request struct {
		Status   *int8 `json:"status"`
		Autosave bool  `json:"autosave"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, false
	}
	if request.Autosave {
		return []string{service.AgentScopeArticlesWrite}, true
	}
	status := int8(0)
	if request.Status != nil {
		status = *request.Status
	}
	switch status {
	case 0, 4, 5:
		return []string{service.AgentScopeArticlesWrite}, true
	case 1, 2, 3:
		return []string{service.AgentScopeArticlesWrite, service.AgentScopeArticlesPublish}, true
	case 6:
		return []string{service.AgentScopeArticlesWrite, service.AgentScopeArticlesDelete}, true
	default:
		return nil, false
	}
}

func requiredMediaScopes(method, actualPath string) []string {
	root := strings.TrimRight(static_config.StorageRootUrl, "/")
	if root == "" {
		return nil
	}
	switch {
	case method == fiber.MethodGet && (agentPathMatches(actualPath, root+"/files") || agentPathMatches(actualPath, root+"/files/:id/usages")):
		return []string{service.AgentScopeMediaRead}
	case method == fiber.MethodPost && (agentPathMatches(actualPath, root+"/files/vditor_upload") ||
		agentPathMatches(actualPath, root+"/files/upload") || agentPathMatches(actualPath, root+"/save_url_file")):
		return []string{service.AgentScopeMediaWrite}
	case method == fiber.MethodPut && agentPathMatches(actualPath, root+"/files/:id"):
		return []string{service.AgentScopeMediaWrite}
	case method == fiber.MethodDelete && agentPathMatches(actualPath, root+"/files/:id"):
		return []string{service.AgentScopeMediaDelete}
	default:
		return nil
	}
}

func agentPathMatches(actual, pattern string) bool {
	actual = strings.TrimSuffix(actual, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	actualParts, patternParts := strings.Split(actual, "/"), strings.Split(pattern, "/")
	if len(actualParts) != len(patternParts) {
		return false
	}
	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			if actualParts[i] == "" || actualParts[i] == "." || actualParts[i] == ".." || strings.ContainsAny(actualParts[i], "%\\") {
				return false
			}
			switch part {
			case ":key":
				if !safeAgentPathSegment(actualParts[i], true) {
					return false
				}
			case ":article_ref":
				if !safeAgentPathSegment(actualParts[i], false) {
					return false
				}
			default:
				if !numericAgentPathSegment(actualParts[i]) {
					return false
				}
			}
			continue
		}
		if actualParts[i] != part {
			return false
		}
	}
	return true
}

func numericAgentPathSegment(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}

func safeAgentPathSegment(value string, allowDot bool) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || (allowDot && r == '.') {
			continue
		}
		return false
	}
	return value != ""
}
