package mysql

import (
	"gorm.io/gorm/clause"
	"myecho/config"
	"myecho/model"
)

type ArticleDBRepo struct {
}

type articleModel = model.Article

func (a *ArticleDBRepo) Create(article *model.Article) error {
	return config.Database.Model(&articleModel{}).Preload(clause.Associations).Create(&article).Error
}

func (a *ArticleDBRepo) PageFindAll(param *PageFindParam) ([]*articleModel, error) {
	result := make([]*articleModel, 0)
	err := config.Database.Model(&articleModel{}).Scopes(Paginate(param)).Preload(clause.Associations).Order("post_time desc").Find(&result).Error
	return result, err
}

func (a *ArticleDBRepo) CountAll() int64 {
	var total int64
	config.Database.Find(&[]model.Article{}).Count(&total)
	return total
}
