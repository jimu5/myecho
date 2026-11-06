package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
)

func ArticlePreviewCreate(c *fiber.Ctx) error {
	article, actor, responseErr, handled := articlePreviewRequestContext(c)
	if handled {
		return responseErr
	}
	var req rtype.ArticlePreviewCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if req.ExpectedEditVersion == nil {
		return ValidateErrorResponse(c, "expected_edit_version is required")
	}
	link, err := service.CreateArticlePreview(article.ID, actor.ID, *req.ExpectedEditVersion)
	if err != nil {
		if errors.Is(err, mysql.ErrArticlePreviewNotManageable) {
			return ValidateErrorResponse(c, "forbidden")
		}
		if errors.Is(err, mysql.ErrArticleEditConflict) || errors.Is(err, mysql.ErrArticleTrashed) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article state changed; refresh and try again")
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFoundErrorResponse(c, "article not found")
		}
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.Success(c, rtype.ArticlePreviewResponse{
		PreviewURL: "/article-preview/" + link.Token,
		ExpiresAt:  link.ExpiresAt,
	})
}

func ArticlePreviewRevoke(c *fiber.Ctx) error {
	article, actor, responseErr, handled := articlePreviewRequestContext(c)
	if handled {
		return responseErr
	}
	if err := service.RevokeArticlePreviews(article.ID, actor.ID); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.Success(c, nil)
}

func articlePreviewRequestContext(c *fiber.Ctx) (*mysql.ArticleModel, *model.User, error, bool) {
	actor := currentUser(c)
	if actor == nil {
		return nil, nil, UnauthorizedErrorResponse(c), true
	}
	articleID, err := parseArticlePreviewID(c.Params("id"))
	if err != nil {
		return nil, nil, ValidateErrorResponse(c, "invalid article id"), true
	}
	article, err := dal.MySqlDB.Article.FindByID(articleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, NotFoundErrorResponse(c, "article not found"), true
		}
		return nil, nil, InternalErrorResponse(c, InternalSQLError, err.Error()), true
	}
	if !canManageArticle(actor, &article) {
		return nil, nil, ValidateErrorResponse(c, "forbidden"), true
	}
	return &article, actor, nil, false
}

func parseArticlePreviewID(value string) (uint, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("missing id")
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 || parsed > uint64(^uint(0)) {
		return 0, errors.New("invalid id")
	}
	return uint(parsed), nil
}
