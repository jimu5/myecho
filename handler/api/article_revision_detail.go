package api

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	apierrors "myecho/handler/api/errors"
	"myecho/handler/rtype"
)

// ArticleRevisionRetrieve returns the content snapshot of one article revision.
func ArticleRevisionRetrieve(c *fiber.Ctx) error {
	articleID, err := parseArticleRevisionPathID(c.Params("id"))
	if err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	revisionID, err := parseArticleRevisionPathID(c.Params("revision_id"))
	if err != nil {
		return ParseErrorResponse(c, err.Error())
	}

	article, err := dal.MySqlDB.Article.FindByID(articleID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFoundErrorResponse(c, "article not found")
	}
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	if !canManageArticle(currentUser(c), &article) {
		return ValidateErrorResponse(c, "forbidden")
	}
	if article.Status == int8(mysql.ARTICLE_STATUS_RECYCLE) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before viewing revisions")
	}

	revision, err := dal.MySqlDB.Article.FindRevision(articleID, revisionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFoundErrorResponse(c, "article revision not found")
	}
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}

	response := rtype.ArticleRevisionDetailResponse{
		ID:             revision.ID,
		ArticleID:      revision.ArticleID,
		CreatedAt:      revision.CreatedAt,
		ActorID:        revision.ActorID,
		Title:          revision.Title,
		Slug:           revision.Slug,
		Type:           revision.Type,
		ContentFormat:  revision.ContentFormat,
		Summary:        revision.Summary,
		SEOTitle:       revision.SEOTitle,
		SEODescription: revision.SEODescription,
		ShareImage:     revision.ShareImage,
		Content:        revision.Content,
	}
	return handler.Success(c, response)
}

func parseArticleRevisionPathID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, strconv.IntSize)
	if err != nil || id == 0 {
		return 0, apierrors.ErrInvalidParams
	}
	return uint(id), nil
}
