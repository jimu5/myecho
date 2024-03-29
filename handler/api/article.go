package api

import (
	"github.com/gofiber/fiber/v2"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
)

// ShowAccount godoc
//
//	@Summary		展示所有文章
//	@Description	分页展示所有文章
//	@Tags			articles
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}	model.Article
//	@Router			/articles [get]
func ArticleDisplayList(c *fiber.Ctx) error {
	queryParam := service.ArticleDisplayListQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	pageInfo, articles, err := service.S.Article.ArticleDisplayList(&queryParam)
	res := rtype.MultiModelToArticleResponse(articles)
	if err != nil {
		return err
	}
	return handler.PaginateData(c, pageInfo.Total, res)
}

type ArticleAllListQueryParam struct {
	CategoryUID *string              `query:"category_uid"`
	Status      *mysql.ArticleStatus `query:"status"`
}

func ArticleAllList(c *fiber.Ctx) error {
	var (
		err   error
		total int64
	)
	queryParam := ArticleAllListQueryParam{}
	if err = c.QueryParser(&queryParam); err != nil {
		return err
	}
	sqlCommonParam := mysql.ArticleCommonQueryParam{
		CategoryUID: queryParam.CategoryUID,
	}
	topStatus := mysql.ARTICLE_STATUS_TOP
	if queryParam.Status != nil {
		sqlCommonParam.Status = queryParam.Status
		total, err = dal.MySqlDB.Article.CountAll(sqlCommonParam)
	} else {
		total, err = dal.MySqlDB.Article.CountAll(sqlCommonParam)
		sqlCommonParam.Status = &topStatus
	}
	if err != nil {
		return err
	}
	sqlParam := mysql.PageFindArticleByNotStatusParam{
		ArticleCommonQueryParam: sqlCommonParam,
	}
	topArticles, pageParam, err := handler.PageFind(c, dal.MySqlDB.Article.PageFindByCommonParam, sqlCommonParam)
	if err != nil {
		return err
	}
	if queryParam.Status != nil {
		res := rtype.MultiModelToArticleResponse(topArticles)
		return handler.PaginateData(c, total, res)
	}
	pageParam.PageSize = pageParam.PageSize - len(topArticles)
	if pageParam.PageSize == 0 {
		res := rtype.MultiModelToArticleResponse(topArticles)
		return handler.PaginateData(c, total, res)
	}
	restArticles, err := dal.MySqlDB.Article.PageFindByNotVisibility(&pageParam, sqlParam)
	if err != nil {
		return err
	}
	articles := topArticles
	articles = append(articles, restArticles...)
	res := rtype.MultiModelToArticleResponse(articles)
	return handler.PaginateData(c, total, res)
}

func ArticleRetrieve(c *fiber.Ctx) error {
	var (
		article mysql.ArticleModel
		err     error
	)
	queryParam := service.ArticleRetrieveQueryParam{}
	if err = c.QueryParser(&queryParam); err != nil {
		return err
	}
	if err = handler.DetailPreHandleByParam(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	queryParam.ID = article.ID
	article, err = service.S.Article.ArticleRetrieve(&queryParam)
	if err != nil {
		return err
	}
	res := rtype.ModelToArticleResponse(&article)
	return c.JSON(&res)
}

func ArticleCreate(c *fiber.Ctx) error {
	var article mysql.ArticleModel
	var detail model.ArticleDetail
	var r rtype.ArticleRequest
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	r.PreHandle()
	// 校验
	err := validator.ValidateArticleRequest(&r)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	detail.Content = r.Content
	r.SetSummary()
	structAssign(&article, &r)
	article.Detail = &detail
	user := handler.GetUserFromCtx(c)
	article.AuthorID = user.ID
	article.Author = user

	tags, err := getTagsByUID(r.TagUIDs)
	if err != nil {
		return err
	}
	article.Tags = tags

	err = dal.MySqlDB.Article.Create(&article)
	if err != nil {
		return err
	}
	res := rtype.ModelToArticleResponse(&article)
	return c.Status(fiber.StatusCreated).JSON(res)
}

// 更新文章
func ArticleUpdate(c *fiber.Ctx) error {
	var article mysql.ArticleModel
	var r rtype.ArticleRequest
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	r.PreHandle()
	// 校验
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if err := validator.ValidateArticleRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	r.SetSummary()
	structAssign(&article, &r)
	article.Detail = &model.ArticleDetail{Content: r.Content}
	tags, err := getTagsByUID(r.TagUIDs)
	if err != nil {
		return err
	}
	article.Tags = tags
	// TODO: content id 为0的情况
	if err := dal.MySqlDB.Article.Update(&article); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	//config.Database.Debug().Model(&article).Omit("User").Updates(&article)
	article, err = dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	res := rtype.ModelToArticleResponse(&article)
	return c.Status(fiber.StatusOK).JSON(&res)
}

// 删除文章
func ArticleDelete(c *fiber.Ctx) error {
	var article model.Article
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if err := dal.MySqlDB.Article.DeleteByID(article.ID); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func getTagsByUID(tagUIDs []string) ([]*model.Tag, error) {
	if len(tagUIDs) == 0 {
		return nil, nil
	}
	return FindTagsByUID(tagUIDs)
}
