package mysql

import (
	"gorm.io/gorm"
	"myecho/handler/api/errors"
	"myecho/model"
	"myecho/utils"
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

type CategoryRepo struct {
}

func (c *CategoryRepo) All() ([]*CategoryModel, error) {
	res := make([]*CategoryModel, 0)
	err := db.Model(&CategoryModel{}).Order("id").Find(&res).Error
	return res, err
}

func (c *CategoryRepo) AllByType(_type model.CategoryType) ([]*CategoryModel, error) {
	res := make([]*CategoryModel, 0)
	err := db.Model(&CategoryModel{}).Where("type = ?", _type).Order("id").Find(&res).Error
	return res, err
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
	if count > 0 {
		return errors.ErrCategoryNotFound
	}
	return nil
}

func (c *CategoryRepo) CheckNameExist(tx *gorm.DB, name string, _type model.CategoryType) (bool, error) {
	var sameNameCount int64
	err := tx.Model(&CategoryModel{}).Where("name = ? and type = ?", name, _type).Count(&sameNameCount).Error
	if err != nil {
		return false, err
	}
	if sameNameCount > 0 {
		return false, nil
	}
	return true, nil
}
