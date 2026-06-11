package view

import (
	"bytes"
	"errors"
	"github.com/gofiber/fiber/v2"
	"myecho/dal"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
	"myecho/utils"
)

func ArticleDisplayList(c *fiber.Ctx) error {
	queryParam := service.ArticleDisplayListQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	pageInfo, data, err := service.S.Article.ArticleDisplayList(&queryParam)
	if err != nil {
		return err
	}
	pageInfoResp := getPageInfoRespByMysqlPageInfo(c, &pageInfo)
	return c.Render("index", respToMap(c, Pagination{PageInfo: pageInfoResp, PageData: data}, PageMeta{
		Description: "最近更新的文章列表",
		Canonical:   absoluteURL(c),
		OGTitle:     "最近更新",
	}))
}

func ArticleRetrieve(c *fiber.Ctx) error {
	queryParam := service.ArticleRetrieveQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	article := new(mysql.ArticleModel)
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return api.NotFoundErrorResponse(c, err.Error())
	}
	return renderArticle(c, article, &queryParam)
}

func PostRetrieveBySlug(c *fiber.Ctx) error {
	return retrieveBySlug(c, model.ArticleTypePost)
}

func PageRetrieveBySlug(c *fiber.Ctx) error {
	return retrieveBySlug(c, model.ArticleTypePage)
}

func retrieveBySlug(c *fiber.Ctx, articleType model.ArticleType) error {
	queryParam := service.ArticleRetrieveQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	article, err := dal.MySqlDB.Article.FindBySlug(c.Params("slug"), articleType)
	if err != nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return renderArticle(c, &article, &queryParam)
}

func renderArticle(c *fiber.Ctx, article *mysql.ArticleModel, queryParam *service.ArticleRetrieveQueryParam) error {
	queryParam.ID = article.ID
	queryParam.PasswordUnlocked = service.ValidateArticlePasswordToken(article, c.Cookies(service.ArticlePasswordCookieName(article.ID)))
	res, err := service.S.Article.ArticleRetrieve(queryParam)
	if err != nil {
		if errors.Is(err, service.ErrArticleNotDisplayable) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		if errors.Is(err, service.ErrArticlePasswordRequired) {
			return c.Status(fiber.StatusForbidden).Render("article_password", respToMap(c, *article, PageMeta{
				Description: article.Summary,
				Canonical:   absoluteURL(c),
				OGTitle:     article.Title,
				OGType:      "article",
			}))
		}
		return err
	}
	// 解析成 markdown
	var buf bytes.Buffer
	if err = utils.MDParser.Convert([]byte(res.Detail.Content), &buf); err != nil {
		return err
	}
	res.Detail.Content = buf.String()
	comments, err := approvedComments(res.UID)
	if err != nil {
		return err
	}
	data := respToMap(c, res, PageMeta{
		Description: res.Summary,
		Canonical:   absoluteURL(c),
		OGTitle:     res.Title,
		OGType:      "article",
	})
	data["Comments"] = comments
	data["IsAllowComment"] = isAllowComment(res.IsAllowComment)
	return c.Render("article", data)
}

func isAllowComment(value *bool) bool {
	return value == nil || *value
}

func approvedComments(articleUID string) ([]rtype.CommentResponse, error) {
	comments := make([]rtype.CommentResponse, 0)
	err := connect.Database.Table("comments").
		Where("article_uid = ?", articleUID).
		Where("status IS NULL OR status = ? OR status = ?", int8(model.CommentStatusLegacyApproved), int8(model.CommentStatusApproved)).
		Order("post_time asc, created_at asc").
		Find(&comments).Error
	if err != nil {
		return nil, err
	}
	return api.BuildCommentTree(comments), nil
}
