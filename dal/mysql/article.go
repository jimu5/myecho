package mysql

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myecho/model"
	"myecho/utils"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type ArticleDBRepo struct {
}

type ArticleModel model.Article

func (ArticleModel) TableName() string {
	return "articles"
}
func (article *ArticleModel) BeforeCreate(tx *gorm.DB) error {
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
	if article.Status == 1 && article.Type == model.ArticleTypePost && len(article.CategoryUID) != 0 {
		return tx.Model(&CategoryModel{}).Where("uid = ?", article.CategoryUID).Update("count", gorm.Expr("count + 1")).Error
	}
	return nil
}

func (article *ArticleModel) ReduceCategoryCount(tx *gorm.DB) error {
	oldArticle, err := articleRepo.TXGet(tx, article.ID)
	if err != nil {
		return err
	}
	if oldArticle.Status == 1 && oldArticle.Type == model.ArticleTypePost && len(oldArticle.CategoryUID) != 0 {
		return tx.Model(&CategoryModel{}).Where("uid = ?", oldArticle.CategoryUID).Update("count", gorm.Expr("count - 1")).Error
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
	CategoryUID *string
	Status      *ArticleStatus
	Type        *model.ArticleType
	Keyword     *string
	TagUID      *string
	DateFrom    *time.Time
	DateTo      *time.Time
}

type PageFindArticleByNotStatusParam struct {
	ArticleCommonQueryParam
}

func (a *ArticleDBRepo) preCreateQuerySQL(db *gorm.DB, param ArticleCommonQueryParam) (*gorm.DB, error) {
	SqlPrefix := make([]string, 0)
	SqlValue := make([]interface{}, 0)
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
	if param.Type != nil {
		sql := "type = ?"
		SqlPrefix = append(SqlPrefix, sql)
		SqlValue = append(SqlValue, *param.Type)
	}
	if param.Keyword != nil && strings.TrimSpace(*param.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(*param.Keyword) + "%"
		SqlPrefix = append(SqlPrefix, "(title LIKE ? OR summary LIKE ?)")
		SqlValue = append(SqlValue, keyword, keyword)
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

func (a *ArticleDBRepo) TXGet(tx *gorm.DB, id uint) (ArticleModel, error) {
	var oldArticle ArticleModel
	err := tx.Model(&ArticleModel{}).Where("id = ?", id).First(&oldArticle).Error
	return oldArticle, err
}

func (a *ArticleDBRepo) PageFindAll(param *PageFindParam, _ *struct{}) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	err := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations).Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) PageFindByCommonParam(param *PageFindParam, queryParam ArticleCommonQueryParam) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	d := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations)
	querySqlDB, err := a.preCreateQuerySQL(d, queryParam)
	if err != nil {
		return nil, err
	}
	err = querySqlDB.Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) PageFindByNotVisibility(param *PageFindParam, queryParam PageFindArticleByNotStatusParam) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	d := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations)
	originStatus := queryParam.ArticleCommonQueryParam.Status
	queryParam.ArticleCommonQueryParam.Status = nil
	querySqlDB, err := a.preCreateQuerySQL(d, queryParam.ArticleCommonQueryParam)
	if err != nil {
		return nil, err
	}
	err = querySqlDB.Where("status is null OR status <> ?", originStatus).Order("post_time desc").Find(&result).Error
	return result, err
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
	err = querySqlDB.Where("status in (?)", []ArticleStatus{ARTICLE_STATUS_TOP, ARTILCE_STATUS_PUBLIC}).Count(&total).Error
	return total, err
}

func (a *ArticleDBRepo) Update(article *ArticleModel) error {
	return db.Transaction(func(tx *gorm.DB) error {
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
			if err := tx.Model(article).Association("Tags").Replace(article.Tags); err != nil {
				return err
			}
		}
		return tx.Model(article).Omit("User", "Tags", "Detail").Updates(article).Error
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

func (a *ArticleDBRepo) DeleteByID(id uint) error {
	article := &ArticleModel{}
	article.ID = id
	return db.Model(&ArticleModel{}).Select("Detail").Delete(article).Error
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

func (a *ArticleDBRepo) BatchUpdateStatus(ids []uint, status ArticleStatus) error {
	for _, id := range ids {
		article, err := a.FindByID(id)
		if err != nil {
			return err
		}
		if err := db.Model(&article).Update("status", int8(status)).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *ArticleDBRepo) BatchDelete(ids []uint) error {
	for _, id := range ids {
		if err := a.DeleteByID(id); err != nil {
			return err
		}
	}
	return nil
}
