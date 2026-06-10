package api

import (
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	apierrors "myecho/handler/api/errors"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/model"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func CommentCreate(c *fiber.Ctx) error {
	var res rtype.CommentRequest
	var article model.Article
	if err := c.BodyParser(&res); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if err := validator.ValidateCommentRequest(&res); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if !isCommentableArticle(&article) {
		return ValidateErrorResponse(c, apierrors.ErrCommentArticleHidden.Error())
	}
	if article.IsAllowComment != nil && !*article.IsAllowComment {
		return ValidateErrorResponse(c, apierrors.ErrCommentArticleClosed.Error())
	}

	pendingStatus := int8(model.CommentStatusPending)
	var comment model.Comment
	comment.ArticleUID = article.UID
	structAssign(&comment, &res)
	comment.Status = &pendingStatus
	comment.AuthorIP = c.IP()
	comment.AuthorAgent = c.Get(fiber.HeaderUserAgent)
	comment.PostTime = time.Now()
	if c.Locals("user") != nil {
		comment.UserID = c.Locals("user").(*model.User).ID
		comment.AuthorName = c.Locals("user").(*model.User).NickName
		comment.AuthorEmail = c.Locals("user").(*model.User).Email
	}
	if err := connect.Database.Save(&comment).Error; err != nil {
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusCreated, &comment)
}

// 更新评论
func CommentUpdate(c *fiber.Ctx) error {
	var r rtype.CommentUpdateReq
	// 校验
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error()) // 使用统一的错误处理
	}

	var comment model.Comment
	if err := handler.DetailPreHandleByParam(c, &comment); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if r.Status != nil {
		if err := validator.ValidateCommentStatus(*r.Status); err != nil {
			return ValidateErrorResponse(c, err.Error())
		}
		status := int8(*r.Status)
		comment.Status = &status
	}
	if r.AuthorName != "" {
		comment.AuthorName = r.AuthorName
	}
	if r.AuthorEmail != "" {
		comment.AuthorEmail = r.AuthorEmail
	}
	if r.AuthorUrl != "" {
		comment.AuthorUrl = r.AuthorUrl
	}
	if r.Content != "" {
		comment.Content = r.Content
	}
	if r.ParentID != 0 {
		if err := validator.ValidateParentCommentID(r.ParentID); err != nil {
			return ValidateErrorResponse(c, err.Error())
		}
		comment.ParentID = r.ParentID
	}
	if err := connect.Database.Model(&comment).Updates(&comment).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.Success(c, &comment)
}

// 获取文章评论
func ArticleCommentList(c *fiber.Ctx) error {
	var comments []rtype.CommentResponse
	var article model.Article
	// 校验
	if err := handler.DetailPreHandleByParam(c, &article); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if err := connect.Database.Table("comments").
		Where("article_uid = ?", article.UID).
		Where(publicCommentStatusSQL()).
		Order("post_time asc, created_at asc").
		Find(&comments).Error; err != nil {
		return err
	}
	return handler.Success(c, comments)
}

func CommentAllList(c *fiber.Ctx) error {
	queryParam := rtype.CommentListQueryParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	if queryParam.Status != nil {
		if err := validator.ValidateCommentStatus(*queryParam.Status); err != nil {
			return ValidateErrorResponse(c, err.Error())
		}
	}
	pageParam, err := handler.ParsePageFindParam(c)
	if err != nil {
		return err
	}
	query, err := buildAdminCommentQuery(&queryParam)
	if err != nil {
		return err
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return err
	}
	comments := make([]model.Comment, 0, pageParam.PageSize)
	if err := query.Scopes(mysql.Paginate(&pageParam)).
		Order("created_at desc").
		Find(&comments).Error; err != nil {
		return err
	}
	pageInfo := mysql.PageInfo{Total: total}
	pageInfo.FillInfoFromParam(&pageParam)
	return handler.PaginateData(c, pageInfo, buildAdminCommentResponses(comments))
}

func CommentDelete(c *fiber.Ctx) error {
	id, err := handler.GetIDByParam(c, &model.Comment{})
	if err != nil {
		return err
	}
	if err := connect.Database.Delete(&model.Comment{}, id).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.Success(c, nil)
}

func CommentBatch(c *fiber.Ctx) error {
	req := rtype.CommentBatchReq{}
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if len(req.IDs) == 0 {
		return ValidateErrorResponse(c, apierrors.ErrCommentBatchEmpty.Error())
	}
	switch req.Action {
	case "delete":
		if err := connect.Database.Delete(&model.Comment{}, req.IDs).Error; err != nil {
			return InternalErrorResponse(c, InternalSQLError, err.Error())
		}
	case "approve":
		req.Status = model.CommentStatusApproved
		fallthrough
	case "reject":
		if req.Action == "reject" {
			req.Status = model.CommentStatusRejected
		}
		fallthrough
	case "spam":
		if req.Action == "spam" {
			req.Status = model.CommentStatusSpam
		}
		fallthrough
	case "pending":
		if req.Action == "pending" {
			req.Status = model.CommentStatusPending
		}
		fallthrough
	case "status", "update_status", "":
		if err := validator.ValidateCommentStatus(req.Status); err != nil {
			return ValidateErrorResponse(c, err.Error())
		}
		if err := connect.Database.Model(&model.Comment{}).
			Where("id in ?", req.IDs).
			Update("status", int8(req.Status)).Error; err != nil {
			return InternalErrorResponse(c, InternalSQLError, err.Error())
		}
	default:
		return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
	}
	return handler.Success(c, nil)
}

func isCommentableArticle(article *model.Article) bool {
	return article.Status == int8(mysql.ARTILCE_STATUS_PUBLIC) || article.Status == int8(mysql.ARTICLE_STATUS_TOP)
}

func publicCommentStatusSQL() *gorm.DB {
	return connect.Database.Where("status IS NULL OR status = ? OR status = ?", int8(model.CommentStatusLegacyApproved), int8(model.CommentStatusApproved))
}

func buildAdminCommentQuery(param *rtype.CommentListQueryParam) (*gorm.DB, error) {
	query := connect.Database.Model(&model.Comment{})
	if param.ArticleID != nil && *param.ArticleID != 0 {
		var article model.Article
		if err := connect.Database.First(&article, *param.ArticleID).Error; err != nil {
			return nil, err
		}
		query = query.Where("article_uid = ?", article.UID)
	}
	if param.ArticleUID != nil && *param.ArticleUID != "" {
		query = query.Where("article_uid = ?", *param.ArticleUID)
	}
	if param.Status != nil {
		if *param.Status == model.CommentStatusApproved {
			query = query.Where("status IS NULL OR status = ? OR status = ?", int8(model.CommentStatusLegacyApproved), int8(model.CommentStatusApproved))
		} else {
			query = query.Where("status = ?", int8(*param.Status))
		}
	}
	return query, nil
}

func buildAdminCommentResponses(comments []model.Comment) []rtype.CommentAdminResponse {
	articleUIDs := make([]string, 0, len(comments))
	seen := make(map[string]struct{}, len(comments))
	for _, comment := range comments {
		if comment.ArticleUID == "" {
			continue
		}
		if _, ok := seen[comment.ArticleUID]; ok {
			continue
		}
		seen[comment.ArticleUID] = struct{}{}
		articleUIDs = append(articleUIDs, comment.ArticleUID)
	}
	articleMap := make(map[string]model.Article, len(articleUIDs))
	if len(articleUIDs) > 0 {
		var articles []model.Article
		_ = connect.Database.Where("uid in ?", articleUIDs).Find(&articles).Error
		for _, article := range articles {
			articleMap[article.UID] = article
		}
	}
	responses := make([]rtype.CommentAdminResponse, 0, len(comments))
	for _, comment := range comments {
		article := articleMap[comment.ArticleUID]
		responses = append(responses, rtype.CommentAdminResponse{
			ID:           comment.ID,
			ArticleUID:   comment.ArticleUID,
			ArticleID:    article.ID,
			ArticleTitle: article.Title,
			AuthorName:   comment.AuthorName,
			AuthorEmail:  comment.AuthorEmail,
			AuthorIP:     comment.AuthorIP,
			AuthorUrl:    comment.AuthorUrl,
			AuthorAgent:  comment.AuthorAgent,
			Content:      comment.Content,
			Status:       normalizeCommentStatus(comment.Status),
			LikeCount:    comment.LikeCount,
			ParentID:     comment.ParentID,
			UserID:       comment.UserID,
			PostTime:     comment.PostTime,
			CreatedAt:    comment.CreatedAt,
			UpdatedAt:    comment.UpdatedAt,
		})
	}
	return responses
}

func normalizeCommentStatus(status *int8) int8 {
	if status == nil || *status == int8(model.CommentStatusLegacyApproved) {
		return int8(model.CommentStatusApproved)
	}
	return *status
}
