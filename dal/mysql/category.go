package mysql

import (
	"gorm.io/gorm"
	"myecho/handler/api/errors"
	"myecho/model"
	"myecho/utils"
	"time"
)

type CategoryModel model.Category

func (CategoryModel) TableName() string {
	return "categories"
}

func (category *CategoryModel) BeforeCreate(tx *gorm.DB) error {
	if len(category.UID) == 0 {
		category.UID = utils.GenUID20()
	}
	if ok, err := categoryRepo.CheckNameExist(tx, category.Name, category.Type); err != nil || !ok {
		if err != nil {
			return err
		}
		if !ok {
			return errors.ErrCategoryNameExist
		}
	}
	return nil
}

func (category *CategoryModel) BeforeUpdate(tx *gorm.DB) error {
	if ok, err := categoryRepo.CheckNameExistExceptID(tx, category.Name, category.Type, category.ID); err != nil || !ok {
		if err != nil {
			return err
		}
		if !ok {
			return errors.ErrCategoryNameExist
		}
	}
	return nil
}

type CategoryRepo struct {
}

func (c *CategoryRepo) All() ([]*CategoryModel, error) {
	res := make([]*CategoryModel, 0)
	err := db.Model(&CategoryModel{}).Order("id").Find(&res).Error
	return res, err
}

func (c *CategoryRepo) AllByType(_type model.CategoryType) ([]*CategoryModel, error) {
	res := make([]*CategoryModel, 0)
	err := db.Model(&CategoryModel{}).Where("type = ? AND uid <> ''", _type).Order("id").Find(&res).Error
	return res, err
}

func (c *CategoryRepo) DisplayablePostCounts() (map[string]uint, error) {
	rows := make([]struct {
		CategoryUID string
		Count       uint
	}, 0)
	err := db.Model(&ArticleModel{}).
		Select("category_uid, COUNT(*) AS count").
		Where("type = ? AND status in ? AND post_time <= ? AND category_uid <> ''", model.ArticleTypePost, []ArticleStatus{ARTILCE_STATUS_PUBLIC, ARTICLE_STATUS_TOP}, time.Now()).
		Group("category_uid").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]uint, len(rows))
	for _, row := range rows {
		counts[row.CategoryUID] = row.Count
	}
	return counts, nil
}

func (c *CategoryRepo) Create(categoryModel *CategoryModel) error {
	return db.Create(categoryModel).Error
}

func (c *CategoryRepo) GetAllChildrenUID(father_uid string) ([]string, error) {
	children := make([]*CategoryModel, 0)
	err := db.Model(&CategoryModel{}).Where("father_uid = ?", father_uid).Find(&children).Error
	if err != nil {
		return nil, err
	}
	childrenUID := make([]string, 0)
	for len(children) != 0 {
		fathersUID := make([]string, 0, len(children))
		for _, category := range children {
			fathersUID = append(fathersUID, category.UID)
			childrenUID = append(childrenUID, category.UID)
		}
		err = db.Model(&CategoryModel{}).Where("father_uid in (?)", fathersUID).Find(&children).Error
		if err != nil {
			return nil, err
		}
	}
	return childrenUID, nil
}

func (c *CategoryRepo) ValidateUIDExist(uid string) error {
	if len(uid) == 0 {
		return nil
	}
	var count int64
	err := db.Model(&CategoryModel{}).Where("uid = ?", uid).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.ErrCategoryNotFound
	}
	return nil
}

func (c *CategoryRepo) CheckNameExist(tx *gorm.DB, name string, _type model.CategoryType) (bool, error) {
	return c.CheckNameExistExceptID(tx, name, _type, 0)
}

func (c *CategoryRepo) CheckNameExistExceptID(tx *gorm.DB, name string, _type model.CategoryType, id uint) (bool, error) {
	var sameNameCount int64
	query := tx.Model(&CategoryModel{}).Where("name = ? and type = ?", name, _type)
	if id != 0 {
		query = query.Where("id <> ?", id)
	}
	err := query.Count(&sameNameCount).Error
	if err != nil {
		return false, err
	}
	if sameNameCount > 0 {
		return false, nil
	}
	return true, nil
}
