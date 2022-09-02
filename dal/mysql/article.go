package mysql

import (
	"gorm.io/gorm/clause"
	"myecho/config"
	"myecho/model"
)

type ArticleDBRepo struct {
	model *model.Article
}

func (a *ArticleDBRepo) Create(article *model.Article) error {
	return config.Database.Model(a.model).Preload(clause.Associations).Create(&article).Error
}
