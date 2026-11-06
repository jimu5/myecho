package api

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
)

type articleSubmissionWorkbenchResponse struct {
	Submission *rtype.ArticleSubmissionResponse `json:"submission"`
	Article    *rtype.ArticleResponse           `json:"article"`
}

func ArticleSubmissionList(c *fiber.Ctx) error {
	param, err := handler.ParsePageFindParam(c)
	if err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	query := connect.Database.Model(&model.ArticleSubmission{})
	status := model.ArticleSubmissionPending
	if raw := c.Query("status"); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 8)
		if parseErr != nil || parsed < int64(model.ArticleSubmissionDraft) || parsed > int64(model.ArticleSubmissionWithdrawn) {
			return ValidateErrorResponse(c, "invalid submission status")
		}
		status = model.ArticleSubmissionStatus(parsed)
	}
	query = query.Where("status = ?", status)
	if raw := c.Query("author_id"); raw != "" {
		authorID, parseErr := strconv.ParseUint(raw, 10, 64)
		if parseErr != nil || authorID == 0 {
			return ValidateErrorResponse(c, "invalid author")
		}
		query = query.Where("submitter_id = ?", uint(authorID))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return err
	}
	var submissions []model.ArticleSubmission
	if err := query.Preload("Submitter").Preload("Reviewer").Scopes(mysql.Paginate(&param)).
		Order("submitted_at asc, id asc").Find(&submissions).Error; err != nil {
		return err
	}
	result := make([]articleSubmissionWorkbenchResponse, 0, len(submissions))
	for i := range submissions {
		item, err := articleSubmissionWorkbench(&submissions[i])
		if err != nil {
			return err
		}
		result = append(result, item)
	}
	pageInfo := mysql.PageInfo{Total: total}
	pageInfo.FillInfoFromParam(&param)
	return handler.PaginateData(c, pageInfo, result)
}

func ArticleSubmissionAuthorList(c *fiber.Ctx) error {
	var users []model.User
	if err := connect.Database.Model(&model.User{}).
		Select("users.id", "users.nick_name", "users.slug", "users.avatar_url").
		Joins("JOIN article_submissions ON article_submissions.submitter_id = users.id").
		Where("article_submissions.status = ?", model.ArticleSubmissionPending).
		Group("users.id").Order("users.nick_name, users.id").Find(&users).Error; err != nil {
		return err
	}
	result := make([]*rtype.User, 0, len(users))
	for i := range users {
		result = append(result, rtype.ModelToUser(&users[i]))
	}
	return handler.Success(c, result)
}

func ArticleSubmissionRetrieve(c *fiber.Ctx) error {
	submission, err := articleSubmissionFromParam(c)
	if err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	user := currentUser(c)
	if user == nil || (!user.CanEditAllContent() && submission.SubmitterID != user.ID) {
		return ValidateErrorResponse(c, "forbidden")
	}
	result, err := articleSubmissionWorkbench(submission)
	if err != nil {
		return err
	}
	return handler.Success(c, result)
}

func ArticleSubmissionReview(c *fiber.Ctx) error {
	user := currentUser(c)
	if user == nil || !user.CanEditAllContent() {
		return ValidateErrorResponse(c, "forbidden")
	}
	submission, err := articleSubmissionFromParam(c)
	if err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	var req articleReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	return reviewArticleSubmission(c, user, submission, req)
}

func ArticleSubmissionWithdraw(c *fiber.Ctx) error {
	user := currentUser(c)
	if user == nil {
		return ValidateErrorResponse(c, "forbidden")
	}
	submission, err := articleSubmissionFromParam(c)
	if err != nil {
		return NotFoundErrorResponse(c, err.Error())
	}
	if !user.CanEditAllContent() && submission.SubmitterID != user.ID {
		return ValidateErrorResponse(c, "forbidden")
	}
	if submission.Status != model.ArticleSubmissionPending {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "投稿已不在待审核状态")
	}
	now := time.Now()
	err = connect.Database.Transaction(func(tx *gorm.DB) error {
		var article model.Article
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status", "edit_version").First(&article, submission.ArticleID).Error; err != nil {
			return err
		}
		if article.Status == int8(mysql.ARTICLE_STATUS_RECYCLE) {
			return mysql.ErrArticleTrashed
		}
		version, err := mysql.ClaimArticleEditVersion(tx, submission.ArticleID, nil)
		if err != nil {
			return err
		}
		updates := map[string]interface{}{"status": model.ArticleSubmissionDraft, "reviewed_at": now}
		if submission.BaseEditVersion != 0 && submission.BaseEditVersion == article.EditVersion {
			updates["base_edit_version"] = version
		}
		result := tx.Model(&model.ArticleSubmission{}).
			Where("id = ? AND status = ?", submission.ID, model.ArticleSubmissionPending).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return mysql.ErrArticleSubmissionChanged
		}
		return tx.Create(&model.ArticleReview{
			ArticleID: submission.ArticleID, SubmissionID: submission.ID, ActorID: user.ID,
			Action: model.ArticleReviewWithdraw, FromStatus: int8(mysql.ARTICLE_STATUS_WAIT_REVIEW), ToStatus: int8(mysql.ARTICLE_STATUS_DRAFT),
		}).Error
	})
	if errors.Is(err, mysql.ErrArticleTrashed) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "restore trashed articles before withdrawing a submission")
	}
	if errors.Is(err, mysql.ErrArticleSubmissionChanged) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, "投稿状态已变化，请刷新后重试")
	}
	if err != nil {
		return err
	}
	return handler.Success(c, nil)
}

func articleSubmissionFromParam(c *fiber.Ctx) (*model.ArticleSubmission, error) {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return nil, errors.New("invalid submission")
	}
	var submission model.ArticleSubmission
	err = connect.Database.Preload("Submitter").Preload("Reviewer").First(&submission, uint(id)).Error
	return &submission, err
}

func articleSubmissionWorkbench(submission *model.ArticleSubmission) (articleSubmissionWorkbenchResponse, error) {
	response, err := rtype.ModelToArticleSubmissionResponse(submission)
	if err != nil {
		return articleSubmissionWorkbenchResponse{}, err
	}
	article, err := service.S.Article.ArticleRetrieve(&service.ArticleRetrieveQueryParam{ID: submission.ArticleID, NoRead: true, IncludeNonPublic: true, PasswordUnlocked: true})
	if err != nil {
		return articleSubmissionWorkbenchResponse{}, err
	}
	return articleSubmissionWorkbenchResponse{Submission: response, Article: rtype.ModelToUnlockedArticleResponse(&article)}, nil
}
