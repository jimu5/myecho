package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/rtype"
)

// ArticleTrashRestore restores an article from the trash as a draft.
func ArticleTrashRestore(c *fiber.Ctx) error {
	articleID, err := articleIDFromParam(c)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	article, err := dal.MySqlDB.Article.FindByID(articleID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFoundErrorResponse(c, err.Error())
	}
	if err != nil {
		return err
	}
	user := currentUser(c)
	if !canManageArticle(user, &article) {
		return ValidateErrorResponse(c, "forbidden")
	}
	if err := dal.MySqlDB.Article.RestoreFromTrash(articleID, user.ID); err != nil {
		if isArticleTrashConflict(err) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, err.Error())
		}
		return err
	}
	restored, err := dal.MySqlDB.Article.FindByID(articleID)
	if err != nil {
		return err
	}
	return handler.Success(c, rtype.ModelToUnlockedArticleResponse(&restored))
}

// ArticlePermanentlyDelete permanently deletes an article after it has been
// moved to the trash.
func ArticlePermanentlyDelete(c *fiber.Ctx) error {
	articleID, err := articleIDFromParam(c)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	article, err := dal.MySqlDB.Article.FindByID(articleID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFoundErrorResponse(c, err.Error())
	}
	if err != nil {
		return err
	}
	if !canManageArticle(currentUser(c), &article) {
		return ValidateErrorResponse(c, "forbidden")
	}
	if err := dal.MySqlDB.Article.PermanentlyDelete(articleID); err != nil {
		if isArticleTrashConflict(err) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, err.Error())
		}
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusOK, nil)
}

func isArticleTrashConflict(err error) bool {
	return errors.Is(err, mysql.ErrArticleStatusChanged) ||
		errors.Is(err, mysql.ErrArticleEditConflict) ||
		errors.Is(err, mysql.ErrArticleNotTrashed)
}
