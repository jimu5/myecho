package mysql

import (
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myecho/model"
	"myecho/utils"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type ArticleDBRepo struct {
}

var ErrArticleStatusChanged = errors.New("article status changed")
var ErrArticleSubmissionChanged = errors.New("article submission changed")
var ErrArticleEditConflict = errors.New("article edit version changed")
var ErrArticleTrashed = errors.New("article is in trash")

type ArticleModel model.Article

const articleRevisionLimit = 20

func (ArticleModel) TableName() string {
	return "articles"
}
func (article *ArticleModel) BeforeCreate(tx *gorm.DB) error {
	if article.EditVersion == 0 {
		article.EditVersion = 1
	}
	if len(article.UID) == 0 {
		article.UID = utils.GenUID20()
	}
	if err := article.prepareArticleFields(tx); err != nil {
		return err
	}
	if article.Detail != nil {
		if len(article.Detail.UID) == 0 {
			uid := utils.GenUID20()
			article.Detail.UID = article.UID + "_" + uid
		}
	}
	// TODO: 根据文章内容生成统计信息 https://github.com/mdigger/goldmark-stats info.Chars, info.Duration(400), 使用协程加版本锁
	return nil
}

func (article *ArticleModel) AfterCreate(tx *gorm.DB) error {
	if err := article.AddCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (article *ArticleModel) BeforeUpdate(tx *gorm.DB) error {
	if article.ID == 0 {
		return nil
	}
	if err := article.prepareArticleFields(tx); err != nil {
		return err
	}
	if err := article.ReduceCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (article *ArticleModel) prepareArticleFields(tx *gorm.DB) error {
	if article.Type == 0 {
		article.Type = model.ArticleTypePost
	}
	if !model.IsValidArticleType(article.Type) {
		article.Type = model.ArticleTypePost
	}
	article.ContentFormat = model.NormalizeArticleContentFormat(article.ContentFormat)
	if !model.IsValidArticleContentFormat(article.ContentFormat) {
		article.ContentFormat = model.ArticleContentFormatMarkdown
	}
	baseSlug := normalizeArticleSlug(article.Slug)
	if baseSlug == "" {
		baseSlug = normalizeArticleSlug(article.Title)
	}
	if baseSlug == "" {
		baseSlug = "post-" + article.UID
	}
	slug, err := uniqueArticleSlug(tx, baseSlug, article.Type, article.ID)
	if err != nil {
		return err
	}
	article.Slug = slug
	return nil
}

func normalizeArticleSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func uniqueArticleSlug(tx *gorm.DB, base string, articleType model.ArticleType, excludeID uint) (string, error) {
	slug := base
	for i := 2; ; i++ {
		var count int64
		query := tx.Model(&ArticleModel{}).Where("slug = ? AND type = ?", slug, articleType)
		if excludeID != 0 {
			query = query.Where("id <> ?", excludeID)
		}
		if err := query.Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			redirectQuery := tx.Model(&model.ArticleSlugRedirect{}).
				Where("slug = ? AND type = ?", slug, articleType)
			if excludeID != 0 {
				redirectQuery = redirectQuery.Where("article_uid <> (SELECT uid FROM articles WHERE id = ?)", excludeID)
			}
			if err := redirectQuery.Count(&count).Error; err != nil {
				return "", err
			}
		}
		if count == 0 {
			return slug, nil
		}
		slug = base + "-" + strconv.Itoa(i)
	}
}

func (article *ArticleModel) AfterUpdate(tx *gorm.DB) error {
	if article.ID == 0 {
		return nil
	}
	if err := article.AddCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (article *ArticleModel) BeforeDelete(tx *gorm.DB) error {
	if err := article.ReduceCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (article *ArticleModel) AddCategoryCount(tx *gorm.DB) error {
	if isCategoryCountedArticle(article.Status, article.Type) && len(article.CategoryUID) != 0 {
		return tx.Model(&CategoryModel{}).Where("uid = ?", article.CategoryUID).Update("count", gorm.Expr("count + 1")).Error
	}
	return nil
}

func (article *ArticleModel) ReduceCategoryCount(tx *gorm.DB) error {
	oldArticle, err := articleRepo.TXGet(tx, article.ID)
	if err != nil {
		return err
	}
	if isCategoryCountedArticle(oldArticle.Status, oldArticle.Type) && len(oldArticle.CategoryUID) != 0 {
		return tx.Model(&CategoryModel{}).Where("uid = ?", oldArticle.CategoryUID).
			Update("count", gorm.Expr("CASE WHEN count > 0 THEN count - 1 ELSE 0 END")).Error
	}
	return nil
}

func isCategoryCountedArticle(status int8, articleType model.ArticleType) bool {
	return articleType == model.ArticleTypePost &&
		(status == int8(ARTILCE_STATUS_PUBLIC) || status == int8(ARTICLE_STATUS_TOP))
}

func lockArticleCategories(tx *gorm.DB, categoryUIDs ...string) error {
	uniqueUIDs := make(map[string]struct{}, len(categoryUIDs))
	uids := make([]string, 0, len(categoryUIDs))
	for _, uid := range categoryUIDs {
		if uid == "" {
			continue
		}
		if _, exists := uniqueUIDs[uid]; exists {
			continue
		}
		uniqueUIDs[uid] = struct{}{}
		uids = append(uids, uid)
	}
	sort.Strings(uids)
	for _, uid := range uids {
		var category CategoryModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("uid").Where("uid = ?", uid).First(&category).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type (
	ArticleStatus int8
)

const (
	ARTILCE_STATUS_PUBLIC ArticleStatus = iota + 1
	ARTICLE_STATUS_TOP
	ARTICLE_STATUS_PRIVATE
	ARTICLE_STATUS_DRAFT
	ARTICLE_STATUS_WAIT_REVIEW
	ARTICLE_STATUS_RECYCLE
)

type ArticleCommonQueryParam struct {
	AuthorID       *uint
	ArticleIDs     []uint
	CategoryUID    *string
	Status         *ArticleStatus
	ExcludedStatus *ArticleStatus
	Type           *model.ArticleType
	Keyword        *string
	TagUID         *string
	DateFrom       *time.Time
	DateTo         *time.Time
}

type PageFindArticleByNotStatusParam struct {
	ArticleCommonQueryParam
}

func (a *ArticleDBRepo) preCreateQuerySQL(db *gorm.DB, param ArticleCommonQueryParam) (*gorm.DB, error) {
	SqlPrefix := make([]string, 0)
	SqlValue := make([]interface{}, 0)
	if param.AuthorID != nil {
		SqlPrefix = append(SqlPrefix, "author_id = ?")
		SqlValue = append(SqlValue, *param.AuthorID)
	}
	if param.ArticleIDs != nil {
		SqlPrefix = append(SqlPrefix, "id IN ?")
		SqlValue = append(SqlValue, param.ArticleIDs)
	}
	if param.TagUID != nil && len(*param.TagUID) != 0 {
		db = db.Joins("JOIN article_tags ON article_tags.article_uid = articles.uid AND article_tags.tag_uid = ?", *param.TagUID)
	}
	if param.CategoryUID != nil && len(*param.CategoryUID) != 0 {
		sql := "category_uid in (?)"
		allUID := make([]string, 0)
		allUID = append(allUID, *param.CategoryUID)
		fatherUIDs, err := categoryRepo.GetAllChildrenUID(*param.CategoryUID)
		if err != nil {
			return nil, err
		}
		allUID = append(allUID, fatherUIDs...)
		SqlPrefix = append(SqlPrefix, sql)
		SqlValue = append(SqlValue, allUID)
	}
	if param.Status != nil {
		sql := "status = ?"
		SqlPrefix = append(SqlPrefix, sql)
		SqlValue = append(SqlValue, *param.Status)
	}
	if param.ExcludedStatus != nil {
		SqlPrefix = append(SqlPrefix, "status <> ?")
		SqlValue = append(SqlValue, *param.ExcludedStatus)
	}
	if param.Type != nil {
		sql := "type = ?"
		SqlPrefix = append(SqlPrefix, sql)
		SqlValue = append(SqlValue, *param.Type)
	}
	if param.Keyword != nil && strings.TrimSpace(*param.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(*param.Keyword) + "%"
		SqlPrefix = append(SqlPrefix, "(title LIKE ? OR summary LIKE ? OR EXISTS (SELECT 1 FROM article_details WHERE article_details.uid = articles.detail_uid AND article_details.content LIKE ?))")
		SqlValue = append(SqlValue, keyword, keyword, keyword)
	}
	if param.DateFrom != nil {
		SqlPrefix = append(SqlPrefix, "post_time >= ?")
		SqlValue = append(SqlValue, *param.DateFrom)
	}
	if param.DateTo != nil {
		SqlPrefix = append(SqlPrefix, "post_time <= ?")
		SqlValue = append(SqlValue, *param.DateTo)
	}
	return db.Where(strings.Join(SqlPrefix, queryAND), SqlValue...), nil
}

func (a *ArticleDBRepo) Create(article *ArticleModel) error {
	return db.Model(&ArticleModel{}).Create(article).Error
}

func (a *ArticleDBRepo) CreateWithReview(article *ArticleModel, review *model.ArticleReview) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ArticleModel{}).Create(article).Error; err != nil {
			return err
		}
		review.ArticleID = article.ID
		return tx.Create(review).Error
	})
}

func (a *ArticleDBRepo) TXGet(tx *gorm.DB, id uint) (ArticleModel, error) {
	var oldArticle ArticleModel
	err := tx.Model(&ArticleModel{}).Where("id = ?", id).First(&oldArticle).Error
	return oldArticle, err
}

func (a *ArticleDBRepo) PageFindAll(param *PageFindParam, _ *struct{}) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	err := preloadArticleListAssociations(db.Model(&ArticleModel{}).Scopes(Paginate(param))).
		Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) PageFindByCommonParam(param *PageFindParam, queryParam ArticleCommonQueryParam) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	d := preloadArticleListAssociations(db.Model(&ArticleModel{}).Scopes(Paginate(param)))
	querySqlDB, err := a.preCreateQuerySQL(d, queryParam)
	if err != nil {
		return nil, err
	}
	err = querySqlDB.Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) PageFindByNotVisibility(param *PageFindParam, queryParam PageFindArticleByNotStatusParam) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	d := preloadArticleListAssociations(db.Model(&ArticleModel{}).Scopes(Paginate(param)))
	originStatus := queryParam.ArticleCommonQueryParam.Status
	queryParam.ArticleCommonQueryParam.Status = nil
	querySqlDB, err := a.preCreateQuerySQL(d, queryParam.ArticleCommonQueryParam)
	if err != nil {
		return nil, err
	}
	err = querySqlDB.Where("status is null OR status <> ?", originStatus).Order("post_time desc").Find(&result).Error
	return result, err
}

func preloadArticleListAssociations(query *gorm.DB) *gorm.DB {
	return query.Preload("Author").Preload("Category").Preload("Tags")
}

func (a *ArticleDBRepo) CountAll(queryParam ArticleCommonQueryParam) (int64, error) {
	var total int64
	d := db.Model(&ArticleModel{})
	querySqlDB, err := a.preCreateQuerySQL(d, queryParam)
	if err != nil {
		return 0, err
	}
	err = querySqlDB.Count(&total).Error
	return total, err
}

func (a *ArticleDBRepo) CountDisplayable(queryParam ArticleCommonQueryParam) (int64, error) {
	var total int64
	queryParam.Status = nil
	querySqlDB, err := a.preCreateQuerySQL(db.Model(&ArticleModel{}), queryParam)
	if err != nil {
		return 0, err
	}
	err = querySqlDB.
		Where("status in (?)", []ArticleStatus{ARTICLE_STATUS_TOP, ARTILCE_STATUS_PUBLIC}).
		Where("post_time <= ?", time.Now()).
		Count(&total).Error
	return total, err
}

func (a *ArticleDBRepo) Update(article *ArticleModel, actorIDs ...uint) error {
	return a.UpdateForEditing(article, nil, actorIDs...)
}

func (a *ArticleDBRepo) UpdateForEditing(article *ArticleModel, expectedVersion *uint64, actorIDs ...uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		actorID := firstActorID(actorIDs)
		if err := updateArticleWithVersion(tx, article, actorID, expectedVersion, false); err != nil {
			return err
		}
		return retireOwnEditableSubmissions(tx, article, actorID)
	})
}

// UpdateForImport applies a version-checked import edit in the caller's transaction.
func (a *ArticleDBRepo) UpdateForImport(tx *gorm.DB, article *ArticleModel, expectedVersion uint64, actorID uint) error {
	var current ArticleModel
	if err := tx.Model(&ArticleModel{}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "status").First(&current, article.ID).Error; err != nil {
		return err
	}
	if current.Status == int8(ARTICLE_STATUS_WAIT_REVIEW) {
		return ErrArticleSubmissionChanged
	}
	return updateArticleWithOptions(tx, article, actorID, &expectedVersion, false)
}

func (a *ArticleDBRepo) UpdateWithReview(article *ArticleModel, review *model.ArticleReview, actorIDs ...uint) error {
	return a.UpdateWithReviewForEditing(article, review, nil, actorIDs...)
}

func (a *ArticleDBRepo) UpdateWithReviewForEditing(article *ArticleModel, review *model.ArticleReview, expectedVersion *uint64, actorIDs ...uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		actorID := firstActorID(actorIDs)
		if err := updateArticleWithVersion(tx, article, actorID, expectedVersion, false); err != nil {
			return err
		}
		review.ArticleID = article.ID
		if err := tx.Create(review).Error; err != nil {
			return err
		}
		return retireOwnEditableSubmissions(tx, article, actorID)
	})
}

func (a *ArticleDBRepo) Review(article *ArticleModel, review *model.ArticleReview, expectedStatus int8) error {
	return db.Transaction(func(tx *gorm.DB) error {
		article.Detail = nil
		article.Tags = nil
		if err := updateArticle(tx, article, review.ActorID, expectedStatus); err != nil {
			return err
		}
		review.ArticleID = article.ID
		return tx.Create(review).Error
	})
}

func updateArticle(tx *gorm.DB, article *ArticleModel, actorID uint, expectedStatuses ...int8) error {
	return updateArticleWithOptions(tx, article, actorID, nil, false, expectedStatuses...)
}

func updateArticleWithVersion(tx *gorm.DB, article *ArticleModel, actorID uint, expectedVersion *uint64, allowPendingSubmission bool, expectedStatuses ...int8) error {
	return updateArticleWithOptions(tx, article, actorID, expectedVersion, allowPendingSubmission, expectedStatuses...)
}

func updateArticleWithOptions(tx *gorm.DB, article *ArticleModel, actorID uint, expectedVersion *uint64, allowPendingSubmission bool, expectedStatuses ...int8) error {
	newVersion, err := claimArticleEditVersion(tx, article.ID, expectedVersion)
	if err != nil {
		return err
	}
	article.EditVersion = newVersion

	var original ArticleModel
	if err := tx.Model(&ArticleModel{}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Detail").
		First(&original, article.ID).Error; err != nil {
		return err
	}
	if original.Status == int8(ARTICLE_STATUS_RECYCLE) {
		return ErrArticleTrashed
	}
	if !allowPendingSubmission {
		var pending int64
		if err := tx.Model(&model.ArticleSubmission{}).
			Where("article_id = ? AND status = ?", article.ID, model.ArticleSubmissionPending).
			Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return ErrArticleSubmissionChanged
		}
	}
	if len(expectedStatuses) > 0 && original.Status != expectedStatuses[0] {
		return ErrArticleStatusChanged
	}
	if err := createArticleRevision(tx, &original, actorID); err != nil {
		return err
	}
	if article.Detail != nil {
		if article.DetailUID == "" {
			article.Detail.UID = utils.GenUID20()
			article.DetailUID = article.Detail.UID
			if err := tx.Create(article.Detail).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&model.ArticleDetail{}).Where("uid = ?", article.DetailUID).Update("content", article.Detail.Content).Error; err != nil {
			return err
		}
	}
	if article.Tags != nil {
		if err := tx.Session(&gorm.Session{SkipHooks: true}).Model(article).Association("Tags").Replace(article.Tags); err != nil {
			return err
		}
	}
	if err := article.prepareArticleFields(tx); err != nil {
		return err
	}
	article.UpdatedAt = time.Now()
	query := tx.Session(&gorm.Session{SkipHooks: true}).Model(&ArticleModel{}).
		Where("id = ? AND edit_version = ?", article.ID, newVersion)
	if len(expectedStatuses) > 0 {
		query = query.Where("status = ?", expectedStatuses[0])
	}
	result := query.Updates(map[string]interface{}{
		"title": article.Title, "slug": article.Slug, "seo_title": article.SEOTitle,
		"seo_description": article.SEODescription, "share_image": article.ShareImage,
		"type": article.Type, "content_format": article.ContentFormat, "summary": article.Summary,
		"is_allow_comment": article.IsAllowComment, "category_uid": article.CategoryUID,
		"detail_uid": article.DetailUID, "post_time": article.PostTime, "status": article.Status,
		"password": article.Password, "edit_version": newVersion, "updated_at": article.UpdatedAt,
	})
	if result.Error != nil {
		return result.Error
	}
	if len(expectedStatuses) > 0 && result.RowsAffected == 0 {
		return ErrArticleStatusChanged
	}
	if result.RowsAffected == 0 {
		return ErrArticleEditConflict
	}
	oldCounted := isCategoryCountedArticle(original.Status, original.Type)
	newCounted := isCategoryCountedArticle(article.Status, article.Type)
	categoryUIDs := make([]string, 0, 2)
	if oldCounted && (!newCounted || original.CategoryUID != article.CategoryUID) && original.CategoryUID != "" {
		categoryUIDs = append(categoryUIDs, original.CategoryUID)
	}
	if newCounted && (!oldCounted || original.CategoryUID != article.CategoryUID) && article.CategoryUID != "" {
		categoryUIDs = append(categoryUIDs, article.CategoryUID)
	}
	if err := lockArticleCategories(tx, categoryUIDs...); err != nil {
		return err
	}
	if oldCounted && (!newCounted || original.CategoryUID != article.CategoryUID) && original.CategoryUID != "" {
		if err := tx.Model(&CategoryModel{}).Where("uid = ?", original.CategoryUID).
			UpdateColumn("count", gorm.Expr("CASE WHEN count > 0 THEN count - 1 ELSE 0 END")).Error; err != nil {
			return err
		}
	}
	if newCounted && (!oldCounted || original.CategoryUID != article.CategoryUID) && article.CategoryUID != "" {
		if err := tx.Model(&CategoryModel{}).Where("uid = ?", article.CategoryUID).
			UpdateColumn("count", gorm.Expr("count + 1")).Error; err != nil {
			return err
		}
	}
	if original.Slug == article.Slug && original.Type == article.Type {
		return nil
	}
	if err := tx.Where("article_uid = ? AND slug = ? AND type = ?", article.UID, article.Slug, article.Type).
		Delete(&model.ArticleSlugRedirect{}).Error; err != nil {
		return err
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ArticleSlugRedirect{
		ArticleUID: original.UID,
		Slug:       original.Slug,
		Type:       original.Type,
	}).Error
}

func retireOwnEditableSubmissions(tx *gorm.DB, article *ArticleModel, actorID uint) error {
	if actorID == 0 || article == nil || article.Status != int8(ARTILCE_STATUS_PUBLIC) && article.Status != int8(ARTICLE_STATUS_TOP) && article.Status != int8(ARTICLE_STATUS_PRIVATE) {
		return nil
	}
	return tx.Model(&model.ArticleSubmission{}).
		Where("article_id = ? AND submitter_id = ? AND status IN ?", article.ID, actorID, []model.ArticleSubmissionStatus{model.ArticleSubmissionDraft, model.ArticleSubmissionRejected}).
		Update("status", model.ArticleSubmissionWithdrawn).Error
}

// claimArticleEditVersion atomically consumes an edit token. It deliberately
// uses UpdateColumn so version claims do not run article category-count hooks.
func claimArticleEditVersion(tx *gorm.DB, articleID uint, expected *uint64) (uint64, error) {
	if articleID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	query := tx.Model(&ArticleModel{}).Where("id = ?", articleID)
	if expected != nil {
		query = query.Where("edit_version = ?", *expected)
	}
	result := query.UpdateColumn("edit_version", gorm.Expr("edit_version + 1"))
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		if expected != nil {
			return 0, ErrArticleEditConflict
		}
		return 0, gorm.ErrRecordNotFound
	}
	var next uint64
	if err := tx.Model(&ArticleModel{}).Where("id = ?", articleID).Select("edit_version").Scan(&next).Error; err != nil {
		return 0, err
	}
	return next, nil
}

// ClaimArticleEditVersion exposes the transaction-scoped token claim to API
// workflows that must commit a submission transition in the same transaction.
func ClaimArticleEditVersion(tx *gorm.DB, articleID uint, expected *uint64) (uint64, error) {
	return claimArticleEditVersion(tx, articleID, expected)
}

func createArticleRevision(tx *gorm.DB, article *ArticleModel, actorIDs ...uint) error {
	revision := model.ArticleRevision{
		ArticleID:      article.ID,
		ActorID:        firstActorID(actorIDs),
		Title:          article.Title,
		Slug:           article.Slug,
		Type:           article.Type,
		ContentFormat:  article.ContentFormat,
		Summary:        article.Summary,
		SEOTitle:       article.SEOTitle,
		SEODescription: article.SEODescription,
		ShareImage:     article.ShareImage,
	}
	if article.Detail != nil {
		revision.Content = article.Detail.Content
	}
	if err := tx.Create(&revision).Error; err != nil {
		return err
	}
	var expiredIDs []uint
	if err := tx.Model(&model.ArticleRevision{}).
		Where("article_id = ?", article.ID).
		Order("id desc").
		Offset(articleRevisionLimit).
		Pluck("id", &expiredIDs).Error; err != nil {
		return err
	}
	if len(expiredIDs) == 0 {
		return nil
	}
	return tx.Where("id IN ?", expiredIDs).Delete(&model.ArticleRevision{}).Error
}

func firstActorID(ids []uint) uint {
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}

func (a *ArticleDBRepo) CreateWithSubmission(article *ArticleModel, submission *model.ArticleSubmission, review *model.ArticleReview) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ArticleModel{}).Create(article).Error; err != nil {
			return err
		}
		submission.ArticleID = article.ID
		submission.BaseEditVersion = article.EditVersion
		if err := tx.Create(submission).Error; err != nil {
			return err
		}
		review.ArticleID = article.ID
		review.SubmissionID = submission.ID
		return tx.Create(review).Error
	})
}

func (a *ArticleDBRepo) SaveSubmission(submission *model.ArticleSubmission, review *model.ArticleReview, expectedVersions ...*uint64) error {
	var expectedVersion *uint64
	if len(expectedVersions) != 0 {
		expectedVersion = expectedVersions[0]
	}
	_, err := a.SaveSubmissionForEditing(submission, review, expectedVersion)
	return err
}

func (a *ArticleDBRepo) SaveSubmissionForEditing(submission *model.ArticleSubmission, review *model.ArticleReview, expectedVersion *uint64) (uint64, error) {
	var committedVersion uint64
	err := db.Transaction(func(tx *gorm.DB) error {
		version, err := claimArticleEditVersion(tx, submission.ArticleID, expectedVersion)
		if err != nil {
			return err
		}
		committedVersion = version
		submission.BaseEditVersion = version
		var article ArticleModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status").First(&article, submission.ArticleID).Error; err != nil {
			return err
		}
		if article.Status == int8(ARTICLE_STATUS_RECYCLE) {
			return ErrArticleTrashed
		}
		var pending int64
		if err := tx.Model(&model.ArticleSubmission{}).
			Where("article_id = ? AND status = ?", submission.ArticleID, model.ArticleSubmissionPending).
			Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return ErrArticleSubmissionChanged
		}
		if submission.Status == model.ArticleSubmissionDraft {
			if err := tx.Model(&model.ArticleSubmission{}).
				Where("article_id = ? AND submitter_id = ? AND status = ?", submission.ArticleID, submission.SubmitterID, model.ArticleSubmissionRejected).
				Update("status", model.ArticleSubmissionWithdrawn).Error; err != nil {
				return err
			}
			var draft model.ArticleSubmission
			err := tx.Where("article_id = ? AND submitter_id = ? AND status = ?", submission.ArticleID, submission.SubmitterID, model.ArticleSubmissionDraft).
				Order("id desc").First(&draft).Error
			if err == nil {
				submission.ID = draft.ID
				return tx.Model(&draft).Updates(map[string]interface{}{
					"snapshot": submission.Snapshot, "base_edit_version": version, "updated_at": time.Now(),
				}).Error
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if submission.Status == model.ArticleSubmissionPending {
			if err := tx.Model(&model.ArticleSubmission{}).
				Where("article_id = ? AND submitter_id = ? AND status IN ?", submission.ArticleID, submission.SubmitterID, []model.ArticleSubmissionStatus{model.ArticleSubmissionDraft, model.ArticleSubmissionRejected}).
				Update("status", model.ArticleSubmissionWithdrawn).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(submission).Error; err != nil {
			return err
		}
		if review == nil {
			return nil
		}
		review.ArticleID = submission.ArticleID
		review.SubmissionID = submission.ID
		return tx.Create(review).Error
	})
	if err != nil {
		return 0, err
	}
	return committedVersion, nil
}

func (a *ArticleDBRepo) ReviewSubmission(submissionID uint, article *ArticleModel, tags []*model.Tag, reviewerID uint, action model.ArticleReviewAction, note string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var submission model.ArticleSubmission
		if err := tx.Select("id", "article_id").First(&submission, submissionID).Error; err != nil {
			return err
		}
		var current ArticleModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status", "edit_version").First(&current, submission.ArticleID).Error; err != nil {
			return err
		}
		if current.Status == int8(ARTICLE_STATUS_RECYCLE) {
			return ErrArticleTrashed
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("article_id = ? AND status = ?", current.ID, model.ArticleSubmissionPending).
			First(&submission, submissionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrArticleSubmissionChanged
			}
			return err
		}
		now := time.Now()
		nextStatus := model.ArticleSubmissionRejected
		toStatus := current.Status
		baseEditVersion := submission.BaseEditVersion
		if action == model.ArticleReviewApprove {
			nextStatus = model.ArticleSubmissionApproved
			article.Tags = tags
			if err := updateArticleWithVersion(tx, article, reviewerID, nil, true); err != nil {
				return err
			}
			toStatus = article.Status
		} else {
			version, err := claimArticleEditVersion(tx, submission.ArticleID, nil)
			if err != nil {
				return err
			}
			if baseEditVersion != 0 && baseEditVersion == current.EditVersion {
				baseEditVersion = version
			}
		}
		result := tx.Model(&submission).Where("status = ?", model.ArticleSubmissionPending).Updates(map[string]interface{}{
			"status": nextStatus, "reviewer_id": reviewerID, "reviewed_at": now, "note": note, "base_edit_version": baseEditVersion,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrArticleSubmissionChanged
		}
		return tx.Create(&model.ArticleReview{
			ArticleID: submission.ArticleID, SubmissionID: submission.ID, ActorID: reviewerID,
			Action: action, FromStatus: int8(ARTICLE_STATUS_WAIT_REVIEW), ToStatus: toStatus, Note: note,
		}).Error
	})
}

func (a *ArticleDBRepo) FindByID(id uint) (ArticleModel, error) {
	result := ArticleModel{}
	err := db.Model(&ArticleModel{}).Preload(clause.Associations).First(&result, id).Error
	return result, err
}

func (a *ArticleDBRepo) FindBySlug(slug string, articleType model.ArticleType) (ArticleModel, error) {
	result := ArticleModel{}
	err := db.Model(&ArticleModel{}).Preload(clause.Associations).
		Where("slug = ? AND type = ?", slug, articleType).
		First(&result).Error
	return result, err
}

func (a *ArticleDBRepo) FindByRedirectSlug(slug string, articleType model.ArticleType) (ArticleModel, error) {
	var redirect model.ArticleSlugRedirect
	if err := db.Where("slug = ? AND type = ?", slug, articleType).First(&redirect).Error; err != nil {
		return ArticleModel{}, err
	}
	var article ArticleModel
	err := db.Model(&ArticleModel{}).Preload(clause.Associations).
		Where("uid = ?", redirect.ArticleUID).
		First(&article).Error
	return article, err
}

func (a *ArticleDBRepo) ListRevisions(articleID uint) ([]model.ArticleRevision, error) {
	revisions := make([]model.ArticleRevision, 0)
	err := db.Preload("Actor").Where("article_id = ?", articleID).
		Order("id desc").
		Limit(articleRevisionLimit).
		Find(&revisions).Error
	return revisions, err
}

func (a *ArticleDBRepo) FindRevision(articleID, revisionID uint) (model.ArticleRevision, error) {
	var revision model.ArticleRevision
	err := db.Where("article_id = ? AND id = ?", articleID, revisionID).First(&revision).Error
	return revision, err
}

func (a *ArticleDBRepo) FindPostNeighbors(article *ArticleModel) (*ArticleModel, *ArticleModel, error) {
	if article == nil || article.ID == 0 || article.Type != model.ArticleTypePost {
		return nil, nil, nil
	}
	query := func() *gorm.DB {
		return db.Model(&ArticleModel{}).
			Select("id", "title", "slug", "type", "post_time").
			Where("type = ? AND status in ? AND post_time <= ?", model.ArticleTypePost, []ArticleStatus{ARTILCE_STATUS_PUBLIC, ARTICLE_STATUS_TOP}, time.Now())
	}

	var previous ArticleModel
	err := query().
		Where("post_time < ? OR (post_time = ? AND id < ?)", article.PostTime, article.PostTime, article.ID).
		Order("post_time desc, id desc").
		First(&previous).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}
	var previousPtr *ArticleModel
	if err == nil {
		previousPtr = &previous
	}

	var next ArticleModel
	err = query().
		Where("post_time > ? OR (post_time = ? AND id > ?)", article.PostTime, article.PostTime, article.ID).
		Order("post_time asc, id asc").
		First(&next).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}
	if err != nil {
		return previousPtr, nil, nil
	}
	return previousPtr, &next, nil
}

func (a *ArticleDBRepo) FindRelatedPosts(article *ArticleModel, limit int) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	if article == nil || article.ID == 0 || article.Type != model.ArticleTypePost || limit < 1 {
		return result, nil
	}
	conditions := make([]string, 0, 2)
	values := make([]interface{}, 0, 2)
	if article.CategoryUID != "" {
		conditions = append(conditions, "category_uid = ?")
		values = append(values, article.CategoryUID)
	}
	tagUIDs := make([]string, 0, len(article.Tags))
	for _, tag := range article.Tags {
		tagUIDs = append(tagUIDs, tag.UID)
	}
	if len(tagUIDs) > 0 {
		conditions = append(conditions, "uid IN (SELECT article_uid FROM article_tags WHERE tag_uid IN ?)")
		values = append(values, tagUIDs)
	}
	if len(conditions) == 0 {
		return result, nil
	}
	err := preloadArticleListAssociations(db.Model(&ArticleModel{})).
		Where("id <> ? AND type = ? AND status in ? AND post_time <= ?", article.ID, model.ArticleTypePost, []ArticleStatus{ARTILCE_STATUS_PUBLIC, ARTICLE_STATUS_TOP}, time.Now()).
		Where("("+strings.Join(conditions, " OR ")+")", values...).
		Order("post_time desc").
		Limit(limit).
		Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) DeleteByID(id uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		return deleteArticleByID(tx, id)
	})
}

func deleteArticleByID(tx *gorm.DB, id uint) error {
	var article ArticleModel
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&article, id).Error; err != nil {
		return err
	}
	for _, deletion := range []struct {
		query *gorm.DB
		value interface{}
	}{
		{tx.Unscoped().Where("article_id = ?", article.ID), &model.ArticlePreview{}},
		{tx.Where("article_id = ?", article.ID), &model.ArticleRevision{}},
		{tx.Where("article_id = ?", article.ID), &model.ArticleSubmission{}},
		{tx.Where("article_id = ?", article.ID), &model.ArticleReview{}},
		{tx.Where("article_uid = ?", article.UID), &model.ArticleSlugRedirect{}},
		{tx.Where("article_uid = ?", article.UID), &model.ArticleDailyStat{}},
	} {
		if err := deletion.query.Delete(deletion.value).Error; err != nil {
			return err
		}
	}
	if err := tx.Unscoped().Where("article_uid = ?", article.UID).Delete(&model.Comment{}).Error; err != nil {
		return err
	}
	if err := tx.Exec("DELETE FROM article_tags WHERE article_uid = ?", article.UID).Error; err != nil {
		return err
	}
	if err := tx.Delete(&article).Error; err != nil {
		return err
	}
	return tx.Where("uid = ?", article.DetailUID).Delete(&model.ArticleDetail{}).Error
}

func (a *ArticleDBRepo) AddReadCountByID(id uint, addCount uint) error {
	result := db.Model(&ArticleModel{}).Where("id = ?", id).Update("read_count", gorm.Expr("read_count + ?", addCount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (a *ArticleDBRepo) RecordPublicView(article *ArticleModel) error {
	if article == nil || article.ID == 0 || article.UID == "" {
		return gorm.ErrRecordNotFound
	}
	return db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ArticleModel{}).
			Where("id = ?", article.ID).
			Update("read_count", gorm.Expr("read_count + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		stat := model.ArticleDailyStat{
			ArticleUID: article.UID,
			Date:       time.Now().Format("2006-01-02"),
			Views:      1,
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "article_uid"}, {Name: "date"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"views": gorm.Expr("article_daily_stats.views + 1")}),
		}).Create(&stat).Error
	})
}

func (a *ArticleDBRepo) BatchUpdateStatus(ids []uint, status ArticleStatus) error {
	var err error
	ids, err = normalizeArticleIDs(ids)
	if err != nil {
		return err
	}
	if status == ARTICLE_STATUS_RECYCLE {
		return ErrArticleTrashed
	}
	return db.Transaction(func(tx *gorm.DB) error {
		articles := make([]ArticleModel, 0, len(ids))
		for _, id := range ids {
			var article ArticleModel
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Select("id", "status", "type", "category_uid", "edit_version").First(&article, id).Error; err != nil {
				return err
			}
			if article.Status == int8(ARTICLE_STATUS_RECYCLE) {
				return ErrArticleTrashed
			}
			articles = append(articles, article)
		}
		for i := range articles {
			article := &articles[i]
			var pending int64
			if err := tx.Model(&model.ArticleSubmission{}).
				Where("article_id = ? AND status = ?", article.ID, model.ArticleSubmissionPending).
				Count(&pending).Error; err != nil {
				return err
			}
			if pending > 0 {
				return ErrArticleSubmissionChanged
			}
		}
		categoryUIDs := make([]string, 0, len(articles))
		for i := range articles {
			article := &articles[i]
			wasCounted := isCategoryCountedArticle(article.Status, article.Type)
			willBeCounted := isCategoryCountedArticle(int8(status), article.Type)
			if article.CategoryUID != "" && wasCounted != willBeCounted {
				categoryUIDs = append(categoryUIDs, article.CategoryUID)
			}
		}
		if err := lockArticleCategories(tx, categoryUIDs...); err != nil {
			return err
		}
		for i := range articles {
			article := &articles[i]
			newVersion, err := claimArticleEditVersion(tx, article.ID, nil)
			if err != nil {
				return err
			}
			result := tx.Model(&ArticleModel{}).
				Where("id = ? AND status = ? AND edit_version = ?", article.ID, article.Status, newVersion).
				UpdateColumns(map[string]interface{}{"status": int8(status), "updated_at": time.Now()})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrArticleStatusChanged
			}
			wasCounted := isCategoryCountedArticle(article.Status, article.Type)
			willBeCounted := isCategoryCountedArticle(int8(status), article.Type)
			if article.CategoryUID != "" && wasCounted != willBeCounted {
				update := gorm.Expr("count + 1")
				if wasCounted {
					update = gorm.Expr("CASE WHEN count > 0 THEN count - 1 ELSE 0 END")
				}
				if err := tx.Model(&CategoryModel{}).Where("uid = ?", article.CategoryUID).UpdateColumn("count", update).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (a *ArticleDBRepo) BatchDelete(ids []uint) error {
	for _, id := range ids {
		if err := a.DeleteByID(id); err != nil {
			return err
		}
	}
	return nil
}
