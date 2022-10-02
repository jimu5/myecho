package api

import (
	"github.com/gofiber/fiber/v2"
	"log"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
)

func ArticleDisplayList(c *fiber.Ctx) error {
	queryParam := service.ArticleDisplayListQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	total, res, err := service.S.Article.ArticleDisplayList(&queryParam)
	if err != nil {
		return err
	}
	return handler.PaginateData(c, total, res)
}

type ArticleAllListQueryParam struct {
	CategoryID *uint                `query:"category_id"`
	Status     *mysql.ArticleStatus `query:"status"`
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
		CategoryID: queryParam.CategoryID,
	}

	if queryParam.Status != nil {
		sqlCommonParam.Status = queryParam.Status
		total, err = dal.MySqlDB.Article.CountAll(sqlCommonParam)
	} else {
		total, err = dal.MySqlDB.Article.CountAll(sqlCommonParam)
		status := mysql.ARTICLE_STATUS_TOP
		sqlCommonParam.Status = &status
	}
	if err != nil {
		return err
	}
	sqlParam := mysql.PageFindArticleByNotStatusParam{
		ArticleCommonQueryParam: sqlCommonParam,
	}
	FirstArticles, pageParam, err := handler.PageFind(c, dal.MySqlDB.Article.PageFindByCommonParam, sqlCommonParam)
	if err != nil {
		return err
	}
	if queryParam.Status != nil {
		res := rtype.MultiModelToArticleResponse(FirstArticles)
		return handler.PaginateData(c, total, res)
	}
	pageParam.PageSize = pageParam.PageSize - len(FirstArticles)
	if pageParam.PageSize == 0 {
		res := rtype.MultiModelToArticleResponse(FirstArticles)
		return handler.PaginateData(c, total, res)
	}
	restArticles, err := dal.MySqlDB.Article.PageFindByNotVisibility(&pageParam, sqlParam)
	if err != nil {
		return err
	}
	articles := FirstArticles
	articles = append(articles, restArticles...)
	res := rtype.MultiModelToArticleResponse(articles)
	return handler.PaginateData(c, total, res)
}

type ArticleRetrieveQueryParam struct {
	NoRead bool `query:"no_read"`
}

func ArticleRetrieve(c *fiber.Ctx) error {
	var article model.Article
	if err := handler.DetailPreHandle(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	afterArticle, err := dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		return err
	}
	res := rtype.ModelToArticleResponse(&afterArticle)
	queryParam := ArticleRetrieveQueryParam{}
	if err = c.QueryParser(&queryParam); err != nil {
		return err
	}
	if !queryParam.NoRead {
		go func() {
			if err := dal.MySqlDB.Article.AddReadCountByID(article.ID, 1); err != nil {
				log.Println(err)
			}
		}()
	}
	return c.JSON(&res)
}

func ArticleCreate(c *fiber.Ctx) error {
	var article model.Article
	var detail model.ArticleDetail
	var r rtype.ArticleRequest
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
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

	article.Tags = getTags(r.TagIDs)

	err = dal.MySqlDB.Article.Create(&article)
	if err != nil {
		return err
	}
	res := rtype.ModelToArticleResponse(&article)
	return c.Status(fiber.StatusCreated).JSON(res)
}

// 更新文章
func ArticleUpdate(c *fiber.Ctx) error {
	var article model.Article
	var r rtype.ArticleRequest
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	// 校验
	if err := handler.DetailPreHandle(c, &article); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if err := validator.ValidateArticleRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	r.SetSummary()
	structAssign(&article, &r)
	article.Detail = &model.ArticleDetail{Content: r.Content}
	tags := getTags(r.TagIDs)
	article.Tags = tags

	if err := dal.MySqlDB.Article.Update(&article); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	//config.Database.Debug().Model(&article).Omit("User").Updates(&article)
	article, err := dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	res := rtype.ModelToArticleResponse(&article)
	return c.Status(fiber.StatusOK).JSON(&res)
}

// 删除文章
func ArticleDelete(c *fiber.Ctx) error {
	var article model.Article
	if err := handler.DetailPreHandle(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if err := dal.MySqlDB.Article.DeleteByID(article.ID); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func getTags(tagIDs []uint) []*model.Tag {
	if len(tagIDs) == 0 {
		return nil
	}
	tags := make([]*model.Tag, len(tagIDs))
	for i, id := range tagIDs {
		tag := &model.Tag{}
		tag.ID = id
		tags[i] = tag
	}
	FindTags(tags)
	return tags
}
