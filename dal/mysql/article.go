package mysql

import (
	"gorm.io/gorm/clause"
	"myecho/model"
)

type ArticleDBRepo struct {
}

type ArticleModel = model.Article

func (a *ArticleDBRepo) Create(article *model.Article) error {
	return db.Model(&ArticleModel{}).Preload(clause.Associations).Create(&article).Error
}

func (a *ArticleDBRepo) PageFindAll(param *PageFindParam) ([]*ArticleModel, error) {
	result := make([]*ArticleModel, 0)
	err := db.Model(&ArticleModel{}).Scopes(Paginate(param)).Preload(clause.Associations).Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) CountAll() (int64, error) {
	var total int64
	err := db.Model(&ArticleModel{}).Count(&total).Error
	return total, err
}

func (a *ArticleDBRepo) Update(article *ArticleModel) error {
	return db.Model(&ArticleModel{}).Omit("User").Updates(&article).Error
}

func (a *ArticleDBRepo) FindByID(id uint) (ArticleModel, error) {
	result := ArticleModel{}
	err := db.Model(&ArticleModel{}).Preload(clause.Associations).First(&result, id).Error
	return result, err
}
