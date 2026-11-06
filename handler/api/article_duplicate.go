package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/rtype"
	"myecho/service"
)

// ArticleDuplicate creates a new draft from an article the current user can manage.
func ArticleDuplicate(c *fiber.Ctx) error {
	articleID, err := articleIDFromParam(c)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	source, err := dal.MySqlDB.Article.FindByID(articleID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFoundErrorResponse(c, err.Error())
	}
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	user := currentUser(c)
	if !canManageArticle(user, &source) {
		return ValidateErrorResponse(c, "forbidden")
	}

	duplicate, err := service.S.Article.DuplicateAsDraft(articleID, user.ID)
	if errors.Is(err, mysql.ErrArticleTrashed) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, err.Error())
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFoundErrorResponse(c, err.Error())
	}
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.SuccessWithStatus(c, fiber.StatusCreated, rtype.ModelToUnlockedArticleResponse(&duplicate))
}
