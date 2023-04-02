package mysql

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myecho/model"
	"myecho/utils"
	"strings"
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
	if err := article.ReduceCategoryCount(tx); err != nil {
		return err
	}
	return nil
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
	if article.Status == 1 && len(article.CategoryUID) != 0 {
		return tx.Model(&CategoryModel{}).Where("uid = ?", article.CategoryUID).Update("count", gorm.Expr("count + 1")).Error
	}
	return nil
}

func (article *ArticleModel) ReduceCategoryCount(tx *gorm.DB) error {
	oldArticle, err := articleRepo.TXGet(tx, article.ID)
	if err != nil {
		return err
	}
	if oldArticle.Status == 1 && len(oldArticle.CategoryUID) != 0 {
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
}

type PageFindArticleByNotStatusParam struct {
	ArticleCommonQueryParam
}

func (a *ArticleDBRepo) preCreateQuerySQL(db *gorm.DB, param ArticleCommonQueryParam) (*gorm.DB, error) {
	SqlPrefix := make([]string, 0)
	SqlValue := make([]interface{}, 0)
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
	if article.Tags != nil {
		if err := db.Model(article).Association("Tags").Replace(article.Tags); err != nil {
			return err
		}
	}
	err := db.Model(article).Omit("User", "Tags").Updates(article).Error
	return err
}

func (a *ArticleDBRepo) FindByID(id uint) (ArticleModel, error) {
	result := ArticleModel{}
	err := db.Model(&ArticleModel{}).Preload(clause.Associations).First(&result, id).Error
	return result, err
}

func (a *ArticleDBRepo) DeleteByID(id uint) error {
	article := &ArticleModel{}
	article.ID = id
	return db.Model(&ArticleModel{}).Select("Detail").Delete(article).Error
}

func (a *ArticleDBRepo) AddReadCountByID(id uint, addCount uint) error {
	article := &ArticleModel{}
	err := db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&ArticleModel{}).Select("read_count").First(article, id).Error
		if err != nil {
			return err
		}
		err = tx.Model(&ArticleModel{}).Where("id = ?", id).Update("read_count", article.ReadCount+addCount).Error
		return err
	})
	return err
}
