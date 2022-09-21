package mysql

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myecho/model"
)

type ArticleDBRepo struct {
}

type ArticleModel = model.Article

func (a *ArticleDBRepo) Create(article *model.Article) error {
	return db.Model(&ArticleModel{}).Preload(clause.Associations).Create(article).Error
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
