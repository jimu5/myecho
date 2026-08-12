package api

import (
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	apierrors "myecho/handler/api/errors"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

var commentRateLimiter = newCommentRateLimiter(30, time.Minute, 10_000)

type commentRateState struct {
	Count       int
	WindowStart time.Time
}

type commentRateLimit struct {
	sync.Mutex
	limit      int
	window     time.Duration
	maxEntries int
	hits       map[string]commentRateState
}

func newCommentRateLimiter(limit int, window time.Duration, maxEntries int) *commentRateLimit {
	if maxEntries < 1 {
		maxEntries = 1
	}
	return &commentRateLimit{limit: limit, window: window, maxEntries: maxEntries, hits: make(map[string]commentRateState)}
}

func (r *commentRateLimit) allow(key string, now time.Time) bool {
	r.Lock()
	defer r.Unlock()
	state, exists := r.hits[key]
	if !exists && len(r.hits) >= r.maxEntries {
		for existingKey, existing := range r.hits {
			if now.Sub(existing.WindowStart) > r.window {
				delete(r.hits, existingKey)
			}
		}
		// ponytail: fixed in-memory cap fits one process; use a shared limiter if deployments become distributed.
		if len(r.hits) >= r.maxEntries {
			return false
		}
	}
	if state.WindowStart.IsZero() || now.Sub(state.WindowStart) > r.window {
		r.hits[key] = commentRateState{Count: 1, WindowStart: now}
		return true
	}
	if state.Count >= r.limit {
		return false
	}
	state.Count++
	r.hits[key] = state
	return true
}

func CommentCreate(c *fiber.Ctx) error {
	var res rtype.CommentRequest
	var article model.Article
	if err := c.BodyParser(&res); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if strings.TrimSpace(res.Trap) != "" {
		return handler.SuccessWithStatus(c, fiber.StatusCreated, nil)
	}
	if !commentRateLimiter.allow(c.IP(), time.Now()) {
		return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
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
	if err := validator.ValidateParentCommentForArticle(res.ParentID, article.UID); err != nil {
		return ValidateErrorResponse(c, err.Error())
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
	_ = refreshApprovedCommentCount(article.UID)
	go func() {
		_ = service.NotifyPendingComment(&article, &comment)
	}()
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
	articleUID := comment.ArticleUID
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
		if err := validator.ValidateParentCommentForArticle(r.ParentID, comment.ArticleUID); err != nil {
			return ValidateErrorResponse(c, err.Error())
		}
		comment.ParentID = r.ParentID
	}
	if err := connect.Database.Model(&comment).Updates(&comment).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	_ = refreshApprovedCommentCount(articleUID)
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
	if !isCommentableArticle(&article) {
		return NotFoundErrorResponse(c, service.ErrArticleNotDisplayable.Error())
	}
	articleModel := mysql.ArticleModel(article)
	if article.Password != "" && !service.ValidateArticlePasswordToken(&articleModel, c.Cookies(service.ArticlePasswordCookieName(article.ID))) {
		return ValidateErrorResponse(c, service.ErrArticlePasswordRequired.Error())
	}
	if err := connect.Database.Table("comments").
		Where("article_uid = ?", article.UID).
		Where(publicCommentStatusSQL()).
		Order("post_time asc, created_at asc").
		Find(&comments).Error; err != nil {
		return err
	}
	return handler.Success(c, BuildCommentTree(comments))
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
		return ValidateErrorResponse(c, err.Error())
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
	var comment model.Comment
	if err := connect.Database.First(&comment, id).Error; err != nil {
		return err
	}
	if err := connect.Database.Delete(&model.Comment{}, id).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	_ = refreshApprovedCommentCount(comment.ArticleUID)
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
	articleUIDs, err := commentArticleUIDs(req.IDs)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	switch req.Action {
	case "delete":
		if err := connect.Database.Delete(&model.Comment{}, req.IDs).Error; err != nil {
			return InternalErrorResponse(c, InternalSQLError, err.Error())
		}
		refreshApprovedCommentCounts(articleUIDs)
		return handler.Success(c, nil)
	case "approve":
		req.Status = model.CommentStatusApproved
	case "reject":
		req.Status = model.CommentStatusRejected
	case "spam":
		req.Status = model.CommentStatusSpam
	case "pending":
		req.Status = model.CommentStatusPending
	case "status", "update_status", "":
	default:
		return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
	}
	if err := validator.ValidateCommentStatus(req.Status); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if err := connect.Database.Model(&model.Comment{}).
		Where("id in ?", req.IDs).
		Update("status", int8(req.Status)).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	refreshApprovedCommentCounts(articleUIDs)
	return handler.Success(c, nil)
}

func CommentReply(c *fiber.Ctx) error {
	var req rtype.CommentReplyReq
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return ValidateErrorResponse(c, apierrors.ErrCommentContentEmpty.Error())
	}
	if len([]rune(req.Content)) > 2000 {
		return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
	}
	var parent model.Comment
	if err := handler.DetailPreHandleByParam(c, &parent); err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	user := handler.GetUserFromCtx(c)
	authorName := strings.TrimSpace(user.NickName)
	if authorName == "" {
		authorName = user.Name
	}
	status := int8(model.CommentStatusApproved)
	reply := model.Comment{
		ArticleUID:  parent.ArticleUID,
		AuthorName:  authorName,
		AuthorEmail: user.Email,
		Content:     req.Content,
		Status:      &status,
		ParentID:    parent.ID,
		UserID:      user.ID,
		PostTime:    time.Now(),
	}
	if err := connect.Database.Create(&reply).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	_ = refreshApprovedCommentCount(parent.ArticleUID)
	response := buildAdminCommentResponses([]model.Comment{reply})
	return handler.SuccessWithStatus(c, fiber.StatusCreated, &response[0])
}

func isCommentableArticle(article *model.Article) bool {
	return service.IsArticlePubliclyVisible(article.Status, article.PostTime)
}

func publicCommentStatusSQL() *gorm.DB {
	return connect.Database.Where("status IS NULL OR status = ? OR status = ?", int8(model.CommentStatusLegacyApproved), int8(model.CommentStatusApproved))
}

func buildAdminCommentQuery(param *rtype.CommentListQueryParam) (*gorm.DB, error) {
	query := connect.Database.Model(&model.Comment{})
	if param.ArticleID != nil && *param.ArticleID != 0 {
		query = query.Where("article_uid IN (?)", connect.Database.Model(&model.Article{}).Select("uid").Where("id = ?", *param.ArticleID))
	}
	if param.ArticleUID != nil && *param.ArticleUID != "" {
		query = query.Where("article_uid = ?", *param.ArticleUID)
	}
	if keyword := strings.TrimSpace(param.Keyword); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("content LIKE ? OR author_name LIKE ? OR author_email LIKE ?", pattern, pattern, pattern)
	}
	if param.DateFrom != "" {
		from, err := time.ParseInLocation("2006-01-02", param.DateFrom, time.Local)
		if err != nil {
			return nil, apierrors.ErrInvalidParams
		}
		query = query.Where("created_at >= ?", from)
	}
	if param.DateTo != "" {
		to, err := time.ParseInLocation("2006-01-02", param.DateTo, time.Local)
		if err != nil {
			return nil, apierrors.ErrInvalidParams
		}
		query = query.Where("created_at < ?", to.AddDate(0, 0, 1))
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

func BuildCommentTree(comments []rtype.CommentResponse) []rtype.CommentResponse {
	index := make(map[uint]*rtype.CommentResponse, len(comments))
	children := make(map[uint][]*rtype.CommentResponse, len(comments))
	roots := make([]*rtype.CommentResponse, 0, len(comments))
	for i := range comments {
		comments[i].Replies = nil
		index[comments[i].ID] = &comments[i]
	}
	for i := range comments {
		comment := &comments[i]
		if comment.ParentID != 0 {
			if _, ok := index[comment.ParentID]; ok {
				children[comment.ParentID] = append(children[comment.ParentID], comment)
				continue
			}
		}
		roots = append(roots, comment)
	}
	var cloneWithReplies func(*rtype.CommentResponse) rtype.CommentResponse
	cloneWithReplies = func(comment *rtype.CommentResponse) rtype.CommentResponse {
		cloned := *comment
		cloned.Replies = make([]rtype.CommentResponse, 0, len(children[comment.ID]))
		for _, child := range children[comment.ID] {
			cloned.Replies = append(cloned.Replies, cloneWithReplies(child))
		}
		if len(cloned.Replies) == 0 {
			cloned.Replies = nil
		}
		return cloned
	}
	tree := make([]rtype.CommentResponse, 0, len(roots))
	for _, root := range roots {
		tree = append(tree, cloneWithReplies(root))
	}
	return tree
}

func commentArticleUIDs(ids []uint) ([]string, error) {
	comments := make([]model.Comment, 0, len(ids))
	if err := connect.Database.Where("id in ?", ids).Find(&comments).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(comments))
	result := make([]string, 0, len(comments))
	for _, comment := range comments {
		if comment.ArticleUID == "" {
			continue
		}
		if _, ok := seen[comment.ArticleUID]; ok {
			continue
		}
		seen[comment.ArticleUID] = struct{}{}
		result = append(result, comment.ArticleUID)
	}
	return result, nil
}

func refreshApprovedCommentCounts(articleUIDs []string) {
	for _, articleUID := range articleUIDs {
		_ = refreshApprovedCommentCount(articleUID)
	}
}

func refreshApprovedCommentCount(articleUID string) error {
	if articleUID == "" {
		return nil
	}
	var count int64
	if err := connect.Database.Model(&model.Comment{}).
		Where("article_uid = ?", articleUID).
		Where(publicCommentStatusSQL()).
		Count(&count).Error; err != nil {
		return err
	}
	return connect.Database.Model(&model.Article{}).
		Where("uid = ?", articleUID).
		Update("comment_count", uint(count)).Error
}
