package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"github.com/gofiber/fiber/v2"
	"myecho/config/static_config"
	"myecho/dal"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	apierrors "myecho/handler/api/errors"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"
	"myecho/utils"
	"strconv"
	"strings"
	"time"
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
	AuthorID    *uint                `query:"author_id"`
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
	if queryParam.Status == nil {
		recycledStatus := mysql.ARTICLE_STATUS_RECYCLE
		sqlCommonParam.ExcludedStatus = &recycledStatus
	}
	if user := currentUser(c); user != nil && user.PermissionType == model.Author {
		sqlCommonParam.AuthorID = &user.ID
	} else if queryParam.AuthorID != nil {
		sqlCommonParam.AuthorID = queryParam.AuthorID
	}
	if queryParam.Status != nil {
		if *queryParam.Status == mysql.ARTICLE_STATUS_WAIT_REVIEW {
			idsQuery := connect.Database.Model(&model.ArticleSubmission{}).Select("article_id").
				Where("status = ?", model.ArticleSubmissionPending)
			if sqlCommonParam.AuthorID != nil {
				idsQuery = idsQuery.Where("submitter_id = ?", *sqlCommonParam.AuthorID)
			}
			var articleIDs []uint
			if err := idsQuery.Pluck("article_id", &articleIDs).Error; err != nil {
				return err
			}
			sqlCommonParam.ArticleIDs = articleIDs
			sqlCommonParam.Status = nil
		} else {
			sqlCommonParam.Status = queryParam.Status
		}
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
		if err := attachLatestArticleSubmissions(res, articles, currentUser(c)); err != nil {
			return err
		}
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
	if err := attachLatestArticleSubmissions(res, articles, currentUser(c)); err != nil {
		return err
	}
	return handler.PaginateData(c, pageInfo, res)
}

func attachLatestArticleSubmissions(res []*rtype.ArticleResponse, articles []*mysql.ArticleModel, viewer *model.User) error {
	if len(articles) == 0 || viewer == nil {
		return nil
	}
	ids := make([]uint, 0, len(articles))
	for _, article := range articles {
		ids = append(ids, article.ID)
	}
	var submissions []model.ArticleSubmission
	query := connect.Database.Preload("Submitter").Preload("Reviewer").Where("article_id IN ? AND status = ?", ids, model.ArticleSubmissionPending)
	query = query.Or("article_id IN ? AND submitter_id = ? AND status IN ?", ids, viewer.ID, []model.ArticleSubmissionStatus{
		model.ArticleSubmissionDraft, model.ArticleSubmissionRejected,
	})
	if err := query.Order("id desc").Find(&submissions).Error; err != nil {
		return err
	}
	pending := make(map[uint]*model.ArticleSubmission, len(submissions))
	own := make(map[uint]*model.ArticleSubmission, len(submissions))
	for i := range submissions {
		if submissions[i].Status == model.ArticleSubmissionPending {
			if pending[submissions[i].ArticleID] == nil {
				pending[submissions[i].ArticleID] = &submissions[i]
			}
		} else if own[submissions[i].ArticleID] == nil {
			own[submissions[i].ArticleID] = &submissions[i]
		}
	}
	for i, article := range articles {
		var latest *model.ArticleSubmission
		if viewer.CanEditAllContent() || viewer.ID == article.AuthorID {
			latest = pending[article.ID]
		}
		if latest == nil {
			latest = own[article.ID]
		}
		var err error
		res[i].Submission, err = rtype.ModelToArticleSubmissionResponse(latest)
		if err != nil {
			return err
		}
	}
	return nil
}

func ArticleBatch(c *fiber.Ctx) error {
	req := rtype.ArticleBatchReq{}
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if len(req.IDs) == 0 {
		return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
	}
	for _, id := range req.IDs {
		if id == 0 {
			return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
		}
	}
	var actorIDs []uint
	if user := currentUser(c); user != nil {
		actorIDs = append(actorIDs, user.ID)
	}
	switch req.Action {
	case "delete":
		if err := dal.MySqlDB.Article.BatchTrash(req.IDs, actorIDs...); err != nil {
			if stderrors.Is(err, mysql.ErrArticleEditConflict) || stderrors.Is(err, mysql.ErrArticleStatusChanged) {
				return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article state changed; refresh and try again")
			}
			return InternalErrorResponse(c, InternalSQLError, err.Error())
		}
	case "restore":
		if err := dal.MySqlDB.Article.BatchRestoreFromTrash(req.IDs, actorIDs...); err != nil {
			if stderrors.Is(err, mysql.ErrArticleEditConflict) || stderrors.Is(err, mysql.ErrArticleTrashed) ||
				stderrors.Is(err, mysql.ErrArticleNotTrashed) || stderrors.Is(err, mysql.ErrArticleStatusChanged) {
				return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article state changed; refresh and try again")
			}
			return InternalErrorResponse(c, InternalSQLError, err.Error())
		}
	case "permanent_delete":
		if err := dal.MySqlDB.Article.BatchPermanentlyDelete(req.IDs); err != nil {
			if stderrors.Is(err, mysql.ErrArticleEditConflict) || stderrors.Is(err, mysql.ErrArticleNotTrashed) ||
				stderrors.Is(err, mysql.ErrArticleStatusChanged) {
				return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article state changed; refresh and try again")
			}
			return InternalErrorResponse(c, InternalSQLError, err.Error())
		}
	case "status", "update_status", "":
		status := mysql.ArticleStatus(req.Status)
		if status < mysql.ARTILCE_STATUS_PUBLIC || status >= mysql.ARTICLE_STATUS_RECYCLE {
			return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
		}
		var trashed int64
		if err := connect.Database.Model(&model.Article{}).
			Where("id IN ? AND status = ?", req.IDs, int8(mysql.ARTICLE_STATUS_RECYCLE)).
			Count(&trashed).Error; err != nil {
			return err
		}
		if trashed > 0 {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before changing status")
		}
		var pendingReviews int64
		if err := connect.Database.Model(&model.Article{}).
			Where("id IN ? AND status = ?", req.IDs, int8(mysql.ARTICLE_STATUS_WAIT_REVIEW)).
			Count(&pendingReviews).Error; err != nil {
			return err
		}
		if pendingReviews > 0 {
			return ValidateErrorResponse(c, "review pending articles individually")
		}
		if err := dal.MySqlDB.Article.BatchUpdateStatus(req.IDs, status); err != nil {
			if stderrors.Is(err, mysql.ErrArticleTrashed) || stderrors.Is(err, mysql.ErrArticleEditConflict) ||
				stderrors.Is(err, mysql.ErrArticleSubmissionChanged) || stderrors.Is(err, mysql.ErrArticleStatusChanged) {
				return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article state changed; refresh and try again")
			}
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
	queryParam.IncludeNonPublic = canManageArticle(currentUser(c), &article)
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
	if queryParam.IncludeNonPublic {
		user := currentUser(c)
		var submission *model.ArticleSubmission
		if pending, err := service.PendingArticleSubmission(article.ID); err != nil {
			return err
		} else if pending != nil {
			submission = pending
		} else if user != nil {
			submission, err = service.LatestEditableSubmissionForSubmitter(article.ID, user.ID)
			if err != nil {
				return err
			}
		}
		if res.Submission, err = rtype.ModelToArticleSubmissionResponse(submission); err != nil {
			return err
		}
	}
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
	if r.Autosave {
		r.Status = int8(mysql.ARTICLE_STATUS_DRAFT)
	}
	user := handler.GetUserFromCtx(c)
	if err := validateArticleStatusForUser(&r, user); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	key := c.Get("Idempotency-Key")
	if (key == "" && middleware.IsAgentRequest(c)) || len(key) > 128 || strings.IndexFunc(key, func(r rune) bool { return r < 33 || r > 126 }) >= 0 {
		return ParseErrorResponse(c, "Idempotency-Key must contain 1 to 128 printable non-space ASCII characters; required for agent requests")
	}
	digest := sha256.Sum256(c.Body())
	requestHash := hex.EncodeToString(digest[:])
	if response, err := dal.MySqlDB.Article.ReplayCreateRequest(user.ID, key, requestHash); err != nil {
		if stderrors.Is(err, mysql.ErrArticleCreateKeyConflict) || stderrors.Is(err, mysql.ErrArticleCreateGone) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, err.Error())
		}
		return err
	} else if response != nil {
		c.Set("Idempotency-Replayed", "true")
		return c.Status(fiber.StatusCreated).Type("json").Send(response)
	}
	if err := validator.ValidateArticleRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	requestedStatus := r.Status
	needsReview := (isUntrustedAuthor(user) || middleware.IsAgentRequest(c)) && requestedStatus == int8(mysql.ARTICLE_STATUS_WAIT_REVIEW)
	if needsReview {
		r.Status = int8(mysql.ARTICLE_STATUS_DRAFT)
	}
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

	var submission *model.ArticleSubmission
	var review *model.ArticleReview
	if needsReview {
		submission, err = buildArticleSubmission(&article, &r, user.ID, model.ArticleSubmissionPending, int8(mysql.ARTILCE_STATUS_PUBLIC))
		if err != nil {
			return err
		}
		review = newArticleReview(user.ID, model.ArticleReviewSubmit, int8(mysql.ARTICLE_STATUS_DRAFT), int8(mysql.ARTICLE_STATUS_WAIT_REVIEW), "")
	}
	response, replayed, err := dal.MySqlDB.Article.CreateForRequest(&article, submission, review, key, requestHash, func() ([]byte, error) {
		res := rtype.ModelToArticleResponse(&article)
		var err error
		res.Submission, err = rtype.ModelToArticleSubmissionResponse(submission)
		if err != nil {
			return nil, err
		}
		return json.Marshal(handler.GetSuccessCommonResp(res))
	})
	if err != nil {
		if stderrors.Is(err, mysql.ErrArticleCreateKeyConflict) || stderrors.Is(err, mysql.ErrArticleCreateGone) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, err.Error())
		}
		return err
	}
	if replayed {
		c.Set("Idempotency-Replayed", "true")

	}
	return c.Status(fiber.StatusCreated).Type("json").Send(response)
}

// 更新文章
func ArticleUpdate(c *fiber.Ctx) error {
	var article mysql.ArticleModel
	var r rtype.ArticleRequest
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if middleware.IsAgentRequest(c) && r.ExpectedEditVersion == nil {
		return ParseErrorResponse(c, "expected_edit_version is required for agent updates")
	}
	r.PreHandle()
	// 校验
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	user := handler.GetUserFromCtx(c)
	if !canManageArticle(user, &article) {
		return ValidateErrorResponse(c, "forbidden")
	}
	if article.Status == int8(mysql.ARTICLE_STATUS_RECYCLE) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before editing")
	}
	if r.ContentFormat == "" {
		r.ContentFormat = model.NormalizeArticleContentFormat(article.ContentFormat)
	}
	if r.Autosave {
		r.Status = int8(mysql.ARTICLE_STATUS_DRAFT)
	}
	if err := validator.ValidateArticleRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	if err := validateArticleStatusForUser(&r, user); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	originalStatus := article.Status
	latestSubmission, err := service.LatestEditableSubmission(article.ID)
	if err != nil {
		return err
	}
	if latestSubmission != nil && latestSubmission.Status == model.ArticleSubmissionPending {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "已有待审核投稿，请先撤回或等待审核")
	}
	ownSubmission, err := service.LatestEditableSubmissionForSubmitter(article.ID, user.ID)
	if err != nil {
		return err
	}
	if r.ExpectedEditVersion == nil && ownSubmission != nil {
		r.ExpectedEditVersion = &ownSubmission.BaseEditVersion
	}
	originPassword := article.Password
	if ownSubmission != nil && ownSubmission.Status != model.ArticleSubmissionPending && !r.ClearPassword && strings.TrimSpace(r.Password) == "" {
		ownSnapshot, decodeErr := service.DecodeArticleSubmissionSnapshot(ownSubmission)
		if decodeErr != nil {
			return decodeErr
		}
		originPassword = ownSnapshot.PasswordHash
	}
	prepareArticleRequestForSave(&r, user)
	r.SetSummary()
	candidate := article
	structAssign(&candidate, &r)
	if r.IsAllowComment == nil {
		candidate.IsAllowComment = article.IsAllowComment
	}
	if err := setArticlePassword(&candidate, &r, originPassword); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	candidate.Detail = &model.ArticleDetail{Content: r.Content}
	tags, err := getTagsByUID(r.TagUIDs)
	if err != nil {
		return err
	}
	candidate.Tags = tags
	needsReview := (isUntrustedAuthor(user) || middleware.IsAgentRequest(c)) && r.Status == int8(mysql.ARTICLE_STATUS_WAIT_REVIEW)
	needsIsolatedDraft := r.Autosave || (r.Status == int8(mysql.ARTICLE_STATUS_DRAFT) && (isOnlineArticleStatus(originalStatus) || latestSubmission != nil))
	needsSubmission := needsReview || needsIsolatedDraft || (isUntrustedAuthor(user) && (isOnlineArticleStatus(originalStatus) || latestSubmission != nil))
	if needsSubmission {
		targetStatus := originalStatus
		if !isOnlineArticleStatus(targetStatus) {
			targetStatus = int8(mysql.ARTILCE_STATUS_PUBLIC)
		}
		submissionStatus := model.ArticleSubmissionDraft
		if needsReview {
			submissionStatus = model.ArticleSubmissionPending
		}
		submission, err := buildArticleSubmission(&candidate, &r, user.ID, submissionStatus, targetStatus)
		if err != nil {
			return err
		}
		var review *model.ArticleReview
		if submissionStatus == model.ArticleSubmissionPending {
			review = newArticleReview(user.ID, model.ArticleReviewSubmit, originalStatus, int8(mysql.ARTICLE_STATUS_WAIT_REVIEW), "")
		}
		committedVersion, err := dal.MySqlDB.Article.SaveSubmissionForEditing(submission, review, r.ExpectedEditVersion)
		if err != nil {
			if stderrors.Is(err, mysql.ErrArticleEditConflict) {
				return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article changed since it was loaded; refresh and try again")
			}
			if stderrors.Is(err, mysql.ErrArticleTrashed) {
				return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before editing")
			}
			if stderrors.Is(err, mysql.ErrArticleSubmissionChanged) {
				return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "已有待审核投稿，请先撤回或等待审核")
			}
			return err
		}
		updated, err := dal.MySqlDB.Article.FindByID(article.ID)
		if err != nil {
			return err
		}
		updated.EditVersion = committedVersion
		submission.Submitter = user
		res := rtype.ModelToUnlockedArticleResponse(&updated)
		res.Submission, err = rtype.ModelToArticleSubmissionResponse(submission)
		if err != nil {
			return err
		}
		return handler.Success(c, &res)
	}
	article = candidate
	var review *model.ArticleReview
	if article.Status == int8(mysql.ARTICLE_STATUS_WAIT_REVIEW) &&
		(originalStatus != int8(mysql.ARTICLE_STATUS_WAIT_REVIEW) || user.PermissionType == model.Author) {
		review = newArticleReview(user.ID, model.ArticleReviewSubmit, originalStatus, article.Status, "")
	}
	if review != nil {
		err = dal.MySqlDB.Article.UpdateWithReviewForEditing(&article, review, r.ExpectedEditVersion, user.ID)
	} else {
		err = dal.MySqlDB.Article.UpdateForEditing(&article, r.ExpectedEditVersion, user.ID)
	}
	if err != nil {
		if stderrors.Is(err, mysql.ErrArticleEditConflict) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article changed since it was loaded; refresh and try again")
		}
		if stderrors.Is(err, mysql.ErrArticleTrashed) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before editing")
		}
		if stderrors.Is(err, mysql.ErrArticleSubmissionChanged) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "已有待审核投稿，请先撤回或等待审核")
		}
		if stderrors.Is(err, mysql.ErrArticleStatusChanged) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article state changed; refresh and try again")
		}
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	committedVersion := article.EditVersion
	//config.Database.Debug().Model(&article).Omit("User").Updates(&article)
	article, err = dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	article.EditVersion = committedVersion
	res := rtype.ModelToArticleResponse(&article)
	return handler.Success(c, &res)
}

type articleRevisionSummary struct {
	ID        uint        `json:"id"`
	Title     string      `json:"title"`
	Slug      string      `json:"slug"`
	CreatedAt time.Time   `json:"created_at"`
	Actor     *rtype.User `json:"actor"`
}

func ArticleRevisionList(c *fiber.Ctx) error {
	articleID, err := articleIDFromParam(c)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	article, err := dal.MySqlDB.Article.FindByID(articleID)
	if err != nil || !canManageArticle(currentUser(c), &article) {
		return ValidateErrorResponse(c, "forbidden")
	}
	revisions, err := service.S.Article.ArticleRevisions(articleID)
	if err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	result := make([]articleRevisionSummary, 0, len(revisions))
	for _, revision := range revisions {
		result = append(result, articleRevisionSummary{
			ID:        revision.ID,
			Title:     revision.Title,
			Slug:      revision.Slug,
			CreatedAt: revision.CreatedAt,
			Actor:     rtype.ModelToUser(revision.Actor),
		})
	}
	return handler.Success(c, result)
}

func ArticleRevisionRestore(c *fiber.Ctx) error {
	articleID, err := articleIDFromParam(c)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	revisionID, err := strconv.ParseUint(c.Params("revision_id"), 10, 64)
	if err != nil || revisionID == 0 {
		return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
	}
	existing, err := dal.MySqlDB.Article.FindByID(articleID)
	if err != nil || !canManageArticle(currentUser(c), &existing) {
		return ValidateErrorResponse(c, "forbidden")
	}
	if existing.Status == int8(mysql.ARTICLE_STATUS_RECYCLE) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before editing")
	}
	article, submission, err := service.S.Article.RestoreArticleRevisionWithSubmission(articleID, uint(revisionID), currentUser(c).ID)
	if err != nil {
		if stderrors.Is(err, mysql.ErrArticleEditConflict) || stderrors.Is(err, mysql.ErrArticleTrashed) ||
			stderrors.Is(err, mysql.ErrArticleSubmissionChanged) || stderrors.Is(err, mysql.ErrArticleStatusChanged) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article state changed; refresh and try again")
		}
		return NotFoundErrorResponse(c, err.Error())
	}
	response := rtype.ModelToUnlockedArticleResponse(&article)
	if submission != nil {
		submission.Submitter = currentUser(c)
		response.Submission, err = rtype.ModelToArticleSubmissionResponse(submission)
		if err != nil {
			return err
		}
	}
	return handler.Success(c, response)
}

func articleIDFromParam(c *fiber.Ctx) (uint, error) {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, apierrors.ErrInvalidParams
	}
	return uint(id), nil
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
	if !service.IsArticlePubliclyVisible(article.Status, article.PostTime) {
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
	var article mysql.ArticleModel
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if !canManageArticle(currentUser(c), &article) {
		return ValidateErrorResponse(c, "forbidden")
	}
	user := currentUser(c)
	if err := dal.MySqlDB.Article.TrashByID(article.ID, user.ID); err != nil {
		if stderrors.Is(err, mysql.ErrArticleEditConflict) || stderrors.Is(err, mysql.ErrArticleStatusChanged) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article state changed; refresh and try again")
		}
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.SuccessWithStatus(c, fiber.StatusOK, nil)
}

func getTagsByUID(tagUIDs []string) ([]*model.Tag, error) {
	if len(tagUIDs) == 0 {
		return make([]*model.Tag, 0), nil
	}
	return FindTagsByUID(tagUIDs)
}

func currentUser(c *fiber.Ctx) *model.User {
	if user, ok := c.Locals("user").(*model.User); ok {
		return user
	}
	user, _ := middleware.RequestUser(c)
	return user
}

func canManageArticle(user *model.User, article *mysql.ArticleModel) bool {
	if user == nil || article == nil {
		return false
	}
	return user.CanEditAllContent() || (user.PermissionType == model.Author && article.AuthorID == user.ID)
}

func validateArticleStatusForUser(req *rtype.ArticleRequest, user *model.User) error {
	if req.Status == 0 {
		req.Status = int8(mysql.ARTICLE_STATUS_DRAFT)
	}
	status := mysql.ArticleStatus(req.Status)
	if status < mysql.ARTILCE_STATUS_PUBLIC || status > mysql.ARTICLE_STATUS_RECYCLE {
		return apierrors.ErrInvalidParams
	}
	if isUntrustedAuthor(user) &&
		status != mysql.ARTICLE_STATUS_DRAFT && status != mysql.ARTICLE_STATUS_WAIT_REVIEW {
		return stderrors.New("authors can only save drafts or submit for review")
	}
	return nil
}

func isUntrustedAuthor(user *model.User) bool {
	return user != nil && user.PermissionType == model.Author && !user.CanPublishContent()
}

func isOnlineArticleStatus(status int8) bool {
	return status == int8(mysql.ARTILCE_STATUS_PUBLIC) || status == int8(mysql.ARTICLE_STATUS_TOP) || status == int8(mysql.ARTICLE_STATUS_PRIVATE)
}

func buildArticleSubmission(article *mysql.ArticleModel, req *rtype.ArticleRequest, submitterID uint, status model.ArticleSubmissionStatus, targetStatus int8) (*model.ArticleSubmission, error) {
	snapshot := model.ArticleSubmissionSnapshot{
		Title: article.Title, Slug: article.Slug, SEOTitle: article.SEOTitle,
		SEODescription: article.SEODescription, ShareImage: article.ShareImage,
		Type: article.Type, ContentFormat: article.ContentFormat, Summary: article.Summary,
		Content: req.Content, CategoryUID: article.CategoryUID, IsAllowComment: article.IsAllowComment,
		PostTime: article.PostTime, TargetStatus: targetStatus, PasswordHash: article.Password,
		TagUIDs: append([]string(nil), req.TagUIDs...),
	}
	encoded, err := service.EncodeArticleSubmissionSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	submission := &model.ArticleSubmission{
		ArticleID: article.ID, SubmitterID: submitterID, Status: status, Snapshot: encoded,
	}
	if status == model.ArticleSubmissionPending {
		now := time.Now()
		submission.SubmittedAt = &now
	}
	return submission, nil
}

func newArticleReview(actorID uint, action model.ArticleReviewAction, fromStatus, toStatus int8, note string) *model.ArticleReview {
	return &model.ArticleReview{
		ActorID: actorID, Action: action, FromStatus: fromStatus,
		ToStatus: toStatus, Note: strings.TrimSpace(note),
	}
}

func ArticleReviewList(c *fiber.Ctx) error {
	articleID, err := articleIDFromParam(c)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	article, err := dal.MySqlDB.Article.FindByID(articleID)
	if err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if !canManageArticle(currentUser(c), &article) {
		return ValidateErrorResponse(c, "forbidden")
	}
	var reviews []model.ArticleReview
	if err := connect.Database.Preload("Actor").Where("article_id = ?", articleID).
		Order("created_at desc, id desc").Find(&reviews).Error; err != nil {
		return err
	}
	result := make([]articleReviewResponse, 0, len(reviews))
	for _, review := range reviews {
		result = append(result, articleReviewResponse{
			ID: review.ID, CreatedAt: review.CreatedAt, Action: review.Action,
			SubmissionID: review.SubmissionID,
			FromStatus:   review.FromStatus, ToStatus: review.ToStatus, Note: review.Note,
			Actor: rtype.ModelToUser(review.Actor),
		})
	}
	return handler.Success(c, result)
}

type articleReviewResponse struct {
	ID           uint                      `json:"id"`
	CreatedAt    time.Time                 `json:"created_at"`
	Actor        *rtype.User               `json:"actor"`
	SubmissionID uint                      `json:"submission_id"`
	Action       model.ArticleReviewAction `json:"action"`
	FromStatus   int8                      `json:"from_status"`
	ToStatus     int8                      `json:"to_status"`
	Note         string                    `json:"note"`
}

type articleReviewRequest struct {
	Action model.ArticleReviewAction `json:"action"`
	Note   string                    `json:"note"`
}

func ArticleReview(c *fiber.Ctx) error {
	user := currentUser(c)
	if user == nil || !user.CanEditAllContent() {
		return ValidateErrorResponse(c, "forbidden")
	}
	articleID, err := articleIDFromParam(c)
	if err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	article, err := dal.MySqlDB.Article.FindByID(articleID)
	if err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if article.Status == int8(mysql.ARTICLE_STATUS_RECYCLE) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before reviewing")
	}
	var req articleReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	submission, err := service.PendingArticleSubmission(articleID)
	if err != nil {
		return err
	}
	if submission != nil {
		return reviewArticleSubmission(c, user, submission, req)
	}
	return reviewLegacyArticle(c, user, articleID, req)
}

func reviewArticleSubmission(c *fiber.Ctx, user *model.User, submission *model.ArticleSubmission, req articleReviewRequest) error {
	if len([]rune(req.Note)) > 500 {
		return ValidateErrorResponse(c, "review note is too long")
	}
	if req.Action != model.ArticleReviewApprove && req.Action != model.ArticleReviewReject {
		return ValidateErrorResponse(c, "invalid review action")
	}
	if req.Action == model.ArticleReviewReject && strings.TrimSpace(req.Note) == "" {
		return ValidateErrorResponse(c, "rejection note is required")
	}
	article, err := dal.MySqlDB.Article.FindByID(submission.ArticleID)
	if err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if article.Status == int8(mysql.ARTICLE_STATUS_RECYCLE) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before reviewing")
	}
	snapshot, err := service.DecodeArticleSubmissionSnapshot(submission)
	if err != nil {
		return err
	}
	article.Title, article.Slug = snapshot.Title, snapshot.Slug
	article.SEOTitle, article.SEODescription, article.ShareImage = snapshot.SEOTitle, snapshot.SEODescription, snapshot.ShareImage
	article.Type, article.ContentFormat, article.Summary = snapshot.Type, snapshot.ContentFormat, snapshot.Summary
	article.CategoryUID, article.IsAllowComment, article.PostTime = snapshot.CategoryUID, snapshot.IsAllowComment, snapshot.PostTime
	article.Status, article.Password = snapshot.TargetStatus, snapshot.PasswordHash
	article.Detail = &model.ArticleDetail{Content: snapshot.Content}
	tags, err := getTagsByUID(snapshot.TagUIDs)
	if err != nil {
		return err
	}
	if err := dal.MySqlDB.Article.ReviewSubmission(submission.ID, &article, tags, user.ID, req.Action, strings.TrimSpace(req.Note)); err != nil {
		if stderrors.Is(err, mysql.ErrArticleTrashed) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before reviewing")
		}
		if stderrors.Is(err, mysql.ErrArticleEditConflict) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "article state changed; refresh and try again")
		}
		if stderrors.Is(err, mysql.ErrArticleSubmissionChanged) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "投稿状态已变化，请刷新后重试")
		}
		return err
	}

	article, err = dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		return err
	}
	return handler.Success(c, rtype.ModelToUnlockedArticleResponse(&article))
}

func reviewLegacyArticle(c *fiber.Ctx, user *model.User, articleID uint, req articleReviewRequest) error {
	article, err := dal.MySqlDB.Article.FindByID(articleID)
	if err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if article.Status == int8(mysql.ARTICLE_STATUS_RECYCLE) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before reviewing")
	}
	if article.Status != int8(mysql.ARTICLE_STATUS_WAIT_REVIEW) {
		return ValidateErrorResponse(c, "article is not waiting for review")
	}
	if len([]rune(req.Note)) > 500 {
		return ValidateErrorResponse(c, "review note is too long")
	}
	originalStatus := article.Status
	switch req.Action {
	case model.ArticleReviewApprove:
		article.Status = int8(mysql.ARTILCE_STATUS_PUBLIC)
	case model.ArticleReviewReject:
		if strings.TrimSpace(req.Note) == "" {
			return ValidateErrorResponse(c, "rejection note is required")
		}
		article.Status = int8(mysql.ARTICLE_STATUS_DRAFT)
	default:
		return ValidateErrorResponse(c, "invalid review action")
	}
	if err := dal.MySqlDB.Article.Review(&article, newArticleReview(user.ID, req.Action, originalStatus, article.Status, req.Note), originalStatus); err != nil {
		if stderrors.Is(err, mysql.ErrArticleTrashed) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before reviewing")
		}
		if stderrors.Is(err, mysql.ErrArticleSubmissionChanged) {
			return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "已有待审核投稿，请先撤回或等待审核")
		}
		if stderrors.Is(err, mysql.ErrArticleStatusChanged) {
			return ValidateErrorResponse(c, "article review status changed; refresh and try again")
		}
		return err
	}
	article, err = dal.MySqlDB.Article.FindByID(article.ID)
	if err != nil {
		return err
	}
	return handler.Success(c, rtype.ModelToUnlockedArticleResponse(&article))
}
