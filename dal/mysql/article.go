package mysql

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myecho/model"
)

type ArticleDBRepo struct {
}

type ArticleModel = model.Article

type VISIBILITY int8

const (
	VISIBILITY_NORMAL VISIBILITY = iota + 1
	VISIBILITY_TOP
	VISIBILITY_PRIVATE
)

type ArticleCommonQueryParam struct {
	CategoryID *uint
}

type PageFindArticleVisibilityParam struct {
	ArticleCommonQueryParam
	Visibility VISIBILITY
}

func (a *ArticleDBRepo) preCreateQuerySQL(db *gorm.DB, param ArticleCommonQueryParam) *gorm.DB {
	SqlPrefix := make([]string, 0)
	SqlValue := make([]interface{}, 0)
	if param.CategoryID != nil {
		sql := "category_id = ?"
		SqlPrefix = append(SqlPrefix, sql)
		SqlValue = append(SqlValue, *param.CategoryID)
	}
	return db.Where(SqlPrefix, SqlValue...)
}

func (a *ArticleDBRepo) Create(article *model.Article) error {
	return db.Model(&ArticleModel{}).Preload(clause.Associations).Create(article).Error
}

func (a *ArticleDBRepo) PageFindAll(param *PageFindParam, _ *struct{}) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	err := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations).Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) PageFindByVisibility(param *PageFindParam, queryParam PageFindArticleVisibilityParam) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	d := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations)
	querySqlDB := a.preCreateQuerySQL(d, queryParam.ArticleCommonQueryParam)
	err := querySqlDB.Where("visibility = ?", queryParam.Visibility).Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) PageFindByNotVisibility(param *PageFindParam, queryParam PageFindArticleVisibilityParam) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	d := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations)
	querySqlDB := a.preCreateQuerySQL(d, queryParam.ArticleCommonQueryParam)
	err := querySqlDB.Where("visibility is null OR visibility <> ?", queryParam.Visibility).Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) CountAll(queryParam ArticleCommonQueryParam) (int64, error) {
	var total int64
	d := db.Model(&ArticleModel{})
	querySqlDB := a.preCreateQuerySQL(d, queryParam)
	err := querySqlDB.Count(&total).Error
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
