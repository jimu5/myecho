package api

import (
	"github.com/gofiber/fiber/v2"
	"myecho/config/static_config"
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
	return handler.PaginateData(c, pageInfo, res)
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
	if queryParam.Status != nil {
		sqlCommonParam.Status = queryParam.Status
		total, err = dal.MySqlDB.Article.CountAll(sqlCommonParam)
		if err != nil {
			return err
		}
		articles, pageParam, err := handler.PageFind(c, dal.MySqlDB.Article.PageFindByCommonParam, sqlCommonParam)
		if err != nil {
			return err
		}
		pageInfo := mysql.PageInfo{Total: total}
		pageInfo.FillInfoFromParam(&pageParam)
		res := rtype.MultiModelToArticleResponse(articles)
		return handler.PaginateData(c, pageInfo, res)
	}

	total, err = dal.MySqlDB.Article.CountAll(sqlCommonParam)
	if err != nil {
		return err
	}
	topStatus := mysql.ARTICLE_STATUS_TOP
	topSQLParam := sqlCommonParam
	topSQLParam.Status = &topStatus
	topTotal, err := dal.MySqlDB.Article.CountAll(topSQLParam)
	if err != nil {
		return err
	}
	pageParam, err := handler.ParsePageFindParam(c)
	if err != nil {
		return err
	}
	if pageParam.Page < 1 {
		pageParam.Page = 1
	}
	if pageParam.PageSize < 1 {
		pageParam.PageSize = static_config.PageSize
	}
	pageInfo := mysql.PageInfo{Total: total}
	pageInfo.FillInfoFromParam(&pageParam)
	offset := (pageParam.Page - 1) * pageParam.PageSize
	articles := make([]*mysql.ArticleModel, 0, pageParam.PageSize)
	if offset < int(topTotal) {
		topParam := pageParam
		topParam.UseForceOffset = true
		topParam.ForceOffset = offset
		topParam.PageSize = min(pageParam.PageSize, int(topTotal)-offset)
		topArticles, err := dal.MySqlDB.Article.PageFindByCommonParam(&topParam, topSQLParam)
		if err != nil {
			return err
		}
		articles = append(articles, topArticles...)
	}
	restLimit := pageParam.PageSize - len(articles)
	if restLimit > 0 {
		restOffset := offset - int(topTotal)
		if restOffset < 0 {
			restOffset = 0
		}
		restParam := pageParam
		restParam.UseForceOffset = true
		restParam.ForceOffset = restOffset
		restParam.PageSize = restLimit
		sqlParam := mysql.PageFindArticleByNotStatusParam{ArticleCommonQueryParam: topSQLParam}
		restArticles, err := dal.MySqlDB.Article.PageFindByNotVisibility(&restParam, sqlParam)
		if err != nil {
			return err
		}
		articles = append(articles, restArticles...)
	}
	res := rtype.MultiModelToArticleResponse(articles)
	return handler.PaginateData(c, pageInfo, res)
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
	return handler.Success(c, &res)
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
	return handler.SuccessWithStatus(c, fiber.StatusCreated, res)
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
	return handler.Success(c, &res)
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
	return handler.SuccessWithStatus(c, fiber.StatusOK, nil)
}

func getTagsByUID(tagUIDs []string) ([]*model.Tag, error) {
	if len(tagUIDs) == 0 {
		return nil, nil
	}
	return FindTagsByUID(tagUIDs)
}
