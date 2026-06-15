package api

import (
	stderrors "errors"
	"github.com/gofiber/fiber/v2"
	"myecho/config/static_config"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	apierrors "myecho/handler/api/errors"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
	"myecho/utils"
	"strings"
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
	Type        *model.ArticleType   `query:"type"`
	Keyword     *string              `query:"keyword"`
	TagUID      *string              `query:"tag_uid"`
	DateFrom    string               `query:"date_from"`
	DateTo      string               `query:"date_to"`
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
	sqlCommonParam, err := service.BuildArticleCommonQueryParam(queryParam.CategoryUID, queryParam.Keyword, queryParam.TagUID, queryParam.DateFrom, queryParam.DateTo)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	sqlCommonParam.Type = queryParam.Type
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

func ArticleBatch(c *fiber.Ctx) error {
	req := rtype.ArticleBatchReq{}
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if len(req.IDs) == 0 {
		return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
	}
	switch req.Action {
	case "delete":
		if err := dal.MySqlDB.Article.BatchDelete(req.IDs); err != nil {
			return InternalErrorResponse(c, InternalSQLError, err.Error())
		}
	case "status", "update_status", "":
		status := mysql.ArticleStatus(req.Status)
		if status < mysql.ARTILCE_STATUS_PUBLIC || status > mysql.ARTICLE_STATUS_RECYCLE {
			return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
		}
		if err := dal.MySqlDB.Article.BatchUpdateStatus(req.IDs, status); err != nil {
			return InternalErrorResponse(c, InternalSQLError, err.Error())
		}
	default:
		return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
	}
	return handler.Success(c, nil)
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
	queryParam.IncludeNonPublic = isAdminRequest(c)
	queryParam.PasswordUnlocked = queryParam.IncludeNonPublic || service.ValidateArticlePasswordToken(&article, c.Cookies(service.ArticlePasswordCookieName(article.ID)))
	article, err = service.S.Article.ArticleRetrieve(&queryParam)
	if err != nil {
		if stderrors.Is(err, service.ErrArticleNotDisplayable) {
			return NotFoundErrorResponse(c, err.Error())
		}
		if stderrors.Is(err, service.ErrArticlePasswordRequired) {
			return ValidateErrorResponse(c, err.Error())
		}
		return err
	}
	res := rtype.ModelToUnlockedArticleResponse(&article)
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
	user := handler.GetUserFromCtx(c)
	prepareArticleRequestForSave(&r, user)
	detail.Content = r.Content
	r.SetSummary()
	structAssign(&article, &r)
	if err := setArticlePassword(&article, &r, ""); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	article.Detail = &detail
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
	if r.ContentFormat == "" {
		r.ContentFormat = model.NormalizeArticleContentFormat(article.ContentFormat)
	}
	if err := validator.ValidateArticleRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	prepareArticleRequestForSave(&r, handler.GetUserFromCtx(c))
	r.SetSummary()
	originPassword := article.Password
	structAssign(&article, &r)
	if err := setArticlePassword(&article, &r, originPassword); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
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

func prepareArticleRequestForSave(req *rtype.ArticleRequest, user *model.User) {
	req.ContentFormat = model.NormalizeArticleContentFormat(req.ContentFormat)
	if req.ContentFormat != model.ArticleContentFormatHTML {
		return
	}
	if user != nil && user.PermissionType == model.Admin {
		return
	}
	req.Content = utils.SanitizeArticleHTML(req.Content)
}

type ArticlePasswordRequest struct {
	Password string `json:"password"`
}

func ArticlePasswordUnlock(c *fiber.Ctx) error {
	var article mysql.ArticleModel
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if !isAPIArticleDisplayable(article.Status) {
		return NotFoundErrorResponse(c, service.ErrArticleNotDisplayable.Error())
	}
	if article.Password == "" {
		return handler.Success(c, nil)
	}
	var req ArticlePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if err := service.CheckArticlePassword(article.Password, req.Password); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	token, err := service.CreateArticlePasswordToken(&article)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	c.Cookie(&fiber.Cookie{
		Name:     service.ArticlePasswordCookieName(article.ID),
		Value:    token,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
	return handler.Success(c, nil)
}

func setArticlePassword(article *mysql.ArticleModel, req *rtype.ArticleRequest, originPassword string) error {
	if req.ClearPassword {
		article.Password = ""
		return nil
	}
	if strings.TrimSpace(req.Password) == "" {
		article.Password = originPassword
		return nil
	}
	hashedPassword, err := service.HashArticlePassword(req.Password)
	if err != nil {
		return err
	}
	article.Password = hashedPassword
	return nil
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

func isAdminRequest(c *fiber.Ctx) bool {
	auth := c.Get("Authorization")
	fields := strings.Fields(auth)
	if len(fields) != 2 || strings.ToLower(fields[0]) != "token" {
		return false
	}
	user, err := middleware.GetUserByToken(fields[1])
	if err != nil {
		return false
	}
	return user.PermissionType == model.Admin
}

func isAPIArticleDisplayable(status int8) bool {
	return status == int8(mysql.ARTILCE_STATUS_PUBLIC) || status == int8(mysql.ARTICLE_STATUS_TOP)
}
