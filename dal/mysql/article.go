package mysql

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myecho/model"
	"strings"
)

type ArticleDBRepo struct {
}

type ArticleModel = model.Article

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

func (a *ArticleDBRepo) preCreateQuerySQL(db *gorm.DB, param ArticleCommonQueryParam) *gorm.DB {
	SqlPrefix := make([]string, 0)
	SqlValue := make([]interface{}, 0)
	and := " AND "
	if param.CategoryUID != nil {
		sql := "category_uid = ?"
		SqlPrefix = append(SqlPrefix, sql)
		SqlValue = append(SqlValue, *param.CategoryUID)
	}
	if param.Status != nil {
		sql := "status = ?"
		SqlPrefix = append(SqlPrefix, sql)
		SqlValue = append(SqlValue, *param.Status)
	}
	return db.Where(strings.Join(SqlPrefix, and), SqlValue...)
}

func (a *ArticleDBRepo) Create(article *model.Article) error {
	return db.Model(&ArticleModel{}).Create(article).Error
}

func (a *ArticleDBRepo) PageFindAll(param *PageFindParam, _ *struct{}) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	err := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations).Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) PageFindByCommonParam(param *PageFindParam, queryParam ArticleCommonQueryParam) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	d := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations)
	querySqlDB := a.preCreateQuerySQL(d, queryParam)
	err := querySqlDB.Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) PageFindByNotVisibility(param *PageFindParam, queryParam PageFindArticleByNotStatusParam) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	d := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations)
	originStatus := queryParam.ArticleCommonQueryParam.Status
	queryParam.ArticleCommonQueryParam.Status = nil
	querySqlDB := a.preCreateQuerySQL(d, queryParam.ArticleCommonQueryParam)
	err := querySqlDB.Where("status is null OR status <> ?", originStatus).Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) CountAll(queryParam ArticleCommonQueryParam) (int64, error) {
	var total int64
	d := db.Model(&ArticleModel{})
	querySqlDB := a.preCreateQuerySQL(d, queryParam)
	err := querySqlDB.Count(&total).Error
	return total, err
}

func (a *ArticleDBRepo) CountDisplayable(queryParam ArticleCommonQueryParam) (int64, error) {
	var total int64
	queryParam.Status = nil
	querySqlDB := a.preCreateQuerySQL(db.Model(&ArticleModel{}), queryParam)
	err := querySqlDB.Where("status in (?)", []ArticleStatus{ARTICLE_STATUS_TOP, ARTILCE_STATUS_PUBLIC}).Count(&total).Error
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
	return db.Model(&ArticleModel{}).Select("Detail").Delete(&ArticleModel{}, id).Error
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
